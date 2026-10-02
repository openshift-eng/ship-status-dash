package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"ship-status-dash/pkg/types"
)

func TestLoadAndValidateConfigForValidateOnly(t *testing.T) {
	validConfig := types.DashboardConfig{
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
	}

	invalidConfigNoOwner := types.DashboardConfig{
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
	}

	tests := []struct {
		name    string
		config  any
		wantErr bool
	}{
		{
			name:   "valid config",
			config: validConfig,
		},
		{
			name:    "invalid config missing owner",
			config:  invalidConfigNoOwner,
			wantErr: true,
		},
		{
			name:    "invalid YAML",
			config:  nil,
			wantErr: true,
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
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
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
