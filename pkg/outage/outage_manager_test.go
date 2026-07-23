package outage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database for testing and migrates the standard outage-related models.
// The database is automatically closed when the test completes.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	err = db.AutoMigrate(&types.Outage{}, &types.Reason{}, &types.SlackThread{}, &types.OutageAuditLog{}, &types.OutageReport{}, &types.TriageNote{}, &types.OutageLink{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("test database sql.DB: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Fatalf("test database close: %v", err)
		}
	})
	return db
}

type testManager struct {
	manager    OutageManager
	db         *gorm.DB
	mockServer *MockSlackServer
}

func setupTestManager(t *testing.T, cfg *types.DashboardConfig) *testManager {
	db := setupTestDB(t)

	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	cfgManager.Get()

	mockServer := NewMockSlackServer(t)
	slackClient := mockServer.Client()

	manager := NewDBOutageManager(db, slackClient, cfgManager, "https://test.example.com/", "https://rhsandbox.slack.com/", logrus.New())

	return &testManager{
		manager:    manager,
		db:         db,
		mockServer: mockServer,
	}
}

func (tm *testManager) close() {
	tm.mockServer.Close()
}

func assertSlackMessages(t *testing.T, mockServer *MockSlackServer, want []PostedMessage) {
	postedMsgs := mockServer.PostedMessages()
	if diff := cmp.Diff(want, postedMsgs, testhelper.EquateNilEmpty); diff != "" {
		t.Errorf("Slack messages mismatch (-want +got):\n%s", diff)
	}
}

func TestOutageManager_CreateOutage(t *testing.T) {
	tests := []struct {
		name              string
		outage            *types.Outage
		reasons           []types.Reason
		config            *types.DashboardConfig
		verifyOutage      func(t *testing.T, outage *types.Outage)
		verifyReasons     func(t *testing.T, db *gorm.DB, outageID uint)
		wantSlackMessages []PostedMessage
	}{
		{
			name: "successful creation with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						SlackReporting: []types.SlackReportingConfig{
							{Channel: "#test-channel"},
						},
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				wantStartTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				assert.True(t, outage.StartTime.Equal(wantStartTime), "StartTime = %v, want %v", outage.StartTime, wantStartTime)
				assert.Equal(t, "system", outage.CreatedBy)
				assert.Equal(t, "component-monitor", outage.DiscoveredFrom)
				assert.NotZero(t, outage.ID, "ID should be set")
			},
			wantSlackMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "🚨 Outage Detected: Test Component/Test Sub\n\nSeverity: `Down`\nDescription:\n>Automated test outage\nStarted: `2024-01-15T10:30:00Z`\nCreated by: `system`\nDiscovered from: `component-monitor`\n\n<https://test.example.com/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
		{
			name: "successful creation with reasons",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			reasons: []types.Reason{
				{
					Type:    types.CheckTypePrometheus,
					Check:   "up{job=\"test\"} == 0",
					Results: "No healthy instances found",
				},
				{
					Type:    types.CheckTypeHTTP,
					Check:   "https://test.example.com/health",
					Results: "Response time > 5s",
				},
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.NotZero(t, outage.ID, "ID should be set")
			},
			verifyReasons: func(t *testing.T, db *gorm.DB, outageID uint) {
				var createdReasons []types.Reason
				err := db.Where("outage_id = ?", outageID).Find(&createdReasons).Error
				assert.NoError(t, err)
				assert.Len(t, createdReasons, 2, "Should create 2 reasons")

				assert.Equal(t, types.CheckTypePrometheus, createdReasons[0].Type)
				assert.Equal(t, "up{job=\"test\"} == 0", createdReasons[0].Check)
				assert.Equal(t, "No healthy instances found", createdReasons[0].Results)
				assert.Equal(t, outageID, createdReasons[0].OutageID)

				assert.Equal(t, types.CheckTypeHTTP, createdReasons[1].Type)
				assert.Equal(t, "https://test.example.com/health", createdReasons[1].Check)
				assert.Equal(t, "Response time > 5s", createdReasons[1].Results)
				assert.Equal(t, outageID, createdReasons[1].OutageID)
			},
			wantSlackMessages: []PostedMessage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.config)
			defer tm.close()

			err := tm.manager.CreateOutage(tt.outage, tt.reasons, "test-user", "")
			if err != nil {
				t.Fatalf("Failed to create outage: %v", err)
			}

			var createdOutages []types.Outage
			err = tm.db.Find(&createdOutages).Error
			if err != nil {
				t.Fatalf("Failed to query created outages: %v", err)
			}

			if len(createdOutages) != 1 {
				t.Fatalf("Expected 1 outage, got %d", len(createdOutages))
			}

			if tt.verifyOutage != nil {
				tt.verifyOutage(t, &createdOutages[0])
			}

			if tt.verifyReasons != nil {
				tt.verifyReasons(t, tm.db, createdOutages[0].ID)
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)

			var logs []types.OutageAuditLog
			err = tm.db.Where("outage_id = ?", createdOutages[0].ID).Find(&logs).Error
			require.NoError(t, err)
			require.Len(t, logs, 1)
			assert.Equal(t, "CREATE", logs[0].Operation)
			assert.Equal(t, "test-user", logs[0].User)
		})
	}
}

