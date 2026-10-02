package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoadAndValidateConfigForValidateOnly(t *testing.T) {
	validConfig := `
components:
  - name: "Test Component"
    description: "A test component"
    sub_components:
      - name: "Sub1"
        description: "A sub-component"
    owners:
      - user: "developer"
`
	invalidConfigNoOwner := `
components:
  - name: "Test Component"
    description: "A test component"
    sub_components:
      - name: "Sub1"
        description: "A sub-component"
`
	invalidYAML := `{{{not yaml`

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "valid config",
			content: validConfig,
		},
		{
			name:    "invalid config missing owner",
			content: invalidConfigNoOwner,
			wantErr: true,
		},
		{
			name:    "invalid YAML",
			content: invalidYAML,
			wantErr: true,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
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
