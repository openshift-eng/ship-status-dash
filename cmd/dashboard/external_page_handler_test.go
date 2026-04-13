package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

func newTestExternalPageCache(url string, ttl time.Duration) *ExternalPageCache {
	return NewExternalPageCache(url, ttl, logrus.New())
}

func TestExternalPageCache_Get(t *testing.T) {
	t.Run("fetches content from upstream", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html><body>Hello</body></html>"))
		}))
		defer upstream.Close()

		cache := newTestExternalPageCache(upstream.URL, 1*time.Hour)
		content, err := cache.Get()
		if err != nil {
			t.Fatalf("Get() unexpected error: %v", err)
		}
		if diff := cmp.Diff("<html><body>Hello</body></html>", string(content)); diff != "" {
			t.Errorf("content mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns cached content without re-fetching", func(t *testing.T) {
		callCount := 0
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Write([]byte("<html>response</html>"))
		}))
		defer upstream.Close()

		cache := newTestExternalPageCache(upstream.URL, 1*time.Hour)

		if _, err := cache.Get(); err != nil {
			t.Fatalf("first Get() unexpected error: %v", err)
		}
		if _, err := cache.Get(); err != nil {
			t.Fatalf("second Get() unexpected error: %v", err)
		}
		if diff := cmp.Diff(1, callCount); diff != "" {
			t.Errorf("fetch count mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("refreshes after TTL expires", func(t *testing.T) {
		callCount := 0
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Write([]byte("<html>response</html>"))
		}))
		defer upstream.Close()

		cache := newTestExternalPageCache(upstream.URL, 1*time.Millisecond)

		if _, err := cache.Get(); err != nil {
			t.Fatalf("first Get() unexpected error: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
		if _, err := cache.Get(); err != nil {
			t.Fatalf("second Get() unexpected error: %v", err)
		}
		if diff := cmp.Diff(2, callCount); diff != "" {
			t.Errorf("fetch count mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("serves stale content on upstream error", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("<html>original</html>"))
		}))

		cache := newTestExternalPageCache(upstream.URL, 1*time.Millisecond)

		content, err := cache.Get()
		if err != nil {
			t.Fatalf("initial Get() unexpected error: %v", err)
		}
		if diff := cmp.Diff("<html>original</html>", string(content)); diff != "" {
			t.Errorf("initial content mismatch (-want +got):\n%s", diff)
		}

		upstream.Close()
		time.Sleep(5 * time.Millisecond)

		content, err = cache.Get()
		if err != nil {
			t.Fatalf("stale Get() unexpected error: %v", err)
		}
		if diff := cmp.Diff("<html>original</html>", string(content)); diff != "" {
			t.Errorf("stale content mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("returns error when no cache and fetch fails", func(t *testing.T) {
		cache := newTestExternalPageCache("http://localhost:1", 1*time.Hour)
		_, err := cache.Get()
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestInjectResizeScript(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantContains     []string
		wantScriptBefore string
		wantSuffix       bool
	}{
		{
			name:  "injects script before closing body tag",
			input: "<html><body>content</body></html>",
			wantContains: []string{
				"window.dispatchEvent(new Event('resize'))",
				"content",
				"</body></html>",
			},
			wantScriptBefore: "</body>",
		},
		{
			name:  "appends script when no body tag present",
			input: "<html><div>content</div></html>",
			wantContains: []string{
				"window.dispatchEvent(new Event('resize'))",
			},
			wantSuffix: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			result := injectResizeScript(input)

			for _, want := range tt.wantContains {
				if !strings.Contains(string(result), want) {
					t.Errorf("result missing expected substring %q", want)
				}
			}

			if tt.wantScriptBefore != "" {
				scriptIdx := bytes.Index(result, []byte("<script>"))
				beforeIdx := bytes.Index(result, []byte(tt.wantScriptBefore))
				if scriptIdx >= beforeIdx {
					t.Errorf("resize script (at %d) should appear before %q (at %d)", scriptIdx, tt.wantScriptBefore, beforeIdx)
				}
			}

			if tt.wantSuffix && !bytes.HasSuffix(result, resizeScript) {
				t.Errorf("result should end with resizeScript when no </body> tag present")
			}

			if diff := cmp.Diff(tt.input, string(input)); diff != "" {
				t.Errorf("original input was mutated (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetExternalPageHTML(t *testing.T) {
	tests := []struct {
		name             string
		slug             string
		setupUpstream    func() *httptest.Server
		caches           map[string]*ExternalPageCache
		wantCode         int
		wantContentType  string
		wantContains     []string
		wantBodyContains string
	}{
		{
			name: "returns 502 on fetch error",
			slug: "broken",
			caches: map[string]*ExternalPageCache{
				"broken": newTestExternalPageCache("http://localhost:1", 1*time.Hour),
			},
			wantCode:         http.StatusBadGateway,
			wantBodyContains: "Failed to fetch external page",
		},
		{
			name: "returns content with resize script for valid slug",
			slug: "test-page",
			setupUpstream: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("<html><body>test page</body></html>"))
				}))
			},
			wantCode:        http.StatusOK,
			wantContentType: "text/html; charset=utf-8",
			wantContains: []string{
				"test page",
				"window.dispatchEvent(new Event('resize'))",
			},
		},
		{
			name:     "returns 404 for unknown slug",
			slug:     "unknown",
			caches:   map[string]*ExternalPageCache{},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caches := tt.caches
			if tt.setupUpstream != nil {
				server := tt.setupUpstream()
				defer server.Close()
				caches = map[string]*ExternalPageCache{
					tt.slug: newTestExternalPageCache(server.URL, 1*time.Hour),
				}
			}

			h := &Handlers{
				logger:             logrus.New(),
				externalPageCaches: caches,
			}

			req := httptest.NewRequest(http.MethodGet, "/api/external-pages/"+tt.slug, nil)
			req = mux.SetURLVars(req, map[string]string{"pageSlug": tt.slug})
			w := httptest.NewRecorder()

			h.GetExternalPageHTML(w, req)

			if diff := cmp.Diff(tt.wantCode, w.Code); diff != "" {
				t.Errorf("status code mismatch (-want +got):\n%s", diff)
			}

			if tt.wantContentType != "" {
				if diff := cmp.Diff(tt.wantContentType, w.Header().Get("Content-Type")); diff != "" {
					t.Errorf("Content-Type mismatch (-want +got):\n%s", diff)
				}
			}

			body := w.Body.String()
			for _, want := range tt.wantContains {
				if !strings.Contains(body, want) {
					t.Errorf("response body missing expected substring %q", want)
				}
			}

			if tt.wantBodyContains != "" && !strings.Contains(body, tt.wantBodyContains) {
				t.Errorf("response body missing expected substring %q", tt.wantBodyContains)
			}
		})
	}
}