func TestOutageManager_UpdateOutage(t *testing.T) {
	tests := []struct {
		name              string
		outage            *types.Outage
		mutateOutage      func(*types.Outage)
		config            *types.DashboardConfig
		slackThreadRepo   *repositories.MockSlackThreadRepository
		verifyOutage      func(t *testing.T, outage *types.Outage)
		wantSlackMessages []PostedMessage
	}{
		{
			name: "successful update with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Description:      "Automated test outage",
				CreatedBy:        "system",
				DiscoveredFrom:   "component-monitor",
			},
			mutateOutage: func(o *types.Outage) {
				o.Severity = types.SeverityDegraded
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Name: "Test Component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub", Name: "Test Sub"},
						},
					},
				},
			},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				ThreadsForOutage: []types.SlackThread{
					{
						Channel:         "#test-channel",
						ThreadTimestamp: "1234567890.123456",
					},
				},
			},
			verifyOutage: func(t *testing.T, outage *types.Outage) {
				assert.Equal(t, uint(1), outage.ID)
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-sub", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.Equal(t, "Automated test outage", outage.Description)
			},
			wantSlackMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "📝 Outage Updated: Test Component/Test Sub (#1)\n\nSeverity changed: `Degraded` → `Down`\n\n<https://test.example.com/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "1234567890.123456",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.config)
			defer tm.close()

			oldOutage := *tt.outage
			tt.mutateOutage(&oldOutage)
			ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
			err := tm.db.WithContext(ctx).Create(&oldOutage).Error
			if err != nil {
				t.Fatalf("Failed to create old outage: %v", err)
			}

			if tt.slackThreadRepo != nil && len(tt.slackThreadRepo.ThreadsForOutage) > 0 {
				thread := tt.slackThreadRepo.ThreadsForOutage[0]
				thread.OutageID = tt.outage.ID
				err := tm.db.Create(&thread).Error
				if err != nil {
					t.Fatalf("Failed to create slack thread: %v", err)
				}
			}

			err = tm.manager.UpdateOutage(tt.outage, "test-user")
			if err != nil {
				t.Fatalf("Failed to update outage: %v", err)
			}

			var savedOutages []types.Outage
			err = tm.db.Where("id = ?", tt.outage.ID).Find(&savedOutages).Error
			if err != nil {
				t.Fatalf("Failed to query saved outages: %v", err)
			}

			if len(savedOutages) != 1 {
				t.Fatalf("Expected 1 outage, got %d", len(savedOutages))
			}

			if tt.verifyOutage != nil {
				tt.verifyOutage(t, &savedOutages[0])
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)

			var logs []types.OutageAuditLog
			err = tm.db.Where("outage_id = ?", tt.outage.ID).Order("created_at DESC").Find(&logs).Error
			require.NoError(t, err)
			require.Len(t, logs, 2)
			assert.Equal(t, "UPDATE", logs[0].Operation)
			assert.Equal(t, "test-user", logs[0].User)
			assert.Equal(t, "CREATE", logs[1].Operation)
			assert.Equal(t, "test-user", logs[1].User)
		})
	}
}

