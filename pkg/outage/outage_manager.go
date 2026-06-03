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

	AddTriageNote(note *types.TriageNote) error
	UpdateTriageNote(noteID, outageID uint, body, user string) (*types.TriageNote, error)
	DeleteTriageNote(noteID, outageID uint, user string) error
	AddOutageLink(link *types.OutageLink) error
	UpdateOutageLink(linkID, outageID uint, url string, linkType types.LinkType, description, user string) (*types.OutageLink, error)
	DeleteOutageLink(outageID, linkID uint, user string) error
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

// snapshotOutage loads the full outage with associations for audit logging.
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

// auditMutation writes an UPDATE audit log entry for child-entity mutations (notes, links).
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

// AddTriageNote saves a new triage note. If Slack is configured, it attempts a thread reply, but
// Slack failures are logged and do not fail the operation.
func (m *DBOutageManager) AddTriageNote(note *types.TriageNote) error {
	old := m.snapshotOutage(note.OutageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	if err := outageRepo.AddTriageNote(note); err != nil {
		return err
	}

	m.auditMutation(note.OutageID, note.Author, old, m.snapshotOutage(note.OutageID))

	if m.slackReporter != nil {
		threads, err := m.slackReporter.slackThreadRepo.GetThreadsForOutage(note.OutageID)
		if err != nil {
			m.logger.WithFields(logrus.Fields{
				"outage_id": note.OutageID,
				"error":     err,
			}).Warn("Failed to get Slack threads for triage note")
		} else if len(threads) > 0 {
			message := fmt.Sprintf("Triage note from `%s`:\n%s", note.Author, formatQuoteBlock(truncateString(note.Body)))
			outage := &types.Outage{}
			outage.ID = note.OutageID
			if err := m.slackReporter.replyToSlackThreads(outage, threads, message); err != nil {
				m.logger.WithFields(logrus.Fields{
					"outage_id": note.OutageID,
					"error":     err,
				}).Error("Failed to post triage note to Slack, but note was saved")
			}
		}
	}

	return nil
}

// UpdateTriageNote updates the body of an existing triage note.
func (m *DBOutageManager) UpdateTriageNote(noteID, outageID uint, body, user string) (*types.TriageNote, error) {
	old := m.snapshotOutage(outageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	result, err := outageRepo.UpdateTriageNote(noteID, outageID, body)
	if err != nil {
		return nil, err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return result, nil
}

// DeleteTriageNote removes a triage note from an outage.
func (m *DBOutageManager) DeleteTriageNote(noteID, outageID uint, user string) error {
	old := m.snapshotOutage(outageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	if err := outageRepo.DeleteTriageNote(noteID, outageID); err != nil {
		return err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return nil
}

// AddOutageLink saves a new user-curated link associated with an outage.
func (m *DBOutageManager) AddOutageLink(link *types.OutageLink) error {
	old := m.snapshotOutage(link.OutageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	if err := outageRepo.AddOutageLink(link); err != nil {
		return err
	}

	m.auditMutation(link.OutageID, link.AddedBy, old, m.snapshotOutage(link.OutageID))
	return nil
}

// UpdateOutageLink updates an existing outage link's fields.
func (m *DBOutageManager) UpdateOutageLink(linkID, outageID uint, url string, linkType types.LinkType, description, user string) (*types.OutageLink, error) {
	old := m.snapshotOutage(outageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	result, err := outageRepo.UpdateOutageLink(linkID, outageID, url, linkType, description)
	if err != nil {
		return nil, err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return result, nil
}

// DeleteOutageLink removes a link from an outage by ID.
func (m *DBOutageManager) DeleteOutageLink(outageID, linkID uint, user string) error {
	old := m.snapshotOutage(outageID)

	outageRepo := repositories.NewGORMOutageRepository(m.db)
	if err := outageRepo.DeleteOutageLink(outageID, linkID); err != nil {
		return err
	}

	m.auditMutation(outageID, user, old, m.snapshotOutage(outageID))
	return nil
}
