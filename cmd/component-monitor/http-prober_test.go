package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

type testServer struct {
	server *httptest.Server
	url    string
}

func setupTestServer(initialStatusCode, retryStatusCode int) *testServer {
	requestCount := 0
	var server *httptest.Server

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			if initialStatusCode == -1 {
				server.CloseClientConnections()
				return
			}
			w.WriteHeader(initialStatusCode)
		} else {
			if retryStatusCode == -1 {
				server.CloseClientConnections()
				return
			}
			w.WriteHeader(retryStatusCode)
		}
	})

	server = httptest.NewServer(handler)
	return &testServer{
		server: server,
		url:    server.URL,
	}
}

func formatExpectedHTTPError(baseError error, url string) error {
	if baseError == nil {
		return nil
	}
	formattedBaseError := baseError
	if baseError.Error() == "EOF" {
		formattedBaseError = errors.New(`Get "` + url + `": EOF`)
	}
	return fmt.Errorf("error running HTTP probe, for component: %s sub-component %s. url: %s. error: %w", "test-component", "test-subcomponent", url, formattedBaseError)
}

func TestHTTPProber_Probe(t *testing.T) {
	tests := []struct {
		name               string
		server             func() *testServer
		expectedStatusCode int
		confirmAfter       time.Duration
		cancelContext      bool
		expectedError      func(*testServer) error
		severity           types.Severity
		expectedResult     func(*testServer) *types.ComponentMonitorReportComponentStatus
	}{
		{
			name:               "success - status code matches expected",
			server:             func() *testServer { return setupTestServer(http.StatusOK, http.StatusOK) },
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: func(server *testServer) *types.ComponentMonitorReportComponentStatus {
				return &types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "test-component",
					SubComponentSlug: "test-subcomponent",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{
							Type:    types.CheckTypeHTTP,
							Check:   server.url,
							Results: "Status code 200 (expected 200)",
						},
					},
				}
			},
		},
		{
			name: "failure - status code doesn't match, retry confirms failure",
			server: func() *testServer {
				return setupTestServer(http.StatusInternalServerError, http.StatusInternalServerError)
			},
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: func(server *testServer) *types.ComponentMonitorReportComponentStatus {
				return &types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "test-component",
					SubComponentSlug: "test-subcomponent",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{
							Type:    types.CheckTypeHTTP,
							Check:   server.url,
							Results: "Status code 500 (expected 200)",
						},
					},
				}
			},
		},
		{
			name:               "failure then recovery - status code doesn't match, retry succeeds",
			server:             func() *testServer { return setupTestServer(http.StatusInternalServerError, http.StatusOK) },
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDown,
			expectedResult: func(server *testServer) *types.ComponentMonitorReportComponentStatus {
				return &types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "test-component",
					SubComponentSlug: "test-subcomponent",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{
							Type:    types.CheckTypeHTTP,
							Check:   server.url,
							Results: "Status code 200 (expected 200)",
						},
					},
				}
			},
		},
		{
			name:               "network error on first request",
			server:             func() *testServer { return setupTestServer(-1, http.StatusOK) },
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectedError: func(server *testServer) error {
				return formatExpectedHTTPError(errors.New("EOF"), server.url)
			},
			severity: types.SeverityDown,
		},
		{
			name:               "network error on retry",
			server:             func() *testServer { return setupTestServer(http.StatusInternalServerError, -1) },
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			expectedError: func(server *testServer) error {
				return formatExpectedHTTPError(errors.New("EOF"), server.url)
			},
			severity: types.SeverityDown,
		},
		{
			name:               "context cancellation during confirmAfter wait",
			server:             func() *testServer { return setupTestServer(http.StatusInternalServerError, http.StatusOK) },
			expectedStatusCode: http.StatusOK,
			confirmAfter:       100 * time.Millisecond,
			cancelContext:      true,
			expectedError: func(server *testServer) error {
				return formatExpectedHTTPError(context.Canceled, server.url)
			},
			severity: types.SeverityDown,
		},
		{
			name: "failure with Degraded severity",
			server: func() *testServer {
				return setupTestServer(http.StatusInternalServerError, http.StatusInternalServerError)
			},
			expectedStatusCode: http.StatusOK,
			confirmAfter:       10 * time.Millisecond,
			severity:           types.SeverityDegraded,
			expectedResult: func(server *testServer) *types.ComponentMonitorReportComponentStatus {
				return &types.ComponentMonitorReportComponentStatus{
					ComponentSlug:    "test-component",
					SubComponentSlug: "test-subcomponent",
					Status:           types.StatusDegraded,
					Reasons: []types.Reason{
						{
							Type:    types.CheckTypeHTTP,
							Check:   server.url,
							Results: "Status code 500 (expected 200)",
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.server()
			defer server.server.Close()

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
				server.url,
				tt.expectedStatusCode,
				tt.confirmAfter,
				tt.severity,
			)

			results := make(chan ProbeResult, 1)

			prober.Probe(ctx, results)

			var probeResult ProbeResult

			// Wait for result with timeout
			timeout := time.After(500 * time.Millisecond)
			select {
			case probeResult = <-results:
			case <-timeout:
				t.Fatal("timeout waiting for result")
			}

			var result types.ComponentMonitorReportComponentStatus
			var err error
			gotResult := false
			if probeResult.Error != nil {
				err = probeResult.Error
			} else {
				result = probeResult.ComponentMonitorReportComponentStatus
				gotResult = true
			}

			// Compare error
			var wantErr error
			if tt.expectedError != nil {
				wantErr = tt.expectedError(server)
			}
			if diff := cmp.Diff(wantErr, err, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("HTTPProber.Probe() error mismatch (-want +got):\n%s", diff)
			}

			// Compare result if we got one
			if gotResult {
				if tt.expectedResult == nil {
					t.Fatal("got result but no expected result defined")
				}

				wantResult := tt.expectedResult(server)
				if diff := cmp.Diff(wantResult, &result); diff != "" {
					t.Errorf("HTTPProber.Probe() mismatch (-want +got):\n%s", diff)
				}
			} else if tt.expectedResult != nil && tt.expectedError == nil {
				t.Error("expected result but got error")
			}
		})
	}
}
