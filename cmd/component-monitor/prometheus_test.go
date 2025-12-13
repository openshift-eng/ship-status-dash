package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"ship-status-dash/pkg/types"
)

func TestIsURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid http URL",
			input:    "http://localhost:9090",
			expected: true,
		},
		{
			name:     "valid https URL",
			input:    "https://prometheus.example.com",
			expected: true,
		},
		{
			name:     "valid https URL with path",
			input:    "https://prometheus.example.com/api/v1",
			expected: true,
		},
		{
			name:     "invalid - no scheme",
			input:    "localhost:9090",
			expected: false,
		},
		{
			name:     "invalid - not http/https",
			input:    "ftp://example.com",
			expected: false,
		},
		{
			name:     "invalid - empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "invalid - cluster name",
			input:    "app.ci",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePrometheusLocations(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test kubeconfig file
	kubeconfigPath := filepath.Join(tmpDir, "app.ci.config")
	err := os.WriteFile(kubeconfigPath, []byte("test kubeconfig content"), 0644)
	assert.NoError(t, err)

	tests := []struct {
		name          string
		components    []types.MonitoringComponent
		kubeconfigDir string
		expectedErr   error
	}{
		{
			name: "valid - URL when kubeconfigDir not set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "http://localhost:9090",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
		},
		{
			name: "valid - cluster name when kubeconfigDir set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "app.ci",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "invalid - URL when kubeconfigDir set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "http://localhost:9090",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation must be a cluster name (not a URL) when --kubeconfig-dir is set, got: http://localhost:9090"),
		},
		{
			name: "invalid - cluster name when kubeconfigDir not set",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "app.ci",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			expectedErr: errors.New("prometheusLocation must be a URL when --kubeconfig-dir is not set, got: app.ci"),
		},
		{
			name: "invalid - empty prometheusLocation",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("prometheusLocation is required for component test/test"),
		},
		{
			name: "invalid - kubeconfig file not found",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "nonexistent",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			kubeconfigDir: tmpDir,
			expectedErr:   errors.New("kubeconfig file not found for cluster nonexistent at"),
		},
		{
			name: "valid - component without PrometheusMonitor",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test",
					SubComponentSlug: "test",
					HTTPMonitor: &types.HTTPMonitor{
						URL: "http://example.com",
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
		{
			name: "valid - multiple components with mixed monitors",
			components: []types.MonitoringComponent{
				{
					ComponentSlug:    "test1",
					SubComponentSlug: "test1",
					HTTPMonitor: &types.HTTPMonitor{
						URL: "http://example.com",
					},
				},
				{
					ComponentSlug:    "test2",
					SubComponentSlug: "test2",
					PrometheusMonitor: &types.PrometheusMonitor{
						PrometheusLocation: "app.ci",
						Queries:            []types.PrometheusQuery{{Query: "up"}},
					},
				},
			},
			kubeconfigDir: tmpDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrometheusLocations(tt.components, tt.kubeconfigDir)
			diff := cmp.Diff(tt.expectedErr, err, cmp.Comparer(func(a, b error) bool {
				if a == nil && b == nil {
					return true
				}
				if a == nil || b == nil {
					return false
				}
				aErr := a.Error()
				bErr := b.Error()
				// For errors that contain paths (like kubeconfig file not found),
				// check if one error message contains the other (symmetric check)
				if strings.Contains(aErr, "at") && strings.Contains(bErr, "at") {
					return strings.Contains(aErr, bErr) || strings.Contains(bErr, aErr)
				}
				return aErr == bErr
			}))
			if diff != "" {
				t.Errorf("validatePrometheusLocations() error mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
