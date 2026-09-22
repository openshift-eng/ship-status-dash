package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	apimachineryerrors "k8s.io/apimachinery/pkg/util/errors"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

const (
	ComponentMonitor = "component-monitor"

	defaultMonitorOutageDescription = "Component monitor detected outage"

	// flapWindow is the lookback period for finding recently-closed outages to reopen.
	// Outages from the same probe that recur within this window are treated as the same issue.
	flapWindow = 1 * time.Hour
)

// ComponentMonitorReportProcessor handles the business logic for processing component monitor reports.
type ComponentMonitorReportProcessor struct {
	outageManager outage.OutageManager
	pingRepo      repositories.ComponentPingRepository
	configManager *config.Manager[types.DashboardConfig]
	logger        *logrus.Logger
}

// NewComponentMonitorReportProcessor creates a new processor instance.
func NewComponentMonitorReportProcessor(outageManager outage.OutageManager, pingRepo repositories.ComponentPingRepository, configManager *config.Manager[types.DashboardConfig], logger *logrus.Logger) *ComponentMonitorReportProcessor {
	return &ComponentMonitorReportProcessor{
		outageManager: outageManager,
		pingRepo:      pingRepo,
		configManager: configManager,
		logger:        logger,
	}
}

func (p *ComponentMonitorReportProcessor) ValidateRequest(req *types.ComponentMonitorReportRequest, serviceAccount string) error {
	var errors []error
	for _, status := range req.Statuses {
		// Component must exist
		component := p.configManager.Get().GetComponentBySlug(status.ComponentSlug)
		if component == nil {
			errors = append(errors, fmt.Errorf("component not found: %s", status.ComponentSlug))
			continue
		}

		// Service account must be an owner of the component
		serviceAccountIsOwner := false
		for _, owner := range component.Owners {
			if owner.ServiceAccount == "" {
				continue
			}
			if owner.ServiceAccount == serviceAccount {
				serviceAccountIsOwner = true
				break
			}
		}
		if !serviceAccountIsOwner {
			errors = append(errors, fmt.Errorf("service account %s is not an owner of component %s", serviceAccount, status.ComponentSlug))
			continue
		}

		// Sub-component must exist
		subComponent := component.GetSubComponentBySlug(status.SubComponentSlug)
		if subComponent == nil {
			errors = append(errors, fmt.Errorf("sub-component not found: %s/%s", status.ComponentSlug, status.SubComponentSlug))
			continue
		}

		// This component-monitor instance must be configured for the sub-component
		if subComponent.Monitoring == nil || subComponent.Monitoring.ComponentMonitor != req.ComponentMonitor {
			errors = append(errors, fmt.Errorf("improper component monitor source: %s for: %s/%s", req.ComponentMonitor, status.ComponentSlug, status.SubComponentSlug))
		}
	}

	return apimachineryerrors.NewAggregate(errors)
}

