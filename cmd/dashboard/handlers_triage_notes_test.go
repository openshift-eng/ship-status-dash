package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
)

func configWithOwners() *types.DashboardConfig {
	return &types.DashboardConfig{
		Components: []*types.Component{
			{
				Name: "Alpha", Slug: "alpha", ShipTeam: "team-a",
				Owners: []types.Owner{{User: "admin-user"}},
				Subcomponents: []types.SubComponent{
					{Name: "One", Slug: "one"},
				},
			},
		},
	}
}

type testHandlerDeps struct {
	cfg            *types.DashboardConfig
	om             outage.OutageManager
	triageNoteRepo *repositories.MockTriageNoteRepository
	outageLinkRepo *repositories.MockOutageLinkRepository
}

func newTestHandlersWithDeps(t *testing.T, deps testHandlerDeps) *Handlers {
	t.Helper()
	cfgManager, err := config.NewManager("", func(string) (*types.DashboardConfig, error) {
		return deps.cfg, nil
	}, logrus.New(), time.Second)
	require.NoError(t, err)
	cfgManager.Get()

	pingRepo := &repositories.MockComponentPingRepository{}
	cache := auth.NewGroupMembershipCache(logrus.New())
	return NewHandlers(logrus.New(), cfgManager, deps.om, pingRepo, deps.triageNoteRepo, deps.outageLinkRepo, cache)
}

func withRouteVars(r *http.Request, vars map[string]string) *http.Request {
	return mux.SetURLVars(r, vars)
}

func withUser(r *http.Request, user string) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func TestResolveTriageNote_AdminAuthorized(t *testing.T) {
	noteRepo := &repositories.MockTriageNoteRepository{
		TriageNoteByID: &types.TriageNote{
			Body:   "test note",
			Author: "someone-else",
		},
	}
	noteRepo.TriageNoteByID.ID = 5

	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: noteRepo,
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	r := httptest.NewRequest(http.MethodPatch, "/api/components/alpha/one/outages/1/triage-notes/5", nil)
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
		"noteId":           "5",
	})
	r = withUser(r, "admin-user")
	w := httptest.NewRecorder()

	outageID, noteID, activeUser, _, ok := h.resolveTriageNote(w, r)
	assert.True(t, ok)
	assert.Equal(t, uint(1), outageID)
	assert.Equal(t, uint(5), noteID)
	assert.Equal(t, "admin-user", activeUser)
}

func TestResolveTriageNote_AuthorAuthorized(t *testing.T) {
	noteRepo := &repositories.MockTriageNoteRepository{
		TriageNoteByID: &types.TriageNote{
			Body:   "my note",
			Author: "note-author",
		},
	}
	noteRepo.TriageNoteByID.ID = 7

	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: noteRepo,
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	r := httptest.NewRequest(http.MethodPatch, "/api/components/alpha/one/outages/1/triage-notes/7", nil)
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
		"noteId":           "7",
	})
	r = withUser(r, "note-author")
	w := httptest.NewRecorder()

	_, _, _, _, ok := h.resolveTriageNote(w, r)
	assert.True(t, ok)
}

func TestResolveTriageNote_UnauthorizedUser(t *testing.T) {
	noteRepo := &repositories.MockTriageNoteRepository{
		TriageNoteByID: &types.TriageNote{
			Body:   "someone else's note",
			Author: "other-user",
		},
	}
	noteRepo.TriageNoteByID.ID = 9

	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: noteRepo,
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	r := httptest.NewRequest(http.MethodPatch, "/api/components/alpha/one/outages/1/triage-notes/9", nil)
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
		"noteId":           "9",
	})
	r = withUser(r, "random-user")
	w := httptest.NewRecorder()

	_, _, _, _, ok := h.resolveTriageNote(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestResolveTriageNote_NoteNotFound(t *testing.T) {
	noteRepo := &repositories.MockTriageNoteRepository{
		GetTriageNoteError: gorm.ErrRecordNotFound,
	}

	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: noteRepo,
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	r := httptest.NewRequest(http.MethodPatch, "/api/components/alpha/one/outages/1/triage-notes/99", nil)
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
		"noteId":           "99",
	})
	r = withUser(r, "admin-user")
	w := httptest.NewRecorder()

	_, _, _, _, ok := h.resolveTriageNote(w, r)
	assert.False(t, ok)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddTriageNoteJSON_EmptyBody(t *testing.T) {
	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: &repositories.MockTriageNoteRepository{},
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	body := `{"body": ""}`
	r := httptest.NewRequest(http.MethodPost, "/api/components/alpha/one/outages/1/triage-notes", bytes.NewBufferString(body))
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
	})
	r = withUser(r, "admin-user")
	w := httptest.NewRecorder()

	h.AddTriageNoteJSON(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddOutageLinkJSON_InvalidURL(t *testing.T) {
	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: &repositories.MockTriageNoteRepository{},
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	body, _ := json.Marshal(types.OutageLinkRequest{URL: "not-a-url", LinkType: "rca"})
	r := httptest.NewRequest(http.MethodPost, "/api/components/alpha/one/outages/1/links", bytes.NewBuffer(body))
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
	})
	r = withUser(r, "admin-user")
	w := httptest.NewRecorder()

	h.AddOutageLinkJSON(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddOutageLinkJSON_InvalidLinkType(t *testing.T) {
	h := newTestHandlersWithDeps(t, testHandlerDeps{
		cfg:            configWithOwners(),
		om:             &outage.MockOutageManager{},
		triageNoteRepo: &repositories.MockTriageNoteRepository{},
		outageLinkRepo: &repositories.MockOutageLinkRepository{},
	})

	body, _ := json.Marshal(types.OutageLinkRequest{URL: "https://example.com", LinkType: "invalid_type"})
	r := httptest.NewRequest(http.MethodPost, "/api/components/alpha/one/outages/1/links", bytes.NewBuffer(body))
	r = withRouteVars(r, map[string]string{
		"componentName":    "alpha",
		"subComponentName": "one",
		"outageId":         "1",
	})
	r = withUser(r, "admin-user")
	w := httptest.NewRecorder()

	h.AddOutageLinkJSON(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
