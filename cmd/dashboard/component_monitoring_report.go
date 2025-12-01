package main

import (
	"database/sql"
	"fmt"
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
	config *types.Config
	logger *logrus.Logger
}

// NewComponentMonitorReportProcessor creates a new processor instance.
func NewComponentMonitorReportProcessor(db *gorm.DB, config *types.Config, logger *logrus.Logger) *ComponentMonitorReportProcessor {
	return &ComponentMonitorReportProcessor{
		repo:   NewGORMOutageRepository(db),
		config: config,
		logger: logger,
	}
}

// Process processes a component monitor report request.
// All components and sub-components are assumed to be valid (validated in the API layer).
func (p *ComponentMonitorReportProcessor) Process(req *types.ComponentMonitorReportRequest, validateOutage func(*types.Outage) (string, bool)) error {
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

		activeOutages, err := p.repo.GetActiveOutages(status.ComponentSlug, status.SubComponentSlug, ComponentMonitor, status.Reason.Type)
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
				statusLogger.WithField("outage_id", activeOutages[0].ID).Debug("Active outage with matching reason type already exists, skipping creation")
				continue
			}

			err = p.repo.Transaction(func(repo OutageRepository) error {
				reason := types.Reason{
					Type:    status.Reason.Type,
					Check:   status.Reason.Check,
					Results: status.Reason.Results,
				}
				if err := repo.CreateReason(&reason); err != nil {
					return err
				}

				description := fmt.Sprintf("Component monitor detected outage via %s - check: %s, results: %s", status.Reason.Type, status.Reason.Check, status.Reason.Results)
				outage := types.Outage{
					ComponentName:    status.ComponentSlug,
					SubComponentName: status.SubComponentSlug,
					Severity:         severity,
					StartTime:        time.Now(),
					EndTime:          sql.NullTime{Valid: false},
					Description:      description,
					DiscoveredFrom:   ComponentMonitor,
					CreatedBy:        req.ComponentMonitor,
					ReasonID:         &reason.ID,
				}

				if !subComponent.RequiresConfirmation {
					outage.ConfirmedBy = &req.ComponentMonitor
					outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
				}

				if message, valid := validateOutage(&outage); !valid {
					return fmt.Errorf("validation failed: %s", message)
				}

				return repo.CreateOutage(&outage)
			})

			if err != nil {
				statusLogger.WithField("error", err).Error("Failed to create outage and reason")
				continue
			}

			statusLogger.Info("Successfully created outage with reason")
		}
	}

	return nil
}
