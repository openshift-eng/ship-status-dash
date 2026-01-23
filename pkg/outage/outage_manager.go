package outage

import (
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
)

// OutageManager handles outage creation and updates with Slack reporting.
type OutageManager struct {
	repo          repositories.OutageRepository
	slackReporter *SlackReporter
	logger        *logrus.Logger
}

// NewOutageManager creates a new OutageManager instance.
func NewOutageManager(
	repo repositories.OutageRepository,
	slackThreadRepo repositories.SlackThreadRepository,
	slackClient *slack.Client,
	configManager *config.Manager[types.DashboardConfig],
	baseURL string,
	slackWorkspaceURL string,
	logger *logrus.Logger,
) *OutageManager {
	var slackReporter *SlackReporter
	if slackClient != nil {
		slackReporter = NewSlackReporter(slackClient, slackThreadRepo, configManager, baseURL, slackWorkspaceURL, logger)
	}

	return &OutageManager{
		repo:          repo,
		slackReporter: slackReporter,
		logger:        logger,
	}
}

// CreateOutage creates a new outage and posts to Slack channels if configured.
func (m *OutageManager) CreateOutage(outage *types.Outage) error {
	if err := m.repo.CreateOutage(outage); err != nil {
		return err
	}

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
func (m *OutageManager) UpdateOutage(outage *types.Outage) error {
	oldOutage, err := m.repo.GetOutageByID(outage.ComponentName, outage.SubComponentName, outage.ID)
	if err != nil {
		return err
	}

	if err := m.repo.SaveOutage(outage); err != nil {
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

// CreateReason delegates to the repository.
func (m *OutageManager) CreateReason(reason *types.Reason) error {
	return m.repo.CreateReason(reason)
}

// Transaction wraps the repository transaction.
func (m *OutageManager) Transaction(fn func(*OutageManager) error) error {
	return m.repo.Transaction(func(repo repositories.OutageRepository) error {
		txManager := &OutageManager{
			repo:          repo,
			slackReporter: m.slackReporter,
			logger:        m.logger,
		}
		return fn(txManager)
	})
}

// Delegate read operations to repository
func (m *OutageManager) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	return m.repo.GetOutageByID(componentSlug, subComponentSlug, outageID)
}

func (m *OutageManager) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return m.repo.GetOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *OutageManager) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	return m.repo.GetOutagesForComponent(componentSlug, subComponentSlugs)
}

func (m *OutageManager) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return m.repo.GetActiveOutagesForSubComponent(componentSlug, subComponentSlug)
}

func (m *OutageManager) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	return m.repo.GetActiveOutagesForComponent(componentSlug)
}

func (m *OutageManager) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	return m.repo.GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy)
}

func (m *OutageManager) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	return m.repo.GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom)
}

func (m *OutageManager) DeleteOutage(outage *types.Outage) error {
	return m.repo.DeleteOutage(outage)
}
