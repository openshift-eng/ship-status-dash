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
		config   *types.DashboardConfig
		wantErrs []error
	}{
		{
			name: "valid config",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Name:        "Test Component",
						Description: "A test component",
						Subcomponents: []types.SubComponent{
							{
								Name:        "Sub1",
								Description: "A sub-component",
							},
						},
						Owners: []types.Owner{
							{User: "developer"},
						},
					},
				},
			},
		},
		{
			name: "invalid config missing owner",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Name:        "Test Component",
						Description: "A test component",
						Subcomponents: []types.SubComponent{
							{
								Name:        "Sub1",
								Description: "A sub-component",
							},
						},
					},
				},
			},
			wantErrs: []error{errors.New("component must have at least one owner")},
		},
		{
			name:     "invalid YAML",
			config:   nil,
			wantErrs: []error{errors.New("yaml:")},
		},
		{
			name: "reports all errors",
			config: &types.DashboardConfig{
				Components: []*types.Component{
					{
						Name:        "Missing Owner A",
						Description: "No owner",
						Subcomponents: []types.SubComponent{
							{Name: "Sub1", Tags: []string{"nonexistent-tag"}},
						},
					},
					{
						Name:        "Missing Owner B",
						Description: "Also no owner",
						Subcomponents: []types.SubComponent{
							{Name: "Sub2"},
						},
					},
				},
			},
			wantErrs: []error{
				errors.New("Missing Owner A"),
				errors.New("Missing Owner B"),
				errors.New("nonexistent-tag"),
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

			_, err := loadAndValidateConfig(log, path)
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
	if err := os.WriteFile(configPath, []byte("components: []\n"), 0o600); err != nil {
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
