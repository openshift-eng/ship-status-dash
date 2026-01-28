package outage

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
)

type testManager struct {
	manager    *OutageManager
	repo       *repositories.MockOutageRepository
	mockServer *MockSlackServer
}

func setupTestManager(t *testing.T, cfg *types.DashboardConfig, repo *repositories.MockOutageRepository, slackThreadRepo *repositories.MockSlackThreadRepository) *testManager {
	if repo == nil {
		repo = &repositories.MockOutageRepository{}
	}
	if slackThreadRepo == nil {
		slackThreadRepo = &repositories.MockSlackThreadRepository{}
	}

	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}
	cfgManager.Get()

	mockServer := NewMockSlackServer(t)
	slackClient := mockServer.Client()

	manager := NewOutageManager(repo, slackThreadRepo, slackClient, cfgManager, "https://test.example.com/", "https://rhsandbox.slack.com/", logrus.New())

	return &testManager{
		manager:    manager,
		repo:       repo,
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
		name               string
		outage             *types.Outage
		config             *types.DashboardConfig
		wantCreatedOutages []*types.Outage
		wantSlackMessages  []PostedMessage
	}{
		{
			name: "successful creation with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
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
			wantCreatedOutages: []*types.Outage{
				{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
					StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
					CreatedBy:        "system",
					DiscoveredFrom:   "component-monitor",
				},
			},
			wantSlackMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "🚨 Outage Detected: Test Component/Test Sub\n\nSeverity: `Down`\nStarted: `2024-01-15T10:30:00Z`\nCreated by: `system`\nDiscovered from: `component-monitor`\n\n<https://test.example.com/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tm := setupTestManager(t, tt.config, nil, nil)
			defer tm.close()

			err := tm.manager.CreateOutage(tt.outage)
			if err != nil {
				t.Fatalf("Failed to create outage: %v", err)
			}

			if diff := cmp.Diff(tt.wantCreatedOutages, tm.repo.CreatedOutages); diff != "" {
				t.Errorf("Created outages mismatch (-want +got):\n%s", diff)
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)
		})
	}
}

func TestOutageManager_UpdateOutage(t *testing.T) {
	tests := []struct {
		name              string
		outage            *types.Outage
		oldOutage         *types.Outage
		config            *types.DashboardConfig
		slackThreadRepo   *repositories.MockSlackThreadRepository
		wantSavedOutages  []*types.Outage
		wantSlackMessages []PostedMessage
	}{
		{
			name: "successful update with slack reporting",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
			},
			oldOutage: &types.Outage{
				Severity: types.SeverityDegraded,
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
			wantSavedOutages: []*types.Outage{
				{
					Model:            gorm.Model{ID: 1},
					ComponentName:    "test-component",
					SubComponentName: "test-sub",
					Severity:         types.SeverityDown,
				},
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
			repo := &repositories.MockOutageRepository{
				OutageByID: tt.oldOutage,
			}

			tm := setupTestManager(t, tt.config, repo, tt.slackThreadRepo)
			defer tm.close()

			err := tm.manager.UpdateOutage(tt.outage)
			if err != nil {
				t.Fatalf("Failed to update outage: %v", err)
			}

			if diff := cmp.Diff(tt.wantSavedOutages, tm.repo.SavedOutages); diff != "" {
				t.Errorf("Saved outages mismatch (-want +got):\n%s", diff)
			}

			assertSlackMessages(t, tm.mockServer, tt.wantSlackMessages)
		})
	}
}
