package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type createdOutageExpectation struct {
	Outage      types.Outage
	Reasons     []types.Reason // nil skips reason content comparison
	ReasonCount int            // used when Reasons is nil
	Confirmed   *bool          // when set, asserts ConfirmedAt.Valid
}

type outageExpectations struct {
	created []createdOutageExpectation
	updated []types.Outage
	links   []types.OutageLink
}

func boolPtr(v bool) *bool {
	return &v
}

func outageWithID(id uint, o types.Outage) types.Outage {
	o.ID = id
	return o
}

func assertOutageExpectations(t *testing.T, m *outage.MockOutageManager, exp *outageExpectations) {
	if exp == nil {
		return
	}
	require.Len(t, m.CreatedOutages, len(exp.created))
	for i, want := range exp.created {
		got := m.CreatedOutages[i]
		if diff := cmp.Diff(want.Outage, *got.Outage, testhelper.OutageCompareOptions(want.Outage, false)); diff != "" {
			t.Errorf("created outage[%d] mismatch (-want +got):\n%s", i, diff)
		}
		if want.Confirmed != nil {
			assert.Equal(t, *want.Confirmed, got.Outage.ConfirmedAt.Valid)
		}
		if want.Reasons != nil {
			if diff := cmp.Diff(want.Reasons, got.Reasons, testhelper.ReasonCompareOptions()); diff != "" {
				t.Errorf("created outage[%d] reasons mismatch (-want +got):\n%s", i, diff)
			}
		} else if want.ReasonCount > 0 {
			assert.Len(t, got.Reasons, want.ReasonCount)
		}
	}
	require.Len(t, m.UpdatedOutages, len(exp.updated))
	for i, want := range exp.updated {
		got := m.UpdatedOutages[i]
		compareEnd := testhelper.OutageComparesEndTime(want)
		if diff := cmp.Diff(want, *got, testhelper.OutageCompareOptions(want, compareEnd)); diff != "" {
			t.Errorf("updated outage[%d] mismatch (-want +got):\n%s", i, diff)
		}
	}
	require.Len(t, m.AddedLinks, len(exp.links))
	for i, want := range exp.links {
		got := m.AddedLinks[i]
		if diff := cmp.Diff(want, *got, testhelper.OutageLinkCompareOptions(want)); diff != "" {
			t.Errorf("added link[%d] mismatch (-want +got):\n%s", i, diff)
		}
	}
}

