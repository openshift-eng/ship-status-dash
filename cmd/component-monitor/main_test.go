package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoadAndValidateConfigFrequency(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name:    "missing instance frequency",
			yaml:    "components: []\n",
			wantErr: "failed to parse frequency",
		},
		{
			name: "invalid instance frequency",
			yaml: `frequency: not-a-duration
components: []
`,
			wantErr: "failed to parse frequency",
		},
		{
			name: "non-positive instance frequency",
			yaml: `frequency: 0s
components: []
`,
			wantErr: "frequency must be a positive duration",
		},
		{
			name: "invalid entry frequency",
			yaml: `frequency: 20s
components:
  - component_slug: trt-incidents
    sub_component_slug: incidents
    frequency: bogus
    jira_monitor:
      url: https://redhat.atlassian.net
      jql: labels = trt-incident
`,
			wantErr: "failed to parse frequency for component trt-incidents/incidents",
		},
		{
			name: "entry frequency less than instance frequency",
			yaml: `frequency: 5m
components:
  - component_slug: trt-incidents
    sub_component_slug: incidents
    frequency: 20s
    jira_monitor:
      url: https://redhat.atlassian.net
      jql: labels = trt-incident
`,
			wantErr: "is less than instance frequency",
		},
		{
			name: "http retry_after greater than resolved frequency",
			yaml: `frequency: 20s
components:
  - component_slug: prow
    sub_component_slug: deck
    frequency: 1m
    http_monitor:
      url: http://example.com
      code: 200
      retry_after: 90s
`,
			wantErr: "retry after duration is greater than frequency",
		},
		{
			name: "same sub-component mismatched frequencies",
			yaml: `frequency: 20s
components:
  - component_slug: build-farm
    sub_component_slug: build01
    frequency: 1m
    http_monitor:
      url: http://example.com
      code: 200
      retry_after: 5s
  - component_slug: build-farm
    sub_component_slug: build01
    frequency: 2m
    prometheus_monitor:
      prometheus_location:
        url: http://localhost:9090
      queries:
        - query: up
`,
			wantErr: "frequency mismatch for component build-farm/build01",
		},
		{
			name: "valid entry frequency override",
			yaml: `frequency: 20s
components:
  - component_slug: trt-incidents
    sub_component_slug: incidents
    frequency: 5m
    jira_monitor:
      url: https://redhat.atlassian.net
      jql: labels = trt-incident
`,
		},
		{
			name: "same sub-component matching frequencies",
			yaml: `frequency: 20s
components:
  - component_slug: prow
    sub_component_slug: deck
    frequency: 1m
    http_monitor:
      url: http://example.com
      code: 200
      retry_after: 5s
  - component_slug: prow
    sub_component_slug: deck
    frequency: 1m
    prometheus_monitor:
      prometheus_location:
        url: http://localhost:9090
      queries:
        - query: up
`,
		},
	}

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			_, err := loadAndValidateConfig(log, path, "")
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
