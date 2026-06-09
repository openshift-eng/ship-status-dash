package outage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"gorm.io/gorm"
)

var (
	ErrOutageAlreadyTracked = errors.New("an outage is already being tracked for this component")
	ErrAlreadyReported      = errors.New("you have already reported this outage")
)

// ReportResult contains the outcome of a community outage report.
type ReportResult struct {
	Outage      *types.Outage
	Created     bool
	ReportCount int64
}

// OutageManager defines the interface for outage management operations.
type OutageManager interface {
	CreateOutage(outage *types.Outage, reasons []types.Reason, user string) error
	UpdateOutage(outage *types.Outage, user string) error
	GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error)
	GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error)
	GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error)
	GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error)
	FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error)
	AppendReasons(outageID uint, reasons []types.Reason) error
	GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error)
	GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error)
	DeleteOutage(outage *types.Outage, user string) error
	ReportSuspectedOutage(componentSlug, subComponentSlug, description, user string, threshold int) (*ReportResult, error)
}

// DBOutageManager handles outage creation and updates with Slack reporting using a database.
type DBOutageManager struct {
	slackThreadRepo repositories.SlackThreadRepository
	db              *gorm.DB
	slackReporter   *SlackReporter
	logger          *logrus.Logger
}

// NewDBOutageManager creates a new DBOutageManager instance.
func NewDBOutageManager(
	db *gorm.DB,
	slackClient *slack.Client,
	configManager *config.Manager[types.DashboardConfig],
	baseURL string,
	slackWorkspaceURL string,
	logger *logrus.Logger,
) *DBOutageManager {
	slackThreadRepo := repositories.NewGORMSlackThreadRepository(db)
	var slackReporter *SlackReporter
	if slackClient != nil {
		slackReporter = NewSlackReporter(slackClient, slackThreadRepo, configManager, baseURL, slackWorkspaceURL, logger)
	}

	return &DBOutageManager{
		slackThreadRepo: slackThreadRepo,
		db:              db,
		slackReporter:   slackReporter,
		logger:          logger,
	}
}

// CreateOutage creates a new outage and posts to Slack channels if configured.
func (m *DBOutageManager) CreateOutage(outage *types.Outage, reasons []types.Reason, user string) error {
	if msg, ok := outage.Validate(); !ok {
		return fmt.Errorf("validation failed: %s", msg)
	}

	if err := m.db.Transaction(func(tx *gorm.DB) error {
		outageRepo := repositories.NewGORMOutageRepository(tx)
		if err := outageRepo.CreateOutage(outage, user); err != nil {
			return err
		}

		for _, reason := range reasons {
			reason.OutageID = outage.ID
			if err := outageRepo.CreateReason(&reason); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// Slack reporting is done outside the transaction as we don't want to fail to create the outage due to slack reporting issues
	if m.slackReporter != nil {
		if err := m.slackReporter.ReportOutage(outage); err != nil {
			m.logger.WithFields(logrus.Fields{
				"outage_id": outage.ID,
				"error":     err,
			}).Error("Failed to report outage to Slack, but outage was created")
		}
	}

	return nil
}

// UpdateOutage updates an existing outage and posts thread replies to Slack.
func (m *DBOutageManager) UpdateOutage(outage *types.Outage, user string) error {
	if msg, ok := outage.Validate(); !ok {
		return fmt.Errorf("validation failed: %s", msg)
	}

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	oldOutage, err := outageRepo.GetOutageByID(outage.ComponentName, outage.SubComponentName, outage.ID)
	if err != nil {
		return err
	}

	if err := outageRepo.SaveOutage(outage, user); err != nil {
		return err
	}

	if m.slackReporter != nil {
		if err := m.slackReporter.ReportOutageUpdate(outage, oldOutage); err != nil {
			m.logger.WithFields(logrus.Fields{
				"outage_id": outage.ID,
				"error":     err,
			}).Error("Failed to report outage update to Slack, but outage was updated")
		}
	}

	return nil
}

// GetOutageByID delegates read operations to the repository.
func (m *DBOutageManager) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutageByID(componentSlug, subComponentSlug, outageID)
}

func (m *DBOutageManager) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *DBOutageManager) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesForComponent(componentSlug, subComponentSlugs)
}

func (m *DBOutageManager) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *DBOutageManager) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesForComponent(componentSlug)
}

func (m *DBOutageManager) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy)
}

func (m *DBOutageManager) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom)
}

