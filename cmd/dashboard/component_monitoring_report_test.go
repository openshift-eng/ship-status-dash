package main

import (
	"errors"
	"testing"
	"time"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// mockOutageRepository is a mock implementation of OutageRepository for testing.
type mockOutageRepository struct {
	activeOutages      []types.Outage
	activeOutagesError error
	saveOutageError    error
	createReasonError  error
	createOutageError  error
	transactionError   error
	createReasonFn     func(*types.Reason)
	createOutageFn     func(*types.Outage)
	saveOutageFn       func(*types.Outage)
	transactionFn      func(func(OutageRepository) error) error
	// Captured data for assertions
	savedOutages   []*types.Outage
	createdReasons []*types.Reason
	createdOutages []*types.Outage
	saveCount      int
}

// mockComponentPingRepository is a mock implementation of ComponentPingRepository for testing.
type mockComponentPingRepository struct {
	upsertError error
	upsertFn    func(string, string, time.Time)
	// Captured data for assertions
	upsertedPings []struct {
		componentSlug    string
		subComponentSlug string
		timestamp        time.Time
	}
}

func (m *mockOutageRepository) GetActiveOutagesFromSource(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	if m.activeOutagesError != nil {
		return nil, m.activeOutagesError
	}
	return m.activeOutages, nil
}

func (m *mockOutageRepository) SaveOutage(outage *types.Outage) error {
	m.saveCount++
	outageCopy := *outage
	m.savedOutages = append(m.savedOutages, &outageCopy)
	if m.saveOutageFn != nil {
		m.saveOutageFn(outage)
	}
	return m.saveOutageError
}

func (m *mockOutageRepository) CreateReason(reason *types.Reason) error {
	reasonCopy := *reason
	m.createdReasons = append(m.createdReasons, &reasonCopy)
	if m.createReasonFn != nil {
		m.createReasonFn(reason)
	}
	return m.createReasonError
}

func (m *mockOutageRepository) CreateOutage(outage *types.Outage) error {
	outageCopy := *outage
	m.createdOutages = append(m.createdOutages, &outageCopy)
	if m.createOutageFn != nil {
		m.createOutageFn(outage)
	}
	return m.createOutageError
}

func (m *mockOutageRepository) Transaction(fn func(OutageRepository) error) error {
	if m.transactionError != nil {
		return m.transactionError
	}
	if m.transactionFn != nil {
		return m.transactionFn(fn)
	}
	return fn(m)
}

func (m *mockComponentPingRepository) UpsertComponentReportPing(componentSlug, subComponentSlug string, timestamp time.Time) error {
	m.upsertedPings = append(m.upsertedPings, struct {
		componentSlug    string
		subComponentSlug string
		timestamp        time.Time
	}{componentSlug, subComponentSlug, timestamp})
	if m.upsertFn != nil {
		m.upsertFn(componentSlug, subComponentSlug, timestamp)
	}
	return m.upsertError
}

func testConfig(autoResolve, requiresConfirmation bool) *types.DashboardConfig {
	subComponent := types.SubComponent{
		Slug: "test-subcomponent",
		Monitoring: types.Monitoring{
			AutoResolve:      autoResolve,
			ComponentMonitor: "test-monitor",
		},
		RequiresConfirmation: requiresConfirmation,
	}
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Slug:          "test-component",
				Subcomponents: []types.SubComponent{subComponent},
				Owners: []types.Owner{
					{ServiceAccount: "test-sa"},
				},
			},
		},
	}
}

