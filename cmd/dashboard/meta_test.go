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

	t.Run("active outage", func(t *testing.T) {
		mock := &outage.MockOutageManager{
			GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
				return &types.Outage{
					Severity:    types.SeverityDown,
					Description: "Deck is not responding to health checks",
				}, nil
			},
		}
		meta := resolveOutageMetadata("prow", "deck", "42", cfg, mock, logger)
		assert.Equal(t, "Outage #42 - Deck (Prow) - SHIP Status Dashboard", meta.Title)
		assert.Contains(t, meta.Description, "Active")
		assert.Contains(t, meta.Description, "Down")
		assert.Contains(t, meta.Description, "Deck (Prow)")
		assert.Contains(t, meta.Description, "Deck is not responding")
	})

	t.Run("resolved outage", func(t *testing.T) {
		mock := &outage.MockOutageManager{
			GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
				return &types.Outage{
					Severity:    types.SeverityDegraded,
					EndTime:     sql.NullTime{Time: time.Now(), Valid: true},
					Description: "Intermittent failures",
				}, nil
			},
		}
		meta := resolveOutageMetadata("prow", "deck", "10", cfg, mock, logger)
		assert.Contains(t, meta.Description, "Resolved")
		assert.Contains(t, meta.Description, "Degraded")
	})

	t.Run("long description is truncated", func(t *testing.T) {
		longDesc := strings.Repeat("x", 200)
		mock := &outage.MockOutageManager{
			GetOutageByIDFn: func(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
				return &types.Outage{
					Severity:    types.SeverityDown,
					Description: longDesc,
				}, nil
			},
		}
		meta := resolveOutageMetadata("prow", "deck", "1", cfg, mock, logger)
		assert.LessOrEqual(t, len(meta.Description), 300)
		assert.Contains(t, meta.Description, "...")
	})

	t.Run("nil outage manager returns fallback", func(t *testing.T) {
		meta := resolveOutageMetadata("prow", "deck", "5", cfg, nil, logger)
		assert.Equal(t, "Outage #5 - Deck (Prow) - SHIP Status Dashboard", meta.Title)
		assert.Contains(t, meta.Description, "Outage details for Deck")
	})

	t.Run("invalid outage ID returns fallback", func(t *testing.T) {
		meta := resolveOutageMetadata("prow", "deck", "abc", cfg, nil, logger)
		assert.Equal(t, "Outage - Deck (Prow) - SHIP Status Dashboard", meta.Title)
	})
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

	t.Run("default metadata preserves original values plus OG tags", func(t *testing.T) {
		result := string(injectMetadata(indexHTML, defaultMetadata()))
		assert.Contains(t, result, "<title>SHIP Status Dashboard</title>")
		assert.Contains(t, result, `og:title`)
		assert.Contains(t, result, `og:description`)
		assert.Contains(t, result, `og:type`)
		assert.Contains(t, result, `og:site_name`)
	})

	t.Run("custom metadata injects title and description", func(t *testing.T) {
		meta := pageMetadata{
			Title:       "Outage #42 - Deck (Prow) - SHIP Status Dashboard",
			Description: "Active | Severity: Down | Deck (Prow)",
		}
		result := string(injectMetadata(indexHTML, meta))
		assert.Contains(t, result, "<title>Outage #42 - Deck (Prow) - SHIP Status Dashboard</title>")
		assert.Contains(t, result, `content="Active | Severity: Down | Deck (Prow)"`)
		assert.Contains(t, result, `og:title`)
	})

	t.Run("HTML special characters are escaped", func(t *testing.T) {
		meta := pageMetadata{
			Title:       `Test & "quotes" <tags>`,
			Description: `A <b>bold</b> & "quoted" description`,
		}
		result := string(injectMetadata(indexHTML, meta))
		assert.Contains(t, result, "Test &amp; &#34;quotes&#34; &lt;tags&gt;")
		assert.NotContains(t, result, `<b>bold</b>`)
	})
}

func TestDeslugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"build-farm", "Build Farm"},
		{"prow", "Prow"},
		{"build-clusters", "Build Clusters"},
		{"spc-dashboard", "Spc Dashboard"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, deslugify(tt.input))
		})
	}
}
