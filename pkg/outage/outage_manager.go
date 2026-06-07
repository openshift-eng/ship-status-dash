package outage

import (
	"encoding/json"
	"fmt"
	"time"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"gorm.io/gorm"
)

// OutageManager is the service-layer interface for outage lifecycle operations, including triage notes and links.
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

	AddTriageNote(note *types.TriageNote) error
	UpdateTriageNote(outageID, noteID uint, body, user string) (*types.TriageNote, error)
	DeleteTriageNote(outageID, noteID uint, user string) error
	AddOutageLink(link *types.OutageLink, user string) error
	UpdateOutageLink(outageID, linkID uint, url string, linkType types.LinkType, description, user string) (*types.OutageLink, error)
	DeleteOutageLink(outageID, linkID uint, user string) error
}

// DBOutageManager implements OutageManager with PostgreSQL persistence and optional Slack reporting.
type DBOutageManager struct {
	slackThreadRepo repositories.SlackThreadRepository
	db              *gorm.DB
	slackReporter   *SlackReporter
	logger          *logrus.Logger
}

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

// snapshotOutage captures the full outage state as JSON for before/after audit log comparison.
func (m *DBOutageManager) snapshotOutage(outageID uint) []byte {
	var outage types.Outage
	if err := m.db.Preload("Reasons").Preload("SlackThreads").Preload("TriageNotes").Preload("Links").First(&outage, outageID).Error; err != nil {
		return nil
	}
	data, err := json.Marshal(outage)
	if err != nil {
		m.logger.WithFields(logrus.Fields{
			"outage_id": outageID,
			"error":     err,
		}).Warn("Failed to marshal outage snapshot for audit log")
		return nil
	}
	return data
}

func (m *DBOutageManager) auditMutation(outageID uint, user string, old, new []byte) {
	if err := m.db.Create(&types.OutageAuditLog{
		OutageID:  outageID,
		User:      user,
		Operation: string(types.Update),
		Old:       old,
		New:       new,
	}).Error; err != nil {
		m.logger.WithFields(logrus.Fields{
			"outage_id": outageID,
			"error":     err,
		}).Error("Failed to write audit log for child mutation")
	}
}

// AddTriageNote saves a new triage note. Slack failures are logged but do not fail the operation.
func (m *DBOutageManager) AddTriageNote(note *types.TriageNote) error {
	oldOutage := m.loadOutage(note.OutageID)
	old := m.snapshotOutage(note.OutageID)

	noteRepo := repositories.NewGORMTriageNoteRepository(m.db)
	if err := noteRepo.AddTriageNote(note); err != nil {
		return err
	}

	m.auditMutation(note.OutageID, note.Author, old, m.snapshotOutage(note.OutageID))
	m.reportChildUpdate(note.OutageID, oldOutage)
	return nil
}

func (m *DBOutageManager) UpdateTriageNote(outageID, noteID uint, body, user string) (*types.TriageNote, error) {
	old := m.snapshotOutage(outageID)

	noteRepo := repositories.NewGORMTriageNoteRepository(m.db)
	result, err := noteRepo.UpdateTriageNote(outageID, noteID, body)
	if err != nil {
		return nil, err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return result, nil
}

func (m *DBOutageManager) DeleteTriageNote(outageID, noteID uint, user string) error {
	old := m.snapshotOutage(outageID)

	noteRepo := repositories.NewGORMTriageNoteRepository(m.db)
	if err := noteRepo.DeleteTriageNote(outageID, noteID); err != nil {
		return err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return nil
}

func (m *DBOutageManager) AddOutageLink(link *types.OutageLink, user string) error {
	oldOutage := m.loadOutage(link.OutageID)
	old := m.snapshotOutage(link.OutageID)

	linkRepo := repositories.NewGORMOutageLinkRepository(m.db)
	if err := linkRepo.AddOutageLink(link); err != nil {
		return err
	}

	m.auditMutation(link.OutageID, user, old, m.snapshotOutage(link.OutageID))
	m.reportChildUpdate(link.OutageID, oldOutage)
	return nil
}

func (m *DBOutageManager) UpdateOutageLink(outageID, linkID uint, url string, linkType types.LinkType, description, user string) (*types.OutageLink, error) {
	oldOutage := m.loadOutage(outageID)
	old := m.snapshotOutage(outageID)

	linkRepo := repositories.NewGORMOutageLinkRepository(m.db)
	result, err := linkRepo.UpdateOutageLink(outageID, linkID, url, linkType, description)
	if err != nil {
		return nil, err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	m.reportChildUpdate(outageID, oldOutage)
	return result, nil
}

func (m *DBOutageManager) DeleteOutageLink(outageID, linkID uint, user string) error {
	old := m.snapshotOutage(outageID)

	linkRepo := repositories.NewGORMOutageLinkRepository(m.db)
	if err := linkRepo.DeleteOutageLink(outageID, linkID); err != nil {
		return err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return nil
}

// loadOutage captures the pre-mutation state so reportChildUpdate can diff against post-mutation.
func (m *DBOutageManager) loadOutage(outageID uint) *types.Outage {
	var outage types.Outage
	if err := m.db.Preload("Reasons").Preload("SlackThreads").Preload("TriageNotes").Preload("Links").First(&outage, outageID).Error; err != nil {
		return nil
	}
	return &outage
}

// reportChildUpdate triggers Slack thread replies by diffing the pre/post outage state.
func (m *DBOutageManager) reportChildUpdate(outageID uint, oldOutage *types.Outage) {
	if m.slackReporter == nil || oldOutage == nil {
		return
	}
	newOutage := m.loadOutage(outageID)
	if newOutage == nil {
		return
	}
	if err := m.slackReporter.ReportOutageUpdate(newOutage, oldOutage); err != nil {
		m.logger.WithFields(logrus.Fields{
			"outage_id": outageID,
			"error":     err,
		}).Error("Failed to report child update to Slack")
	}
}
