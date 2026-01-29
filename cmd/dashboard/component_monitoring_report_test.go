package main

import (
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
)

func TestComponentMonitorReportProcessor_Process(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name                     string
		config                   *types.DashboardConfig
		request                  *types.ComponentMonitorReportRequest
		setupOutageManager       func(*outage.MockOutageManager)
		wantErr                  error
		verifyOutageExpectations func(*testing.T, *outage.MockOutageManager)
		verifyPingExpectations   func(*testing.T, *repositories.MockComponentPingRepository)
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Len(t, m.UpdatedOutages, 2, "Should update 2 outages")
				assert.Empty(t, m.CreatedOutages, "Should not create new outages")
				for _, outage := range m.UpdatedOutages {
					assert.True(t, outage.EndTime.Valid)
					assert.Equal(t, "test-monitor", *outage.ResolvedBy)
				}
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Empty(t, m.UpdatedOutages, "No outages should be updated")
				assert.Empty(t, m.CreatedOutages, "No new outages should be created")
			},
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Len(t, m.CreatedOutages, 1)
				created := m.CreatedOutages[0]
				assert.Len(t, created.Reasons, 1)
				assert.Equal(t, types.CheckTypePrometheus, created.Reasons[0].Type)
				assert.Equal(t, "test-component", created.Outage.ComponentName)
				assert.Equal(t, types.SeverityDown, created.Outage.Severity)
				assert.Equal(t, "test-monitor", *created.Outage.ConfirmedBy)
				assert.True(t, created.Outage.ConfirmedAt.Valid)
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Len(t, m.CreatedOutages, 1)
				created := m.CreatedOutages[0]
				assert.Len(t, created.Reasons, 1)
				assert.Nil(t, created.Outage.ConfirmedBy)
				assert.False(t, created.Outage.ConfirmedAt.Valid)
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Empty(t, m.CreatedOutages, "Should not create new outage")
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
				m.UpdateOutageFn = func(*types.Outage) error {
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
			verifyOutageExpectations: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Len(t, m.CreatedOutages, 1)
				created := m.CreatedOutages[0]
				assert.Equal(t, "test-component", created.Outage.ComponentName)
				assert.Equal(t, types.SeverityDown, created.Outage.Severity)
				assert.Len(t, created.Reasons, 3, "Should create all three reasons")
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

			if tt.verifyOutageExpectations != nil {
				tt.verifyOutageExpectations(t, mockOutageManager)
			}

			if tt.verifyPingExpectations != nil {
				tt.verifyPingExpectations(t, pingRepo)
			}
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
