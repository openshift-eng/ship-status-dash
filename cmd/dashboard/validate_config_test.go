package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"ship-status-dash/pkg/types"
)

func TestLoadAndValidateConfigForValidateOnly(t *testing.T) {
	tests := []struct {
		name    string
		config  *types.DashboardConfig
		wantErr bool
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

func TestLoadAndValidateConfigReportsAllErrors(t *testing.T) {
	cfg := &types.DashboardConfig{
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
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	content, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	_, err = loadAndValidateConfig(log, path)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "Missing Owner A") {
		t.Errorf("expected error about Missing Owner A, got: %s", msg)
	}
	if !strings.Contains(msg, "Missing Owner B") {
		t.Errorf("expected error about Missing Owner B, got: %s", msg)
	}
	if !strings.Contains(msg, "nonexistent-tag") {
		t.Errorf("expected error about nonexistent-tag, got: %s", msg)
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