func TestComponentMonitorReportProcessor_Process(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name                     string
		config                   *types.DashboardConfig
		request                  *types.ComponentMonitorReportRequest
		setupRepo                func(*mockOutageRepository)
		wantErr                  error
		verifyOutageExpectations func(*testing.T, *mockOutageRepository)
		verifyPingExpectations   func(*testing.T, *mockComponentPingRepository)
	}{
		{
			name:   "healthy status with no active outages",
			config: testConfig(true, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
				ping := pingRepo.upsertedPings[0]
				assert.Equal(t, "test-component", ping.componentSlug)
				assert.Equal(t, "test-subcomponent", ping.subComponentSlug)
				assert.False(t, ping.timestamp.IsZero())
			},
		},
		{
			name:   "healthy status with active outages and auto-resolve enabled",
			config: testConfig(true, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Len(t, repo.savedOutages, 2)
				assert.Empty(t, repo.createdOutages)
				assert.Empty(t, repo.createdReasons)
				for _, outage := range repo.savedOutages {
					assert.True(t, outage.EndTime.Valid)
					assert.Equal(t, "test-monitor", *outage.ResolvedBy)
				}
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
				ping := pingRepo.upsertedPings[0]
				assert.Equal(t, "test-component", ping.componentSlug)
				assert.Equal(t, "test-subcomponent", ping.subComponentSlug)
			},
		},
		{
			name:   "healthy status with active outages and auto-resolve disabled",
			config: testConfig(false, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Empty(t, repo.savedOutages)
				assert.Empty(t, repo.createdOutages)
				assert.Empty(t, repo.createdReasons)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status creates new outage without confirmation requirement",
			config: testConfig(false, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
				repo.createReasonFn = func(r *types.Reason) {
					r.ID = 1
				}
				repo.transactionFn = func(fn func(OutageRepository) error) error {
					return fn(repo)
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Len(t, repo.createdReasons, 1)
				reason := repo.createdReasons[0]
				assert.Equal(t, types.CheckTypePrometheus, reason.Type)
				assert.Len(t, repo.createdOutages, 1)
				outage := repo.createdOutages[0]
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.Equal(t, "test-monitor", *outage.ConfirmedBy)
				assert.True(t, outage.ConfirmedAt.Valid)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
				ping := pingRepo.upsertedPings[0]
				assert.Equal(t, "test-component", ping.componentSlug)
				assert.Equal(t, "test-subcomponent", ping.subComponentSlug)
			},
		},
		{
			name:   "unhealthy status creates new outage with confirmation requirement",
			config: testConfig(false, true),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
				repo.createReasonFn = func(r *types.Reason) {
					r.ID = 1
				}
				repo.transactionFn = func(fn func(OutageRepository) error) error {
					return fn(repo)
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Len(t, repo.createdReasons, 1)
				assert.Len(t, repo.createdOutages, 1)
				outage := repo.createdOutages[0]
				assert.Nil(t, outage.ConfirmedBy)
				assert.False(t, outage.ConfirmedAt.Valid)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
			},
		},
		{
			name:   "unhealthy status skips creation when active outage exists",
			config: testConfig(false, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Empty(t, repo.createdOutages)
				assert.Empty(t, repo.createdReasons)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
			},
		},
		{
			name:   "component not found returns error",
			config: testConfig(false, false),
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
			setupRepo: func(*mockOutageRepository) {},
			wantErr:   errors.New("component not found: nonexistent"),
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Empty(t, pingRepo.upsertedPings, "ping should not be called when component not found")
			},
		},
		{
			name:   "sub-component not found returns error",
			config: testConfig(false, false),
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
			setupRepo: func(*mockOutageRepository) {},
			wantErr:   errors.New("sub-component not found: test-component/nonexistent"),
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Empty(t, pingRepo.upsertedPings, "ping should not be called when sub-component not found")
			},
		},
		{
			name:   "get active outages error returns error",
			config: testConfig(false, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutagesError = errors.New("database error")
			},
			wantErr: errors.New("database error"),
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1, "ping should be called before checking active outages")
			},
		},
		{
			name:   "save outage error continues processing",
			config: testConfig(true, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
				repo.saveOutageError = errors.New("save error")
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
			},
		},
		{
			name:   "multiple statuses processed sequentially",
			config: testConfig(false, false),
			request: &types.ComponentMonitorReportRequest{
				ComponentMonitor: "test-monitor",
				Statuses: []types.ComponentMonitorReportComponentStatus{
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus}},
					},
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    types.CheckTypeHTTP,
								Check:   "url",
								Results: "timeout",
							},
						},
					},
				},
			},
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
				repo.createReasonFn = func(r *types.Reason) {
					r.ID = 1
				}
				repo.transactionFn = func(fn func(OutageRepository) error) error {
					return fn(repo)
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Len(t, repo.createdReasons, 1)
				reason := repo.createdReasons[0]
				assert.Equal(t, types.CheckTypeHTTP, reason.Type)
				assert.Len(t, repo.createdOutages, 1)
				outage := repo.createdOutages[0]
				assert.Equal(t, types.SeverityDown, outage.Severity)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 2, "ping should be called for each status")
				ping1 := pingRepo.upsertedPings[0]
				ping2 := pingRepo.upsertedPings[1]
				assert.Equal(t, "test-component", ping1.componentSlug)
				assert.Equal(t, "test-component", ping2.componentSlug)
			},
		},
		{
			name:   "unhealthy status creates outage with multiple reasons",
			config: testConfig(false, false),
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
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
				repo.createOutageFn = func(o *types.Outage) {
					o.ID = 1
				}
				repo.transactionFn = func(fn func(OutageRepository) error) error {
					return fn(repo)
				}
			},
			verifyOutageExpectations: func(t *testing.T, repo *mockOutageRepository) {
				assert.Len(t, repo.createdOutages, 1)
				outage := repo.createdOutages[0]
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)

				assert.Len(t, repo.createdReasons, 3, "Should create all three reasons")
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *mockComponentPingRepository) {
				assert.Len(t, pingRepo.upsertedPings, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockOutageRepository)
			tt.setupRepo(repo)
			pingRepo := new(mockComponentPingRepository)

			processor := &ComponentMonitorReportProcessor{
				outageRepo: repo,
				pingRepo:   pingRepo,
				config:     tt.config,
				logger:     logger,
			}

			err := processor.Process(tt.request)

			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("Process() error mismatch (-want +got):\n%s", diff)
			}

			if tt.verifyOutageExpectations != nil {
				tt.verifyOutageExpectations(t, repo)
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
			config: testConfig(false, false),
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
			config: testConfig(false, false),
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
			config: testConfig(false, false),
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
			config: testConfig(false, false),
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
			config: testConfig(false, false),
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
			config: testConfig(false, false),
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
								Monitoring: types.Monitoring{ComponentMonitor: "test-monitor"},
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
			processor := &ComponentMonitorReportProcessor{
				outageRepo: new(mockOutageRepository),
				pingRepo:   new(mockComponentPingRepository),
				config:     tt.config,
				logger:     logger,
			}

			err := processor.ValidateRequest(tt.request, tt.serviceAccount)

			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("validateRequest() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
