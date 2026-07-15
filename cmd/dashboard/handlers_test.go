package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

// newTestHandlers returns Handlers backed by cfg, the given outage manager, mock pings, and a mock group cache.
func newTestHandlers(t *testing.T, cfg *types.DashboardConfig, om outage.OutageManager) *Handlers {
	return newTestHandlersWithGroups(t, cfg, om, nil)
}

// newTestHandlersWithGroups is like newTestHandlers but pre-populates group membership.
func newTestHandlersWithGroups(t *testing.T, cfg *types.DashboardConfig, om outage.OutageManager, groups map[string][]string) *Handlers {
	t.Helper()
	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return cfg, nil
	}, logrus.New(), time.Second)
	require.NoError(t, err)
	cfgManager.Get()

	pingRepo := &repositories.MockComponentPingRepository{}
	triageNoteRepo := &repositories.MockTriageNoteRepository{}
	outageLinkRepo := &repositories.MockOutageLinkRepository{}
	cache := &auth.MockGroupMembershipProvider{Groups: groups}
	return NewHandlers(logrus.New(), cfgManager, om, pingRepo, triageNoteRepo, outageLinkRepo, cache)
}

// minimalDashboardConfig is a tiny valid config (one component, one sub-component) for handler tests.
func minimalDashboardConfig() *types.DashboardConfig {
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha", ShipTeam: "team-a",
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
			},
		},
	}
}

func TestIsUserAuthorizedForComponent(t *testing.T) {
	component := &types.Component{
		Name: "Test", Slug: "test",
		Owners: []types.Owner{
			{User: "developer"},
			{RoverGroup: "test-group"},
			{ServiceAccount: "system:serviceaccount:ship-status:chai-bot"},
		},
	}

	tests := []struct {
		name       string
		user       string
		authorized bool
	}{
		{
			name:       "user owner is authorized",
			user:       "developer",
			authorized: true,
		},
		{
			name:       "service account owner is authorized",
			user:       "system:serviceaccount:ship-status:chai-bot",
			authorized: true,
		},
		{
			name:       "rover group member is authorized",
			user:       "groupuser",
			authorized: true,
		},
		{
			name:       "unlisted user is not authorized",
			user:       "stranger",
			authorized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &types.DashboardConfig{Components: []*types.Component{component}}
			groups := map[string][]string{"test-group": {"groupuser", "anotheruser"}}
			h := newTestHandlersWithGroups(t, cfg, &outage.MockOutageManager{}, groups)
			assert.Equal(t, tt.authorized, h.IsUserAuthorizedForComponent(tt.user, component))
		})
	}
}

func TestGetComponentStatusJSON_CriticalSubComponent(t *testing.T) {
	now := time.Now()
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha",
				Subcomponents: []types.SubComponent{
					{Name: "Critical One", Slug: "critical-one", Critical: true},
					{Name: "Critical Three", Slug: "critical-three", Critical: true},
					{Name: "Normal Two", Slug: "normal-two"},
				},
			},
		},
	}

	confirmedOutage := func(sub string, sev types.Severity) types.Outage {
		return types.Outage{
			ComponentName:    "alpha",
			SubComponentName: sub,
			Severity:         sev,
			ConfirmedAt:      sql.NullTime{Time: now, Valid: true},
		}
	}

	tests := []struct {
		name             string
		outages          []types.Outage
		suspectedOutages []types.Outage
		expectedStatus   types.Status
	}{
		{
			name:           "critical sub-component down bypasses Partial",
			outages:        []types.Outage{confirmedOutage("critical-one", types.SeverityDown)},
			expectedStatus: types.StatusDown,
		},
		{
			name:           "critical sub-component degraded bypasses Partial",
			outages:        []types.Outage{confirmedOutage("critical-one", types.SeverityDegraded)},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "suspected outage on critical sub-component shows Suspected",
			suspectedOutages: []types.Outage{
				{ComponentName: "alpha", SubComponentName: "critical-one", Severity: types.SeveritySuspected},
			},
			expectedStatus: types.StatusSuspected,
		},
		{
			name: "multiple critical sub-components: most severe wins",
			outages: []types.Outage{
				confirmedOutage("critical-one", types.SeverityDown),
				confirmedOutage("critical-three", types.SeverityDegraded),
			},
			expectedStatus: types.StatusDown,
		},
		{
			name: "all sub-components affected uses most severe status",
			outages: []types.Outage{
				confirmedOutage("critical-one", types.SeverityDegraded),
				confirmedOutage("normal-two", types.SeverityDown),
				confirmedOutage("critical-three", types.SeverityDegraded),
			},
			expectedStatus: types.StatusDown,
		},
		{
			name:           "non-critical sub-component only shows Partial",
			outages:        []types.Outage{confirmedOutage("normal-two", types.SeverityDown)},
			expectedStatus: types.StatusPartial,
		},
		{
			name:           "no outages shows Healthy",
			outages:        []types.Outage{},
			expectedStatus: types.StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOM := &outage.MockOutageManager{}
			mockOM.GetActiveOutagesForComponentFn = func(slug string) ([]types.Outage, error) {
				return tt.outages, nil
			}
			mockOM.GetActiveSuspectedOutagesForComponentFn = func(slug string) ([]types.Outage, error) {
				return tt.suspectedOutages, nil
			}

			h := newTestHandlers(t, cfg, mockOM)
			got, err := h.getComponentStatus(cfg.Components[0], logrus.NewEntry(logrus.New()))
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, got.Status)
		})
	}
}

