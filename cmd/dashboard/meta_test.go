package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/types"
)

func testConfig() *types.DashboardConfig {
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Prow", Slug: "prow", Description: "Prow CI system",
				Subcomponents: []types.SubComponent{
					{Name: "Deck", Slug: "deck", Description: "Prow web UI"},
					{Name: "Tide", Slug: "tide"},
				},
			},
			{
				Name: "Build Clusters", Slug: "build-clusters", Description: "Build farm clusters",
				Subcomponents: []types.SubComponent{
					{Name: "Build01", Slug: "build01"},
				},
			},
		},
	}
}

func newTestConfigManager(t *testing.T, cfg *types.DashboardConfig) *config.Manager[types.DashboardConfig] {
	t.Helper()
	cm, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	require.NoError(t, err)
	return cm
}

func TestResolveMetadata(t *testing.T) {
	cfg := testConfig()
	cm := newTestConfigManager(t, cfg)
	logger := logrus.New()

	tests := []struct {
		name     string
		path     string
		wantMeta pageMetadata
	}{
		{
			name:     "root path",
			path:     "/",
			wantMeta: defaultMetadata(),
		},
		{
			name: "status history",
			path: "/status-history",
			wantMeta: pageMetadata{
				Title:       "Status History - SHIP Status Dashboard",
				Description: "Historical status timeline for all OpenShift CI components.",
			},
		},
		{
			name: "tag page",
			path: "/tags/build-farm",
			wantMeta: pageMetadata{
				Title:       "Tag: Build Farm - SHIP Status Dashboard",
				Description: "Components tagged with Build Farm on SHIP Status Dashboard.",
			},
		},
		{
			name: "team page",
			path: "/team/team-a",
			wantMeta: pageMetadata{
				Title:       "Team: Team A - SHIP Status Dashboard",
				Description: "Components managed by Team A on SHIP Status Dashboard.",
			},
		},
		{
			name: "external page",
			path: "/pages/spc-dashboard",
			wantMeta: pageMetadata{
				Title:       "Spc Dashboard - SHIP Status Dashboard",
				Description: "View the Spc Dashboard page on SHIP Status Dashboard.",
			},
		},
		{
			name: "component page with config match",
			path: "/prow",
			wantMeta: pageMetadata{
				Title:       "Prow - SHIP Status Dashboard",
				Description: "Prow CI system",
			},
		},
		{
			name: "component page with multi-word slug",
			path: "/build-clusters",
			wantMeta: pageMetadata{
				Title:       "Build Clusters - SHIP Status Dashboard",
				Description: "Build farm clusters",
			},
		},
		{
			name:     "unknown component slug falls back to default",
			path:     "/nonexistent",
			wantMeta: defaultMetadata(),
		},
		{
			name: "sub-component page with description",
			path: "/prow/deck",
			wantMeta: pageMetadata{
				Title:       "Deck (Prow) - SHIP Status Dashboard",
				Description: "Prow web UI",
			},
		},
		{
			name: "sub-component page without description",
			path: "/prow/tide",
			wantMeta: pageMetadata{
				Title:       "Tide (Prow) - SHIP Status Dashboard",
				Description: "Status and outage information for Tide (Prow).",
			},
		},
		{
			name: "trailing slash is ignored",
			path: "/prow/",
			wantMeta: pageMetadata{
				Title:       "Prow - SHIP Status Dashboard",
				Description: "Prow CI system",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			got := resolveMetadata(r, cm, nil, logger)
			assert.Equal(t, tt.wantMeta, got)
		})
	}
}

