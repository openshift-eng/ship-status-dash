package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/outage"
)

const (
	suspectedExpiryDuration = 24 * time.Hour
	suspectedExpiryResolver = "suspected-expiry"
)

// SuspectedOutageExpiryChecker resolves suspected outages that have not received
// a new report within the expiry duration.
type SuspectedOutageExpiryChecker struct {
	outageManager outage.OutageManager
	checkInterval time.Duration
	logger        *logrus.Logger
}

// NewSuspectedOutageExpiryChecker creates a new SuspectedOutageExpiryChecker.
func NewSuspectedOutageExpiryChecker(outageManager outage.OutageManager, checkInterval time.Duration, logger *logrus.Logger) *SuspectedOutageExpiryChecker {
	return &SuspectedOutageExpiryChecker{
		outageManager: outageManager,
		checkInterval: checkInterval,
		logger:        logger,
	}
}

// Start begins the periodic expiry check loop.
func (c *SuspectedOutageExpiryChecker) Start(ctx context.Context) {
	c.logger.WithField("check_interval", c.checkInterval).Info("Starting suspected outage expiry checker")
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping suspected outage expiry checker")
			return
		case <-ticker.C:
			c.expireStaleOutages()
		}
	}
}

// expireStaleOutages finds active suspected outages where the most recent report
// is older than 24 hours and resolves them.
func (c *SuspectedOutageExpiryChecker) expireStaleOutages() {
	logger := c.logger.WithField("check", "suspected_expiry")

	cutoff := time.Now().Add(-suspectedExpiryDuration)

	staleOutages, err := c.outageManager.GetStaleSuspectedOutages(cutoff)
	if err != nil {
		logger.WithField("error", err).Error("Failed to query stale suspected outages")
		return
	}

	if len(staleOutages) == 0 {
		return
	}

	now := time.Now()
	for i := range staleOutages {
		staleOutages[i].EndTime = sql.NullTime{Time: now, Valid: true}
		if err := c.outageManager.UpdateOutage(&staleOutages[i], suspectedExpiryResolver); err != nil {
			logger.WithFields(logrus.Fields{
				"outage_id": staleOutages[i].ID,
				"error":     err,
			}).Error("Failed to auto-resolve stale suspected outage")
			continue
		}
		logger.WithField("outage_id", staleOutages[i].ID).Info("Auto-resolved stale suspected outage (no reports in 24h)")
	}
}
