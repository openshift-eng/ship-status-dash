package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

const (
	ComponentMonitor = "component-monitor"
)

// ComponentMonitorReportProcessor handles the business logic for processing component monitor reports.
type ComponentMonitorReportProcessor struct {
	repo   OutageRepository
	config *types.DashboardConfig
	logger *logrus.Logger
}

// NewComponentMonitorReportProcessor creates a new processor instance.
func NewComponentMonitorReportProcessor(db *gorm.DB, config *types.DashboardConfig, logger *logrus.Logger) *ComponentMonitorReportProcessor {
	return &ComponentMonitorReportProcessor{
		repo:   NewGORMOutageRepository(db),
		config: config,
		logger: logger,
	}
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

		component := p.config.GetComponentBySlug(status.ComponentSlug)
		if component == nil {
			statusLogger.Error("Component not found in processor config")
			return fmt.Errorf("component not found: %s", status.ComponentSlug)
		}

		subComponent := component.GetSubComponentBySlug(status.SubComponentSlug)
		if subComponent == nil {
			statusLogger.Error("Sub-component not found in processor config")
			return fmt.Errorf("sub-component not found: %s/%s", status.ComponentSlug, status.SubComponentSlug)
		}

		// Find all the active outages that this component-monitor has reported. This will not pick up any outages that were created by other sources.
		// It will result in multiple outages if users (or other systems) have created outages for this component/sub-component.
		activeOutages, err := p.repo.GetActiveOutagesFromSource(status.ComponentSlug, status.SubComponentSlug, req.ComponentMonitor)
		if err != nil {
			statusLogger.WithField("error", err).Error("Failed to query active outages")
			return err
		}

		if status.Status == types.StatusHealthy {
			if len(activeOutages) == 0 {
				statusLogger.Debug("Sub Component reported healthy, and no active outages to resolve")
				continue
			}

			if !subComponent.Monitoring.AutoResolve {
				statusLogger.Debug("Auto-resolve disabled, skipping healthy status processing")
				continue
			}

			now := time.Now()
			resolvedBy := req.ComponentMonitor
			for i := range activeOutages {
				activeOutages[i].EndTime = sql.NullTime{Time: now, Valid: true}
				activeOutages[i].ResolvedBy = &resolvedBy
				if err := p.repo.SaveOutage(&activeOutages[i]); err != nil {
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

			err = p.repo.Transaction(func(repo OutageRepository) error {
				var descriptionParts []string
				for _, reason := range status.Reasons {
					descriptionParts = append(descriptionParts, fmt.Sprintf("%s - check: %s, results: %s", reason.Type, reason.Check, reason.Results))
				}
				description := fmt.Sprintf("Component monitor detected outage via %s", strings.Join(descriptionParts, "; "))

				outage := types.Outage{
					ComponentName:    status.ComponentSlug,
					SubComponentName: status.SubComponentSlug,
					Severity:         severity,
					StartTime:        time.Now(),
					EndTime:          sql.NullTime{Valid: false},
					Description:      description,
					DiscoveredFrom:   ComponentMonitor,
					CreatedBy:        req.ComponentMonitor,
				}

				if !subComponent.RequiresConfirmation {
					outage.ConfirmedBy = &req.ComponentMonitor
					outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
				}

				if message, valid := outage.Validate(); !valid {
					return fmt.Errorf("validation failed: %s", message)
				}

				if err := repo.CreateOutage(&outage); err != nil {
					return err
				}

				for _, reasonData := range status.Reasons {
					reason := types.Reason{
						OutageID: outage.ID,
						Type:     reasonData.Type,
						Check:    reasonData.Check,
						Results:  reasonData.Results,
					}
					if err := repo.CreateReason(&reason); err != nil {
						return err
					}
				}
				return nil
			})

			if err != nil {
				statusLogger.WithField("error", err).Error("Failed to create outage and reasons")
				continue
			}

			statusLogger.WithField("reason_count", len(status.Reasons)).Info("Successfully created outage with reasons")
		}
	}

	return nil
}