func TestComponentMonitorReportProcessor_Process(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name                   string
		config                 *types.DashboardConfig
		request                *types.ComponentMonitorReportRequest
		setupOutageManager     func(*outage.MockOutageManager)
		wantErr                error
		wantOutages            *outageExpectations
		verifyPingExpectations func(*testing.T, *repositories.MockComponentPingRepository)
	}{
		{
			name:   "healthy status with no active outages",
			config: repositories.TestConfig(true, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				// No initial data needed
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
				ping := pingRepo.UpsertedPings[0]
				assert.Equal(t, "test-component", ping.ComponentSlug)
				assert.Equal(t, "test-subcomponent", ping.SubComponentSlug)
				assert.False(t, ping.Timestamp.IsZero())
			},
		},
		{
			name:   "healthy status with active outages and auto-resolve enabled",
			config: repositories.TestConfig(true, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent", CreatedBy: "test-monitor"},
					{ComponentName: "test-component", SubComponentName: "test-subcomponent", CreatedBy: "test-monitor"},
				}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{
					{EndTime: sql.NullTime{Valid: true}},
					{EndTime: sql.NullTime{Valid: true}},
				},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
				ping := pingRepo.UpsertedPings[0]
				assert.Equal(t, "test-component", ping.ComponentSlug)
				assert.Equal(t, "test-subcomponent", ping.SubComponentSlug)
			},
		},
		{
			name:   "healthy status with active outages and auto-resolve disabled",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
			},
			wantOutages: &outageExpectations{},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status creates new outage without confirmation requirement",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    types.CheckTypePrometheus,
								Check:   "query",
								Results: "error",
							},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				// No initial data needed
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{
					Outage: types.Outage{
						ComponentName: "test-component",
						Severity:      types.SeverityDown,
						ConfirmedAt:   sql.NullTime{Valid: true},
					},
					ReasonCount: 1,
				}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
				ping := pingRepo.UpsertedPings[0]
				assert.Equal(t, "test-component", ping.ComponentSlug)
				assert.Equal(t, "test-subcomponent", ping.SubComponentSlug)
			},
		},
		{
			name:   "unhealthy status creates new outage with confirmation requirement",
			config: repositories.TestConfig(false, true),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    types.CheckTypePrometheus,
								Check:   "query",
								Results: "error",
							},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				// No initial data needed
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{
					ReasonCount: 1,
					Confirmed:   boolPtr(false),
				}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status skips creation when active outage exists",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{
					{
						ComponentName:    "test-component",
						SubComponentName: "test-subcomponent",
						CreatedBy:        "test-monitor",
						Severity:         types.SeverityDown,
						StartTime:        time.Now().Add(-10 * time.Minute),
						DiscoveredFrom:   ComponentMonitor,
					},
				}
			},
			wantOutages: &outageExpectations{},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "repeats reported links onto an existing outage",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReasonWithLink("TRT-1", "First incident", "https://redhat.atlassian.net/browse/TRT-1"),
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{
					{
						ComponentName:    "test-component",
						SubComponentName: "test-subcomponent",
						CreatedBy:        "test-monitor",
						Severity:         types.SeverityDown,
						StartTime:        time.Now().Add(-10 * time.Minute),
						DiscoveredFrom:   ComponentMonitor,
					},
				}
				m.ActiveOutagesCreatedBy[0].ID = 7
			},
			wantOutages: &outageExpectations{
				links: []types.OutageLink{{
					OutageID: 7,
					URL:      "https://redhat.atlassian.net/browse/TRT-1",
					LinkType: types.LinkTypeJira,
				}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status reopens recently-closed outage with matching probe",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "up == 0", Results: "no instances"},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.RecentlyClosedOutages = []types.Outage{
					{
						ComponentName:    "test-component",
						SubComponentName: "test-subcomponent",
						CreatedBy:        "test-monitor",
						Severity:         types.SeverityDegraded,
						EndTime:          sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true},
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus, Check: "up == 0"}},
					},
				}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{{
					EndTime:  sql.NullTime{Valid: false},
					Severity: types.SeverityDown,
				}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status creates new outage when recent outage has no matching probe",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "error_rate > 0.1"},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.RecentlyClosedOutages = []types.Outage{
					{
						ComponentName:    "test-component",
						SubComponentName: "test-subcomponent",
						CreatedBy:        "test-monitor",
						Severity:         types.SeverityDown,
						EndTime:          sql.NullTime{Time: time.Now().Add(-30 * time.Minute), Valid: true},
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus, Check: "up == 0"}},
					},
				}
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status creates new outage when recent outage is outside flap window",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "up == 0"},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				// Simulate the SQL returning nil because no outage falls within the flap window.
				m.FindReopenableOutageFn = func(_, _, _ string, _ time.Time, _ []types.Reason) (*types.Outage, error) {
					return nil, nil
				}
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "component not found returns error",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "nonexistent",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(*outage.MockOutageManager) {},
			wantErr:            errors.New("component not found: nonexistent"),
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Empty(t, pingRepo.UpsertedPings, "ping should not be called when component not found")
			},
		},
		{
			name:   "sub-component not found returns error",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "nonexistent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(*outage.MockOutageManager) {},
			wantErr:            errors.New("sub-component not found: test-component/nonexistent"),
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Empty(t, pingRepo.UpsertedPings, "ping should not be called when sub-component not found")
			},
		},
		{
			name:   "get active outages error returns error",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedByError = errors.New("database error")
			},
			wantErr: errors.New("database error"),
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1, "ping should be called before checking active outages")
			},
		},
		{
			name:   "update outage error continues processing",
			config: repositories.TestConfig(true, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
				m.UpdateOutageFn = func(*types.Outage, string) error {
					return errors.New("update error")
				}
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status creates outage with multiple reasons",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "prometheus",
								Check:   "up{job=\"deck\"} == 0",
								Results: "No healthy instances found",
							},
							{
								Type:    "http",
								Check:   "https://deck.example.com/health",
								Results: "Response time > 5s",
							},
							{
								Type:    "prometheus",
								Check:   "error_rate > 0.1",
								Results: "Error rate exceeded threshold",
							},
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				// No initial data needed
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{
					Outage: types.Outage{
						ComponentName: "test-component",
						Description:   "Component monitor detected outage",
						Severity:      types.SeverityDown,
					},
					ReasonCount: 3,
				}},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "unions reason links onto one outage",
			config: repositories.TestConfig(true, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReasonWithLink("TRT-1", "First incident", "https://redhat.atlassian.net/browse/TRT-1"),
							jiraReasonWithLink("TRT-2", "Second incident", "https://redhat.atlassian.net/browse/TRT-2"),
						},
					},
				},
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}},
				links: []types.OutageLink{
					{OutageID: 1, URL: "https://redhat.atlassian.net/browse/TRT-1"},
					{OutageID: 1, URL: "https://redhat.atlassian.net/browse/TRT-2"},
				},
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pingRepo := &repositories.MockComponentPingRepository{}
			mockOutageManager := &outage.MockOutageManager{}

			configManager := config.CreateTestConfigManager(tt.config)

			if tt.setupOutageManager != nil {
				tt.setupOutageManager(mockOutageManager)
			}

			processor := &ComponentMonitorReportProcessor{
				outageManager: mockOutageManager,
				pingRepo:      pingRepo,
				configManager: configManager,
				logger:        logger,
			}

			err := processor.Process(tt.request)

			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("Process() error mismatch (-want +got):\n%s", diff)
			}

			assertOutageExpectations(t, mockOutageManager, tt.wantOutages)

			if tt.verifyPingExpectations != nil {
				tt.verifyPingExpectations(t, pingRepo)
			}
		})
	}
}

