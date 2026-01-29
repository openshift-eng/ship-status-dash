package main

import (
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

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

	db := testhelper.SetupTestDB(t)

	tests := []struct {
		name                     string
		config                   *types.DashboardConfig
		request                  *types.ComponentMonitorReportRequest
		setupRepo                func(*gorm.DB)
		wantErr                  error
		verifyOutageExpectations func(*testing.T, *gorm.DB)
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
			setupRepo: func(db *gorm.DB) {
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
			setupRepo: func(db *gorm.DB) {
				outages := []types.Outage{
					{Model: gorm.Model{ID: 1}, ComponentName: "test-component", SubComponentName: "test-subcomponent", CreatedBy: "test-monitor"},
					{Model: gorm.Model{ID: 2}, ComponentName: "test-component", SubComponentName: "test-subcomponent", CreatedBy: "test-monitor"},
				}
				for i := range outages {
					db.Create(&outages[i])
				}
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var savedOutages []types.Outage
				db.Where("component_name = ? AND sub_component_name = ?", "test-component", "test-subcomponent").Find(&savedOutages)
				assert.Len(t, savedOutages, 2)
				var createdOutages []types.Outage
				db.Where("component_name = ? AND sub_component_name = ? AND id NOT IN (1, 2)", "test-component", "test-subcomponent").Find(&createdOutages)
				assert.Empty(t, createdOutages)
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Empty(t, createdReasons)
				for _, outage := range savedOutages {
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
			setupRepo: func(db *gorm.DB) {
				outage := types.Outage{ComponentName: "test-component", SubComponentName: "test-subcomponent"}
				db.Create(&outage)
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var savedOutages []types.Outage
				db.Where("component_name = ? AND sub_component_name = ?", "test-component", "test-subcomponent").Find(&savedOutages)
				// Filter out the initial outage
				var updatedCount int
				for _, o := range savedOutages {
					if o.EndTime.Valid {
						updatedCount++
					}
				}
				assert.Equal(t, 0, updatedCount, "No outages should be updated")
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				assert.LessOrEqual(t, len(createdOutages), 1, "No new outages should be created")
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Empty(t, createdReasons)
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
			setupRepo: func(db *gorm.DB) {
				// No initial data needed
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Len(t, createdReasons, 1)
				reason := createdReasons[0]
				assert.Equal(t, types.CheckTypePrometheus, reason.Type)
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				assert.Len(t, createdOutages, 1)
				outage := createdOutages[0]
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.Equal(t, "test-monitor", *outage.ConfirmedBy)
				assert.True(t, outage.ConfirmedAt.Valid)
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
			setupRepo: func(db *gorm.DB) {
				// No initial data needed
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Len(t, createdReasons, 1)
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				assert.Len(t, createdOutages, 1)
				outage := createdOutages[0]
				assert.Nil(t, outage.ConfirmedBy)
				assert.False(t, outage.ConfirmedAt.Valid)
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
			setupRepo: func(db *gorm.DB) {
				outage := types.Outage{
					ComponentName:    "test-component",
					SubComponentName: "test-subcomponent",
					CreatedBy:        "test-monitor",
					Severity:         types.SeverityDown,
					StartTime:        time.Now().Add(-10 * time.Minute),
					DiscoveredFrom:   ComponentMonitor,
				}
				db.Create(&outage)
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				// Should only have the initial outage
				assert.LessOrEqual(t, len(createdOutages), 1)
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Empty(t, createdReasons)
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
			setupRepo: func(*gorm.DB) {},
			wantErr:   errors.New("component not found: nonexistent"),
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
			setupRepo: func(*gorm.DB) {},
			wantErr:   errors.New("sub-component not found: test-component/nonexistent"),
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Empty(t, pingRepo.UpsertedPings, "ping should not be called when sub-component not found")
			},
		},
		{
			name:   "save outage error continues processing",
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
			setupRepo: func(db *gorm.DB) {
				outage := types.Outage{ComponentName: "test-component", SubComponentName: "test-subcomponent"}
				db.Create(&outage)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
		{
			name:   "multiple statuses processed sequentially",
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
			setupRepo: func(db *gorm.DB) {
				// No initial data needed
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Len(t, createdReasons, 1)
				reason := createdReasons[0]
				assert.Equal(t, types.CheckTypeHTTP, reason.Type)
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				assert.Len(t, createdOutages, 1)
				outage := createdOutages[0]
				assert.Equal(t, types.SeverityDown, outage.Severity)
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 2, "ping should be called for each status")
				ping1 := pingRepo.UpsertedPings[0]
				ping2 := pingRepo.UpsertedPings[1]
				assert.Equal(t, "test-component", ping1.ComponentSlug)
				assert.Equal(t, "test-component", ping2.ComponentSlug)
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
			setupRepo: func(db *gorm.DB) {
				// No initial data needed
			},
			verifyOutageExpectations: func(t *testing.T, db *gorm.DB) {
				var createdOutages []types.Outage
				db.Find(&createdOutages)
				assert.Len(t, createdOutages, 1)
				outage := createdOutages[0]
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)

				var createdReasons []types.Reason
				db.Find(&createdReasons)
				assert.Len(t, createdReasons, 3, "Should create all three reasons")
			},
			verifyPingExpectations: func(t *testing.T, pingRepo *repositories.MockComponentPingRepository) {
				assert.Len(t, pingRepo.UpsertedPings, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear data from previous test cases
			db.Exec("DELETE FROM reasons")
			db.Exec("DELETE FROM slack_threads")
			db.Exec("DELETE FROM outages")

			pingRepo := &repositories.MockComponentPingRepository{}

			configManager := config.CreateTestConfigManager(tt.config)

			if tt.setupRepo != nil {
				tt.setupRepo(db)
			}

			outageManager := outage.NewOutageManager(db, nil, configManager, "", "https://rhsandbox.slack.com/", logger)
			processor := &ComponentMonitorReportProcessor{
				outageManager: outageManager,
				pingRepo:      pingRepo,
				configManager: configManager,
				logger:        logger,
			}

			err := processor.Process(tt.request)

			if diff := cmp.Diff(tt.wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("Process() error mismatch (-want +got):\n%s", diff)
			}

			if tt.verifyOutageExpectations != nil {
				tt.verifyOutageExpectations(t, db)
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

	db := testhelper.SetupTestDB(t)

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
			outageManager := outage.NewOutageManager(db, nil, configManager, "", "https://rhsandbox.slack.com/", logger)
			processor := &ComponentMonitorReportProcessor{
				outageManager: outageManager,
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
