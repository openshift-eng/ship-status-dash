package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

type mockHTTPDoer struct {
	responses map[string]mockHTTPResponse
}

type mockHTTPResponse struct {
	body       string
	statusCode int
	err        error
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	r, ok := m.responses[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected URL: %s", req.URL.String())
	}
	if r.err != nil {
		return nil, r.err
	}
	code := r.statusCode
	if code == 0 {
		code = http.StatusOK
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

const validJUnit = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="build-farm-canary" tests="4" failures="0" errors="0">
  <testcase name="scheduling" classname="build-farm-canary"/>
  <testcase name="git-clone" classname="build-farm-canary"/>
  <testcase name="internal-registry" classname="build-farm-canary"/>
  <testcase name="quay-pull" classname="build-farm-canary"/>
</testsuite>`

const failingJUnit = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="build-farm-canary" tests="4" failures="2" errors="0">
  <testcase name="scheduling" classname="build-farm-canary"/>
  <testcase name="git-clone" classname="build-farm-canary"><failure/></testcase>
  <testcase name="internal-registry" classname="build-farm-canary"/>
  <testcase name="quay-pull" classname="build-farm-canary"><failure/></testcase>
</testsuite>`

const validJUnitWrapper = `<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="build-farm-canary" tests="4" failures="0" errors="0">
    <testcase name="scheduling" classname="build-farm-canary"/>
    <testcase name="git-clone" classname="build-farm-canary"/>
    <testcase name="internal-registry" classname="build-farm-canary"/>
    <testcase name="quay-pull" classname="build-farm-canary"/>
  </testsuite>
</testsuites>`

const zeroTestsJUnit = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="build-farm-canary" tests="0" failures="0" errors="0"/>`

func recentStarted() string {
	return fmt.Sprintf(`{"timestamp": %d, "node": "node1"}`, time.Now().Add(-30*time.Minute).Unix())
}

func staleStarted() string {
	return fmt.Sprintf(`{"timestamp": %d, "node": "node1"}`, time.Now().Add(-3*time.Hour).Unix())
}

func gcsBase(bucket, job string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s", bucket, job)
}

func TestJUnitProber_Probe(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const build = "123"

	base := gcsBase(defaultGCSBucket, job)
	latestURL := base + "/latest-build.txt"
	startedURL := base + "/" + build + "/started.json"
	xmlURL := base + "/" + build + "/artifacts/junit_canary.xml"

	customBase := gcsBase("my-bucket", job)
	customLatestURL := customBase + "/latest-build.txt"
	customStartedURL := customBase + "/456/started.json"
	customXMLURL := customBase + "/456/artifacts/junit_canary.xml"

	tests := []struct {
		name           string
		bucket         string
		severity       types.Severity
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedResult *types.ComponentMonitorReportComponentStatus
		expectedStatus types.Status
	}{
		{
			name:      "all tests pass",
			severity:  types.SeverityDegraded,
			responses: map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {body: validJUnit}},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: all 4 tests passed",
				}},
			},
		},
		{
			name:      "some tests fail",
			severity:  types.SeverityDegraded,
			responses: map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {body: failingJUnit}},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: 2/4 tests failed: git-clone, quay-pull",
				}},
			},
		},
		{
			name:           "stale build",
			severity:       types.SeverityDegraded,
			responses:      map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: staleStarted()}},
			expectedStatus: types.StatusDegraded,
		},
		{
			name:          "latest-build.txt fetch error",
			responses:     map[string]mockHTTPResponse{latestURL: {err: fmt.Errorf("network error")}},
			expectedError: true,
		},
		{
			name:          "started.json returns 404",
			responses:     map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {statusCode: 404, body: "not found"}},
			expectedError: true,
		},
		{
			name:          "junit xml fetch error",
			responses:     map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {statusCode: 404, body: "not found"}},
			expectedError: true,
		},
		{
			name:     "custom gcs bucket",
			bucket:   "my-bucket",
			severity: types.SeverityDegraded,
			responses: map[string]mockHTTPResponse{
				customLatestURL:  {body: "456"},
				customStartedURL: {body: recentStarted()},
				customXMLURL:     {body: validJUnit},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 456: all 4 tests passed",
				}},
			},
		},
		{
			name:      "default severity is degraded when unset",
			responses: map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {body: failingJUnit}},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: 2/4 tests failed: git-clone, quay-pull",
				}},
			},
		},
		{
			name:      "testsuites wrapper root all pass",
			severity:  types.SeverityDegraded,
			responses: map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {body: validJUnitWrapper}},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: all 4 tests passed",
				}},
			},
		},
		{
			name:          "invalid timestamp in started.json",
			severity:      types.SeverityDegraded,
			responses:     map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: `{"timestamp": 0}`}},
			expectedError: true,
		},
		{
			name:      "zero tests in junit xml",
			severity:  types.SeverityDegraded,
			responses: map[string]mockHTTPResponse{latestURL: {body: build}, startedURL: {body: recentStarted()}, xmlURL: {body: zeroTestsJUnit}},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: "build 123: zero tests found",
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prober := NewJUnitProber(testComponentSlug, testSubComponentSlug, tt.bucket, job, 2*time.Hour, tt.severity, &mockHTTPDoer{responses: tt.responses})

			results := make(chan ProbeResult, 1)
			prober.Probe(context.Background(), results)

			var probeResult ProbeResult
			select {
			case probeResult = <-results:
			case <-time.After(500 * time.Millisecond):
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

			if tt.expectedError {
				if diff := cmp.Diff(true, err != nil, testhelper.EquateErrorMessage); diff != "" {
					t.Errorf("JUnitProber.Probe() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("JUnitProber.Probe() unexpected error: %v", err)
				return
			}

			if !gotResult {
				t.Fatal("expected result but got none")
			}

			if tt.expectedResult != nil {
				if diff := cmp.Diff(tt.expectedResult, &result); diff != "" {
					t.Errorf("JUnitProber.Probe() mismatch (-want +got):\n%s", diff)
				}
			} else if tt.expectedStatus != "" {
				if result.Status != tt.expectedStatus {
					t.Errorf("JUnitProber.Probe() status = %q, want %q", result.Status, tt.expectedStatus)
				}
			}
		})
	}
}