func TestGetOutagesDuringJSON(t *testing.T) {
	cfg := minimalDashboardConfig()
	t0 := time.Date(2025, 4, 1, 10, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)

	mockOM := &outage.MockOutageManager{}
	mockOM.GetOutagesDuringFn = func(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
		if len(refs) == 0 {
			return []types.Outage{}, nil
		}
		if len(refs) == 1 && refs[0].ComponentSlug == "alpha" && refs[0].SubSlug == "one" &&
			queryStart.Equal(t1) && queryEnd.Equal(t1) {
			return []types.Outage{{
				ComponentName:    "alpha",
				SubComponentName: "one",
				Severity:         types.SeverityDown,
				StartTime:        t0,
				Description:      "x",
				DiscoveredFrom:   "test",
				CreatedBy:        "u",
			}}, nil
		}
		return []types.Outage{}, nil
	}

	h := newTestHandlers(t, cfg, mockOM)

	intPtr := func(n int) *int { return &n }

	tests := []struct {
		name            string
		query           string
		wantCode        int
		wantOutageCount *int
	}{
		{
			name:            "200_with_start_only",
			query:           "start=" + t1.UTC().Format(time.RFC3339),
			wantCode:        http.StatusOK,
			wantOutageCount: intPtr(1),
		},
		{
			name:     "400_no_time_params",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "400_sub_without_component",
			query:    "start=" + t0.Format(time.RFC3339) + "&subComponentName=one",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "400_start_after_end",
			query:    "start=" + t2.Format(time.RFC3339) + "&end=" + t0.Format(time.RFC3339),
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "404_unknown_component",
			query:    "start=" + t0.Format(time.RFC3339) + "&componentName=nope",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "404_unknown_sub",
			query:    "start=" + t0.Format(time.RFC3339) + "&componentName=alpha&subComponentName=nope",
			wantCode: http.StatusNotFound,
		},
		{
			name:            "200_empty_when_tag_excludes",
			query:           "start=" + t1.Format(time.RFC3339) + "&tag=nonexistent-tag",
			wantCode:        http.StatusOK,
			wantOutageCount: intPtr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/api/outages/during"
			if tt.query != "" {
				path += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.GetOutagesDuringJSON(rec, req)
			res := rec.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantCode, res.StatusCode)
			if tt.wantOutageCount == nil {
				return
			}
			var got []types.Outage
			require.NoError(t, json.NewDecoder(res.Body).Decode(&got))
			assert.Len(t, got, *tt.wantOutageCount)
		})
	}
}

func requestWithUser(method, path string, body []byte, user string) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	ctx := context.WithValue(req.Context(), userContextKey, user)
	return req.WithContext(ctx)
}

