package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

const (
	suspectedExpiryDuration = 24 * time.Hour
	suspectedExpiryResolver = "suspected-expiry"
)

// SuspectedOutageExpiryChecker resolves suspected outages that have not received
// a new report within the expiry duration.
type SuspectedOutageExpiryChecker struct {
	db            *gorm.DB
	checkInterval time.Duration
	logger        *logrus.Logger
}

// NewSuspectedOutageExpiryChecker creates a new SuspectedOutageExpiryChecker.
func NewSuspectedOutageExpiryChecker(db *gorm.DB, checkInterval time.Duration, logger *logrus.Logger) *SuspectedOutageExpiryChecker {
	return &SuspectedOutageExpiryChecker{
		db:            db,
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

	// Find suspected outages where the latest report is older than the cutoff.
	// Uses a subquery to get the max report time per outage.
	var staleOutages []types.Outage
	err := c.db.
		Where("severity = ? AND end_time IS NULL AND confirmed_at IS NULL", types.SeveritySuspected).
		Where("id IN (?)",
			c.db.Model(&types.OutageReport{}).
				Select("outage_id").
				Group("outage_id").
				Having("MAX(created_at) < ?", cutoff),
		).
		Find(&staleOutages).Error

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
		ctx := context.WithValue(context.Background(), types.CurrentUserKey, suspectedExpiryResolver)
		if err := c.db.WithContext(ctx).Save(&staleOutages[i]).Error; err != nil {
			logger.WithFields(logrus.Fields{
				"outage_id": staleOutages[i].ID,
				"error":     err,
			}).Error("Failed to auto-resolve stale suspected outage")
			continue
		}
		logger.WithField("outage_id", staleOutages[i].ID).Info("Auto-resolved stale suspected outage (no reports in 24h)")
	}
}