func perReasonTestConfig() *types.DashboardConfig {
	cfg := repositories.TestConfig(true, false)
	cfg.Components[0].Subcomponents[0].Monitoring.OutagePerReason = true
	return cfg
}

func jiraReason(key, summary string) types.Reason {
	return types.Reason{Type: types.CheckTypeJira, Check: key, Results: summary}
}

func jiraReasonWithLink(key, summary, browse string) types.Reason {
	reason := jiraReason(key, summary)
	reason.Links = []types.ReportedLink{{URL: browse, LinkType: types.LinkTypeJira}}
	return reason
}

func jiraOutage(id uint, key, summary string) types.Outage {
	outage := types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-subcomponent",
		CreatedBy:        "test-monitor",
		Description:      summary,
		Reasons:          []types.Reason{jiraReason(key, summary)},
	}
	outage.ID = id
	return outage
}

func TestComponentMonitorReportProcessor_ProcessPerReason(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name               string
		config             *types.DashboardConfig
		request            *types.ComponentMonitorReportRequest
		setupOutageManager func(*outage.MockOutageManager)
		wantOutages        *outageExpectations
	}{
		{
			name:   "two reasons create two outages",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReason("TRT-1", "First incident"),
							jiraReason("TRT-2", "Second incident"),
						},
					},
				},
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{
					{
						Outage:  types.Outage{Description: "First incident"},
						Reasons: []types.Reason{jiraReason("TRT-1", "First incident")},
					},
					{
						Outage:  types.Outage{Description: "Second incident"},
						Reasons: []types.Reason{jiraReason("TRT-2", "Second incident")},
					},
				},
			},
		},
		{
			name:   "dropping one reason resolves only that outage",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{jiraReason("TRT-2", "Second incident")},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				a := jiraOutage(1, "TRT-1", "First incident")
				a.ID = 1
				b := jiraOutage(2, "TRT-2", "Second incident")
				b.ID = 2
				m.ActiveOutagesCreatedBy = []types.Outage{a, b}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{
					outageWithID(1, types.Outage{EndTime: sql.NullTime{Valid: true}}),
				},
			},
		},
		{
			name:   "healthy empty reasons resolve remaining outages",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				a := jiraOutage(1, "TRT-1", "First incident")
				a.ID = 1
				m.ActiveOutagesCreatedBy = []types.Outage{a}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{{
					EndTime: sql.NullTime{Valid: true},
				}},
			},
		},
		{
			name:   "does not duplicate an active reason",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{jiraReason("TRT-1", "First incident")},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				a := jiraOutage(1, "TRT-1", "First incident")
				a.ID = 1
				m.ActiveOutagesCreatedBy = []types.Outage{a}
			},
			wantOutages: &outageExpectations{},
		},
		{
			name:   "repeats reported links onto an existing per-reason outage",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReasonWithLink("TRT-1", "First incident", "https://redhat.atlassian.net/browse/TRT-1"),
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.ActiveOutagesCreatedBy = []types.Outage{jiraOutage(1, "TRT-1", "First incident")}
			},
			wantOutages: &outageExpectations{
				links: []types.OutageLink{{
					OutageID: 1,
					URL:      "https://redhat.atlassian.net/browse/TRT-1",
					LinkType: types.LinkTypeJira,
				}},
			},
		},
		{
			name:   "does not re-add an existing reported link",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReasonWithLink("TRT-1", "First incident", "https://redhat.atlassian.net/browse/TRT-1"),
						},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				a := jiraOutage(1, "TRT-1", "First incident")
				a.Links = []types.OutageLink{{
					OutageID: 1,
					URL:      "https://redhat.atlassian.net/browse/TRT-1",
					LinkType: types.LinkTypeJira,
				}}
				m.ActiveOutagesCreatedBy = []types.Outage{a}
			},
			wantOutages: &outageExpectations{},
		},
		{
			name:   "updates description when summary changes",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{jiraReason("TRT-1", "Updated summary")},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				a := jiraOutage(1, "TRT-1", "Old summary")
				a.ID = 1
				m.ActiveOutagesCreatedBy = []types.Outage{a}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{{
					Description: "Updated summary",
					EndTime:     sql.NullTime{Valid: false},
				}},
			},
		},
		{
			name:   "reopens flap-window outage by type and check",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{jiraReason("TRT-1", "First incident")},
					},
				},
			},
			setupOutageManager: func(m *outage.MockOutageManager) {
				closed := jiraOutage(7, "TRT-1", "First incident")
				closed.ID = 7
				closed.EndTime = sql.NullTime{Time: time.Now().Add(-10 * time.Minute), Valid: true}
				m.RecentlyClosedOutages = []types.Outage{closed}
			},
			wantOutages: &outageExpectations{
				updated: []types.Outage{
					outageWithID(7, types.Outage{EndTime: sql.NullTime{Valid: false}}),
				},
			},
		},
		{
			name:   "two reasons with links attach one link per outage",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							jiraReasonWithLink("TRT-1", "First incident", "https://redhat.atlassian.net/browse/TRT-1"),
							jiraReasonWithLink("TRT-2", "Second incident", "https://redhat.atlassian.net/browse/TRT-2"),
						},
					},
				},
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}, {}},
				links: []types.OutageLink{
					{OutageID: 1, URL: "https://redhat.atlassian.net/browse/TRT-1", LinkType: types.LinkTypeJira},
					{OutageID: 2, URL: "https://redhat.atlassian.net/browse/TRT-2", LinkType: types.LinkTypeJira},
				},
			},
		},
		{
			name:   "invalid reported link is skipped",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    types.CheckTypeJira,
								Check:   "TRT-1",
								Results: "First incident",
								Links: []types.ReportedLink{
									{URL: "not-a-url", LinkType: types.LinkTypeJira},
									{URL: "https://redhat.atlassian.net/browse/TRT-1", LinkType: types.LinkTypeJira},
								},
							},
						},
					},
				},
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}},
				links: []types.OutageLink{{
					OutageID: 1,
					URL:      "https://redhat.atlassian.net/browse/TRT-1",
				}},
			},
		},
		{
			name:   "empty link type defaults to other",
			config: perReasonTestConfig(),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    types.CheckTypeJira,
								Check:   "TRT-1",
								Results: "First incident",
								Links:   []types.ReportedLink{{URL: "https://example.com/runbook"}},
							},
						},
					},
				},
			},
			wantOutages: &outageExpectations{
				created: []createdOutageExpectation{{}},
				links: []types.OutageLink{{
					OutageID: 1,
					URL:      "https://example.com/runbook",
					LinkType: types.LinkTypeOther,
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pingRepo := &repositories.MockComponentPingRepository{}
			mockOutageManager := &outage.MockOutageManager{}
			if tt.setupOutageManager != nil {
				tt.setupOutageManager(mockOutageManager)
			}
			processor := &ComponentMonitorReportProcessor{
				outageManager: mockOutageManager,
				pingRepo:      pingRepo,
				configManager: config.CreateTestConfigManager(tt.config),
				logger:        logger,
			}
			err := processor.Process(tt.request)
			assert.NoError(t, err)
			assert.Len(t, pingRepo.UpsertedPings, 1)
			assertOutageExpectations(t, mockOutageManager, tt.wantOutages)
		})
	}
}

