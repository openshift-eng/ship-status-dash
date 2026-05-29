package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// OutageRepository defines the interface for outage and reason database operations.
type OutageRepository interface {
	CreateOutage(outage *types.Outage, user string) error
	CreateReason(reason *types.Reason) error

	SaveOutage(outage *types.Outage, user string) error

	GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error)
	GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error)
	GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error)
	GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error)
	GetAllActiveOutages() ([]types.Outage, error)
	GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error)
	FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error)
	AppendReasons(outageID uint, reasons []types.Reason) error

	GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error)

	GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error)

	AddTriageNote(note *types.TriageNote) error
	AddOutageLink(link *types.OutageLink) error
	DeleteOutageLink(outageID, linkID uint) error

	DeleteOutage(outage *types.Outage, user string) error
}

// gormOutageRepository is a GORM implementation of OutageRepository.
type gormOutageRepository struct {
	db *gorm.DB
}

// NewGORMOutageRepository creates a new GORM-based OutageRepository.
func NewGORMOutageRepository(db *gorm.DB) OutageRepository {
	return &gormOutageRepository{db: db}
}

// roundOutageTimes truncates all time fields in an outage down to the nearest second.
func roundOutageTimes(outage *types.Outage) {
	outage.StartTime = outage.StartTime.Truncate(time.Second).UTC()
	if outage.EndTime.Valid {
		outage.EndTime = sql.NullTime{
			Time:  outage.EndTime.Time.Truncate(time.Second).UTC(),
			Valid: true,
		}
	}
	if outage.ConfirmedAt.Valid {
		outage.ConfirmedAt = sql.NullTime{
			Time:  outage.ConfirmedAt.Time.Truncate(time.Second).UTC(),
			Valid: true,
		}
	}
}

// CreateOutage creates a new outage record in the database.
func (r *gormOutageRepository) CreateOutage(outage *types.Outage, user string) error {
	roundOutageTimes(outage)
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Create(outage).Error
}

// CreateReason creates a new reason record in the database.
func (r *gormOutageRepository) CreateReason(reason *types.Reason) error {
	return r.db.Create(reason).Error
}

// SaveOutage updates an existing outage record in the database.
// If the outage does not exist, it will be created.
func (r *gormOutageRepository) SaveOutage(outage *types.Outage, user string) error {
	roundOutageTimes(outage)
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Save(outage).Error
}

// GetOutageByID retrieves a specific outage by ID for a component/sub-component combination.
// Returns gorm.ErrRecordNotFound if the outage is not found.
func (r *gormOutageRepository) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	var outage types.Outage
	err := r.db.Preload("Reasons").Preload("SlackThreads").Preload("TriageNotes").Preload("Links").
		Where("id = ? AND component_name = ? AND sub_component_name = ?", outageID, componentSlug, subComponentSlug).
		First(&outage).Error
	return &outage, err
}

// GetOutagesForSubComponent retrieves all outages for a specific sub-component.
// Reasons are preloaded.
func (r *gormOutageRepository) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	err := r.db.Preload("Reasons").
		Where("component_name = ? AND sub_component_name = ?", componentSlug, subComponentSlug).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetOutagesForComponent retrieves all outages for a component across multiple sub-components.
// Reasons are preloaded.
func (r *gormOutageRepository) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	var outages []types.Outage
	err := r.db.Preload("Reasons").
		Where("component_name = ? AND sub_component_name IN ?", componentSlug, subComponentSlugs).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesForSubComponent retrieves active outages for a specific sub-component.
// An outage is considered active if end_time IS NULL OR end_time > now (UTC for consistent DB comparison).
func (r *gormOutageRepository) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("component_name = ? AND sub_component_name = ? AND (end_time IS NULL OR end_time > ?)", componentSlug, subComponentSlug, now).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesForComponent retrieves active outages for a component across all sub-components.
// An outage is considered active if end_time IS NULL OR end_time > now (UTC for consistent DB comparison).
func (r *gormOutageRepository) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("component_name = ? AND (end_time IS NULL OR end_time > ?)", componentSlug, now).
		Order("start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetAllActiveOutages retrieves every outage that is still considered active (end_time IS NULL OR end_time > now UTC).
func (r *gormOutageRepository) GetAllActiveOutages() ([]types.Outage, error) {
	var outages []types.Outage
	now := time.Now().UTC()
	err := r.db.Where("end_time IS NULL OR end_time > ?", now).
		Order("component_name, sub_component_name, start_time DESC").
		Find(&outages).Error
	return outages, err
}

// GetActiveOutagesCreatedBy retrieves all active outages for a specific component and sub-component
// that were created by the given creator. Note that the reasons are not considered here.
// An outage is considered active if its end_time is NULL.
func (r *gormOutageRepository) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	var activeOutages []types.Outage
	err := r.db.
		Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL AND created_by = ?",
			componentSlug, subComponentSlug, createdBy).
		Find(&activeOutages).Error
	return activeOutages, err
}

