package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

func TestHTTPProber_Probe(t *testing.T) {
	tests := []struct {
		name               string
		expectedStatusCode int
		initialStatusCode  int
		retryStatusCode    int
		confirmAfter       time.Duration
		cancelContext      bool
		expectedError      error
		severity           types.Severity
		expectedResult     *types.ComponentMonitorReportComponentStatus
	}{
		{
			name:               "success - status code matches expected",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusOK,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    "test-component",
				SubComponentSlug: "test-subcomponent",
				Status:           types.StatusHealthy,
				Reasons: []types.Reason{
					{
						Type:    types.CheckTypeHTTP,
						Check:   "", // Will be set to server.URL in test
						Results: "", // Will be set from actual result
					},
				},
			},
		},
		{
			name:               "failure - status code doesn't match, retry confirms failure",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusInternalServerError,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    "test-component",
				SubComponentSlug: "test-subcomponent",
				Status:           types.StatusDown,
				Reasons: []types.Reason{
					{
						Type:    types.CheckTypeHTTP,
						Check:   "", // Will be set to server.URL in test
						Results: "", // Will be set from actual result
					},
				},
			},
		},
		{
			name:               "failure then recovery - status code doesn't match, retry succeeds",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    "test-component",
				SubComponentSlug: "test-subcomponent",
				Status:           types.StatusHealthy,
				Reasons: []types.Reason{
					{
						Type:    types.CheckTypeHTTP,
						Check:   "", // Will be set to server.URL in test
						Results: "", // Will be set from actual result
					},
				},
			},
		},
		{
			name:               "network error on first request",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  -1, // Special value to trigger server close
			retryStatusCode:    http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectedError:      errors.New("EOF"),
			severity:           types.SeverityDown,
		},
		{
			name:               "network error on retry",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    -1, // Special value to trigger server close
			confirmAfter:       10 * time.Millisecond,
			expectedError:      errors.New("EOF"),
			severity:           types.SeverityDown,
		},
		{
			name:               "context cancellation during confirmAfter wait",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusOK,
			confirmAfter:       100 * time.Millisecond,
			cancelContext:      true,
			expectedError:      context.Canceled,
			severity:           types.SeverityDown,
		},
		{
			name:               "failure with Degraded severity",
			expectedStatusCode: http.StatusOK,
			initialStatusCode:  http.StatusInternalServerError,
			retryStatusCode:    http.StatusInternalServerError,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDegraded,
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    "test-component",
				SubComponentSlug: "test-subcomponent",
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{
					{
						Type:    types.CheckTypeHTTP,
						Check:   "", // Will be set to server.URL in test
						Results: "", // Will be set from actual result
					},
				},
			},
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
				tt.severity,
			)

			results := make(chan types.ComponentMonitorReportComponentStatus, 1)
			errChan := make(chan error, 1)

			prober.Probe(ctx, results, errChan)

			var result types.ComponentMonitorReportComponentStatus
			var err error
			var gotResult, gotError bool

			// Wait for either result or error with timeout
			timeout := time.After(500 * time.Millisecond)
			for !gotResult && !gotError {
				select {
				case result = <-results:
					gotResult = true
				case err = <-errChan:
					gotError = true
				case <-timeout:
					t.Fatal("timeout waiting for result or error")
				}
			}

			// Check for additional error that may have been sent
			if gotResult {
				select {
				case additionalErr := <-errChan:
					err = additionalErr
					gotError = true
				case <-time.After(100 * time.Millisecond):
					// No additional error
				}
			}

			// Compare error
			if tt.expectedError != nil {
				if !gotError {
					t.Error("expected error but got none")
				} else {
					// For errors that include the URL, construct expected error with actual server URL
					expectedError := tt.expectedError
					if expectedError.Error() == "EOF" {
						expectedError = errors.New(`Get "` + server.URL + `": EOF`)
					}
					diff := cmp.Diff(expectedError, err, testhelper.EquateErrorMessage)
					if diff != "" {
						t.Errorf("HTTPProber.Probe() error mismatch (-want +got):\n%s", diff)
					}
				}
			} else if gotError && err != nil {
				// If no error expected but we got one, that's only a problem if we also expected a result
				if tt.expectedResult != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			// Compare result if we got one
			if gotResult {
				if tt.expectedResult == nil {
					t.Fatal("got result but no expected result defined")
				}

				// Check that we have exactly one reason
				if len(result.Reasons) != 1 {
					t.Errorf("expected 1 reason, got %d", len(result.Reasons))
				}

				// Set dynamic fields in expected result
				expected := *tt.expectedResult
				expected.Reasons[0].Check = server.URL
				expected.Reasons[0].Results = result.Reasons[0].Results // Copy actual results for comparison

				diff := cmp.Diff(expected, result)
				if diff != "" {
					t.Errorf("HTTPProber.Probe() mismatch (-want +got):\n%s", diff)
				}

				// Verify results string contains expected information
				if result.Reasons[0].Results == "" {
					t.Error("expected Results to be non-empty")
				}
			} else if tt.expectedResult != nil && tt.expectedError == nil {
				t.Error("expected result but got error")
			}
		})
	}
}
