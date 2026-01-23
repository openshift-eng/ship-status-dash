package outage

import (
	"database/sql"
	"errors"
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

func severityPtr(s types.Severity) *types.Severity {
	return &s
}

func stringPtr(s string) *string {
	return &s
}

func TestGetEffectiveSeverity(t *testing.T) {
	tests := []struct {
		name   string
		config types.SlackReportingConfig
		want   types.Severity
	}{
		{
			name: "severity defined",
			config: types.SlackReportingConfig{
				Channel:  "#test",
				Severity: severityPtr(types.SeverityDown),
			},
			want: types.SeverityDown,
		},
		{
			name: "severity nil defaults to Suspected",
			config: types.SlackReportingConfig{
				Channel:  "#test",
				Severity: nil,
			},
			want: types.SeveritySuspected,
		},
		{
			name: "severity empty string defaults to Suspected",
			config: types.SlackReportingConfig{
				Channel:  "#test",
				Severity: severityPtr(types.Severity("")),
			},
			want: types.SeveritySuspected,
		},
		{
			name: "severity Degraded",
			config: types.SlackReportingConfig{
				Channel:  "#test",
				Severity: severityPtr(types.SeverityDegraded),
			},
			want: types.SeverityDegraded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getEffectiveSeverity(tt.config)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getEffectiveSeverity mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFilterChannelsBySeverity(t *testing.T) {
	tests := []struct {
		name           string
		reporting      []types.SlackReportingConfig
		outageSeverity types.Severity
		want           []string
	}{
		{
			name: "all channels pass when no severity threshold",
			reporting: []types.SlackReportingConfig{
				{Channel: "#channel1", Severity: nil},
				{Channel: "#channel2", Severity: nil},
			},
			outageSeverity: types.SeveritySuspected,
			want:           []string{"#channel1", "#channel2"},
		},
		{
			name: "channels filtered by severity threshold",
			reporting: []types.SlackReportingConfig{
				{Channel: "#all", Severity: nil},
				{Channel: "#degraded", Severity: severityPtr(types.SeverityDegraded)},
				{Channel: "#down", Severity: severityPtr(types.SeverityDown)},
			},
			outageSeverity: types.SeveritySuspected,
			want:           []string{"#all"},
		},
		{
			name: "channels filtered by severity threshold - Degraded outage",
			reporting: []types.SlackReportingConfig{
				{Channel: "#all", Severity: nil},
				{Channel: "#degraded", Severity: severityPtr(types.SeverityDegraded)},
				{Channel: "#down", Severity: severityPtr(types.SeverityDown)},
			},
			outageSeverity: types.SeverityDegraded,
			want:           []string{"#all", "#degraded"},
		},
		{
			name: "channels filtered by severity threshold - Down outage",
			reporting: []types.SlackReportingConfig{
				{Channel: "#all", Severity: nil},
				{Channel: "#degraded", Severity: severityPtr(types.SeverityDegraded)},
				{Channel: "#down", Severity: severityPtr(types.SeverityDown)},
			},
			outageSeverity: types.SeverityDown,
			want:           []string{"#all", "#degraded", "#down"},
		},
		{
			name:           "empty reporting config",
			reporting:      []types.SlackReportingConfig{},
			outageSeverity: types.SeverityDown,
			want:           nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterChannelsBySeverity(tt.reporting, tt.outageSeverity)
			if diff := cmp.Diff(tt.want, got, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("filterChannelsBySeverity mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSlackReporter_BuildOutageLink(t *testing.T) {
	tests := []struct {
		name    string
		outage  *types.Outage
		baseURL string
		want    string
	}{
		{
			name: "simple component and subcomponent",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 123},
				ComponentName:    "Test Component",
				SubComponentName: "Test Sub",
			},
			baseURL: "https://ship-status.ci.openshift.org/",
			want:    "https://ship-status.ci.openshift.org/test-component/test-sub/outages/123",
		},
		{
			name: "component with spaces",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 456},
				ComponentName:    "Build Farm",
				SubComponentName: "CI System",
			},
			baseURL: "https://ship-status.ci.openshift.org/",
			want:    "https://ship-status.ci.openshift.org/build-farm/ci-system/outages/456",
		},
		{
			name: "baseURL without trailing slash",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 789},
				ComponentName:    "Prow",
				SubComponentName: "Deck",
			},
			baseURL: "http://localhost:8080",
			want:    "http://localhost:8080/prow/deck/outages/789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSlackReporter(nil, nil, nil, tt.baseURL, "", nil)
			got := r.buildOutageLink(tt.outage)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("buildOutageLink mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSlackReporter_GetSlackReportingForSubComponent(t *testing.T) {
	tests := []struct {
		name             string
		componentSlug    string
		subComponentSlug string
		config           *types.DashboardConfig
		want             []types.SlackReportingConfig
	}{
		{
			name:             "subcomponent has own config",
			componentSlug:    "test-component",
			subComponentSlug: "test-sub",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						SlackReporting: []types.SlackReportingConfig{
							{Channel: "#component-channel"},
						},
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-sub",
								SlackReporting: []types.SlackReportingConfig{
									{Channel: "#sub-channel"},
								},
							},
						},
					},
				},
			},
			want: []types.SlackReportingConfig{
				{Channel: "#sub-channel"},
			},
		},
		{
			name:             "subcomponent uses component config",
			componentSlug:    "test-component",
			subComponentSlug: "test-sub",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						SlackReporting: []types.SlackReportingConfig{
							{Channel: "#component-channel"},
						},
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-sub",
							},
						},
					},
				},
			},
			want: []types.SlackReportingConfig{
				{Channel: "#component-channel"},
			},
		},
		{
			name:             "component not found",
			componentSlug:    "missing-component",
			subComponentSlug: "test-sub",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
					},
				},
			},
			want: nil,
		},
		{
			name:             "subcomponent not found",
			componentSlug:    "test-component",
			subComponentSlug: "missing-sub",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub"},
						},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
				return tt.config, nil
			}, logrus.New(), time.Second)
			if err != nil {
				t.Fatalf("Failed to create config manager: %v", err)
			}
			cfgManager.Get() // trigger load

			r := &SlackReporter{
				configManager: cfgManager,
			}
			got := r.getSlackReportingForSubComponent(tt.componentSlug, tt.subComponentSlug)
			if diff := cmp.Diff(tt.want, got, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("getSlackReportingForSubComponent mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSlackReporter_ReportOutage(t *testing.T) {
	tests := []struct {
		name            string
		outage          *types.Outage
		config          *types.DashboardConfig
		slackThreadRepo *repositories.MockSlackThreadRepository
		wantErr         error
		wantMessages    []PostedMessage
		wantReactions   []AddedReaction
	}{
		{
			name: "successful report - with slack reporting config",
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
			slackThreadRepo: &repositories.MockSlackThreadRepository{},
			wantErr:         nil,
			wantMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "🚨 Outage Detected: Test Component/Test Sub\n\nSeverity: `Down`\nStarted: `2024-01-15T10:30:00Z`\nCreated by: `system`\nDiscovered from: `component-monitor`\n\n<https://ship-status.ci.openshift.org/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
		{
			name: "no slack reporting config",
			outage: &types.Outage{
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
			},
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{Slug: "test-sub"},
						},
					},
				},
			},
			slackThreadRepo: &repositories.MockSlackThreadRepository{},
			wantErr:         nil,
			wantMessages:    []PostedMessage{},
		},
		{
			name: "outage created as already resolved",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 2},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				StartTime:        time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				EndTime: sql.NullTime{
					Time:  time.Date(2024, 1, 15, 10, 35, 0, 0, time.UTC),
					Valid: true,
				},
				ResolvedBy:     stringPtr("test-user"),
				CreatedBy:      "system",
				DiscoveredFrom: "component-monitor",
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
			slackThreadRepo: &repositories.MockSlackThreadRepository{},
			wantErr:         nil,
			wantMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "🚨 Outage Detected: Test Component/Test Sub\n\nSeverity: `Down`\nStarted: `2024-01-15T10:30:00Z`\nCreated by: `system`\nDiscovered from: `component-monitor`\n\n<https://ship-status.ci.openshift.org/test-component/test-sub/outages/2|View Outage>",
					ThreadTimestamp: "",
					ResponseTS:      "1234567890.000001",
				},
			},
			wantReactions: []AddedReaction{
				{
					Channel:   "#test-channel",
					Timestamp: "1234567890.000001",
					Name:      "outage_resolved",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
				return tt.config, nil
			}, logrus.New(), time.Second)
			if err != nil {
				t.Fatalf("Failed to create config manager: %v", err)
			}
			cfgManager.Get() // trigger load

			mockServer := NewMockSlackServer(t)
			defer mockServer.Close()
			slackClient := mockServer.Client()

			r := NewSlackReporter(
				slackClient,
				tt.slackThreadRepo,
				cfgManager,
				"https://ship-status.ci.openshift.org/",
				"https://rhsandbox.slack.com/",
				logrus.New(),
			)

			err = r.ReportOutage(tt.outage)
			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("ReportOutage error mismatch (-want +got):\n%s", diff)
			}

			postedMsgs := mockServer.PostedMessages()
			if diff := cmp.Diff(tt.wantMessages, postedMsgs, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("Posted messages mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(len(tt.wantMessages), len(tt.slackThreadRepo.CreatedThreads)); diff != "" {
				t.Errorf("Created threads count mismatch (-want +got):\n%s", diff)
			}

			addedReactions := mockServer.AddedReactions()
			if diff := cmp.Diff(tt.wantReactions, addedReactions, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("Added reactions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSlackReporter_ReportOutageUpdate(t *testing.T) {
	tests := []struct {
		name            string
		outage          *types.Outage
		oldOutage       *types.Outage
		slackThreadRepo *repositories.MockSlackThreadRepository
		wantErr         error
		wantMessages    []PostedMessage
		wantReactions   []AddedReaction
	}{
		{
			name: "successful update - replies to existing thread",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
			},
			oldOutage: &types.Outage{
				Severity: types.SeverityDegraded,
			},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				ThreadsForOutage: []types.SlackThread{
					{
						Channel:         "#test-channel",
						ChannelID:       "C1234567890",
						ThreadTimestamp: "1234567890.123456",
					},
				},
			},
			wantMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            "📝 Outage Updated: Test Component/Test Sub (#1)\n\nSeverity changed: `Degraded` → `Down`\n\n<https://ship-status.ci.openshift.org/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "1234567890.123456",
					ResponseTS:      "1234567890.000001",
				},
			},
		},
		{
			name: "no threads found",
			outage: &types.Outage{
				Model: gorm.Model{ID: 1},
			},
			oldOutage: &types.Outage{},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				ThreadsForOutage: []types.SlackThread{},
			},
			wantMessages:  []PostedMessage{},
			wantReactions: []AddedReaction{},
		},
		{
			name: "error getting threads",
			outage: &types.Outage{
				Model: gorm.Model{ID: 1},
			},
			oldOutage: &types.Outage{},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				GetThreadsError: errors.New("database error"),
			},
			wantErr: errors.New("database error"),
		},
		{
			name: "outage resolved",
			outage: &types.Outage{
				Model:            gorm.Model{ID: 1},
				ComponentName:    "test-component",
				SubComponentName: "test-sub",
				Severity:         types.SeverityDown,
				EndTime: sql.NullTime{
					Time:  time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
					Valid: true,
				},
				ResolvedBy: stringPtr("test-user"),
			},
			oldOutage: &types.Outage{
				Severity: types.SeverityDown,
				EndTime:  sql.NullTime{Valid: false},
			},
			slackThreadRepo: &repositories.MockSlackThreadRepository{
				ThreadsForOutage: []types.SlackThread{
					{
						Channel:         "#test-channel",
						ChannelID:       "C1234567890",
						ThreadTimestamp: "1234567890.123456",
					},
				},
			},
			wantMessages: []PostedMessage{
				{
					Channel:         "#test-channel",
					Text:            ":outage_resolved: Outage Updated: Test Component/Test Sub (#1)\n\nResolved: by `test-user` at `2024-01-15T11:00:00Z`\n\n<https://ship-status.ci.openshift.org/test-component/test-sub/outages/1|View Outage>",
					ThreadTimestamp: "1234567890.123456",
					ResponseTS:      "1234567890.000001",
				},
			},
			wantReactions: []AddedReaction{
				{
					Channel:   "C1234567890",
					Timestamp: "1234567890.123456",
					Name:      "outage_resolved",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
				return &types.DashboardConfig{
					Components: []*types.Component{
						{
							Slug: "test-component",
							Name: "Test Component",
							Subcomponents: []types.SubComponent{
								{Slug: "test-sub", Name: "Test Sub"},
							},
						},
					},
				}, nil
			}, logrus.New(), time.Second)
			if err != nil {
				t.Fatalf("Failed to create config manager: %v", err)
			}
			cfgManager.Get() // trigger load

			mockServer := NewMockSlackServer(t)
			defer mockServer.Close()
			slackClient := mockServer.Client()

			r := NewSlackReporter(
				slackClient,
				tt.slackThreadRepo,
				cfgManager,
				"https://ship-status.ci.openshift.org/",
				"https://rhsandbox.slack.com/",
				logrus.New(),
			)

			err = r.ReportOutageUpdate(tt.outage, tt.oldOutage)
			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("ReportOutageUpdate error mismatch (-want +got):\n%s", diff)
			}

			postedMsgs := mockServer.PostedMessages()
			if diff := cmp.Diff(tt.wantMessages, postedMsgs, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("Posted messages mismatch (-want +got):\n%s", diff)
			}

			addedReactions := mockServer.AddedReactions()
			if diff := cmp.Diff(tt.wantReactions, addedReactions, testhelper.EquateNilEmpty); diff != "" {
				t.Errorf("Added reactions mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