// GetActiveOutagesDiscoveredFrom retrieves all active outages for a specific component and sub-component
// that were discovered from the given source. Note that the reasons are not considered here.
// An outage is considered active if its end_time is NULL.
func (r *gormOutageRepository) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	var activeOutages []types.Outage
	err := r.db.
		Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL AND discovered_from = ?",
			componentSlug, subComponentSlug, discoveredFrom).
		Find(&activeOutages).Error
	return activeOutages, err
}

// AppendReasons creates new reason records for an existing outage.
func (r *gormOutageRepository) AppendReasons(outageID uint, reasons []types.Reason) error {
	if len(reasons) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, reason := range reasons {
			reason.OutageID = outageID
			if err := tx.Create(&reason).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindReopenableOutage returns the most recently-closed outage within the flap window that
// shares at least one probe reason (same Type and Check) with the incoming reasons, or nil if none found.
func (r *gormOutageRepository) FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error) {
	if len(reasons) == 0 {
		return nil, nil
	}
	reasonConds := make([]string, len(reasons))
	reasonArgs := make([]interface{}, 0, len(reasons)*2)
	for i, reason := range reasons {
		reasonConds[i] = "(r.type = ? AND r.check = ?)"
		reasonArgs = append(reasonArgs, reason.Type, reason.Check)
	}
	var outage types.Outage
	err := r.db.Preload("Reasons").
		Joins("JOIN reasons r ON r.outage_id = outages.id AND r.deleted_at IS NULL").
		Where("outages.component_name = ? AND outages.sub_component_name = ? AND outages.created_by = ? AND outages.end_time IS NOT NULL AND outages.end_time >= ?",
			componentSlug, subComponentSlug, createdBy, since.UTC()).
		Where("("+strings.Join(reasonConds, " OR ")+")", reasonArgs...).
		Order("outages.end_time DESC").
		Limit(1).
		First(&outage).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &outage, err
}

// GetOutagesDuring returns outages that overlap the query window: start_time <= queryEnd and
// (end_time IS NULL OR end_time > queryStart). When queryStart equals queryEnd this matches "active at that instant".
// refs limits rows to the given (component_slug, sub_slug) pairs; empty refs returns an empty slice.
func (r *gormOutageRepository) GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
	if len(refs) == 0 {
		return []types.Outage{}, nil
	}
	qs := queryStart.UTC()
	qe := queryEnd.UTC()
	q := r.db.Preload("Reasons").
		Where("start_time <= ? AND (end_time IS NULL OR end_time > ?)", qe, qs)
	conds := make([]string, len(refs))
	args := make([]interface{}, 0, len(refs)*2)
	for i, ref := range refs {
		conds[i] = "(component_name = ? AND sub_component_name = ?)"
		args = append(args, ref.ComponentSlug, ref.SubSlug)
	}
	q = q.Where("("+strings.Join(conds, " OR ")+")", args...)
	var outages []types.Outage
	if err := q.Order("start_time DESC").Find(&outages).Error; err != nil {
		return nil, fmt.Errorf("OutageRepository.GetOutagesDuring: query outages: %w", err)
	}
	return outages, nil
}

func (r *gormOutageRepository) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	var outageAuditLogs []types.OutageAuditLog
	err := r.db.Where("outage_id = ?", outageID).Order("created_at DESC").Find(&outageAuditLogs).Error
	return outageAuditLogs, err
}

// AddTriageNote creates a new triage note for an outage.
func (r *gormOutageRepository) AddTriageNote(note *types.TriageNote) error {
	return r.db.Create(note).Error
}

// AddOutageLink creates a new link associated with an outage.
func (r *gormOutageRepository) AddOutageLink(link *types.OutageLink) error {
	return r.db.Create(link).Error
}

// DeleteOutageLink removes a link by ID, scoped to the given outage.
// Returns gorm.ErrRecordNotFound if no matching link exists.
func (r *gormOutageRepository) DeleteOutageLink(outageID, linkID uint) error {
	result := r.db.Where("id = ? AND outage_id = ?", linkID, outageID).Delete(&types.OutageLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteOutage deletes an outage from the database.
func (r *gormOutageRepository) DeleteOutage(outage *types.Outage, user string) error {
	return r.db.WithContext(context.WithValue(context.Background(), types.CurrentUserKey, user)).Delete(outage).Error
}