func TestComponentMonitorReportProcessor_ValidateRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name           string
		config         *types.DashboardConfig
		request        *types.ComponentMonitorReportRequest
		serviceAccount string
		wantErr        error
	}{
		{
			name:   "valid request",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
					},
				},
			},
			serviceAccount: "test-sa",
		},
		{
			name:   "component not found",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "nonexistent",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "test-sa",
			wantErr:        errors.New("component not found: nonexistent"),
		},
		{
			name:   "service account not an owner",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "unauthorized-sa",
			wantErr:        errors.New("service account unauthorized-sa is not an owner of component test-component"),
		},
		{
			name:   "sub-component not found",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "nonexistent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "test-sa",
			wantErr:        errors.New("sub-component not found: test-component/nonexistent"),
		},
		{
			name:   "wrong component monitor source",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "wrong-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "test-sa",
			wantErr:        errors.New("improper component monitor source: wrong-monitor for: test-component/test-subcomponent"),
		},
		{
			name:   "multiple errors aggregated",
			config: repositories.TestConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "nonexistent",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
					},
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "nonexistent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "test-sa",
			wantErr:        errors.New("[component not found: nonexistent, sub-component not found: test-component/nonexistent]"),
		},
		{
			name: "owner with only rover group is not service account owner",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug:       "test-subcomponent",
								Monitoring: &types.Monitoring{ComponentMonitor: "test-monitor"},
							},
						},
						Owners: []types.Owner{
							{RoverGroup: "some-rover-group"},
						},
					},
				},
			},
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
					},
				},
			},
			serviceAccount: "test-sa",
			wantErr:        errors.New("service account test-sa is not an owner of component test-component"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configManager := config.CreateTestConfigManager(tt.config)
			// ValidateRequest doesn't use OutageManager, so we can use a nil mock
			mockOutageManager := &outage.MockOutageManager{}
			processor := &ComponentMonitorReportProcessor{
				outageManager: mockOutageManager,
				pingRepo:      &repositories.MockComponentPingRepository{},
				configManager: configManager,
				logger:        logger,
			}

			err := processor.ValidateRequest(tt.request, tt.serviceAccount)

			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("validateRequest() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewReasons(t *testing.T) {
	prometheus := types.Reason{Type: types.CheckTypePrometheus, Check: "up == 0"}
	http := types.Reason{Type: types.CheckTypeHTTP, Check: "status != 200"}
	systemd := types.Reason{Type: types.CheckTypeSystemd, Check: "service-down"}

	tests := []struct {
		name     string
		existing []types.Reason
		incoming []types.Reason
		want     []types.Reason
	}{
		{
			name:     "probe has reason not in outage",
			existing: []types.Reason{prometheus},
			incoming: []types.Reason{http},
			want:     []types.Reason{http},
		},
		{
			name:     "probe has same reasons as outage",
			existing: []types.Reason{prometheus, http},
			incoming: []types.Reason{prometheus, http},
			want:     nil,
		},
		{
			name:     "probe has both matching and new reasons",
			existing: []types.Reason{prometheus},
			incoming: []types.Reason{prometheus, http},
			want:     []types.Reason{http},
		},
		{
			name:     "outage has no existing reasons",
			existing: nil,
			incoming: []types.Reason{prometheus, http},
			want:     []types.Reason{prometheus, http},
		},
		{
			name:     "probe has no reasons",
			existing: []types.Reason{prometheus},
			incoming: nil,
			want:     nil,
		},
		{
			name:     "probe has multiple reasons not in outage",
			existing: []types.Reason{prometheus},
			incoming: []types.Reason{http, systemd},
			want:     []types.Reason{http, systemd},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newReasons(tt.existing, tt.incoming)
			assert.Equal(t, tt.want, got)
		})
	}
}