func TestOutageManager_GetOutageAuditLogs(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, tm *testManager) uint
		wantLogs []types.OutageAuditLog
	}{
		{
			name: "create only returns one CREATE log",
			setup: func(t *testing.T, tm *testManager) uint {
				outage := &types.Outage{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
					StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					Description:      "Automated test outage",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				config := &types.DashboardConfig{
					Components: []*types.Component{
						{
							Slug: "test-component",
							Name: "Test Component",
							Subcomponents: []types.SubComponent{
								{Slug: "test-sub", Name: "Test Sub"},
							},
						},
					},
				}
				tm2 := setupTestManager(t, config)
				*tm = *tm2
				err := tm.manager.CreateOutage(outage, nil, "test-user", "")
				require.NoError(t, err)
				return outage.ID
			},
			wantLogs: []types.OutageAuditLog{
				{Operation: "CREATE", User: "test-user"},
			},
		},
		{
			name: "create and update returns two logs newest first",
			setup: func(t *testing.T, tm *testManager) uint {
				config := &types.DashboardConfig{
					Components: []*types.Component{
						{
							Slug: "test-component",
							Name: "Test Component",
							Subcomponents: []types.SubComponent{
								{Slug: "test-sub", Name: "Test Sub"},
							},
						},
					},
				}
				tm2 := setupTestManager(t, config)
				*tm = *tm2
				outage := &types.Outage{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
					StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					Description:      "Automated test outage",
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				}
				ctx := context.WithValue(context.Background(), types.CurrentUserKey, "test-user")
				err := tm.db.WithContext(ctx).Create(outage).Error
				require.NoError(t, err)
				outage.Severity = types.SeverityDegraded
				err = tm.manager.UpdateOutage(outage, "test-user")
				require.NoError(t, err)
				return outage.ID
			},
			wantLogs: []types.OutageAuditLog{
				{Operation: "UPDATE", User: "test-user"},
				{Operation: "CREATE", User: "test-user"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tm testManager
			outageID := tt.setup(t, &tm)
			defer tm.close()

			logs, err := tm.manager.GetOutageAuditLogs(outageID)
			require.NoError(t, err)
			require.Len(t, logs, len(tt.wantLogs))
			for i := range tt.wantLogs {
				assert.Equal(t, tt.wantLogs[i].Operation, logs[i].Operation)
				assert.Equal(t, tt.wantLogs[i].User, logs[i].User)
			}
		})
	}
}

func TestOutageManager_DeleteOutage(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		Model:            gorm.Model{ID: 1},
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Automated test outage",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "test-user", "")
	require.NoError(t, err)
	outageID := outage.ID

	err = tm.manager.DeleteOutage(outage, "test-user")
	require.NoError(t, err)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outageID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "DELETE", logs[0].Operation)
	assert.Equal(t, "test-user", logs[0].User)
	assert.Equal(t, "CREATE", logs[1].Operation)
	assert.Equal(t, "test-user", logs[1].User)
}

func TestOutageManager_CreateOutage_WithInitialTriageNote(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage with initial note",
		CreatedBy:        "reporter",
		DiscoveredFrom:   "frontend",
	}

	err := tm.manager.CreateOutage(outage, nil, "reporter", "Initial investigation started")
	require.NoError(t, err)
	assert.NotZero(t, outage.ID)

	var notes []types.TriageNote
	err = tm.db.Where("outage_id = ?", outage.ID).Find(&notes).Error
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "Initial investigation started", notes[0].Body)
	assert.Equal(t, "reporter", notes[0].Author)
	assert.Equal(t, outage.ID, notes[0].OutageID)
}

