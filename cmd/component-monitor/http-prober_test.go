package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"ship-status-dash/pkg/types"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPProber_Probe(t *testing.T) {
	tests := []struct {
		name               string
		expectedStatusCode int
		initialStatusCode  int
		retryStatusCode    int
		confirmAfter       time.Duration
		serverDelay        time.Duration
		cancelContext      bool
		expectHealthy      bool
		expectError        bool
		expectedReasonType types.CheckType
	}{
		{
			name:               "success - status code matches expected",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusOK,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectHealthy:      true,
			expectedReasonType: types.CheckTypeHTTP,
		},
		{
			name:               "failure - status code doesn't match, retry confirms failure",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusInternalServerError,
			confirmAfter:       10 * time.Millisecond,
			expectHealthy:      false,
			expectedReasonType: types.CheckTypeHTTP,
		},
		{
			name:               "failure then recovery - status code doesn't match, retry succeeds",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectHealthy:      true,
			expectedReasonType: types.CheckTypeHTTP,
		},
		{
			name:               "network error on first request",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  -1, // Special value to trigger server close
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectHealthy:      false,
			expectError:        true,
			expectedReasonType: types.CheckTypeHTTP,
		},
		{
			name:               "network error on retry",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    -1, // Special value to trigger server close
			confirmAfter:       10 * time.Millisecond,
			expectHealthy:      false,
			expectError:        true,
			expectedReasonType: types.CheckTypeHTTP,
		},
		{
			name:               "context cancellation during confirmAfter wait",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       100 * time.Millisecond,
			cancelContext:      true,
			expectError:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0
			var server *httptest.Server

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++
				if requestCount == 1 {
					if tt.initialStatusCode == -1 {
						server.CloseClientConnections()
						return
					}
					w.WriteHeader(tt.initialStatusCode)
				} else {
					if tt.retryStatusCode == -1 {
						server.CloseClientConnections()
						return
					}
					w.WriteHeader(tt.retryStatusCode)
				}
			})

			server = httptest.NewServer(handler)
			defer server.Close()

			ctx := context.Background()
			if tt.cancelContext {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(50 * time.Millisecond)
					cancel()
				}()
			}

			prober := NewHTTPProber(
				"test-component",
				"test-subcomponent",
				server.URL,
				tt.expectedStatusCode,
				tt.confirmAfter,
			)

			results := make(chan types.ComponentMonitorReportComponentStatus, 1)
			errChan := make(chan error, 1)

			prober.Probe(ctx, results, errChan)

			select {
			case result := <-results:
				assert.Equal(t, "test-component", result.ComponentSlug)
				assert.Equal(t, "test-subcomponent", result.SubComponentSlug)
				assert.Equal(t, tt.expectedReasonType, result.Reason.Type)
				assert.Equal(t, server.URL, result.Reason.Check)

				if tt.expectHealthy {
					assert.Equal(t, types.StatusHealthy, result.Status)
				} else {
					assert.Equal(t, types.StatusDown, result.Status)
				}

				if tt.expectError {
					select {
					case err := <-errChan:
						assert.NotNil(t, err)
					case <-time.After(100 * time.Millisecond):
						// Error may have been sent before result
					}
				}

			case err := <-errChan:
				if tt.expectError {
					assert.NotNil(t, err)
				} else {
					t.Fatalf("unexpected error: %v", err)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result or error")
			}
		})
	}
}
