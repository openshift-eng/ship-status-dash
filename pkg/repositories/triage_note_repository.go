package repositories

import (
	"ship-status-dash/pkg/types"

	"gorm.io/gorm"
)

// TriageNoteRepository handles persistence for triage notes attached to outages.
type TriageNoteRepository interface {
	AddTriageNote(note *types.TriageNote) error
	GetTriageNote(outageID, noteID uint) (*types.TriageNote, error)
	UpdateTriageNote(outageID, noteID uint, body string) (*types.TriageNote, error)
	DeleteTriageNote(outageID, noteID uint) error
}

type gormTriageNoteRepository struct {
	db *gorm.DB
}

func NewGORMTriageNoteRepository(db *gorm.DB) TriageNoteRepository {
	return &gormTriageNoteRepository{db: db}
}

func (r *gormTriageNoteRepository) AddTriageNote(note *types.TriageNote) error {
	return r.db.Create(note).Error
}

// Returns gorm.ErrRecordNotFound if no matching note exists for the given outage.
func (r *gormTriageNoteRepository) GetTriageNote(outageID, noteID uint) (*types.TriageNote, error) {
	var note types.TriageNote
	if err := r.db.Where("id = ? AND outage_id = ?", noteID, outageID).First(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

// Returns gorm.ErrRecordNotFound if no matching note exists for the given outage.
func (r *gormTriageNoteRepository) UpdateTriageNote(outageID, noteID uint, body string) (*types.TriageNote, error) {
	result := r.db.Model(&types.TriageNote{}).Where("id = ? AND outage_id = ?", noteID, outageID).Update("body", body)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var note types.TriageNote
	if err := r.db.Where("id = ? AND outage_id = ?", noteID, outageID).First(&note).Error; err != nil {
		return nil, err
	}
	return &note, nil
}

// Returns gorm.ErrRecordNotFound if no matching note exists for the given outage.
func (r *gormTriageNoteRepository) DeleteTriageNote(outageID, noteID uint) error {
	result := r.db.Where("id = ? AND outage_id = ?", noteID, outageID).Delete(&types.TriageNote{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