func TestResolveOutageMetadata(t *testing.T) {
	cfg := testConfig()
	logger := logrus.New()

	tests := []struct {
		name            string
		compSlug        string
		subSlug         string
		outageID        string
		outageManager   outage.OutageManager
		wantTitle       string
		wantDescContain []string
		maxDescLen      int
	}{
		{
			name:     "active outage",
			compSlug: "prow", subSlug: "deck", outageID: "42",
			outageManager: &outage.MockOutageManager{
				GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
					return &types.Outage{
						Severity:    types.SeverityDown,
						Description: "Deck is not responding to health checks",
					}, nil
				},
			},
			wantTitle:       "Outage #42 - Deck (Prow) - SHIP Status Dashboard",
			wantDescContain: []string{"Active", "Down", "Deck (Prow)", "Deck is not responding"},
		},
		{
			name:     "resolved outage",
			compSlug: "prow", subSlug: "deck", outageID: "10",
			outageManager: &outage.MockOutageManager{
				GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
					return &types.Outage{
						Severity:    types.SeverityDegraded,
						EndTime:     sql.NullTime{Time: time.Now(), Valid: true},
						Description: "Intermittent failures",
					}, nil
				},
			},
			wantDescContain: []string{"Resolved", "Degraded"},
		},
		{
			name:     "long description is truncated",
			compSlug: "prow", subSlug: "deck", outageID: "1",
			outageManager: &outage.MockOutageManager{
				GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
					return &types.Outage{
						Severity:    types.SeverityDown,
						Description: strings.Repeat("x", 200),
					}, nil
				},
			},
			wantDescContain: []string{"..."},
			maxDescLen:      300,
		},
		{
			name:     "nil outage manager returns fallback",
			compSlug: "prow", subSlug: "deck", outageID: "5",
			outageManager:   nil,
			wantTitle:       "Outage #5 - Deck (Prow) - SHIP Status Dashboard",
			wantDescContain: []string{"Outage details for Deck"},
		},
		{
			name:     "invalid outage ID returns fallback",
			compSlug: "prow", subSlug: "deck", outageID: "abc",
			outageManager: nil,
			wantTitle:     "Outage - Deck (Prow) - SHIP Status Dashboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := resolveOutageMetadata(tt.compSlug, tt.subSlug, tt.outageID, cfg, tt.outageManager, logger)
			if tt.wantTitle != "" {
				assert.Equal(t, tt.wantTitle, meta.Title)
			}
			for _, s := range tt.wantDescContain {
				assert.Contains(t, meta.Description, s)
			}
			if tt.maxDescLen > 0 {
				assert.LessOrEqual(t, len(meta.Description), tt.maxDescLen)
			}
		})
	}
}

func TestInjectMetadata(t *testing.T) {
	indexHTML := []byte(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="description" content="SHIP Status Dashboard" />
    <title>SHIP Status Dashboard</title>
  </head>
  <body><div id="root"></div></body>
</html>`)

	tests := []struct {
		name           string
		meta           pageMetadata
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "default metadata preserves original values plus OG tags",
			meta: defaultMetadata(),
			wantContains: []string{
				"<title>SHIP Status Dashboard</title>",
				`og:title`, `og:description`, `og:type`, `og:site_name`,
			},
		},
		{
			name: "custom metadata injects title and description",
			meta: pageMetadata{
				Title:       "Outage #42 - Deck (Prow) - SHIP Status Dashboard",
				Description: "Active | Severity: Down | Deck (Prow)",
			},
			wantContains: []string{
				"<title>Outage #42 - Deck (Prow) - SHIP Status Dashboard</title>",
				`content="Active | Severity: Down | Deck (Prow)"`,
				`og:title`,
			},
		},
		{
			name: "HTML special characters are escaped",
			meta: pageMetadata{
				Title:       `Test & "quotes" <tags>`,
				Description: `A <b>bold</b> & "quoted" description`,
			},
			wantContains:   []string{"Test &amp; &#34;quotes&#34; &lt;tags&gt;"},
			wantNotContain: []string{`<b>bold</b>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(injectMetadata(indexHTML, tt.meta))
			for _, s := range tt.wantContains {
				assert.Contains(t, result, s)
			}
			for _, s := range tt.wantNotContain {
				assert.NotContains(t, result, s)
			}
		})
	}
}