func (m *DBOutageManager) FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.FindReopenableOutage(componentSlug, subComponentSlug, createdBy, since, reasons)
}

func (m *DBOutageManager) AppendReasons(outageID uint, reasons []types.Reason) error {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.AppendReasons(outageID, reasons)
}

func (m *DBOutageManager) GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutagesDuring(queryStart, queryEnd, refs)
}

func (m *DBOutageManager) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.GetOutageAuditLogs(outageID)
}

func (m *DBOutageManager) DeleteOutage(outage *types.Outage, user string) error {
	outageRepo := repositories.NewGORMOutageRepository(m.db)
	return outageRepo.DeleteOutage(outage, user)
}

// ReportSuspectedOutage handles a community report for a sub-component.
// It creates a new suspected outage or +1s an existing one, upgrading severity when the threshold is met.
func (m *DBOutageManager) ReportSuspectedOutage(componentSlug, subComponentSlug, description, user string, threshold int) (*ReportResult, error) {
	logger := m.logger.WithFields(logrus.Fields{
		"component":     componentSlug,
		"sub_component": subComponentSlug,
		"user":          user,
	})

	var result ReportResult

	if err := m.db.Transaction(func(tx *gorm.DB) error {
		// Find any active outage on this sub-component
		var activeOutage types.Outage
		err := tx.Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL",
			componentSlug, subComponentSlug).First(&activeOutage).Error

		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to query active outages: %w", err)
		}

		if err == gorm.ErrRecordNotFound {
			// No active outage — create a new suspected outage
			desc := "Suspected outage reported by community"
			if description != "" {
				desc = description
			}
			activeOutage = types.Outage{
				ComponentName:    componentSlug,
				SubComponentName: subComponentSlug,
				Severity:         types.SeveritySuspected,
				StartTime:        time.Now(),
				Description:      desc,
				DiscoveredFrom:   "community",
				CreatedBy:        user,
			}
			if err := tx.WithContext(withUser(tx.Statement.Context, user)).Create(&activeOutage).Error; err != nil {
				return fmt.Errorf("failed to create suspected outage: %w", err)
			}
			result.Created = true
			logger.WithField("outage_id", activeOutage.ID).Info("Created new suspected outage from community report")
		} else if activeOutage.Severity != types.SeveritySuspected || activeOutage.ConfirmedAt.Valid {
			// Active outage exists that isn't a community-reported suspected outage — reject
			return ErrOutageAlreadyTracked
		}

		// Check if user already reported this outage
		var existingReport types.OutageReport
		err = tx.Where("outage_id = ? AND \"user\" = ?", activeOutage.ID, user).First(&existingReport).Error
		if err == nil {
			return ErrAlreadyReported
		}
		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check existing report: %w", err)
		}

		// Add the report
		report := types.OutageReport{
			OutageID: activeOutage.ID,
			User:     user,
		}
		if err := tx.Create(&report).Error; err != nil {
			return fmt.Errorf("failed to create outage report: %w", err)
		}

		// Count total reports
		var count int64
		if err := tx.Model(&types.OutageReport{}).Where("outage_id = ?", activeOutage.ID).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to count reports: %w", err)
		}
		result.ReportCount = count

		// Check threshold — upgrade to Degraded if met
		if count >= int64(threshold) && activeOutage.Severity == types.SeveritySuspected {
			activeOutage.Severity = types.SeverityDegraded
			activeOutage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
			if err := tx.WithContext(withUser(tx.Statement.Context, user)).Save(&activeOutage).Error; err != nil {
				return fmt.Errorf("failed to upgrade outage severity: %w", err)
			}
			logger.WithFields(logrus.Fields{
				"outage_id":    activeOutage.ID,
				"report_count": count,
				"threshold":    threshold,
			}).Info("Suspected outage reached threshold, upgraded to Degraded")
		}

		result.Outage = &activeOutage
		return nil
	}); err != nil {
		return nil, err
	}

	// Fire Slack reporting outside the transaction if threshold was just met
	if result.Outage != nil && result.Outage.Severity == types.SeverityDegraded && result.Outage.ConfirmedAt.Valid && m.slackReporter != nil {
		if err := m.slackReporter.ReportOutage(result.Outage); err != nil {
			logger.WithFields(logrus.Fields{
				"outage_id": result.Outage.ID,
				"error":     err,
			}).Error("Failed to report upgraded outage to Slack")
		}
	}

	return &result, nil
}

func withUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, types.CurrentUserKey, user)
}