func TestOutageManager_ReportSuspectedOutage(t *testing.T) {
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				SlackReporting: []types.SlackReportingConfig{
					{Channel: "#test-channel"},
				},
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}

	t.Run("first report creates a suspected outage", func(t *testing.T) {
		tm := setupTestManager(t, cfg)
		defer tm.close()

		result, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "things seem broken", "user1", 3)
		require.NoError(t, err)

		assert.True(t, result.Created)
		assert.Equal(t, int64(1), result.ReportCount)
		assert.Equal(t, types.SeveritySuspected, result.Outage.Severity)
		assert.Equal(t, "things seem broken", result.Outage.Description)
		assert.Equal(t, CommunityReportSource, result.Outage.DiscoveredFrom)
		assert.Equal(t, "user1", result.Outage.CreatedBy)
		assert.False(t, result.Outage.ConfirmedAt.Valid)
		assert.False(t, result.Outage.EndTime.Valid)

		// Verify the report is persisted
		var reports []types.OutageReport
		require.NoError(t, tm.db.Where("outage_id = ?", result.Outage.ID).Find(&reports).Error)
		require.Len(t, reports, 1)
		assert.Equal(t, "user1", reports[0].User)

		// Slack should NOT fire for a suspected outage
		assertSlackMessages(t, tm.mockServer, nil)
	})

	t.Run("subsequent reports +1 the same outage", func(t *testing.T) {
		tm := setupTestManager(t, cfg)
		defer tm.close()

		first, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "broken", "user1", 3)
		require.NoError(t, err)

		second, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "", "user2", 3)
		require.NoError(t, err)

		// Core invariant: same outage, not a new one
		assert.Equal(t, first.Outage.ID, second.Outage.ID)
		assert.False(t, second.Created)
		assert.Equal(t, int64(2), second.ReportCount)
		// Severity remains Suspected below threshold
		assert.Equal(t, types.SeveritySuspected, second.Outage.Severity)
		assert.False(t, second.Outage.ConfirmedAt.Valid)
	})

	t.Run("same user cannot report twice", func(t *testing.T) {
		tm := setupTestManager(t, cfg)
		defer tm.close()

		_, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "broken", "user1", 3)
		require.NoError(t, err)

		_, err = tm.manager.ReportSuspectedOutage("test-component", "test-sub", "", "user1", 3)
		assert.Error(t, err)
	})

	t.Run("threshold triggers upgrade to degraded and fires slack", func(t *testing.T) {
		tm := setupTestManager(t, cfg)
		defer tm.close()

		_, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "broken", "user1", 3)
		require.NoError(t, err)
		_, err = tm.manager.ReportSuspectedOutage("test-component", "test-sub", "", "user2", 3)
		require.NoError(t, err)

		// Third report hits threshold
		result, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "", "user3", 3)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result.ReportCount)
		assert.Equal(t, types.SeverityDegraded, result.Outage.Severity)
		assert.False(t, result.Outage.ConfirmedAt.Valid)

		// Slack fires on upgrade
		msgs := tm.mockServer.PostedMessages()
		require.Len(t, msgs, 1)
		assert.Equal(t, "#test-channel", msgs[0].Channel)
	})

	t.Run("creates suspected outage even when confirmed outage exists", func(t *testing.T) {
		tm := setupTestManager(t, cfg)
		defer tm.close()

		confirmed := &types.Outage{
			ComponentName:    "test-component",
			SubComponentName: "test-sub",
			Severity:         types.SeverityDown,
			StartTime:        time.Now(),
			Description:      "Admin confirmed outage",
			DiscoveredFrom:   "frontend",
			CreatedBy:        "admin",
			ConfirmedAt:      sql.NullTime{Time: time.Now(), Valid: true},
		}
		require.NoError(t, tm.manager.CreateOutage(confirmed, nil, "admin", ""))

		result, err := tm.manager.ReportSuspectedOutage("test-component", "test-sub", "seems broken", "user1", 3)
		require.NoError(t, err)
		assert.True(t, result.Created)
	})
}

func TestOutageManager_AddTriageNote(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				SlackReporting: []types.SlackReportingConfig{
					{Channel: "#test-channel"},
				},
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for triage note test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	note := &types.TriageNote{
		OutageID: outage.ID,
		Body:     "Investigating root cause",
		Author:   "on-call-user",
	}
	err = tm.manager.AddTriageNote(note)
	require.NoError(t, err)
	assert.NotZero(t, note.ID)

	var saved []types.TriageNote
	err = tm.db.Where("outage_id = ?", outage.ID).Find(&saved).Error
	require.NoError(t, err)
	require.Len(t, saved, 1)
	assert.Equal(t, "Investigating root cause", saved[0].Body)
	assert.Equal(t, "on-call-user", saved[0].Author)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "on-call-user", logs[0].User)

	msgs := tm.mockServer.PostedMessages()
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[1].Text, "Triage note from `on-call-user`")
	assert.Contains(t, msgs[1].Text, "Investigating root cause")
}

