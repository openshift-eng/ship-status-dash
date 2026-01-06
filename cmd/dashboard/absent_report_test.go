package main

import (
	"testing"
	"time"

	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsentMonitoredComponentReportChecker_checkForAbsentReports(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name                 string
		config               *types.DashboardConfig
		setupPingRepo        func(*repositories.MockComponentPingRepository)
		setupOutageRepo      func(*repositories.MockOutageRepository)
		verifyCreatedOutages func(*testing.T, []*types.Outage) // Required - must verify all expected outages
	}{
		{
			name: "skips sub-component without monitoring frequency",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "",
								},
							},
						},
					},
				},
			},
			setupPingRepo:   func(*repositories.MockComponentPingRepository) {},
			setupOutageRepo: func(*repositories.MockOutageRepository) {},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				assert.Len(t, outages, 0, "Expected no outages to be created")
			},
		},
		{
			name: "skips sub-component with invalid frequency",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "invalid",
								},
							},
						},
					},
				},
			},
			setupPingRepo:   func(*repositories.MockComponentPingRepository) {},
			setupOutageRepo: func(*repositories.MockOutageRepository) {},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				assert.Len(t, outages, 0, "Expected no outages to be created")
			},
		},
		{
			name: "creates outage when no ping record exists",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
								RequiresConfirmation: false,
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				repo.LastPingTimes = nil
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				require.Len(t, outages, 1, "Expected 1 outage to be created")
				outage := outages[0]
				assert.Equal(t, "test-component", outage.ComponentName)
				assert.Equal(t, "test-subcomponent", outage.SubComponentName)
				assert.Equal(t, types.SeverityDown, outage.Severity)
				assert.Equal(t, AbsentReportSource, outage.DiscoveredFrom)
				assert.Equal(t, AbsentReportCreator, outage.CreatedBy)
				assert.Contains(t, outage.Description, "No report from component-monitor found")
				assert.True(t, outage.ConfirmedAt.Valid)
				assert.Equal(t, AbsentReportCreator, *outage.ConfirmedBy)
			},
		},
		{
			name: "creates outage when ping exceeds threshold",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
								RequiresConfirmation: false,
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				// Last ping was 20 minutes ago (threshold is 15 minutes for 5m frequency)
				pastTime := time.Now().Add(-20 * time.Minute)
				repo.LastPingTimes = map[string]*time.Time{
					"test-component/test-subcomponent": &pastTime,
				}
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				require.Len(t, outages, 1, "Expected 1 outage to be created")
				assert.Contains(t, outages[0].Description, "exceeding threshold")
			},
		},
		{
			name: "does not create outage when ping is within threshold",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				// Last ping was 5 minutes ago (threshold is 15 minutes for 5m frequency)
				pastTime := time.Now().Add(-5 * time.Minute)
				repo.LastPingTimes = map[string]*time.Time{
					"test-component/test-subcomponent": &pastTime,
				}
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				assert.Len(t, outages, 0, "Expected no outages to be created")
			},
		},
		{
			name: "does not create duplicate outage when active outage exists",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				repo.LastPingTimes = nil
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{
					{
						ComponentName:    "test-component",
						SubComponentName: "test-subcomponent",
						DiscoveredFrom:   AbsentReportSource,
					},
				}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				assert.Len(t, outages, 0, "Expected no outages to be created")
			},
		},
		{
			name: "does not auto-confirm when requires confirmation",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "test-component",
						Subcomponents: []types.SubComponent{
							{
								Slug: "test-subcomponent",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
								RequiresConfirmation: true,
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				repo.LastPingTimes = nil
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				require.Len(t, outages, 1, "Expected 1 outage to be created")
				assert.False(t, outages[0].ConfirmedAt.Valid)
				assert.Nil(t, outages[0].ConfirmedBy)
			},
		},
		{
			name: "handles multiple components and sub-components",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Slug: "component-1",
						Subcomponents: []types.SubComponent{
							{
								Slug: "sub-1",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
							},
							{
								Slug: "sub-2",
								Monitoring: types.Monitoring{
									Frequency: "10m",
								},
							},
						},
					},
					{
						Slug: "component-2",
						Subcomponents: []types.SubComponent{
							{
								Slug: "sub-1",
								Monitoring: types.Monitoring{
									Frequency: "5m",
								},
							},
						},
					},
				},
			},
			setupPingRepo: func(repo *repositories.MockComponentPingRepository) {
				// component-1/sub-1 has a recent ping (within threshold), so no outage should be created
				recentPing := time.Now().Add(-2 * time.Minute)
				repo.LastPingTimes = map[string]*time.Time{
					"component-1/sub-1": &recentPing,
					// component-1/sub-2 and component-2/sub-1 have no pings, so outages should be created
				}
			},
			setupOutageRepo: func(repo *repositories.MockOutageRepository) {
				repo.ActiveOutages = []types.Outage{}
			},
			verifyCreatedOutages: func(t *testing.T, outages []*types.Outage) {
				// Verify outages were created for component-1/sub-2 and component-2/sub-1
				// but NOT for component-1/sub-1 (which has a recent ping)
				require.Len(t, outages, 2, "Expected 2 outages to be created")
				expectedOutages := map[string]bool{
					"component-1/sub-2": false,
					"component-2/sub-1": false,
				}
				for _, outage := range outages {
					key := outage.ComponentName + "/" + outage.SubComponentName
					if _, expected := expectedOutages[key]; expected {
						expectedOutages[key] = true
					} else {
						t.Errorf("Unexpected outage created for %s", key)
					}
				}
				for key, found := range expectedOutages {
					assert.True(t, found, "Expected outage to be created for %s", key)
				}
				// Verify no outage was created for component-1/sub-1
				for _, outage := range outages {
					if outage.ComponentName == "component-1" && outage.SubComponentName == "sub-1" {
						t.Error("Unexpected outage created for component-1/sub-1, which has a recent ping")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pingRepo := &repositories.MockComponentPingRepository{}
			outageRepo := &repositories.MockOutageRepository{}
			tt.setupPingRepo(pingRepo)
			tt.setupOutageRepo(outageRepo)

			checker := NewAbsentMonitoredComponentReportChecker(tt.config, outageRepo, pingRepo, 5*time.Minute, logger)
			checker.checkForAbsentReports()

			tt.verifyCreatedOutages(t, outageRepo.CreatedOutages)
		})
	}
}
