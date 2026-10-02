package main

import (
	"os"
	"path/filepath"
	"testing"

	"ship-status-dash/pkg/types"
)

func TestRunValidateConfig(t *testing.T) {
	validConfig := `
frequency: 20s
components:
  - component_slug: "test"
    sub_component_slug: "frontend"
    http_monitor:
      url: "http://localhost:8080/health"
      code: 200
      retry_after: 5s
`
	invalidConfigBadFrequency := `
frequency: not-a-duration
components:
  - component_slug: "test"
    sub_component_slug: "frontend"
    http_monitor:
      url: "http://localhost:8080/health"
      code: 200
      retry_after: 5s
`
	invalidYAML := `{{{not yaml`

	clusterBasedConfig := `
frequency: 20s
components:
  - component_slug: "test"
    sub_component_slug: "prom-check"
    prometheus_monitor:
      prometheus_location:
        cluster: "app.ci"
        namespace: "openshift-monitoring"
        route: "thanos-querier"
      queries:
        - query: "up"
`

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
			name:     "invalid config bad frequency",
			content:  invalidConfigBadFrequency,
			wantCode: 1,
		},
		{
			name:     "invalid YAML",
			content:  invalidYAML,
			wantCode: 1,
		},
		{
			name:     "missing config-path flag",
			args:     []string{},
			wantCode: 1,
		},
		{
			name:     "nonexistent config file",
			args:     []string{"--config-path", "/nonexistent/path.yaml"},
			wantCode: 1,
		},
		{
			name:     "cluster-based prometheus without kubeconfig-dir succeeds",
			content:  clusterBasedConfig,
			wantCode: 0,
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
				args = []string{"--config-path", path}
			}

			code := runValidateConfig(args)
			if code != tt.wantCode {
				t.Errorf("runValidateConfig() = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestValidatePrometheusConfigurationSkipsKubeconfigChecks(t *testing.T) {
	components := []types.MonitoringComponent{
		{
			ComponentSlug:    "test",
			SubComponentSlug: "test",
			PrometheusMonitor: &types.PrometheusMonitor{
				PrometheusLocation: types.PrometheusLocation{
					Cluster:   "app.ci",
					Namespace: "openshift-monitoring",
					Route:     "thanos-querier",
				},
				Queries: []types.PrometheusQuery{{Query: "up", Severity: types.SeverityDown}},
			},
		},
	}

	err := validatePrometheusConfiguration(components, "", false)
	if err != nil {
		t.Errorf("expected no error with validateKubeconfigs=false, got: %v", err)
	}

	err = validatePrometheusConfiguration(components, "", true)
	if err == nil {
		t.Error("expected error with validateKubeconfigs=true and empty kubeconfigDir")
	}
}
