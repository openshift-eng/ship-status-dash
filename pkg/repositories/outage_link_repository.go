package repositories

import (
	"ship-status-dash/pkg/types"

	"gorm.io/gorm"
)

// OutageLinkRepository handles persistence for user-curated links attached to outages.
type OutageLinkRepository interface {
	AddOutageLink(link *types.OutageLink) error
	GetOutageLink(outageID, linkID uint) (*types.OutageLink, error)
	UpdateOutageLink(outageID, linkID uint, url string, linkType types.LinkType, description string) (*types.OutageLink, error)
	DeleteOutageLink(outageID, linkID uint) error
}

type gormOutageLinkRepository struct {
	db *gorm.DB
}

func NewGORMOutageLinkRepository(db *gorm.DB) OutageLinkRepository {
	return &gormOutageLinkRepository{db: db}
}

func (r *gormOutageLinkRepository) AddOutageLink(link *types.OutageLink) error {
	return r.db.Create(link).Error
}

// Returns gorm.ErrRecordNotFound if no matching link exists for the given outage.
func (r *gormOutageLinkRepository) GetOutageLink(outageID, linkID uint) (*types.OutageLink, error) {
	var link types.OutageLink
	if err := r.db.Where("id = ? AND outage_id = ?", linkID, outageID).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

// Returns gorm.ErrRecordNotFound if no matching link exists for the given outage.
func (r *gormOutageLinkRepository) UpdateOutageLink(outageID, linkID uint, url string, linkType types.LinkType, description string) (*types.OutageLink, error) {
	result := r.db.Model(&types.OutageLink{}).
		Where("id = ? AND outage_id = ?", linkID, outageID).
		Updates(map[string]interface{}{
			"url":         url,
			"link_type":   linkType,
			"description": description,
		})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var link types.OutageLink
	if err := r.db.Where("id = ? AND outage_id = ?", linkID, outageID).First(&link).Error; err != nil {
		return nil, err
	}
	return &link, nil
}

// Returns gorm.ErrRecordNotFound if no matching link exists for the given outage.
func (r *gormOutageLinkRepository) DeleteOutageLink(outageID, linkID uint) error {
	result := r.db.Where("id = ? AND outage_id = ?", linkID, outageID).Delete(&types.OutageLink{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
