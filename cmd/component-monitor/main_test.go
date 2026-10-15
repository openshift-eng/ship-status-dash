package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func writeMonitorConfig(t *testing.T, cfg types.ComponentMonitorConfig) string {
	t.Helper()
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadAndValidateConfigFrequency(t *testing.T) {
	jiraIncidents := func(frequency string) types.MonitoringComponent {
		return types.MonitoringComponent{
			ComponentSlug:    "trt-incidents",
			SubComponentSlug: "incidents",
			Frequency:        frequency,
			JiraMonitor: &types.JiraMonitor{
				URL: "https://redhat.atlassian.net",
				JQL: "labels = trt-incident",
			},
		}
	}
	httpDeck := func(frequency, retryAfter string) types.MonitoringComponent {
		return types.MonitoringComponent{
			ComponentSlug:    "prow",
			SubComponentSlug: "deck",
			Frequency:        frequency,
			HTTPMonitor: &types.HTTPMonitor{
				URL:        "http://example.com",
				Code:       200,
				RetryAfter: retryAfter,
			},
		}
	}
	promDeck := func(frequency string) types.MonitoringComponent {
		return types.MonitoringComponent{
			ComponentSlug:    "prow",
			SubComponentSlug: "deck",
			Frequency:        frequency,
			PrometheusMonitor: &types.PrometheusMonitor{
				PrometheusLocation: types.PrometheusLocation{URL: "http://localhost:9090"},
				Queries:            []types.PrometheusQuery{{Query: "up"}},
			},
		}
	}

	tests := []struct {
		name    string
		cfg     types.ComponentMonitorConfig
		wantErr string
	}{
		{
			name:    "missing instance frequency",
			cfg:     types.ComponentMonitorConfig{},
			wantErr: "failed to parse frequency",
		},
		{
			name: "invalid instance frequency",
			cfg: types.ComponentMonitorConfig{
				Frequency: "not-a-duration",
			},
			wantErr: "failed to parse frequency",
		},
		{
			name: "non-positive instance frequency",
			cfg: types.ComponentMonitorConfig{
				Frequency: "0s",
			},
			wantErr: "frequency must be a positive duration",
		},
		{
			name: "invalid entry frequency",
			cfg: types.ComponentMonitorConfig{
				Frequency:  "20s",
				Components: []types.MonitoringComponent{jiraIncidents("bogus")},
			},
			wantErr: "failed to parse frequency for component trt-incidents/incidents",
		},
		{
			name: "entry frequency less than instance frequency",
			cfg: types.ComponentMonitorConfig{
				Frequency:  "5m",
				Components: []types.MonitoringComponent{jiraIncidents("20s")},
			},
			wantErr: "is less than instance frequency",
		},
		{
			name: "http retry_after greater than resolved frequency",
			cfg: types.ComponentMonitorConfig{
				Frequency:  "20s",
				Components: []types.MonitoringComponent{httpDeck("1m", "90s")},
			},
			wantErr: "retry after duration is greater than frequency",
		},
		{
			name: "same sub-component mismatched frequencies",
			cfg: types.ComponentMonitorConfig{
				Frequency: "20s",
				Components: []types.MonitoringComponent{
					{
						ComponentSlug:    "build-farm",
						SubComponentSlug: "build01",
						Frequency:        "1m",
						HTTPMonitor: &types.HTTPMonitor{
							URL:        "http://example.com",
							Code:       200,
							RetryAfter: "5s",
						},
					},
					{
						ComponentSlug:    "build-farm",
						SubComponentSlug: "build01",
						Frequency:        "2m",
						PrometheusMonitor: &types.PrometheusMonitor{
							PrometheusLocation: types.PrometheusLocation{URL: "http://localhost:9090"},
							Queries:            []types.PrometheusQuery{{Query: "up"}},
						},
					},
				},
			},
			wantErr: "frequency mismatch for component build-farm/build01",
		},
		{
			name: "valid entry frequency override",
			cfg: types.ComponentMonitorConfig{
				Frequency:  "20s",
				Components: []types.MonitoringComponent{jiraIncidents("5m")},
			},
		},
		{
			name: "same sub-component matching frequencies",
			cfg: types.ComponentMonitorConfig{
				Frequency: "20s",
				Components: []types.MonitoringComponent{
					httpDeck("1m", "5s"),
					promDeck("1m"),
				},
			},
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadAndValidateConfig(log, writeMonitorConfig(t, tt.cfg), "")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadAndValidateConfigJUnit(t *testing.T) {
	valid := func() types.JUnitMonitor {
		return types.JUnitMonitor{
			JobNames:            []string{"periodic-build-farm-canary-build01"},
			ArtifactPath:        "artifacts/junit_canary.xml",
			AlertOnNoRecentRun:  true,
			MaxAge:              "2h",
			HistoryRuns:         5,
			FailedRunsThreshold: 3,
		}
	}
	component := func(m types.JUnitMonitor) types.ComponentMonitorConfig {
		return types.ComponentMonitorConfig{
			Frequency: "5m",
			Components: []types.MonitoringComponent{{
				ComponentSlug:    "build-farm",
				SubComponentSlug: "build01",
				JUnitMonitor:     &m,
			}},
		}
	}
	patch := func(fn func(*types.JUnitMonitor)) types.JUnitMonitor {
		m := valid()
		fn(&m)
		return m
	}

	tests := []struct {
		name    string
		cfg     types.ComponentMonitorConfig
		wantErr string
	}{
		{name: "canary", cfg: component(valid())},
		{
			name: "multiple jobs",
			cfg: component(types.JUnitMonitor{
				JobNames: []string{
					"periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn",
					"periodic-ci-openshift-release-main-ci-5.0-e2e-aws-ovn",
				},
				ArtifactPath:             "artifacts/e2e-aws-ovn/gather-must-gather/artifacts/junit_install.xml",
				MissingArtifactUnhealthy: boolPtr(false),
				GroupBy:                  types.JUnitGroupByLeaseResourcePrefix,
				TestcasePrefix:           "install should succeed:",
				MaxAge:                   "48h",
				HistoryRuns:              15,
				FailedRunsThreshold:      3,
				Severity:                 types.SeverityDegraded,
			}),
		},
		{name: "missing job_names", cfg: component(patch(func(m *types.JUnitMonitor) { m.JobNames = nil })), wantErr: "job_names is required"},
		{name: "blank job name", cfg: component(patch(func(m *types.JUnitMonitor) { m.JobNames = []string{"  "} })), wantErr: "job_names entries must be non-empty"},
		{name: "missing artifact_path", cfg: component(patch(func(m *types.JUnitMonitor) { m.ArtifactPath = " " })), wantErr: "artifact_path is required"},
		{name: "no recent run requires max_age", cfg: component(patch(func(m *types.JUnitMonitor) { m.MaxAge = "" })), wantErr: "max_age"},
		{name: "stale alert with lease group", cfg: component(patch(func(m *types.JUnitMonitor) {
			m.GroupBy = types.JUnitGroupByLeaseResourcePrefix
		})), wantErr: "alert_on_no_recent_run"},
		{name: "unknown group_by", cfg: component(patch(func(m *types.JUnitMonitor) {
			m.AlertOnNoRecentRun = false
			m.GroupBy = "account"
		})), wantErr: "group_by"},
		{name: "latest run group window", cfg: component(patch(func(m *types.JUnitMonitor) {
			m.LatestRunGroupWindow = "30m"
		}))},
		{name: "latest run group window zero", cfg: component(patch(func(m *types.JUnitMonitor) {
			m.LatestRunGroupWindow = "0s"
		})), wantErr: "latest_run_group_window"},
		{name: "latest run group window invalid", cfg: component(patch(func(m *types.JUnitMonitor) {
			m.LatestRunGroupWindow = "soon"
		})), wantErr: "latest_run_group_window"},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadAndValidateConfig(log, writeMonitorConfig(t, tt.cfg), "")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
