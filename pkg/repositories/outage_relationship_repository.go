package repositories

import (
	"ship-status-dash/pkg/types"

	"gorm.io/gorm"
)

// OutageRelationshipRepository handles persistence for outage-to-outage relationships.
type OutageRelationshipRepository interface {
	AddOutageRelationship(rel *types.OutageRelationship) (*types.OutageRelationship, error)
	ListOutageRelationships(outageID uint) ([]types.OutageRelationship, error)
	GetOutageRelationship(outageID, relationshipID uint) (*types.OutageRelationship, error)
	DeleteOutageRelationship(outageID, relationshipID uint) error
}

type gormOutageRelationshipRepository struct {
	db *gorm.DB
}

func NewGORMOutageRelationshipRepository(db *gorm.DB) OutageRelationshipRepository {
	return &gormOutageRelationshipRepository{db: db}
}

// AddOutageRelationship creates the relationship and its reciprocal row atomically.
func (r *gormOutageRelationshipRepository) AddOutageRelationship(rel *types.OutageRelationship) (*types.OutageRelationship, error) {
	inverse := &types.OutageRelationship{
		OutageID:         rel.RelatedOutageID,
		RelatedOutageID:  rel.OutageID,
		RelationshipType: types.InverseRelationshipType(rel.RelationshipType),
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rel).Error; err != nil {
			return err
		}
		return tx.Create(inverse).Error
	}); err != nil {
		return nil, err
	}

	if err := r.db.Preload("RelatedOutage").First(rel, rel.ID).Error; err != nil {
		return nil, err
	}
	return rel, nil
}

func (r *gormOutageRelationshipRepository) ListOutageRelationships(outageID uint) ([]types.OutageRelationship, error) {
	var rels []types.OutageRelationship
	if err := r.db.Preload("RelatedOutage").Where("outage_id = ?", outageID).Order("created_at ASC").Find(&rels).Error; err != nil {
		return nil, err
	}
	return rels, nil
}

func (r *gormOutageRelationshipRepository) GetOutageRelationship(outageID, relationshipID uint) (*types.OutageRelationship, error) {
	var rel types.OutageRelationship
	if err := r.db.Preload("RelatedOutage").Where("id = ? AND outage_id = ?", relationshipID, outageID).First(&rel).Error; err != nil {
		return nil, err
	}
	return &rel, nil
}

// DeleteOutageRelationship removes the relationship and its reciprocal row atomically.
func (r *gormOutageRelationshipRepository) DeleteOutageRelationship(outageID, relationshipID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var rel types.OutageRelationship
		if err := tx.Where("id = ? AND outage_id = ?", relationshipID, outageID).First(&rel).Error; err != nil {
			return err
		}

		inverseType := types.InverseRelationshipType(rel.RelationshipType)
		if err := tx.Where("outage_id = ? AND related_outage_id = ? AND relationship_type = ?",
			rel.RelatedOutageID, rel.OutageID, inverseType).Delete(&types.OutageRelationship{}).Error; err != nil {
			return err
		}

		return tx.Delete(&rel).Error
	})
}
