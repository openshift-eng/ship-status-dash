package repositories

import (
	"time"

	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// ComponentPingRepository defines the interface for component report ping database operations.
type ComponentPingRepository interface {
	UpsertComponentReportPing(componentSlug, subComponentSlug string, timestamp time.Time) error
	GetLastPingTime(componentSlug, subComponentSlug string) (*time.Time, error)
	GetMostRecentPingTimeForAnySubComponent(componentSlug string) (*time.Time, error)
}

// gormComponentPingRepository is a GORM implementation of ComponentPingRepository.
type gormComponentPingRepository struct {
	db *gorm.DB
}

// NewGORMComponentPingRepository creates a new GORM-based ComponentPingRepository.
func NewGORMComponentPingRepository(db *gorm.DB) ComponentPingRepository {
	return &gormComponentPingRepository{db: db}
}

// UpsertComponentReportPing creates or updates a ComponentReportPing record.
// There should only be one record per component/sub_component combination.
// The unique constraint on (component_name, sub_component_name) ensures this at the database level.
func (r *gormComponentPingRepository) UpsertComponentReportPing(componentSlug, subComponentSlug string, timestamp time.Time) error {
	ping := types.ComponentReportPing{
		ComponentName:    componentSlug,
		SubComponentName: subComponentSlug,
		Time:             timestamp,
	}

	var existing types.ComponentReportPing
	result := r.db.Where("component_name = ? AND sub_component_name = ?", componentSlug, subComponentSlug).First(&existing)

	if result.Error == gorm.ErrRecordNotFound {
		err := r.db.Create(&ping).Error
		// Handle race condition: if another goroutine created the record between our check and create,
		// we'll get a duplicate key error. In that case, update the existing record instead.
		if err == gorm.ErrDuplicatedKey {
			var existingAfterRace types.ComponentReportPing
			if findErr := r.db.Where("component_name = ? AND sub_component_name = ?", componentSlug, subComponentSlug).First(&existingAfterRace).Error; findErr == nil {
				existingAfterRace.Time = timestamp
				return r.db.Save(&existingAfterRace).Error
			}
			return err
		}
		return err
	} else if result.Error != nil {
		return result.Error
	}

	existing.Time = timestamp
	return r.db.Save(&existing).Error
}

// GetLastPingTime retrieves the last ping time for a component/sub-component combination.
// Returns nil if no ping record exists.
func (r *gormComponentPingRepository) GetLastPingTime(componentSlug, subComponentSlug string) (*time.Time, error) {
	var ping types.ComponentReportPing
	err := r.db.Where("component_name = ? AND sub_component_name = ?", componentSlug, subComponentSlug).First(&ping).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ping.Time, nil
}

// GetMostRecentPingTimeForAnySubComponent retrieves the most recent ping time for any sub-component of a component.
// Returns nil if no ping record exists.
func (r *gormComponentPingRepository) GetMostRecentPingTimeForAnySubComponent(componentSlug string) (*time.Time, error) {
	var ping types.ComponentReportPing
	err := r.db.Where("component_name = ?", componentSlug).Order("time DESC").First(&ping).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ping.Time, nil
}
