package main

import (
	"errors"
	"testing"

	"ship-status-dash/pkg/types"

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

func testConfig(autoResolve, requiresConfirmation bool) *types.Config {
	subComponent := types.SubComponent{
		Slug:                 "test-subcomponent",
		Monitoring:           types.Monitoring{AutoResolve: autoResolve},
		RequiresConfirmation: requiresConfirmation,
	}
	return &types.Config{
		Components: []*types.Component{
			{
				Slug:          "test-component",
				Subcomponents: []types.SubComponent{subComponent},
			},
		},
	}
}

func TestComponentMonitorReportProcessor_Process(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name                     string
		config                   *types.Config
		request                  *types.ComponentMonitorReportRequest
		setupRepo                func(*mockOutageRepository)
		wantErr                  bool
		errContains              string
		verifyOutageExpectations func(*testing.T, *mockOutageRepository)
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
				},
			},
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{}
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
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
								Type:    "prometheus",
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
				assert.Equal(t, "prometheus", repo.createdReasons[0].Type)
				assert.Len(t, repo.createdOutages, 1)
				assert.Equal(t, "test-component", repo.createdOutages[0].ComponentName)
				assert.Equal(t, types.SeverityDown, repo.createdOutages[0].Severity)
				assert.Equal(t, "test-monitor", *repo.createdOutages[0].ConfirmedBy)
				assert.True(t, repo.createdOutages[0].ConfirmedAt.Valid)
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
								Type:    "prometheus",
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
				assert.Nil(t, repo.createdOutages[0].ConfirmedBy)
				assert.False(t, repo.createdOutages[0].ConfirmedAt.Valid)
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
				},
			},
			setupRepo:   func(*mockOutageRepository) {},
			wantErr:     true,
			errContains: "component not found: nonexistent",
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
				},
			},
			setupRepo:   func(*mockOutageRepository) {},
			wantErr:     true,
			errContains: "sub-component not found: test-component/nonexistent",
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
				},
			},
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutagesError = errors.New("database error")
			},
			wantErr:     true,
			errContains: "database error",
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
				},
			},
			setupRepo: func(repo *mockOutageRepository) {
				repo.activeOutages = []types.Outage{
					{ComponentName: "test-component", SubComponentName: "test-subcomponent"},
				}
				repo.saveOutageError = errors.New("save error")
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
						Reasons:          []types.Reason{{Type: "prometheus"}},
					},
					{
						ComponentSlug:    "test-component",
						SubComponentSlug: "test-subcomponent",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{
								Type:    "http",
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
				assert.Equal(t, "http", repo.createdReasons[0].Type)
				assert.Len(t, repo.createdOutages, 1)
				assert.Equal(t, types.SeverityDown, repo.createdOutages[0].Severity)
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
				assert.Equal(t, "test-component", repo.createdOutages[0].ComponentName)
				assert.Equal(t, types.SeverityDown, repo.createdOutages[0].Severity)

				assert.Len(t, repo.createdReasons, 3, "Should create all three reasons")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(mockOutageRepository)
			tt.setupRepo(repo)

			processor := &ComponentMonitorReportProcessor{
				repo:   repo,
				config: tt.config,
				logger: logger,
			}

			err := processor.Process(tt.request, func(*types.Outage) (string, bool) { return "", true })

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}

			if tt.verifyOutageExpectations != nil {
				tt.verifyOutageExpectations(t, repo)
			}
		})
	}
}
