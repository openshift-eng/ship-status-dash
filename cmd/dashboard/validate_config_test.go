package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunValidateConfig(t *testing.T) {
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
		name     string
		args     []string
		content  string
		wantCode int
	}{
		{
			name:     "valid config",
			content:  validConfig,
			wantCode: 0,
		},
		{
			name:     "invalid config missing owner",
			content:  invalidConfigNoOwner,
			wantCode: 1,
		},
		{
			name:     "invalid YAML",
			content:  invalidYAML,
			wantCode: 1,
		},
		{
			name:     "missing config flag",
			args:     []string{},
			wantCode: 1,
		},
		{
			name:     "nonexistent config file",
			args:     []string{"--config", "/nonexistent/path.yaml"},
			wantCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []string
			if tt.args != nil {
				args = tt.args
			} else {
				path := filepath.Join(t.TempDir(), "config.yaml")
				if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
					t.Fatalf("write config: %v", err)
				}
				args = []string{"--config", path}
			}

			code := runValidateConfig(args)
			if code != tt.wantCode {
				t.Errorf("runValidateConfig() = %d, want %d", code, tt.wantCode)
			}
		})
	}
}