// Process processes a component monitor report request.
// All components and sub-components are assumed to be valid (validated in the API layer).
func (p *ComponentMonitorReportProcessor) Process(req *types.ComponentMonitorReportRequest) error {
	logger := p.logger.WithFields(logrus.Fields{
		"component_monitor": req.ComponentMonitor,
		"status_count":      len(req.Statuses),
	})

	for _, status := range req.Statuses {
		statusLogger := logger.WithFields(logrus.Fields{
			"component":     status.ComponentSlug,
			"sub_component": status.SubComponentSlug,
			"status":        string(status.Status),
		})

		component := p.configManager.Get().GetComponentBySlug(status.ComponentSlug)
		if component == nil {
			// This should never happen, since the request validation should have caught this.
			return fmt.Errorf("component not found: %s", status.ComponentSlug)
		}

		subComponent := component.GetSubComponentBySlug(status.SubComponentSlug)
		if subComponent == nil {
			// This should never happen, since the request validation should have caught this.
			return fmt.Errorf("sub-component not found: %s/%s", status.ComponentSlug, status.SubComponentSlug)
		}

		now := time.Now()
		if err := p.pingRepo.UpsertComponentReportPing(status.ComponentSlug, status.SubComponentSlug, now); err != nil {
			statusLogger.WithField("error", err).Error("Failed to upsert component report ping")
			return err
		}

		var err error
		if subComponent.Monitoring.OutagePerReason {
			err = p.processPerReason(status, req.ComponentMonitor, subComponent, now, statusLogger)
		} else {
			err = p.processSingleOutage(status, req.ComponentMonitor, subComponent, now, statusLogger)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func monitorOutageDescription(results string) string {
	if results == "" {
		return defaultMonitorOutageDescription
	}
	return results
}

// processSingleOutage keeps at most one active outage created by this monitor for the sub-component.
func (p *ComponentMonitorReportProcessor) processSingleOutage(status types.ComponentMonitorReportComponentStatus, monitorName string, subComponent *types.SubComponent, now time.Time, logger *logrus.Entry) error {
	activeOutages, err := p.activeMonitorOutages(status.ComponentSlug, status.SubComponentSlug, monitorName, logger)
	if err != nil {
		return err
	}

	if status.Status == types.StatusHealthy {
		if len(activeOutages) == 0 {
			logger.Debug("Sub Component reported healthy, and no active outages to resolve")
			return nil
		}
		if !subComponent.Monitoring.AutoResolve {
			logger.Debug("Auto-resolve disabled, skipping healthy status processing")
			return nil
		}
		for i := range activeOutages {
			p.resolveOutage(&activeOutages[i], now, monitorName, logger)
		}
		return nil
	}

	severity := status.Status.ToSeverity()
	if severity == "" {
		logger.Warn("Invalid status for severity conversion, skipping")
		return nil
	}

	if len(activeOutages) > 0 {
		logger.WithField("outage_id", activeOutages[0].ID).Debug("Active outage from this component-monitor already exists, skipping creation")
		return nil
	}

	if len(status.Reasons) == 0 {
		logger.Warn("No reasons provided for unhealthy status, skipping")
		return nil
	}

	return p.createOrReopenOutage(status, subComponent, now, severity, defaultMonitorOutageDescription, status.Reasons, nil, monitorName, logger)
}

// processPerReason keeps one active outage per incoming probe reason (Type+Check).
func (p *ComponentMonitorReportProcessor) processPerReason(status types.ComponentMonitorReportComponentStatus, monitorName string, subComponent *types.SubComponent, now time.Time, logger *logrus.Entry) error {
	activeOutages, err := p.activeMonitorOutages(status.ComponentSlug, status.SubComponentSlug, monitorName, logger)
	if err != nil {
		return err
	}

	if status.Status == types.StatusHealthy {
		if len(activeOutages) == 0 {
			logger.Debug("Sub Component reported healthy, and no active outages to resolve")
			return nil
		}
		if !subComponent.Monitoring.AutoResolve {
			logger.Debug("Auto-resolve disabled, skipping healthy status processing")
			return nil
		}
		for i := range activeOutages {
			p.resolveOutage(&activeOutages[i], now, monitorName, logger)
		}
		return nil
	}

	severity := status.Status.ToSeverity()
	if severity == "" {
		logger.Warn("Invalid status for severity conversion, skipping")
		return nil
	}

	activeByIdentity := make(map[string]*types.Outage, len(activeOutages))
	for i := range activeOutages {
		id, ok := outageReasonIdentity(activeOutages[i])
		if !ok {
			continue
		}
		activeByIdentity[id] = &activeOutages[i]
	}

	incomingIdentities := make(map[string]struct{}, len(status.Reasons))
	for _, reason := range status.Reasons {
		if reason.Check == "" {
			continue
		}
		identity := reasonIdentity(reason)
		incomingIdentities[identity] = struct{}{}

		if existing, ok := activeByIdentity[identity]; ok {
			p.syncOutageDescription(existing, reason.Results, monitorName, logger)
			continue
		}

		description := monitorOutageDescription(reason.Results)
		if err := p.createOrReopenOutage(status, subComponent, now, severity, description, []types.Reason{reason}, &description, monitorName, logger); err != nil {
			return err
		}
	}

	if !subComponent.Monitoring.AutoResolve {
		return nil
	}

	for identity, existing := range activeByIdentity {
		if _, keep := incomingIdentities[identity]; keep {
			continue
		}
		p.resolveOutage(existing, now, monitorName, logger)
	}

	return nil
}

func (p *ComponentMonitorReportProcessor) activeMonitorOutages(componentSlug, subComponentSlug, monitorName string, logger *logrus.Entry) ([]types.Outage, error) {
	activeOutages, err := p.outageManager.GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, monitorName)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query active outages")
		return nil, err
	}
	return activeOutages, nil
}

func (p *ComponentMonitorReportProcessor) resolveOutage(o *types.Outage, now time.Time, monitorName string, logger *logrus.Entry) {
	o.EndTime = sql.NullTime{Time: now, Valid: true}
	outageLogger := logger.WithField("outage_id", o.ID)
	if err := p.outageManager.UpdateOutage(o, monitorName); err != nil {
		outageLogger.WithField("error", err).Error("Failed to resolve outage")
		return
	}
	outageLogger.Info("Successfully auto-resolved outage")
}

func (p *ComponentMonitorReportProcessor) syncOutageDescription(o *types.Outage, description, monitorName string, logger *logrus.Entry) {
	if description == "" || o.Description == description {
		return
	}
	o.Description = description
	if err := p.outageManager.UpdateOutage(o, monitorName); err != nil {
		logger.WithFields(logrus.Fields{"outage_id": o.ID, "error": err}).Error("Failed to update outage description")
	}
}

// createOrReopenOutage reopens a matching outage inside the flap window, otherwise creates one.
// reopenDescription, when non-nil, is written onto a reopened outage.
func (p *ComponentMonitorReportProcessor) createOrReopenOutage(
	status types.ComponentMonitorReportComponentStatus,
	subComponent *types.SubComponent,
	now time.Time,
	severity types.Severity,
	description string,
	reasons []types.Reason,
	reopenDescription *string,
	monitorName string,
	logger *logrus.Entry,
) error {
	recent, err := p.outageManager.FindReopenableOutage(status.ComponentSlug, status.SubComponentSlug, monitorName, now.Add(-flapWindow), reasons)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query recently-closed outages")
		return err
	}
	if recent != nil {
		p.reopenOutage(recent, severity, reopenDescription, reasons, monitorName, logger)
		return nil
	}
	return p.createMonitorOutage(status, subComponent, now, severity, description, reasons, monitorName, logger)
}