type resolvedOutage struct {
	ID       uint
	Resolver string
}

type discoveredFromQuery struct {
	ComponentSlug    string
	SubComponentSlug string
	DiscoveredFrom   string
}

func absentReportOutage(id uint, createdBy string) types.Outage {
	o := types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-subcomponent",
		Severity:         types.SeverityDown,
		StartTime:        time.Now().Add(-30 * time.Minute),
		DiscoveredFrom:   AbsentReportSource,
		CreatedBy:        createdBy,
	}
	o.ID = id
	return o
}

func serviceHealthOutage(id uint) types.Outage {
	o := types.Outage{
		ComponentName:    "test-component",
		SubComponentName: "test-subcomponent",
		Severity:         types.SeverityDown,
		StartTime:        time.Now().Add(-10 * time.Minute),
		DiscoveredFrom:   ComponentMonitor,
		CreatedBy:        "test-monitor",
	}
	o.ID = id
	return o
}

func singleStatusReport(status types.Status, reasons ...types.Reason) *types.ComponentMonitorReportRequest {
	return &types.ComponentMonitorReportRequest{
		ComponentMonitor: "test-monitor",
		Statuses: []types.ComponentMonitorReportComponentStatus{
			{
				ComponentSlug:    "test-component",
				SubComponentSlug: "test-subcomponent",
				Status:           status,
				Reasons:          reasons,
			},
		},
	}
}

