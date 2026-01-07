package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

const (
	AbsentReportSource  = "absent-monitored-component-report"
	AbsentReportCreator = "dashboard"
)

// AbsentMonitoredComponentReportChecker handles outages for components where pings have not been received within the expected time.
type AbsentMonitoredComponentReportChecker struct {
	config        *types.DashboardConfig
	outageRepo    repositories.OutageRepository
	pingRepo      repositories.ComponentPingRepository
	checkInterval time.Duration
	logger        *logrus.Logger
}

// NewAbsentMonitoredComponentReportChecker creates a new AbsentMonitoredComponentReportChecker instance.
func NewAbsentMonitoredComponentReportChecker(config *types.DashboardConfig, outageRepo repositories.OutageRepository, pingRepo repositories.ComponentPingRepository, checkInterval time.Duration, logger *logrus.Logger) *AbsentMonitoredComponentReportChecker {
	return &AbsentMonitoredComponentReportChecker{
		config:        config,
		outageRepo:    outageRepo,
		pingRepo:      pingRepo,
		checkInterval: checkInterval,
		logger:        logger,
	}
}

// Start begins the absent report checker.
func (a *AbsentMonitoredComponentReportChecker) Start(ctx context.Context) {
	initialDelay := 3 * a.checkInterval
	a.logger.WithFields(logrus.Fields{
		"check_interval": a.checkInterval,
		"initial_delay":  initialDelay,
	}).Info("Starting absent monitored component report checker after initial delay")
	ticker := time.NewTicker(a.checkInterval)
	defer ticker.Stop()

	initialTimer := time.NewTimer(initialDelay)
	defer initialTimer.Stop()

	// Wait for initial delay before first check
	select {
	case <-ctx.Done():
		a.logger.Info("Stopping absent monitored component report checker")
		return
	case <-initialTimer.C:
		a.checkForAbsentReports()
	}

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Stopping absent monitored component report checker")
			return
		case <-ticker.C:
			a.checkForAbsentReports()
		}
	}
}

// checkForAbsentReports iterates through all configured sub-components,
// checking if they have been pinged within 3x their configured frequency.
func (a *AbsentMonitoredComponentReportChecker) checkForAbsentReports() {
	logger := a.logger.WithField("check", "absent_report")
	logger.Info("Checking for absent monitored component reports")

	for _, component := range a.config.Components {
		for _, subComponent := range component.Subcomponents {
			// Skip sub-components without monitoring configuration
			if subComponent.Monitoring.Frequency == "" {
				continue
			}

			componentLogger := logger.WithFields(logrus.Fields{
				"component":     component.Name,
				"sub_component": subComponent.Name,
			})

			frequency, err := time.ParseDuration(subComponent.Monitoring.Frequency)
			if err != nil {
				componentLogger.WithField("error", err).WithField("frequency", subComponent.Monitoring.Frequency).Warn("Failed to parse monitoring frequency, skipping")
				continue
			}

			threshold := 3 * frequency
			lastPingTime, err := a.pingRepo.GetLastPingTime(component.Slug, subComponent.Slug)
			if err != nil {
				componentLogger.WithField("error", err).Error("Failed to get last ping time")
				continue
			}

			now := time.Now()
			var componentInOutage bool
			var reason string

			if lastPingTime == nil {
				// No ping record exists - this is an absent report
				componentInOutage = true
				reason = "No report from component-monitor found"
			} else {
				timeSinceLastPing := now.Sub(*lastPingTime)
				if timeSinceLastPing > threshold {
					componentInOutage = true
					reason = "Last report from component-monitor was " + timeSinceLastPing.Round(time.Second).String() + " ago, exceeding threshold of " + threshold.Round(time.Second).String()
				}
			}

			activeOutages, err := a.outageRepo.GetActiveOutagesDiscoveredFrom(component.Slug, subComponent.Slug, AbsentReportSource)
			if err != nil {
				componentLogger.WithField("error", err).Error("Failed to check for existing outages")
				continue
			}

			if !componentInOutage {
				// Ping is healthy - resolve any existing outages if auto-resolve is enabled
				if subComponent.Monitoring.AutoResolve && len(activeOutages) > 0 {
					for i := range activeOutages {
						activeOutages[i].EndTime = sql.NullTime{Time: now, Valid: true}
						resolver := subComponent.Monitoring.ComponentMonitor
						activeOutages[i].ResolvedBy = &resolver
						if err := a.outageRepo.SaveOutage(&activeOutages[i]); err != nil {
							componentLogger.WithFields(logrus.Fields{
								"outage_id": activeOutages[i].ID,
								"error":     err,
							}).Error("Failed to auto-resolve absent-report outage")
							continue
						}
						componentLogger.WithField("outage_id", activeOutages[i].ID).Info("Auto-resolved absent-report outage after receiving ping")
					}
				}
				continue
			}

			if len(activeOutages) > 0 {
				componentLogger.WithField("outage_id", activeOutages[0].ID).Debug("Active absent-report outage already exists, skipping creation")
				continue
			}

			// Create the outage
			outage := types.Outage{
				ComponentName:    component.Slug,
				SubComponentName: subComponent.Slug,
				Severity:         types.SeverityDown,
				StartTime:        now,
				EndTime:          sql.NullTime{Valid: false},
				Description:      "Component-monitor has not reported status within expected time. " + reason,
				DiscoveredFrom:   AbsentReportSource,
				CreatedBy:        AbsentReportCreator,
			}

			// Auto-confirm if the sub-component doesn't require confirmation
			if !subComponent.RequiresConfirmation {
				creator := AbsentReportCreator
				outage.ConfirmedBy = &creator
				outage.ConfirmedAt = sql.NullTime{Time: now, Valid: true}
			}

			if message, valid := outage.Validate(); !valid {
				componentLogger.WithField("validation_error", message).Error("Failed to validate created outage")
				continue
			}

			if err := a.outageRepo.CreateOutage(&outage); err != nil {
				componentLogger.WithField("error", err).Error("Failed to create absent-report outage")
				continue
			}

			componentLogger.WithFields(logrus.Fields{
				"outage_id": outage.ID,
				"reason":    reason,
			}).Info("Created absent-report outage")
		}
	}
}
