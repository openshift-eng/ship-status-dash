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