func TestCreateOutageJSON_Delegation(t *testing.T) {
	const (
		mcpSA      = "system:serviceaccount:ship-status:mcp-server"
		authorUser = "jdoe"
	)
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha",
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
				Owners: []types.Owner{{User: authorUser}},
			},
		},
		TrustedDelegators: []string{mcpSA},
	}

	tests := []struct {
		name           string
		user           string
		body           types.UpsertOutageRequest
		wantCode       int
		wantCreatedBy  string
		wantDiscovered string
	}{
		{
			name: "trusted delegator with authorized acting_for succeeds",
			user: mcpSA,
			body: types.UpsertOutageRequest{
				Severity:       strPtr("Down"),
				StartTime:      timePtr(time.Now()),
				Description:    strPtr("test outage"),
				DiscoveredFrom: strPtr("chat"),
				ActingFor:      strPtr(authorUser),
			},
			wantCode:       http.StatusCreated,
			wantCreatedBy:  authorUser,
			wantDiscovered: "chat",
		},
		{
			name: "trusted delegator with unauthorized acting_for gets 403",
			user: mcpSA,
			body: types.UpsertOutageRequest{
				Severity:       strPtr("Down"),
				StartTime:      timePtr(time.Now()),
				Description:    strPtr("test outage"),
				DiscoveredFrom: strPtr("chat"),
				ActingFor:      strPtr("stranger"),
			},
			wantCode: http.StatusForbidden,
		},
		{
			name: "trusted delegator without acting_for gets 400",
			user: mcpSA,
			body: types.UpsertOutageRequest{
				Severity:       strPtr("Down"),
				StartTime:      timePtr(time.Now()),
				Description:    strPtr("test outage"),
				DiscoveredFrom: strPtr("chat"),
			},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "regular authorized user without acting_for succeeds normally",
			user: authorUser,
			body: types.UpsertOutageRequest{
				Severity:       strPtr("Down"),
				StartTime:      timePtr(time.Now()),
				Description:    strPtr("test outage"),
				DiscoveredFrom: strPtr("manual"),
			},
			wantCode:      http.StatusCreated,
			wantCreatedBy: authorUser,
		},
		{
			name: "unknown SA (not in TrustedDelegators) is treated as regular user",
			user: "system:serviceaccount:other:unknown",
			body: types.UpsertOutageRequest{
				Severity:       strPtr("Down"),
				StartTime:      timePtr(time.Now()),
				Description:    strPtr("test outage"),
				DiscoveredFrom: strPtr("manual"),
			},
			wantCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOM := &outage.MockOutageManager{}
			h := newTestHandlers(t, cfg, mockOM)

			bodyBytes, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := requestWithUser(http.MethodPost, "/api/components/alpha/sub-components/one/outages", bodyBytes, tt.user)
			req = mux.SetURLVars(req, map[string]string{
				"componentName":    "alpha",
				"subComponentName": "one",
			})

			rec := httptest.NewRecorder()
			h.CreateOutageJSON(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)

			if tt.wantCreatedBy != "" {
				require.Len(t, mockOM.CreatedOutages, 1)
				assert.Equal(t, tt.wantCreatedBy, mockOM.CreatedOutages[0].Outage.CreatedBy)
			}
			if tt.wantDiscovered != "" {
				require.Len(t, mockOM.CreatedOutages, 1)
				assert.Equal(t, tt.wantDiscovered, mockOM.CreatedOutages[0].Outage.DiscoveredFrom)
			}
		})
	}
}

func TestUpdateOutageJSON_Delegation(t *testing.T) {
	const (
		mcpSA      = "system:serviceaccount:ship-status:mcp-server"
		authorUser = "jdoe"
	)
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha",
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
				Owners: []types.Owner{{User: authorUser}},
			},
		},
		TrustedDelegators: []string{mcpSA},
	}

	existingOutage := &types.Outage{
		ComponentName:    "alpha",
		SubComponentName: "one",
		Severity:         types.SeverityDown,
		Description:      "existing outage",
		DiscoveredFrom:   "manual",
		CreatedBy:        authorUser,
		StartTime:        time.Now().Add(-time.Hour),
	}
	existingOutage.ID = 42

	tests := []struct {
		name          string
		user          string
		body          types.UpsertOutageRequest
		wantCode      int
		wantAuditUser string
	}{
		{
			name: "trusted delegator with authorized acting_for can update",
			user: mcpSA,
			body: types.UpsertOutageRequest{
				Severity:  strPtr("Degraded"),
				ActingFor: strPtr(authorUser),
			},
			wantCode:      http.StatusOK,
			wantAuditUser: authorUser,
		},
		{
			name: "trusted delegator with unauthorized acting_for gets 403",
			user: mcpSA,
			body: types.UpsertOutageRequest{
				Severity:  strPtr("Degraded"),
				ActingFor: strPtr("stranger"),
			},
			wantCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var auditUser string
			mockOM := &outage.MockOutageManager{}
			mockOM.GetOutageByIDFn = func(comp, sub string, id uint) (*types.Outage, error) {
				outageCopy := *existingOutage
				return &outageCopy, nil
			}
			mockOM.UpdateOutageFn = func(o *types.Outage, user string) error {
				auditUser = user
				return nil
			}
			h := newTestHandlers(t, cfg, mockOM)

			bodyBytes, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := requestWithUser(http.MethodPut, "/api/components/alpha/sub-components/one/outages/42", bodyBytes, tt.user)
			req = mux.SetURLVars(req, map[string]string{
				"componentName":    "alpha",
				"subComponentName": "one",
				"outageId":         "42",
			})

			rec := httptest.NewRecorder()
			h.UpdateOutageJSON(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
			if tt.wantAuditUser != "" {
				assert.Equal(t, tt.wantAuditUser, auditUser)
			}
		})
	}
}

