package main

import (
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// OutageRepository defines the interface for outage and reason database operations.
type OutageRepository interface {
	GetActiveOutagesFromSource(componentSlug, subComponentSlug, source string) ([]types.Outage, error)
	SaveOutage(outage *types.Outage) error
	CreateReason(reason *types.Reason) error
	CreateOutage(outage *types.Outage) error
	Transaction(fn func(OutageRepository) error) error
}

// gormOutageRepository is a GORM implementation of OutageRepository.
type gormOutageRepository struct {
	db *gorm.DB
}

// NewGORMOutageRepository creates a new GORM-based OutageRepository.
func NewGORMOutageRepository(db *gorm.DB) OutageRepository {
	return &gormOutageRepository{db: db}
}

// GetActiveOutagesFromSource retrieves all active outages for a specific component and sub-component
// that were created by the given component monitor. Note that the reasons are not considered here.
// An outage is considered active if its end_time is NULL.
func (r *gormOutageRepository) GetActiveOutagesFromSource(componentSlug, subComponentSlug, source string) ([]types.Outage, error) {
	var activeOutages []types.Outage
	err := r.db.
		Where("component_name = ? AND sub_component_name = ? AND end_time IS NULL AND created_by = ?",
			componentSlug, subComponentSlug, source).
		Find(&activeOutages).Error
	return activeOutages, err
}

// SaveOutage updates an existing outage record in the database.
// If the outage does not exist, it will be created.
func (r *gormOutageRepository) SaveOutage(outage *types.Outage) error {
	return r.db.Save(outage).Error
}

// CreateReason creates a new reason record in the database.
func (r *gormOutageRepository) CreateReason(reason *types.Reason) error {
	return r.db.Create(reason).Error
}

// CreateOutage creates a new outage record in the database.
func (r *gormOutageRepository) CreateOutage(outage *types.Outage) error {
	return r.db.Create(outage).Error
}

// Transaction executes the provided function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
func (r *gormOutageRepository) Transaction(fn func(OutageRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := &gormOutageRepository{db: tx}
		return fn(txRepo)
	})
}
