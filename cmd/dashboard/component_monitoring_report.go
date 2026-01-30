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

		// Find all the active outages that this component-monitor has reported. This will not pick up any outages that were created by other sources.
		activeOutages, err := p.outageManager.GetActiveOutagesCreatedBy(status.ComponentSlug, status.SubComponentSlug, req.ComponentMonitor)
		if err != nil {
			statusLogger.WithField("error", err).Error("Failed to query active outages")
			return err
		}

		if status.Status == types.StatusHealthy {
			if len(activeOutages) == 0 {
				statusLogger.Debug("Sub Component reported healthy, and no active outages to resolve")
				continue
			}

			if subComponent.Monitoring == nil || !subComponent.Monitoring.AutoResolve {
				statusLogger.Debug("Auto-resolve disabled, skipping healthy status processing")
				continue
			}
			for i := range activeOutages {
				activeOutages[i].EndTime = sql.NullTime{Time: now, Valid: true}
				activeOutages[i].ResolvedBy = &req.ComponentMonitor
				if err := p.outageManager.UpdateOutage(&activeOutages[i]); err != nil {
					statusLogger.WithFields(logrus.Fields{
						"outage_id": activeOutages[i].ID,
						"error":     err,
					}).Error("Failed to resolve outage")
					continue
				}
				statusLogger.WithField("outage_id", activeOutages[i].ID).Info("Successfully auto-resolved outage")
			}
		} else {
			severity := status.Status.ToSeverity()
			if severity == "" {
				statusLogger.Warn("Invalid status for severity conversion, skipping")
				continue
			}

			if len(activeOutages) > 0 {
				statusLogger.WithField("outage_id", activeOutages[0].ID).Debug("Active outage from this component-monitor already exists, skipping creation")
				continue
			}

			if len(status.Reasons) == 0 {
				statusLogger.Warn("No reasons provided for unhealthy status, skipping")
				continue
			}

			description := "Component monitor detected outage"

			outage := types.Outage{
				ComponentName:    status.ComponentSlug,
				SubComponentName: status.SubComponentSlug,
				Severity:         severity,
				StartTime:        now,
				EndTime:          sql.NullTime{Valid: false},
				Description:      description,
				DiscoveredFrom:   ComponentMonitor,
				CreatedBy:        req.ComponentMonitor,
			}

			if !subComponent.RequiresConfirmation {
				outage.ConfirmedBy = &req.ComponentMonitor
				outage.ConfirmedAt = sql.NullTime{Time: now, Valid: true}
			}

			if message, valid := outage.Validate(); !valid {
				return fmt.Errorf("validation failed: %s", message)
			}

			if err := p.outageManager.CreateOutage(&outage, status.Reasons); err != nil {
				statusLogger.WithField("error", err).Error("Failed to create outage and reasons")
				continue
			}

			statusLogger.WithField("reason_count", len(status.Reasons)).Info("Successfully created outage with reasons")
		}
	}

	return nil
}