func TestDeleteOutage_Delegation(t *testing.T) {
	const (
		mcpSA      = "system:serviceaccount:ship-status:mcp-server"
		authorUser = "jdoe"
	)
	cfg := &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha",
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
				Owners: []types.Owner{{User: authorUser}},
			},
		},
		TrustedDelegators: []string{mcpSA},
	}

	existingOutage := &types.Outage{
		ComponentName:    "alpha",
		SubComponentName: "one",
		Severity:         types.SeverityDown,
		StartTime:        time.Now().Add(-time.Hour),
	}
	existingOutage.ID = 42

	tests := []struct {
		name     string
		user     string
		body     *types.DelegatedActionRequest
		wantCode int
	}{
		{
			name:     "trusted delegator with authorized acting_for can delete",
			user:     mcpSA,
			body:     &types.DelegatedActionRequest{ActingFor: strPtr(authorUser)},
			wantCode: http.StatusNoContent,
		},
		{
			name:     "trusted delegator without acting_for gets 400",
			user:     mcpSA,
			body:     nil,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOM := &outage.MockOutageManager{}
			mockOM.GetOutageByIDFn = func(comp, sub string, id uint) (*types.Outage, error) {
				return existingOutage, nil
			}
			mockOM.DeleteOutageFn = func(o *types.Outage, user string) error {
				return nil
			}
			h := newTestHandlers(t, cfg, mockOM)

			var bodyBytes []byte
			if tt.body != nil {
				bodyBytes, _ = json.Marshal(tt.body)
			}

			req := requestWithUser(http.MethodDelete, "/api/components/alpha/sub-components/one/outages/42", bodyBytes, tt.user)
			req = mux.SetURLVars(req, map[string]string{
				"componentName":    "alpha",
				"subComponentName": "one",
				"outageId":         "42",
			})

			rec := httptest.NewRecorder()
			h.DeleteOutage(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestResolveActingUser(t *testing.T) {
	const mcpSA = "system:serviceaccount:ship-status:mcp-server"
	cfg := &types.DashboardConfig{
		Components:        []*types.Component{},
		TrustedDelegators: []string{mcpSA},
	}

	h := newTestHandlers(t, cfg, &outage.MockOutageManager{})
	logger := logrus.NewEntry(logrus.New())

	t.Run("non-delegator returns self", func(t *testing.T) {
		rec := httptest.NewRecorder()
		user, delegated, ok := h.resolveActingUser(rec, "regular-user", nil, logger)
		assert.True(t, ok)
		assert.False(t, delegated)
		assert.Equal(t, "regular-user", user)
	})

	t.Run("trusted delegator with acting_for returns acting_for", func(t *testing.T) {
		rec := httptest.NewRecorder()
		user, delegated, ok := h.resolveActingUser(rec, mcpSA, strPtr("target-user"), logger)
		assert.True(t, ok)
		assert.True(t, delegated)
		assert.Equal(t, "target-user", user)
	})

	t.Run("trusted delegator without acting_for returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		_, _, ok := h.resolveActingUser(rec, mcpSA, nil, logger)
		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("trusted delegator with empty acting_for returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		_, _, ok := h.resolveActingUser(rec, mcpSA, strPtr("  "), logger)
		assert.False(t, ok)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("acting_for is trimmed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		user, _, ok := h.resolveActingUser(rec, mcpSA, strPtr(" target-user "), logger)
		assert.True(t, ok)
		assert.Equal(t, "target-user", user)
	})
}

func strPtr(s string) *string        { return &s }
func timePtr(t time.Time) *time.Time { return &t }