func TestComponentMonitorReportProcessor_ResolvesAbsentReportOutages(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	downReason := types.Reason{Type: types.CheckTypePrometheus, Check: "query", Results: "error"}
	matchingQuery := discoveredFromQuery{"test-component", "test-subcomponent", AbsentReportSource}

	tests := []struct {
		name              string
		config            *types.DashboardConfig
		request           *types.ComponentMonitorReportRequest
		absentOutages     []types.Outage
		absentQueryErr    error
		serviceOutages    []types.Outage
		wantResolved      []resolvedOutage
		wantCreatedCount  int
		wantAbsentQueries []discoveredFromQuery
	}{
		{
			name:              "healthy report resolves absent-report outage",
			config:            repositories.TestConfig(true, false),
			request:           singleStatusReport(types.StatusHealthy, types.Reason{Type: types.CheckTypePrometheus}),
			absentOutages:     []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			wantResolved:      []resolvedOutage{{ID: 10, Resolver: "test-monitor"}},
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:              "unhealthy report resolves absent-report outage and opens service-health outage",
			config:            repositories.TestConfig(true, false),
			request:           singleStatusReport(types.StatusDown, downReason),
			absentOutages:     []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			wantResolved:      []resolvedOutage{{ID: 10, Resolver: "test-monitor"}},
			wantCreatedCount:  1,
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:              "unhealthy report resolves absent-report outage and keeps existing service-health outage",
			config:            repositories.TestConfig(true, false),
			request:           singleStatusReport(types.StatusDown, downReason),
			absentOutages:     []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			serviceOutages:    []types.Outage{serviceHealthOutage(20)},
			wantResolved:      []resolvedOutage{{ID: 10, Resolver: "test-monitor"}},
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:           "healthy report resolves absent-report and service-health outages independently",
			config:         repositories.TestConfig(true, false),
			request:        singleStatusReport(types.StatusHealthy, types.Reason{Type: types.CheckTypePrometheus}),
			absentOutages:  []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			serviceOutages: []types.Outage{serviceHealthOutage(20)},
			wantResolved: []resolvedOutage{
				{ID: 10, Resolver: "test-monitor"},
				{ID: 20, Resolver: "test-monitor"},
			},
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:          "auto-resolve disabled leaves absent-report outage active on healthy report",
			config:        repositories.TestConfig(false, false),
			request:       singleStatusReport(types.StatusHealthy, types.Reason{Type: types.CheckTypePrometheus}),
			absentOutages: []types.Outage{absentReportOutage(10, AbsentReportCreator)},
		},
		{
			name:             "auto-resolve disabled leaves absent-report outage active on unhealthy report",
			config:           repositories.TestConfig(false, false),
			request:          singleStatusReport(types.StatusDown, downReason),
			absentOutages:    []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			wantCreatedCount: 1,
		},
		{
			name:    "only resolves absent-report outages created by the dashboard",
			config:  repositories.TestConfig(true, false),
			request: singleStatusReport(types.StatusHealthy, types.Reason{Type: types.CheckTypePrometheus}),
			absentOutages: []types.Outage{
				absentReportOutage(10, AbsentReportCreator),
				absentReportOutage(11, "some-user"),
			},
			wantResolved:      []resolvedOutage{{ID: 10, Resolver: "test-monitor"}},
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:              "outage-per-reason report resolves absent-report outage",
			config:            perReasonTestConfig(),
			request:           singleStatusReport(types.StatusDown, jiraReason("TRT-1", "First incident")),
			absentOutages:     []types.Outage{absentReportOutage(10, AbsentReportCreator)},
			wantResolved:      []resolvedOutage{{ID: 10, Resolver: "test-monitor"}},
			wantCreatedCount:  1,
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
		{
			name:              "absent-report query failure does not block service-health processing",
			config:            repositories.TestConfig(true, false),
			request:           singleStatusReport(types.StatusDown, downReason),
			absentQueryErr:    errors.New("db unavailable"),
			wantCreatedCount:  1,
			wantAbsentQueries: []discoveredFromQuery{matchingQuery},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pingRepo := &repositories.MockComponentPingRepository{}
			var resolved []resolvedOutage
			var absentQueries []discoveredFromQuery
			m := &outage.MockOutageManager{
				ActiveOutagesCreatedBy: tt.serviceOutages,
				GetActiveOutagesDiscoveredFromFn: func(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
					absentQueries = append(absentQueries, discoveredFromQuery{componentSlug, subComponentSlug, discoveredFrom})
					if tt.absentQueryErr != nil {
						return nil, tt.absentQueryErr
					}
					var matches []types.Outage
					for _, o := range tt.absentOutages {
						if o.ComponentName == componentSlug && o.SubComponentName == subComponentSlug && o.DiscoveredFrom == discoveredFrom {
							matches = append(matches, o)
						}
					}
					return matches, nil
				},
				UpdateOutageFn: func(o *types.Outage, user string) error {
					if o.EndTime.Valid {
						resolved = append(resolved, resolvedOutage{ID: o.ID, Resolver: user})
					}
					return nil
				},
			}

			processor := NewComponentMonitorReportProcessor(m, pingRepo, config.CreateTestConfigManager(tt.config), logger)
			require.NoError(t, processor.Process(tt.request))

			if diff := cmp.Diff(tt.wantResolved, resolved); diff != "" {
				t.Errorf("resolved outages mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantAbsentQueries, absentQueries); diff != "" {
				t.Errorf("absent-report queries mismatch (-want +got):\n%s", diff)
			}
			assert.Len(t, m.CreatedOutages, tt.wantCreatedCount)
			assert.Len(t, pingRepo.UpsertedPings, 1)
		})
	}
}

// Reproduces TRT-3104: reports that resume while the absent-report checker is still in its
// startup delay must clear the absence outage without waiting for the checker.
func TestComponentMonitorReportProcessor_ResolvesAbsentReportOutageDuringCheckerStartupDelay(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	configManager := config.CreateTestConfigManager(repositories.TestConfig(true, false))
	pingRepo := &repositories.MockComponentPingRepository{}
	absent := absentReportOutage(10, AbsentReportCreator)
	m := &outage.MockOutageManager{
		GetActiveOutagesDiscoveredFromFn: func(_, _, _ string) ([]types.Outage, error) {
			return []types.Outage{absent}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	checkerDone := make(chan struct{})
	checker := NewAbsentMonitoredComponentReportChecker(configManager, m, pingRepo, time.Hour, logger)
	go func() {
		defer close(checkerDone)
		checker.Start(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-checkerDone
	})

	processor := NewComponentMonitorReportProcessor(m, pingRepo, configManager, logger)
	require.NoError(t, processor.Process(singleStatusReport(types.StatusHealthy, types.Reason{Type: types.CheckTypePrometheus})))

	require.Len(t, m.UpdatedOutages, 1)
	assert.Equal(t, uint(10), m.UpdatedOutages[0].ID)
	assert.True(t, m.UpdatedOutages[0].EndTime.Valid, "absent-report outage should be resolved by the report itself")
}