func TestOutageManager_UpdateTriageNote(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for update note test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	note := &types.TriageNote{
		OutageID: outage.ID,
		Body:     "Original text",
		Author:   "author-user",
	}
	err = tm.manager.AddTriageNote(note)
	require.NoError(t, err)

	updated, err := tm.manager.UpdateTriageNote(outage.ID, note.ID, "Revised text", "author-user")
	require.NoError(t, err)
	assert.Equal(t, "Revised text", updated.Body)

	var saved types.TriageNote
	err = tm.db.First(&saved, note.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Revised text", saved.Body)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 3)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "author-user", logs[0].User)
}

func TestOutageManager_DeleteTriageNote(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for delete note test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	note := &types.TriageNote{
		OutageID: outage.ID,
		Body:     "Note to delete",
		Author:   "author-user",
	}
	err = tm.manager.AddTriageNote(note)
	require.NoError(t, err)

	err = tm.manager.DeleteTriageNote(outage.ID, note.ID, "author-user")
	require.NoError(t, err)

	var remaining []types.TriageNote
	err = tm.db.Where("outage_id = ?", outage.ID).Find(&remaining).Error
	require.NoError(t, err)
	assert.Empty(t, remaining)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 3)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "author-user", logs[0].User)
}

func TestOutageManager_AddOutageLink(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				SlackReporting: []types.SlackReportingConfig{
					{Channel: "#test-channel"},
				},
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for link test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	link := &types.OutageLink{
		OutageID:    outage.ID,
		URL:         "https://example.com/rca",
		LinkType:    types.LinkTypeRCA,
		Description: "Root cause analysis",
	}
	err = tm.manager.AddOutageLink(link, "on-call-user")
	require.NoError(t, err)
	assert.NotZero(t, link.ID)

	var saved []types.OutageLink
	err = tm.db.Where("outage_id = ?", outage.ID).Find(&saved).Error
	require.NoError(t, err)
	require.Len(t, saved, 1)
	assert.Equal(t, "https://example.com/rca", saved[0].URL)
	assert.Equal(t, types.LinkTypeRCA, saved[0].LinkType)
	assert.Equal(t, "Root cause analysis", saved[0].Description)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "on-call-user", logs[0].User)

	msgs := tm.mockServer.PostedMessages()
	require.Len(t, msgs, 2)
	assert.Contains(t, msgs[1].Text, "Link added:")
	assert.Contains(t, msgs[1].Text, "https://example.com/rca")
}

func TestOutageManager_UpdateOutageLink(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				SlackReporting: []types.SlackReportingConfig{
					{Channel: "#test-channel"},
				},
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for update link test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	link := &types.OutageLink{
		OutageID:    outage.ID,
		URL:         "https://old.example.com",
		LinkType:    types.LinkTypeOther,
		Description: "Original link",
	}
	err = tm.manager.AddOutageLink(link, "on-call-user")
	require.NoError(t, err)

	updated, err := tm.manager.UpdateOutageLink(outage.ID, link.ID, "https://new.example.com", types.LinkTypeRCA, "Updated RCA", "on-call-user")
	require.NoError(t, err)
	assert.Equal(t, "https://new.example.com", updated.URL)
	assert.Equal(t, types.LinkTypeRCA, updated.LinkType)
	assert.Equal(t, "Updated RCA", updated.Description)

	var saved types.OutageLink
	err = tm.db.First(&saved, link.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "https://new.example.com", saved.URL)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 3)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "on-call-user", logs[0].User)

	msgs := tm.mockServer.PostedMessages()
	require.Len(t, msgs, 3)
	assert.Contains(t, msgs[2].Text, "Link updated:")
	assert.Contains(t, msgs[2].Text, "https://new.example.com")
}

