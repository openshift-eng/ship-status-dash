package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/types"
)

type jiraTestServer struct {
	searchHits   int
	searchStatus int
	searchBody   []byte
	mu           sync.Mutex
	sawAuth      bool
}

func (s *jiraTestServer) snapshot() (sawAuth bool, searchHits int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sawAuth, s.searchHits
}

func newJiraTestServer(t *testing.T, searchBody []byte) (*httptest.Server, *jiraTestServer) {
	t.Helper()
	state := &jiraTestServer{
		searchStatus: http.StatusOK,
		searchBody:   searchBody,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		if _, _, ok := r.BasicAuth(); ok {
			state.sawAuth = true
		}
		if r.Header.Get("Authorization") != "" {
			state.sawAuth = true
		}
		if r.Method == http.MethodGet && r.URL.Path == jiraSearchPath {
			state.searchHits++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(state.searchStatus)
			_, _ = w.Write(state.searchBody)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server, state
}

func jiraSearchJSON(t *testing.T, issues []map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"issues": issues, "isLast": true})
	if err != nil {
		t.Fatalf("marshal search json: %v", err)
	}
	return body
}

func collectJiraProbe(t *testing.T, prober *JiraProber) ProbeResult {
	t.Helper()
	results := make(chan ProbeResult, 1)
	prober.Probe(context.Background(), results)
	return <-results
}

func newTestJiraProber(baseURL string) *JiraProber {
	return NewJiraProber(
		"trt-incidents",
		"incidents",
		baseURL,
		"labels = trt-incident AND statusCategory != Done",
		types.SeverityDown,
		&http.Client{Timeout: 5 * time.Second},
	)
}

func jiraReason(baseURL, key, summary string) types.Reason {
	return types.Reason{
		Type:    types.CheckTypeJira,
		Check:   key,
		Results: summary,
		Links: []types.ReportedLink{{
			URL:      baseURL + "/browse/" + key,
			LinkType: types.LinkTypeJira,
		}},
	}
}

func TestJiraProber_Probe(t *testing.T) {
	tests := []struct {
		name              string
		issues            []map[string]any
		searchStatus      int
		wantSearchHits    int
		wantErrorContains string
		want              func(baseURL string) types.ComponentMonitorReportComponentStatus
	}{
		{
			name: "reports matching issues as per-reason outages",
			issues: []map[string]any{
				{"key": "TRT-100", "fields": map[string]any{"summary": "First incident"}},
				{"key": "TRT-200", "fields": map[string]any{"summary": "Second incident"}},
			},
			wantSearchHits: 1,
			want: func(baseURL string) types.ComponentMonitorReportComponentStatus {
				return types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "trt-incidents",
					SubComponentSlug: "incidents",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						jiraReason(baseURL, "TRT-100", "First incident"),
						jiraReason(baseURL, "TRT-200", "Second incident"),
					},
				}
			},
		},
		{
			name:           "healthy when no issues",
			wantSearchHits: 1,
			want: func(baseURL string) types.ComponentMonitorReportComponentStatus {
				return types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "trt-incidents",
					SubComponentSlug: "incidents",
					Status:           types.StatusHealthy,
				}
			},
		},
		{
			name:              "search HTTP error",
			searchStatus:      http.StatusUnauthorized,
			wantSearchHits:    1,
			wantErrorContains: "HTTP 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchBody := jiraSearchJSON(t, tt.issues)
			if tt.searchStatus != 0 && tt.searchStatus != http.StatusOK {
				searchBody = []byte(`{}`)
			}
			server, state := newJiraTestServer(t, searchBody)
			if tt.searchStatus != 0 {
				state.searchStatus = tt.searchStatus
			}

			prober := newTestJiraProber(server.URL)
			got := collectJiraProbe(t, prober)

			sawAuth, searchHits := state.snapshot()
			if sawAuth {
				t.Fatal("jira probe must not send credentials")
			}
			if tt.wantSearchHits != 0 && searchHits != tt.wantSearchHits {
				t.Errorf("search hits = %d, want %d", searchHits, tt.wantSearchHits)
			}

			if tt.wantErrorContains != "" {
				if got.Error == nil {
					t.Fatal("expected probe error")
				}
				if !strings.Contains(got.Error.Error(), tt.wantErrorContains) {
					t.Errorf("error = %v, want substring %q", got.Error, tt.wantErrorContains)
				}
				return
			}

			if got.Error != nil {
				t.Fatalf("unexpected probe error: %v", got.Error)
			}

			if tt.want == nil {
				return
			}
			if diff := cmp.Diff(tt.want(server.URL), got.ComponentMonitorReportComponentStatus); diff != "" {
				t.Errorf("probe status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
