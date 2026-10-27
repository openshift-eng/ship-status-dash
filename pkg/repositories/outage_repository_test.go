package repositories

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"
)

func setupOutageRepositoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.Outage{},
		&types.Reason{},
		&types.SlackThread{},
		&types.TriageNote{},
		&types.OutageLink{},
		&types.OutageRelationship{},
	))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})

	return db
}

func TestGetOutageByIDOrdersTriageNotesChronologically(t *testing.T) {
	db := setupOutageRepositoryDB(t)
	repo := NewGORMOutageRepository(db)
	noteRepo := NewGORMTriageNoteRepository(db)
	baseTime := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		endTime sql.NullTime
	}{
		{name: "active"},
		{
			name: "resolved",
			endTime: sql.NullTime{
				Time:  baseTime.Add(4 * time.Hour),
				Valid: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outage := &types.Outage{
				ComponentName:    "test-component",
				SubComponentName: fmt.Sprintf("test-%s", tt.name),
				Severity:         types.SeverityDegraded,
				StartTime:        baseTime,
				EndTime:          tt.endTime,
				Description:      "Repository ordering test",
				DiscoveredFrom:   "test",
				CreatedBy:        "test",
			}
			require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(outage).Error)

			notes := []types.TriageNote{
				{OutageID: outage.ID, Body: "latest", Author: "test", Model: gorm.Model{CreatedAt: baseTime.Add(3 * time.Hour)}},
				{OutageID: outage.ID, Body: "tie first", Author: "test", Model: gorm.Model{CreatedAt: baseTime.Add(2 * time.Hour)}},
				{OutageID: outage.ID, Body: "earliest", Author: "test", Model: gorm.Model{CreatedAt: baseTime.Add(time.Hour)}},
				{OutageID: outage.ID, Body: "tie second", Author: "test", Model: gorm.Model{CreatedAt: baseTime.Add(2 * time.Hour)}},
			}
			require.NoError(t, db.Create(&notes).Error)

			got, err := repo.GetOutageByID(outage.ComponentName, outage.SubComponentName, outage.ID)
			require.NoError(t, err)
			wantIDs := []uint{notes[2].ID, notes[1].ID, notes[3].ID, notes[0].ID}
			require.Equal(t, wantIDs, triageNoteIDs(got.TriageNotes))

			listed, err := noteRepo.ListTriageNotes(outage.ID)
			require.NoError(t, err)
			require.Equal(t, wantIDs, triageNoteIDs(listed))
		})
	}
}

func triageNoteIDs(notes []types.TriageNote) []uint {
	ids := make([]uint, len(notes))
	for i := range notes {
		ids[i] = notes[i].ID
	}
	return ids
}