func TestOutageManager_DeleteOutageLink(t *testing.T) {
	config := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "test-component",
				Name: "Test Component",
				Subcomponents: []types.SubComponent{
					{Slug: "test-sub", Name: "Test Sub"},
				},
			},
		},
	}
	tm := setupTestManager(t, config)
	defer tm.close()

	outage := &types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-sub",
		Severity:         types.SeverityDown,
		StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Description:      "Outage for delete link test",
		CreatedBy:        "system",
		DiscoveredFrom:   "component-monitor",
	}
	err := tm.manager.CreateOutage(outage, nil, "system", "")
	require.NoError(t, err)

	link := &types.OutageLink{
		OutageID:    outage.ID,
		URL:         "https://example.com/to-delete",
		LinkType:    types.LinkTypeOther,
		Description: "Will be removed",
	}
	err = tm.manager.AddOutageLink(link, "on-call-user")
	require.NoError(t, err)

	err = tm.manager.DeleteOutageLink(outage.ID, link.ID, "on-call-user")
	require.NoError(t, err)

	var remaining []types.OutageLink
	err = tm.db.Where("outage_id = ?", outage.ID).Find(&remaining).Error
	require.NoError(t, err)
	assert.Empty(t, remaining)

	var logs []types.OutageAuditLog
	err = tm.db.Where("outage_id = ?", outage.ID).Order("created_at DESC").Find(&logs).Error
	require.NoError(t, err)
	require.Len(t, logs, 3)
	assert.Equal(t, "UPDATE", logs[0].Operation)
	assert.Equal(t, "on-call-user", logs[0].User)
}

func TestCreateOutage_SuspectedCleanup(t *testing.T) {
	baseCfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug: "prow",
				Name: "Prow",
				Subcomponents: []types.SubComponent{
					{Slug: "tide", Name: "Tide"},
				},
			},
		},
	}

	t.Run("confirmed non-suspected resolves existing suspected", func(t *testing.T) {
		tm := setupTestManager(t, baseCfg)
		defer tm.close()

		suspected := &types.Outage{
			ComponentName:    "prow",
			SubComponentName: "tide",
			Severity:         types.SeveritySuspected,
			StartTime:        time.Now().Add(-1 * time.Hour),
			Description:      "Community reported suspected outage",
			CreatedBy:        "community-user",
			DiscoveredFrom:   "community",
		}
		err := tm.manager.CreateOutage(suspected, nil, "community-user", "")
		require.NoError(t, err)
		require.NotZero(t, suspected.ID)

		confirmed := &types.Outage{
			ComponentName:    "prow",
			SubComponentName: "tide",
			Severity:         types.SeverityDown,
			StartTime:        time.Now(),
			Description:      "Admin confirmed outage",
			CreatedBy:        "admin",
			DiscoveredFrom:   "frontend",
			ConfirmedAt:      sql.NullTime{Time: time.Now(), Valid: true},
		}
		err = tm.manager.CreateOutage(confirmed, nil, "admin", "")
		require.NoError(t, err)

		var resolved types.Outage
		err = tm.db.First(&resolved, suspected.ID).Error
		require.NoError(t, err)
		assert.True(t, resolved.EndTime.Valid, "Suspected outage should be resolved")
	})

	t.Run("suspected outage does not self-resolve", func(t *testing.T) {
		tm := setupTestManager(t, baseCfg)
		defer tm.close()

		suspected := &types.Outage{
			ComponentName:    "prow",
			SubComponentName: "tide",
			Severity:         types.SeveritySuspected,
			StartTime:        time.Now(),
			Description:      "Bot-initiated suspected outage",
			CreatedBy:        "chai-bot",
			DiscoveredFrom:   "mcp",
		}
		err := tm.manager.CreateOutage(suspected, nil, "chai-bot", "")
		require.NoError(t, err)

		var created types.Outage
		err = tm.db.First(&created, suspected.ID).Error
		require.NoError(t, err)
		assert.False(t, created.EndTime.Valid, "Suspected outage should NOT be self-resolved")
	})

	t.Run("unconfirmed outage does not resolve suspected", func(t *testing.T) {
		tm := setupTestManager(t, baseCfg)
		defer tm.close()

		suspected := &types.Outage{
			ComponentName:    "prow",
			SubComponentName: "tide",
			Severity:         types.SeveritySuspected,
			StartTime:        time.Now().Add(-1 * time.Hour),
			Description:      "Community reported suspected outage",
			CreatedBy:        "community-user",
			DiscoveredFrom:   "community",
		}
		err := tm.manager.CreateOutage(suspected, nil, "community-user", "")
		require.NoError(t, err)

		unconfirmed := &types.Outage{
			ComponentName:    "prow",
			SubComponentName: "tide",
			Severity:         types.SeverityDown,
			StartTime:        time.Now(),
			Description:      "Component-monitor detected on requires_confirmation sub",
			CreatedBy:        "component-monitor",
			DiscoveredFrom:   "component-monitor",
			// ConfirmedAt NOT set (requires_confirmation sub-component)
		}
		err = tm.manager.CreateOutage(unconfirmed, nil, "component-monitor", "")
		require.NoError(t, err)

		var existing types.Outage
		err = tm.db.First(&existing, suspected.ID).Error
		require.NoError(t, err)
		assert.False(t, existing.EndTime.Valid, "Suspected outage should NOT be resolved by unconfirmed outage")
	})
}

