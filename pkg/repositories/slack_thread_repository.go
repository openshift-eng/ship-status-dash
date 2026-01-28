package repositories

import (
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

// SlackThreadRepository defines the interface for outage Slack thread database operations.
type SlackThreadRepository interface {
	CreateThread(thread *types.SlackThread) error
	GetThreadsForOutage(outageID uint) ([]types.SlackThread, error)
	GetThreadForOutageAndChannel(outageID uint, channel string) (*types.SlackThread, error)
	UpdateThread(thread *types.SlackThread) error
}

// gormSlackThreadRepository is a GORM implementation of SlackThreadRepository.
type gormSlackThreadRepository struct {
	db *gorm.DB
}

// NewGORMSlackThreadRepository creates a new GORM-based SlackThreadRepository.
func NewGORMSlackThreadRepository(db *gorm.DB) SlackThreadRepository {
	return &gormSlackThreadRepository{db: db}
}

// CreateThread creates a new Slack thread record in the database.
func (r *gormSlackThreadRepository) CreateThread(thread *types.SlackThread) error {
	return r.db.Create(thread).Error
}

// GetThreadsForOutage retrieves all Slack threads for a specific outage.
func (r *gormSlackThreadRepository) GetThreadsForOutage(outageID uint) ([]types.SlackThread, error) {
	var threads []types.SlackThread
	err := r.db.Where("outage_id = ?", outageID).Find(&threads).Error
	return threads, err
}

// GetThreadForOutageAndChannel retrieves a specific Slack thread for an outage and channel.
// Returns gorm.ErrRecordNotFound if the thread is not found.
func (r *gormSlackThreadRepository) GetThreadForOutageAndChannel(outageID uint, channel string) (*types.SlackThread, error) {
	var thread types.SlackThread
	err := r.db.Where("outage_id = ? AND channel = ?", outageID, channel).First(&thread).Error
	if err != nil {
		return nil, err
	}
	return &thread, nil
}

// UpdateThread updates an existing Slack thread record in the database.
func (r *gormSlackThreadRepository) UpdateThread(thread *types.SlackThread) error {
	return r.db.Save(thread).Error
}