func (p *ComponentMonitorReportProcessor) reopenOutage(o *types.Outage, severity types.Severity, description *string, incoming []types.Reason, monitorName string, logger *logrus.Entry) {
	o.EndTime = sql.NullTime{Valid: false}
	o.Severity = severity
	if description != nil {
		o.Description = *description
	}
	outageLogger := logger.WithField("outage_id", o.ID)
	if err := p.outageManager.UpdateOutage(o, monitorName); err != nil {
		outageLogger.WithField("error", err).Error("Failed to reopen outage")
		return
	}
	if added := newReasons(o.Reasons, incoming); len(added) > 0 {
		if err := p.outageManager.AppendReasons(o.ID, added); err != nil {
			outageLogger.WithField("error", err).Error("Failed to append new reasons to reopened outage")
		}
	}
	outageLogger.Info("Reopened recently-closed outage due to recurring probe failure")
}

func (p *ComponentMonitorReportProcessor) createMonitorOutage(
	status types.ComponentMonitorReportComponentStatus,
	subComponent *types.SubComponent,
	now time.Time,
	severity types.Severity,
	description string,
	reasons []types.Reason,
	monitorName string,
	logger *logrus.Entry,
) error {
	outage := types.Outage{
		ComponentName:    status.ComponentSlug,
		SubComponentName: status.SubComponentSlug,
		Severity:         severity,
		StartTime:        now,
		EndTime:          sql.NullTime{Valid: false},
		Description:      description,
		DiscoveredFrom:   ComponentMonitor,
		CreatedBy:        monitorName,
	}
	if !subComponent.RequiresConfirmation {
		outage.ConfirmedAt = sql.NullTime{Time: now, Valid: true}
	}
	if message, valid := outage.Validate(); !valid {
		return fmt.Errorf("validation failed: %s", message)
	}
	if err := p.outageManager.CreateOutage(&outage, reasons, monitorName, ""); err != nil {
		logger.WithField("error", err).Error("Failed to create outage")
		return nil
	}
	p.applyReportedLinks(outage.ID, linksFromReasons(reasons), monitorName, logger)
	logger.WithFields(logrus.Fields{
		"outage_id":    outage.ID,
		"reason_count": len(reasons),
	}).Info("Successfully created outage with reasons")
	return nil
}

// newReasons returns reasons from incoming that are not already present in existing,
// matched by Type and Check.
func newReasons(existing, incoming []types.Reason) []types.Reason {
	var result []types.Reason
	for _, r := range incoming {
		found := false
		for _, e := range existing {
			if e.Type == r.Type && e.Check == r.Check {
				found = true
				break
			}
		}
		if !found {
			result = append(result, r)
		}
	}
	return result
}

func reasonIdentity(r types.Reason) string {
	return string(r.Type) + "\x00" + r.Check
}

func outageReasonIdentity(o types.Outage) (string, bool) {
	if len(o.Reasons) == 0 {
		return "", false
	}
	return reasonIdentity(o.Reasons[0]), true
}

func linksFromReasons(reasons []types.Reason) []types.ReportedLink {
	var links []types.ReportedLink
	for _, reason := range reasons {
		links = append(links, reason.Links...)
	}
	return links
}

func (p *ComponentMonitorReportProcessor) applyReportedLinks(outageID uint, reported []types.ReportedLink, monitorName string, logger *logrus.Entry) {
	seen := make(map[string]struct{}, len(reported))
	for _, link := range reported {
		rawURL, linkType, ok := link.Normalize()
		if !ok {
			logger.WithField("url", link.URL).Warn("Skipping invalid reported link")
			continue
		}
		if _, dup := seen[rawURL]; dup {
			continue
		}
		seen[rawURL] = struct{}{}
		if err := p.outageManager.AddOutageLink(&types.OutageLink{
			OutageID: outageID,
			URL:      rawURL,
			LinkType: linkType,
		}, monitorName); err != nil {
			logger.WithFields(logrus.Fields{
				"outage_id": outageID,
				"url":       rawURL,
				"error":     err,
			}).Error("Failed to add reported link")
		}
	}
}