func TestGetStaleSuspectedOutages_IncludesReportlessOutages(t *testing.T) {
	db := setupTestDB(t)
	repo := repositories.NewGORMOutageRepository(db)
	skipHooks := db.Session(&gorm.Session{SkipHooks: true})

	cutoff := time.Now().Add(-24 * time.Hour)

	// Suspected outage with a stale report (older than cutoff)
	withReport := types.Outage{
		ComponentName:    "prow",
		SubComponentName: "tide",
		Severity:         types.SeveritySuspected,
		StartTime:        time.Now().Add(-48 * time.Hour),
		Description:      "Community reported",
		CreatedBy:        "user1",
		DiscoveredFrom:   "community",
	}
	require.NoError(t, skipHooks.Create(&withReport).Error)
	report := types.OutageReport{
		OutageID: withReport.ID,
		User:     "user1",
	}
	require.NoError(t, skipHooks.Create(&report).Error)
	// Backdate the report to before the cutoff
	skipHooks.Model(&report).Update("created_at", time.Now().Add(-36*time.Hour))

	// Suspected outage with NO reports (bot-initiated), started before cutoff
	withoutReport := types.Outage{
		ComponentName:    "prow",
		SubComponentName: "deck",
		Severity:         types.SeveritySuspected,
		StartTime:        time.Now().Add(-48 * time.Hour),
		Description:      "Bot detected suspected outage",
		CreatedBy:        "chai-bot",
		DiscoveredFrom:   "mcp",
	}
	require.NoError(t, skipHooks.Create(&withoutReport).Error)

	// Suspected outage with NO reports but started AFTER cutoff (should NOT be returned)
	recentNoReport := types.Outage{
		ComponentName:    "prow",
		SubComponentName: "crier",
		Severity:         types.SeveritySuspected,
		StartTime:        time.Now().Add(-1 * time.Hour),
		Description:      "Just created, not stale yet",
		CreatedBy:        "chai-bot",
		DiscoveredFrom:   "mcp",
	}
	require.NoError(t, skipHooks.Create(&recentNoReport).Error)

	// Suspected outage with a FRESH report (should NOT be returned)
	freshReport := types.Outage{
		ComponentName:    "prow",
		SubComponentName: "gangway",
		Severity:         types.SeveritySuspected,
		StartTime:        time.Now().Add(-48 * time.Hour),
		Description:      "Active community reports",
		CreatedBy:        "user2",
		DiscoveredFrom:   "community",
	}
	require.NoError(t, skipHooks.Create(&freshReport).Error)
	recentReport := types.OutageReport{
		OutageID: freshReport.ID,
		User:     "user2",
	}
	require.NoError(t, skipHooks.Create(&recentReport).Error)

	// Non-suspected outage (should never be returned)
	nonSuspected := types.Outage{
		ComponentName:    "prow",
		SubComponentName: "sinker",
		Severity:         types.SeverityDown,
		StartTime:        time.Now().Add(-48 * time.Hour),
		Description:      "Real outage",
		CreatedBy:        "admin",
		DiscoveredFrom:   "frontend",
	}
	require.NoError(t, skipHooks.Create(&nonSuspected).Error)

	stale, err := repo.GetStaleSuspectedOutages(cutoff)
	require.NoError(t, err)

	staleIDs := make([]uint, len(stale))
	for i, o := range stale {
		staleIDs[i] = o.ID
	}

	assert.Contains(t, staleIDs, withReport.ID, "Should include suspected with stale report")
	assert.Contains(t, staleIDs, withoutReport.ID, "Should include reportless suspected older than cutoff")
	assert.NotContains(t, staleIDs, recentNoReport.ID, "Should NOT include recent reportless suspected")
	assert.NotContains(t, staleIDs, freshReport.ID, "Should NOT include suspected with fresh report")
	assert.NotContains(t, staleIDs, nonSuspected.ID, "Should NOT include non-suspected outages")
}
