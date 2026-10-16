package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"ship-status-dash/pkg/types"
)

func TestLoadAndValidateConfigForValidateOnly(t *testing.T) {
	tests := []struct {
		name     string
		config   *types.ComponentMonitorConfig
		wantErrs []error
	}{
		{
			name: "valid config",
			config: &types.ComponentMonitorConfig{
				Frequency: "20s",
				Components: []types.MonitoringComponent{
					{
						ComponentSlug:    "test",
						SubComponentSlug: "frontend",
						HTTPMonitor: &types.HTTPMonitor{
							URL:        "http://localhost:8080/health",
							Code:       200,
							RetryAfter: "5s",
						},
					},
				},
			},
		},
		{
			name: "invalid config bad frequency",
			config: &types.ComponentMonitorConfig{
				Frequency: "not-a-duration",
				Components: []types.MonitoringComponent{
					{
						ComponentSlug:    "test",
						SubComponentSlug: "frontend",
						HTTPMonitor: &types.HTTPMonitor{
							URL:        "http://localhost:8080/health",
							Code:       200,
							RetryAfter: "5s",
						},
					},
				},
			},
			wantErrs: []error{errors.New("failed to parse frequency")},
		},
		{
			name:     "invalid YAML",
			config:   nil,
			wantErrs: []error{errors.New("yaml:")},
		},
		{
			name: "invalid frequency and retry_after both reported",
			config: &types.ComponentMonitorConfig{
				Frequency: "not-a-duration",
				Components: []types.MonitoringComponent{
					{
						ComponentSlug:    "test",
						SubComponentSlug: "frontend",
						HTTPMonitor: &types.HTTPMonitor{
							URL:        "http://localhost:8080/health",
							Code:       200,
							RetryAfter: "also-invalid",
						},
					},
				},
			},
			wantErrs: []error{
				errors.New("failed to parse frequency"),
				errors.New("failed to parse retry after duration"),
			},
		},
		{
			name: "reports all errors",
			config: &types.ComponentMonitorConfig{
				Frequency: "20s",
				Components: []types.MonitoringComponent{
					{
						ComponentSlug:    "comp-a",
						SubComponentSlug: "sub-a",
						JUnitMonitor: &types.JUnitMonitor{
							MaxAge: "1h",
						},
					},
					{
						ComponentSlug:    "comp-b",
						SubComponentSlug: "sub-b",
						SystemdMonitor:   &types.SystemdMonitor{Unit: ""},
					},
				},
			},
			wantErrs: []error{
				errors.New("job_names is required"),
				errors.New("systemd unit is required"),
			},
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")

			var content []byte
			if tt.config == nil {
				content = []byte(`{{{not yaml`)
			} else {
				var err error
				content, err = yaml.Marshal(tt.config)
				if err != nil {
					t.Fatalf("marshal config: %v", err)
				}
			}

			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := loadAndValidateConfig(log, path, "")
			if len(tt.wantErrs) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, want := range tt.wantErrs {
				assert.ErrorContains(t, err, want.Error())
			}
		})
	}
}

func TestValidateOnlyOptionsValidation(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("frequency: 20s\ncomponents: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	opts := &Options{
		ConfigPath:   configPath,
		ValidateOnly: true,
	}
	if err := opts.Validate(); err != nil {
		t.Errorf("expected ValidateOnly to skip runtime checks, got error: %v", err)
	}

	opts = &Options{
		ValidateOnly: true,
	}
	if err := opts.Validate(); err == nil {
		t.Error("expected error when ConfigPath is empty even in ValidateOnly mode")
	}
}
