package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/types"
)

// newJUnitProber fills the canary artifact path when a test omits it. Production NewJUnitProber does not default the path.
func newJUnitProber(componentSlug, subComponentSlug, bucket string, jobNames []string, maxAge time.Duration, severity types.Severity, settings JUnitProberSettings, client junitHTTPClient) *JUnitProber {
	if strings.TrimSpace(settings.ArtifactPath) == "" {
		settings.ArtifactPath = "artifacts/junit_canary.xml"
	}
	return NewJUnitProber(componentSlug, subComponentSlug, bucket, jobNames, maxAge, severity, settings, client)
}

// gcsListObjectURLTest must match the query string built by fetchGCSListPage so the mock can key URLs.
func gcsListObjectURLTest(bucket, job string) string {
	v := url.Values{}
	v.Set("prefix", "logs/"+job+"/")
	v.Set("delimiter", "/")
	v.Set("maxResults", "1000")
	v.Set("fields", "prefixes,nextPageToken")
	return fmt.Sprintf("https://www.googleapis.com/storage/v1/b/%s/o?%s", url.PathEscape(bucket), v.Encode())
}

// gcsProwObjectURL builds a direct GCS object URL (artifact_url_style gcs).
func gcsProwObjectURL(bucket, job string, segs ...string) string {
	tail := strings.Join(segs, "/")
	return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s/%s", bucket, job, tail)
}

func prowSpyglassViewURL(bucket, job, buildID string) string {
	return fmt.Sprintf("%s/view/gs/%s/logs/%s/%s", types.JUnitDefaultProwSpyglassBase, bucket, job, buildID)
}

type mockHTTPDoer struct {
	responses map[string]mockHTTPResponse
}

type mockHTTPResponse struct {
	body       string
	statusCode int
	err        error
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Range") != "" {
		return nil, fmt.Errorf("unexpected Range header on %s", req.URL)
	}
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

func readJUnitFixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join("testdata", "junit", name)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestAggregateJunitFromSuites_errorElement(t *testing.T) {
	const xmlBody = `<?xml version="1.0" encoding="UTF-8"?>
<testsuite tests="1">
  <testcase name="boom" classname="build-farm-canary"><error message="oops"/></testcase>
</testsuite>`
	suites, err := parseSuitesFromJunitBody(xmlBody)
	if err != nil {
		t.Fatalf("parseSuitesFromJunitBody: %v", err)
	}
	total, failed := aggregateJunitFromSuites(suites)
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	want := []string{"boom"}
	if diff := cmp.Diff(want, failed); diff != "" {
		t.Errorf("failed (-want +got):\n%s", diff)
	}
}

func TestProwLogObjectURL(t *testing.T) {
	job := "periodic-ci-foo"
	bucket := "test-platform-results-public"
	cases := []struct {
		name  string
		p     *JUnitProber
		extra []string
		want  string
	}{
		{
			name:  "gcs",
			p:     newJUnitProber("c", "s", bucket, []string{job}, time.Hour, types.SeverityDegraded, JUnitProberSettings{ArtifactURLStyle: types.JUnitArtifactStyleGCS, HistoryRuns: 1}, &http.Client{}),
			extra: []string{"1", "started.json"},
			want:  "https://storage.googleapis.com/test-platform-results-public/logs/periodic-ci-foo/1/started.json",
		},
		{
			name:  "gcsweb",
			p:     newJUnitProber("c", "s", bucket, []string{job}, time.Hour, types.SeverityDegraded, JUnitProberSettings{ArtifactURLStyle: types.JUnitArtifactStyleGCSWeb, GCSWebBaseURL: "https://example-gcsweb.test", HistoryRuns: 1}, &http.Client{}),
			extra: []string{"1", "artifacts", "junit.xml"},
			want:  "https://example-gcsweb.test/gcs/test-platform-results-public/logs/periodic-ci-foo/1/artifacts/junit.xml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.p.prowLogObjectURL(job, tc.extra...)
			if got != tc.want {
				t.Fatalf("prowLogObjectURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJUnitProber_Probe(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const build = "123"
	bucket := defaultGCSBucket

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedURL := gcsProwObjectURL(bucket, job, build, prowObjectStarted)
	finishedURL := gcsProwObjectURL(bucket, job, build, prowObjectFinished)
	xmlURL := gcsProwObjectURL(bucket, job, build, "artifacts", "junit_canary.xml")

	okSingle := readJUnitFixture(t, "ok_single.xml")
	failing := readJUnitFixture(t, "failing.xml")
	okTestSuites := readJUnitFixture(t, "ok_testsuites.xml")
	zero := readJUnitFixture(t, "zero_tests.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	customBucket := "my-bucket"
	customBase := fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s", customBucket, job)
	customLatestURL := customBase + "/latest-build.txt"
	customStartedURL := customBase + "/456/started.json"
	customFinishedURL := customBase + "/456/finished.json"
	customXMLURL := customBase + "/456/artifacts/junit_canary.xml"

	tests := []struct {
		name           string
		bucket         string
		severity       types.Severity
		settings       JUnitProberSettings
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedResult *types.ComponentMonitorReportComponentStatus
		expectedStatus types.Status
	}{
		{
			name:     "all tests pass",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: okSingle},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "some tests fail",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: failing},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("build 123: 2/4 tests failed: git-clone, quay-pull; %s", prowSpyglassViewURL(bucket, job, build)),
				}},
			},
		},
		{
			name:     "stale build",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:                         {body: build},
				startedURL:                        {body: staleStarted()},
				finishedURL:                       {body: finishedBody},
				gcsListObjectURLTest(bucket, job): {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, build)},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name:          "latest-build.txt fetch error",
			settings:      JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses:     map[string]mockHTTPResponse{latestURL: {err: fmt.Errorf("network error")}},
			expectedError: true,
		},
		{
			name:     "started.json returns 404",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:                         {body: build},
				startedURL:                        {statusCode: 404, body: "not found"},
				finishedURL:                       {statusCode: 404, body: "not found"},
				gcsListObjectURLTest(bucket, job): {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, build)},
			},
			expectedError: true,
		},
		{
			name:     "finished build missing junit with started.json is canary failure",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {statusCode: 404, body: "not found"},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("build 123: missing artifacts/junit_canary.xml; %s", prowSpyglassViewURL(bucket, job, build)),
				}},
			},
		},
		{
			name:     "finished build missing junit without started.json",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {statusCode: 404, body: "not found"},
				finishedURL: {body: finishedBody},
				xmlURL:      {statusCode: 404, body: "not found"},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("build 123: missing artifacts/junit_canary.xml; %s", prowSpyglassViewURL(bucket, job, build)),
				}},
			},
		},
		{
			name:     "custom gcs bucket",
			bucket:   "my-bucket",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				customLatestURL:   {body: "456"},
				customStartedURL:  {body: recentStarted()},
				customFinishedURL: {body: finishedBody},
				customXMLURL:      {body: okSingle},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "default severity is degraded when unset",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: failing},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("build 123: 2/4 tests failed: git-clone, quay-pull; %s", prowSpyglassViewURL(bucket, job, build)),
				}},
			},
		},
		{
			name:     "testsuites wrapper root all pass",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: okTestSuites},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusHealthy,
				Reasons:          nil,
			},
		},
		{
			name:     "invalid timestamp in started.json",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:                         {body: build},
				startedURL:                        {body: `{"timestamp": 0}`},
				finishedURL:                       {body: finishedBody},
				gcsListObjectURLTest(bucket, job): {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, build)},
			},
			expectedError: true,
		},
		{
			name:     "zero tests in junit xml",
			severity: types.SeverityDegraded,
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: build},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				xmlURL:      {body: zero},
			},
			expectedResult: &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("build 123: zero tests found; %s", prowSpyglassViewURL(bucket, job, build)),
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bucket
			if tt.bucket != "" {
				b = tt.bucket
			}
			prober := newJUnitProber(
				testComponentSlug,
				testSubComponentSlug,
				b,
				[]string{job},
				2*time.Hour,
				tt.severity,
				tt.settings,
				&mockHTTPDoer{responses: tt.responses},
			)

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
				if err == nil {
					t.Errorf("Probe() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}
			if !gotResult {
				t.Fatal("expected result but got none")
			}

			if tt.expectedResult != nil {
				if diff := cmp.Diff(tt.expectedResult, &result); diff != "" {
					t.Errorf("Probe() mismatch (-want +got):\n%s", diff)
				}
			} else if tt.expectedStatus != "" {
				if result.Status != tt.expectedStatus {
					t.Errorf("Probe() status = %q, want %q", result.Status, tt.expectedStatus)
				}
			}
		})
	}
}

func TestJUnitProber_Probe_fallback(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const latestBuild = "200"
	const prevBuild = "199"
	bucket := defaultGCSBucket

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedLatest := gcsProwObjectURL(bucket, job, latestBuild, prowObjectStarted)
	finishedLatest := gcsProwObjectURL(bucket, job, latestBuild, prowObjectFinished)
	startedPrev := gcsProwObjectURL(bucket, job, prevBuild, prowObjectStarted)
	finishedPrev := gcsProwObjectURL(bucket, job, prevBuild, prowObjectFinished)
	xmlPrev := gcsProwObjectURL(bucket, job, prevBuild, "artifacts", "junit_canary.xml")
	listURL := gcsListObjectURLTest(bucket, job)

	okXML := readJUnitFixture(t, "ok_single.xml")
	failingXML := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	listBody := fmt.Sprintf(`{"prefixes":["logs/%s/%s/","logs/%s/%s/"]}`, job, latestBuild, job, prevBuild)
	singleBuildList := fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, latestBuild)

	tests := []struct {
		name           string
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedStatus types.Status
	}{
		{
			name: "falls back to previous build when latest not finished",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {body: okXML},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "fallback reports failures from previous build",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {body: failingXML},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "fallback to stale previous build reports staleness",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				finishedPrev:   {body: finishedBody},
				startedPrev:    {body: staleStarted()},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "fallback finished build missing junit reports canary failure",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {statusCode: 404, body: "not found"},
			},
			expectedStatus: types.StatusDegraded,
		},
		{
			name: "unfinished stale latest falls back to recent healthy build",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: staleStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: listBody},
				startedPrev:    {body: recentStarted()},
				finishedPrev:   {body: finishedBody},
				xmlPrev:        {body: okXML},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "no previous build returns error",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 404, body: "not found"},
				listURL:        {body: singleBuildList},
			},
			expectedError: true,
		},
		{
			name: "finished.json 500 returns error",
			responses: map[string]mockHTTPResponse{
				latestURL:      {body: latestBuild},
				startedLatest:  {body: recentStarted()},
				finishedLatest: {statusCode: 500, body: "internal server error"},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 2*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
				&mockHTTPDoer{responses: tt.responses})
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if tt.expectedError {
				if res.Error == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != tt.expectedStatus {
				t.Errorf("want %s, got %s", tt.expectedStatus, res.Status)
			}
		})
	}
}

func TestJUnitProber_Probe_history(t *testing.T) {
	const job = "history-job"
	bucket := defaultGCSBucket
	const latest = "200"

	ok := readJUnitFixture(t, "ok_single.xml")
	failing := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedURL := gcsProwObjectURL(bucket, job, latest, prowObjectStarted)
	finishedURL := gcsProwObjectURL(bucket, job, latest, prowObjectFinished)
	finished199 := gcsProwObjectURL(bucket, job, "199", prowObjectFinished)
	startedPrev := gcsProwObjectURL(bucket, job, "199", prowObjectStarted)
	listURL := gcsListObjectURLTest(bucket, job)
	xml200 := gcsProwObjectURL(bucket, job, "200", "artifacts", "junit_canary.xml")
	xml199 := gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml")

	listBody := fmt.Sprintf(`{"prefixes":["logs/%s/200/","logs/%s/199/"]}`, job, job)

	onlyA := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="canary" tests="1" failures="1" errors="0">
  <testcase name="flaky-a" classname="c"><failure/></testcase>
</testsuite>`
	onlyB := `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="canary" tests="1" failures="1" errors="0">
  <testcase name="flaky-b" classname="c"><failure/></testcase>
</testsuite>`

	spy200 := prowSpyglassViewURL(bucket, job, "200")
	spy199 := prowSpyglassViewURL(bucket, job, "199")

	tests := []struct {
		name           string
		responses      map[string]mockHTTPResponse
		expectedError  bool
		expectedStatus types.Status
		wantReason     string
	}{
		{
			name: "both builds fail with same pattern reports degraded",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
				xml200:      {body: failing},
				xml199:      {body: failing},
			},
			expectedStatus: types.StatusDegraded,
			wantReason: "junit: 2 of the last 2 run(s) share the same failure pattern: failed: git-clone, quay-pull (threshold 2) — " +
				fmt.Sprintf("build 200: 2/4 tests failed: git-clone, quay-pull; %s | build 199: 2/4 tests failed: git-clone, quay-pull; %s", spy200, spy199),
		},
		{
			name: "one pass one fail stays healthy below threshold",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
				xml200:      {body: ok},
				xml199:      {body: failing},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "different failure signatures stay healthy below threshold",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
				xml200:      {body: onlyA},
				xml199:      {body: onlyB},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "excludes newest unfinished build from history evaluation",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {statusCode: 404, body: "not found"},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
				xml199:      {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "older unfinished build in window is skipped",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {statusCode: 404, body: "not found"},
				gcsProwObjectURL(bucket, job, "199", prowObjectStarted):  {body: recentStarted()},
				gcsProwObjectURL(bucket, job, "198", prowObjectFinished): {body: finishedBody},
				gcsProwObjectURL(bucket, job, "198", prowObjectStarted):  {body: recentStarted()},
				listURL: {body: fmt.Sprintf(`{"prefixes":["logs/%s/200/","logs/%s/199/","logs/%s/198/"]}`, job, job, job)},
				xml200:  {body: ok},
				gcsProwObjectURL(bucket, job, "198", "artifacts", "junit_canary.xml"): {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "unfinished build outside max age stops the scan",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {statusCode: 404, body: "not found"},
				gcsProwObjectURL(bucket, job, "199", prowObjectStarted): {body: staleStarted()},
				listURL: {body: fmt.Sprintf(`{"prefixes":["logs/%s/200/","logs/%s/199/","logs/%s/198/"]}`, job, job, job)},
				xml200:  {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "no finished builds returns error",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {statusCode: 404, body: "not found"},
				finished199: {statusCode: 404, body: "not found"},
				listURL:     {body: listBody},
				startedPrev: {body: recentStarted()},
			},
			expectedError: true,
		},
		{
			name: "two missing junit runs meet history threshold",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {statusCode: 404, body: "not found"},
				finishedURL: {body: finishedBody},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				startedPrev: {statusCode: 404, body: "not found"},
				xml200:      {statusCode: 404, body: "not found"},
				xml199:      {statusCode: 404, body: "not found"},
			},
			expectedStatus: types.StatusDegraded,
			wantReason: "junit: 2 of the last 2 run(s) share the same failure pattern: missing artifacts/junit_canary.xml (threshold 2) — " +
				fmt.Sprintf("build 200: missing artifacts/junit_canary.xml; %s | build 199: missing artifacts/junit_canary.xml; %s", spy200, spy199),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newJUnitProber(
				testComponentSlug, testSubComponentSlug, bucket, []string{job}, 2*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{HistoryRuns: 2, FailedRunsThreshold: 2, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
				&mockHTTPDoer{responses: tt.responses},
			)
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if tt.expectedError {
				if res.Error == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != tt.expectedStatus {
				t.Errorf("want %s, got %s", tt.expectedStatus, res.Status)
			}
			if tt.wantReason == "" {
				return
			}
			want := []types.Reason{{Type: types.CheckTypeJUnit, Check: job, Results: tt.wantReason}}
			if diff := cmp.Diff(want, res.Reasons); diff != "" {
				t.Errorf("Reasons mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestJUnitProber_Probe_history_early_exit(t *testing.T) {
	const job = "early-exit-job"
	bucket := defaultGCSBucket
	const latest = "200"

	ok := readJUnitFixture(t, "ok_single.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedURL := gcsProwObjectURL(bucket, job, latest, prowObjectStarted)
	finishedURL := gcsProwObjectURL(bucket, job, latest, prowObjectFinished)
	finished199 := gcsProwObjectURL(bucket, job, "199", prowObjectFinished)
	finished198 := gcsProwObjectURL(bucket, job, "198", prowObjectFinished)
	xml200 := gcsProwObjectURL(bucket, job, "200", "artifacts", "junit_canary.xml")
	xml199 := gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml")
	xml198 := gcsProwObjectURL(bucket, job, "198", "artifacts", "junit_canary.xml")
	listURL := gcsListObjectURLTest(bucket, job)

	// List includes an old orphaned build (197) whose finished.json is NOT
	// mocked. If the prober tried to check it the mock would return an
	// error, failing the test. The early exit after collecting HistoryRuns
	// finished builds must prevent that.
	listBody := fmt.Sprintf(
		`{"prefixes":["logs/%s/200/","logs/%s/199/","logs/%s/198/","logs/%s/197/"]}`,
		job, job, job, job,
	)

	tests := []struct {
		name           string
		responses      map[string]mockHTTPResponse
		expectedStatus types.Status
	}{
		{
			name: "skips orphaned old builds once enough finished builds collected",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {body: finishedBody},
				finished199: {body: finishedBody},
				listURL:     {body: listBody},
				gcsProwObjectURL(bucket, job, "199", prowObjectStarted): {body: recentStarted()},
				xml200: {body: ok},
				xml199: {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
		{
			name: "skips orphaned old builds when newest is unfinished",
			responses: map[string]mockHTTPResponse{
				latestURL:   {body: latest},
				startedURL:  {body: recentStarted()},
				finishedURL: {statusCode: 404, body: "not found"},
				finished199: {body: finishedBody},
				finished198: {body: finishedBody},
				gcsProwObjectURL(bucket, job, "199", prowObjectStarted): {body: recentStarted()},
				gcsProwObjectURL(bucket, job, "198", prowObjectStarted): {body: recentStarted()},
				listURL: {body: listBody},
				xml199:  {body: ok},
				xml198:  {body: ok},
			},
			expectedStatus: types.StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newJUnitProber(
				testComponentSlug, testSubComponentSlug, bucket, []string{job}, 2*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{HistoryRuns: 2, FailedRunsThreshold: 2, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
				&mockHTTPDoer{responses: tt.responses},
			)
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != tt.expectedStatus {
				t.Errorf("want %s, got %s", tt.expectedStatus, res.Status)
			}
		})
	}
}

func TestJUnitProber_Probe_staleEnrichment(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const staleBuild = "200"
	const successBuild = "199"
	bucket := defaultGCSBucket
	maxAge := 2 * time.Hour

	latestURL := gcsProwObjectURL(bucket, job, prowObjectLatestBuild)
	startedStale := gcsProwObjectURL(bucket, job, staleBuild, prowObjectStarted)
	finishedStale := gcsProwObjectURL(bucket, job, staleBuild, prowObjectFinished)
	finishedSuccess := gcsProwObjectURL(bucket, job, successBuild, prowObjectFinished)
	xmlSuccess := gcsProwObjectURL(bucket, job, successBuild, "artifacts", "junit_canary.xml")
	listURL := gcsListObjectURLTest(bucket, job)
	staleSpyglass := prowSpyglassViewURL(bucket, job, staleBuild)

	okXML := readJUnitFixture(t, "ok_single.xml")
	failingXML := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	tests := []struct {
		name       string
		settings   JUnitProberSettings
		responses  func(startedAt time.Time) map[string]mockHTTPResponse
		wantSuffix string // appended after "base; spyglass"
	}{
		{
			name:     "stale with prior success includes last success link",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: func(startedAt time.Time) map[string]mockHTTPResponse {
				return map[string]mockHTTPResponse{
					latestURL:       {body: staleBuild},
					startedStale:    {body: startedJSON(startedAt)},
					finishedStale:   {body: finishedBody},
					listURL:         {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/","logs/%s/%s/"]}`, job, staleBuild, job, successBuild)},
					finishedSuccess: {body: finishedBody},
					xmlSuccess:      {body: okXML},
				}
			},
			wantSuffix: fmt.Sprintf("last success: %s %s", successBuild, prowSpyglassViewURL(bucket, job, successBuild)),
		},
		{
			name:     "stale with no prior success notes none found",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: func(startedAt time.Time) map[string]mockHTTPResponse {
				return map[string]mockHTTPResponse{
					latestURL:     {body: staleBuild},
					startedStale:  {body: startedJSON(startedAt)},
					finishedStale: {body: finishedBody},
					listURL:       {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/"]}`, job, staleBuild)},
				}
			},
			wantSuffix: "no recent successful run found",
		},
		{
			name:     "stale skips failing prior runs until success",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: func(startedAt time.Time) map[string]mockHTTPResponse {
				return map[string]mockHTTPResponse{
					latestURL:     {body: staleBuild},
					startedStale:  {body: startedJSON(startedAt)},
					finishedStale: {body: finishedBody},
					listURL: {body: fmt.Sprintf(`{"prefixes":["logs/%s/%s/","logs/%s/199/","logs/%s/198/"]}`,
						job, staleBuild, job, job)},
					gcsProwObjectURL(bucket, job, "199", prowObjectFinished):              {body: finishedBody},
					gcsProwObjectURL(bucket, job, "198", prowObjectFinished):              {body: finishedBody},
					gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml"): {body: failingXML},
					gcsProwObjectURL(bucket, job, "198", "artifacts", "junit_canary.xml"): {body: okXML},
				}
			},
			wantSuffix: fmt.Sprintf("last success: 198 %s", prowSpyglassViewURL(bucket, job, "198")),
		},
		{
			name: "stale history scan limit does not reach older healthy build",
			// HistoryRuns 2 → scan at most two finished priors; 197 is healthy but beyond the limit.
			settings: JUnitProberSettings{HistoryRuns: 2, FailedRunsThreshold: 2, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: func(startedAt time.Time) map[string]mockHTTPResponse {
				return map[string]mockHTTPResponse{
					latestURL:     {body: staleBuild},
					startedStale:  {body: startedJSON(startedAt)},
					finishedStale: {body: finishedBody},
					listURL: {body: fmt.Sprintf(
						`{"prefixes":["logs/%s/%s/","logs/%s/199/","logs/%s/198/","logs/%s/197/"]}`,
						job, staleBuild, job, job, job)},
					gcsProwObjectURL(bucket, job, "199", prowObjectFinished):              {body: finishedBody},
					gcsProwObjectURL(bucket, job, "198", prowObjectFinished):              {body: finishedBody},
					gcsProwObjectURL(bucket, job, "197", prowObjectFinished):              {body: finishedBody},
					gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml"): {body: failingXML},
					gcsProwObjectURL(bucket, job, "198", "artifacts", "junit_canary.xml"): {body: failingXML},
					// 197 junit deliberately unmocked: fetching it would fail the mock and the test.
				}
			},
			wantSuffix: "no recent successful run found",
		},
		{
			name:     "stale still degraded when enrichment list fails",
			settings: JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, AlertOnNoRecentRun: true},
			responses: func(startedAt time.Time) map[string]mockHTTPResponse {
				return map[string]mockHTTPResponse{
					latestURL:     {body: staleBuild},
					startedStale:  {body: startedJSON(startedAt)},
					finishedStale: {body: finishedBody},
					listURL:       {err: fmt.Errorf("gcs list unavailable")},
				}
			},
			wantSuffix: "last success lookup failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startedAt := time.Now().Add(-3 * time.Hour)
			p := newJUnitProber(
				testComponentSlug, testSubComponentSlug, bucket, []string{job}, maxAge, types.SeverityDegraded,
				tt.settings,
				&mockHTTPDoer{responses: tt.responses(startedAt)},
			)
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			var res ProbeResult
			select {
			case res = <-results:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for result")
			}
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			wantResults := formatStaleReason(staleBuild, startedAt, maxAge, staleSpyglass, tt.wantSuffix)
			want := &types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    testComponentSlug,
				SubComponentSlug: testSubComponentSlug,
				Status:           types.StatusDegraded,
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: wantResults,
				}},
			}
			if diff := cmp.Diff(want, &res.ComponentMonitorReportComponentStatus); diff != "" {
				t.Errorf("Probe() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func recentStarted() string {
	return startedJSON(time.Now().Add(-30 * time.Minute))
}

func staleStarted() string {
	return startedJSON(time.Now().Add(-3 * time.Hour))
}

func startedJSON(ts time.Time) string {
	return fmt.Sprintf(`{"timestamp": %d, "node": "node1"}`, ts.Unix())
}

// formatStaleReason mirrors enrichStaleResult output using the same age rounding as checkBuildStaleness.
func formatStaleReason(buildID string, startedAt time.Time, maxAge time.Duration, spyglass, suffix string) string {
	age := time.Since(startedAt).Round(time.Minute)
	base := fmt.Sprintf("latest build %s started %s ago (max age %s); %s", buildID, age, maxAge, spyglass)
	if suffix == "" {
		return base
	}
	return base + "; " + suffix
}

func boolPtr(v bool) *bool { return &v }

func gzipText(t *testing.T, body string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(body)); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.String()
}

func leaseLog(name string) string {
	return "\x1b[36mINFO\x1b[0m[2024-01-01T00:00:00Z] Acquired 1 lease(s) for aws-quota-slice: [" + name + "]\n"
}

func TestJUnitProber_StaleIgnoredWithoutFlag(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	const build = "123"
	bucket := defaultGCSBucket
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                  {body: build},
		gcsProwObjectURL(bucket, job, build, prowObjectStarted):               {body: staleStarted()},
		gcsProwObjectURL(bucket, job, build, prowObjectFinished):              {body: `{"timestamp":1,"result":"SUCCESS","passed":true}`},
		gcsProwObjectURL(bucket, job, build, "artifacts", "junit_canary.xml"): {body: readJUnitFixture(t, "ok_single.xml")},
	}
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 0, types.SeverityDegraded,
		JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Status != types.StatusHealthy {
		t.Fatalf("status = %s, want healthy", res.Status)
	}
}

func TestJUnitProber_MissingArtifactIgnored(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	bucket := defaultGCSBucket
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                  {body: "200"},
		gcsProwObjectURL(bucket, job, "200", prowObjectStarted):               {body: recentStarted()},
		gcsProwObjectURL(bucket, job, "200", prowObjectFinished):              {body: finishedBody},
		gcsProwObjectURL(bucket, job, "199", prowObjectStarted):               {body: recentStarted()},
		gcsProwObjectURL(bucket, job, "199", prowObjectFinished):              {body: finishedBody},
		gcsProwObjectURL(bucket, job, "200", "artifacts", "junit_canary.xml"): {statusCode: 404, body: "missing"},
		gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml"): {statusCode: 404, body: "missing"},
		gcsListObjectURLTest(bucket, job):                                     {body: gcsListBody(job, []string{"200", "199"})},
	}
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:              2,
			FailedRunsThreshold:      2,
			ArtifactURLStyle:         types.JUnitArtifactStyleGCS,
			MissingArtifactUnhealthy: boolPtr(false),
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error == nil || res.Status == types.StatusHealthy {
		t.Fatalf("status = %s err = %v, want error", res.Status, res.Error)
	}
}

func TestJUnitProber_MissingArtifactMixedStaysHealthy(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	bucket := defaultGCSBucket
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	okXML := readJUnitFixture(t, "ok_single.xml")
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                  {body: "200"},
		gcsProwObjectURL(bucket, job, "200", prowObjectStarted):               {body: recentStarted()},
		gcsProwObjectURL(bucket, job, "200", prowObjectFinished):              {body: finishedBody},
		gcsProwObjectURL(bucket, job, "199", prowObjectStarted):               {body: recentStarted()},
		gcsProwObjectURL(bucket, job, "199", prowObjectFinished):              {body: finishedBody},
		gcsProwObjectURL(bucket, job, "200", "artifacts", "junit_canary.xml"): {body: okXML},
		gcsProwObjectURL(bucket, job, "199", "artifacts", "junit_canary.xml"): {statusCode: 404, body: "missing"},
		gcsListObjectURLTest(bucket, job):                                     {body: gcsListBody(job, []string{"200", "199"})},
	}
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:              2,
			FailedRunsThreshold:      2,
			ArtifactURLStyle:         types.JUnitArtifactStyleGCS,
			MissingArtifactUnhealthy: boolPtr(false),
		},
		&mockHTTPDoer{responses: responses})
	res := probeJUnit(t, p)
	if res.Error != nil || res.Status != types.StatusHealthy {
		t.Fatalf("status = %s err = %v, want healthy", res.Status, res.Error)
	}
}

func TestJUnitProber_UnscoredGroup(t *testing.T) {
	bucket := defaultGCSBucket
	const jobA = "job-a"
	const jobB = "job-b"
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	okXML := readJUnitFixture(t, "ok_single.xml")
	settings := JUnitProberSettings{
		HistoryRuns:              1,
		ArtifactURLStyle:         types.JUnitArtifactStyleGCS,
		GroupBy:                  types.JUnitGroupByLeaseResourcePrefix,
		MissingArtifactUnhealthy: boolPtr(false),
	}
	missing := func(job, build string) map[string]mockHTTPResponse {
		return map[string]mockHTTPResponse{
			gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                  {body: build},
			gcsProwObjectURL(bucket, job, build, prowObjectStarted):               {body: recentStarted()},
			gcsProwObjectURL(bucket, job, build, prowObjectFinished):              {body: finishedBody},
			gcsProwObjectURL(bucket, job, build, "artifacts", "junit_canary.xml"): {statusCode: 404, body: "missing"},
		}
	}
	passing := func(job, build string) map[string]mockHTTPResponse {
		out := finishedJUnitResponses(bucket, job, build, okXML)
		out[gcsProwObjectURL(bucket, job, build, prowObjectStarted)] = mockHTTPResponse{body: recentStarted()}
		out[gcsProwObjectURL(bucket, job, build, prowObjectBuildLog)] = mockHTTPResponse{body: leaseLog("aws-2--us-east-2--openshift-org-aws-quota-slice-210")}
		return out
	}
	probe := func(responses map[string]mockHTTPResponse, jobs ...string) ProbeResult {
		t.Helper()
		p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, jobs, 0, types.SeverityDegraded, settings, &mockHTTPDoer{responses: responses})
		return probeJUnit(t, p)
	}

	t.Run("every build missing returns error", func(t *testing.T) {
		res := probe(missing(jobA, "1"), jobA)
		if res.Error == nil || res.Status == types.StatusHealthy {
			t.Fatalf("status = %s err = %v, want error", res.Status, res.Error)
		}
	})

	t.Run("missing job is not hidden by a passing job", func(t *testing.T) {
		responses := missing(jobA, "1")
		for k, v := range passing(jobB, "2") {
			responses[k] = v
		}
		res := probe(responses, jobA, jobB)
		if res.Error == nil || res.Status == types.StatusHealthy {
			t.Fatalf("status = %s err = %v, want error", res.Status, res.Error)
		}
	})
}

func TestAccountFromLeaseLine(t *testing.T) {
	const resource = "aws-2--us-east-2--openshift-org-aws-quota-slice-210"
	ansi := leaseLog(resource)
	account, matched := accountFromLeaseLine(ansi)
	if !matched || account != "aws-2" {
		t.Fatalf("account = %q matched = %v", account, matched)
	}
	account, matched = accountFromLeaseLine("Acquired 1 lease(s) for hypershift-aws-quota-slice: [55e4e678-4873-4a6e-add7-eb4ddbcb89ee]")
	if !matched || account != "" {
		t.Fatalf("uuid account = %q matched = %v", account, matched)
	}
}

func TestJUnitProber_GroupByLeaseResourcePrefix(t *testing.T) {
	const (
		jobA    = "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
		jobB    = "periodic-ci-openshift-release-main-ci-5.0-e2e-aws-ovn"
		account = "aws-2"
	)
	resource := account + "--us-east-2--openshift-org-aws-quota-slice-210"
	bucket := defaultGCSBucket
	failing := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	started := recentStarted()
	responses := map[string]mockHTTPResponse{}
	add := func(job, id, logBody string, ids []string) {
		responses[gcsProwObjectURL(bucket, job, prowObjectLatestBuild)] = mockHTTPResponse{body: id}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: started}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
		responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: failing}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectBuildLog)] = mockHTTPResponse{body: logBody}
		responses[gcsListObjectURLTest(bucket, job)] = mockHTTPResponse{body: gcsListBody(job, ids)}
	}
	add(jobA, "2", leaseLog(resource), []string{"2", "1"})
	responses[gcsProwObjectURL(bucket, jobA, "1", prowObjectStarted)] = mockHTTPResponse{body: started}
	responses[gcsProwObjectURL(bucket, jobA, "1", prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
	responses[gcsProwObjectURL(bucket, jobA, "1", "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: failing}
	responses[gcsProwObjectURL(bucket, jobA, "1", prowObjectBuildLog)] = mockHTTPResponse{body: leaseLog(resource)}
	add(jobB, "8", gzipText(t, leaseLog(resource)), []string{"8"})

	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{jobA, jobB}, time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:         15,
			FailedRunsThreshold: 3,
			ArtifactURLStyle:    types.JUnitArtifactStyleGCS,
			GroupBy:             types.JUnitGroupByLeaseResourcePrefix,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	part := func(job, id string) string {
		return fmt.Sprintf("%s/%s (git-clone, quay-pull) %s", job, id, prowSpyglassViewURL(bucket, job, id))
	}
	want := []types.Reason{{
		Type:  types.CheckTypeJUnit,
		Check: account,
		Results: fmt.Sprintf("cloud-account: %s: 3 consecutive failure(s) (threshold 3): %s", account,
			strings.Join([]string{part(jobB, "8"), part(jobA, "2"), part(jobA, "1")}, "; ")),
	}}
	if diff := cmp.Diff(want, res.Reasons); diff != "" {
		t.Fatalf("reasons (-want +got):\n%s", diff)
	}
}

func TestJUnitProber_UUIDLeaseSkipped(t *testing.T) {
	const job = "periodic-ci-openshift-hypershift-release-5.1-periodics-e2e-aws-ovn-conformance"
	bucket := defaultGCSBucket
	failing := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	started := recentStarted()
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {body: "2"},
		gcsListObjectURLTest(bucket, job):                    {body: gcsListBody(job, []string{"2", "1"})},
	}
	for _, id := range []string{"2", "1"} {
		responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: started}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
		responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: failing}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectBuildLog)] = mockHTTPResponse{body: leaseLog("55e4e678-4873-4a6e-add7-eb4ddbcb89ee")}
	}
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:         15,
			FailedRunsThreshold: 2,
			ArtifactURLStyle:    types.JUnitArtifactStyleGCS,
			GroupBy:             types.JUnitGroupByLeaseResourcePrefix,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Status != types.StatusHealthy || len(res.Reasons) != 0 {
		t.Fatalf("status = %s reasons = %#v, want healthy", res.Status, res.Reasons)
	}
}

func TestAggregatePrefixedJunit(t *testing.T) {
	passWithOtherFailure := []junitSuite{{TestCases: []junitCase{
		{Name: "install should succeed: cluster"},
		{Name: "unrelated", Failure: &struct{}{}},
	}}}
	total, failed := aggregatePrefixedJunit(passWithOtherFailure, "install should succeed:")
	if total != 1 || len(failed) != 0 {
		t.Fatalf("pass total=%d failed=%v", total, failed)
	}
	infra := []junitSuite{{TestCases: []junitCase{
		{Name: "install should succeed: cluster"},
		{Name: "install should succeed: infrastructure", Failure: &struct{}{}},
	}}}
	total, failed = aggregatePrefixedJunit(infra, "install should succeed:")
	if total != 2 || len(failed) != 1 || failed[0] != "install should succeed: infrastructure" {
		t.Fatalf("infra total=%d failed=%v", total, failed)
	}
	total, failed = aggregatePrefixedJunit(passWithOtherFailure, "Run multi-stage step create-hostedcluster")
	if total != 0 || len(failed) != 0 {
		t.Fatalf("no match total=%d failed=%v", total, failed)
	}
}

func TestJUnitProber_TestcasePrefix(t *testing.T) {
	const job = "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
	bucket := defaultGCSBucket
	const build = "10"
	unrelated := `<?xml version="1.0"?><testsuite tests="1"><testcase name="git-clone"><failure/></testcase></testsuite>`
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                                                                     {body: build},
		gcsProwObjectURL(bucket, job, build, prowObjectStarted):                                                                  {body: recentStarted()},
		gcsProwObjectURL(bucket, job, build, prowObjectFinished):                                                                 {body: `{"timestamp":1}`},
		gcsProwObjectURL(bucket, job, build, "artifacts", "e2e-aws-ovn", "gather-must-gather", "artifacts", "junit_install.xml"): {body: unrelated},
	}
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 48*time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:      1,
			ArtifactPath:     "artifacts/e2e-aws-ovn/gather-must-gather/artifacts/junit_install.xml",
			TestcasePrefix:   "install should succeed:",
			ArtifactURLStyle: types.JUnitArtifactStyleGCS,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error == nil || res.Status == types.StatusHealthy {
		t.Fatalf("status=%s err=%v, want error", res.Status, res.Error)
	}
}

func TestJUnitProber_ConsecutiveStreak(t *testing.T) {
	const job = "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
	bucket := defaultGCSBucket
	resource := "aws-2--us-east-2--openshift-org-aws-quota-slice-210"
	failing := readJUnitFixture(t, "failing.xml")
	ok := readJUnitFixture(t, "ok_single.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	now := time.Now()
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {body: "3"},
		gcsListObjectURLTest(bucket, job):                    {body: gcsListBody(job, []string{"3", "2", "1"})},
	}
	add := func(id, xml string, started time.Time) {
		responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: fmt.Sprintf(`{"timestamp":%d}`, started.Unix())}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
		responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: xml}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectBuildLog)] = mockHTTPResponse{body: leaseLog(resource)}
	}
	add("1", failing, now.Add(-10*time.Minute))
	add("2", failing, now.Add(-time.Hour))
	add("3", ok, now.Add(-2*time.Hour))

	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 48*time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:         15,
			FailedRunsThreshold: 2,
			ArtifactURLStyle:    types.JUnitArtifactStyleGCS,
			GroupBy:             types.JUnitGroupByLeaseResourcePrefix,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Status != types.StatusDegraded || len(res.Reasons) != 1 || !strings.Contains(res.Reasons[0].Results, "2 consecutive failure(s)") {
		t.Fatalf("status=%s reasons=%#v", res.Status, res.Reasons)
	}
	if !strings.Contains(res.Reasons[0].Results, job+"/1") || strings.Index(res.Reasons[0].Results, job+"/1") > strings.Index(res.Reasons[0].Results, job+"/2") {
		t.Fatalf("streak order = %s", res.Reasons[0].Results)
	}
}

func TestJUnitProber_LatestRunGroupWindowLease(t *testing.T) {
	const job = "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
	bucket := defaultGCSBucket
	resource := "aws-2--us-east-2--openshift-org-aws-quota-slice-210"
	failing := readJUnitFixture(t, "failing.xml")
	ok := readJUnitFixture(t, "ok_single.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	now := time.Now()
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {body: "4"},
		gcsListObjectURLTest(bucket, job):                    {body: gcsListBody(job, []string{"4", "3", "2", "1"})},
	}
	add := func(id, xml string, started time.Time) {
		responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: fmt.Sprintf(`{"timestamp":%d}`, started.Unix())}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
		responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: xml}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectBuildLog)] = mockHTTPResponse{body: leaseLog(resource)}
	}
	add("4", ok, now)
	add("3", failing, now.Add(-17*time.Minute))
	add("2", failing, now.Add(-20*time.Minute))
	add("1", failing, now.Add(-40*time.Minute))

	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 48*time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:          15,
			FailedRunsThreshold:  2,
			ArtifactURLStyle:     types.JUnitArtifactStyleGCS,
			GroupBy:              types.JUnitGroupByLeaseResourcePrefix,
			LatestRunGroupWindow: 30 * time.Minute,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil {
		t.Fatalf("unexpected error: %v", res.Error)
	}
	if res.Status != types.StatusDegraded || len(res.Reasons) != 1 || !strings.Contains(res.Reasons[0].Results, "cloud-account: aws-2: 2 failure(s) in the latest 30m") {
		t.Fatalf("status=%s reasons=%#v", res.Status, res.Reasons)
	}
	if !strings.Contains(res.Reasons[0].Results, job+"/3") || !strings.Contains(res.Reasons[0].Results, job+"/2") || strings.Contains(res.Reasons[0].Results, job+"/1") {
		t.Fatalf("cohort = %s", res.Reasons[0].Results)
	}
	if strings.Index(res.Reasons[0].Results, job+"/3") > strings.Index(res.Reasons[0].Results, job+"/2") {
		t.Fatalf("cohort order = %s", res.Reasons[0].Results)
	}
}

func TestJUnitProber_LatestRunGroupWindowJob(t *testing.T) {
	const job = "periodic-ci-openshift-hypershift-release-5.1-periodics-e2e-aws-ovn-conformance"
	bucket := defaultGCSBucket
	failing := readJUnitFixture(t, "failing.xml")
	ok := readJUnitFixture(t, "ok_single.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	now := time.Now()

	tests := []struct {
		name       string
		window     time.Duration
		maxAge     time.Duration
		wantSubstr string
		absent     string
	}{
		{name: "window keeps the nearby failure", window: 30 * time.Minute, maxAge: 48 * time.Hour, wantSubstr: "build 2", absent: "build 1"},
		{name: "zero max age still honors the window", window: 30 * time.Minute, wantSubstr: "build 2", absent: "build 1"},
		{name: "unset window keeps the full history", maxAge: 48 * time.Hour, wantSubstr: "build 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mockHTTPResponse{
				gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {body: "3"},
				gcsListObjectURLTest(bucket, job):                    {body: gcsListBody(job, []string{"3", "2", "1"})},
			}
			add := func(id, xml string, started time.Time) {
				responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: fmt.Sprintf(`{"timestamp":%d}`, started.Unix())}
				responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
				responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: xml}
			}
			add("3", ok, now)
			add("2", failing, now.Add(-17*time.Minute))
			add("1", failing, now.Add(-40*time.Minute))
			p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, tt.maxAge, types.SeverityDegraded,
				JUnitProberSettings{
					HistoryRuns:          15,
					FailedRunsThreshold:  1,
					ArtifactURLStyle:     types.JUnitArtifactStyleGCS,
					LatestRunGroupWindow: tt.window,
				},
				&mockHTTPDoer{responses: responses})
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			res := <-results
			if res.Error != nil {
				t.Fatalf("unexpected error: %v", res.Error)
			}
			if res.Status != types.StatusDegraded || len(res.Reasons) != 1 || !strings.Contains(res.Reasons[0].Results, tt.wantSubstr) {
				t.Fatalf("status=%s reasons=%#v", res.Status, res.Reasons)
			}
			if tt.absent != "" && strings.Contains(res.Reasons[0].Results, tt.absent) {
				t.Fatalf("cohort = %s", res.Reasons[0].Results)
			}
		})
	}
}

func TestJUnitProber_AllReadsFail(t *testing.T) {
	bucket := defaultGCSBucket
	ok := readJUnitFixture(t, "ok_single.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`

	tests := []struct {
		name    string
		job     string
		groupBy string
		failLog bool
	}{
		{name: "junit fetch fails for every build", job: "periodic-build-farm-canary-build01"},
		{name: "lease log fetch fails for every build", job: "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn", groupBy: types.JUnitGroupByLeaseResourcePrefix, failLog: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := map[string]mockHTTPResponse{
				gcsProwObjectURL(bucket, tt.job, prowObjectLatestBuild): {body: "2"},
				gcsListObjectURLTest(bucket, tt.job):                    {body: gcsListBody(tt.job, []string{"2", "1"})},
			}
			for _, id := range []string{"2", "1"} {
				responses[gcsProwObjectURL(bucket, tt.job, id, prowObjectStarted)] = mockHTTPResponse{body: recentStarted()}
				responses[gcsProwObjectURL(bucket, tt.job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
				junit := mockHTTPResponse{body: ok}
				if !tt.failLog {
					junit = mockHTTPResponse{statusCode: 500, body: "unavailable"}
				}
				responses[gcsProwObjectURL(bucket, tt.job, id, "artifacts", "junit_canary.xml")] = junit
				if tt.failLog {
					responses[gcsProwObjectURL(bucket, tt.job, id, prowObjectBuildLog)] = mockHTTPResponse{statusCode: 500, body: "unavailable"}
				}
			}
			p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{tt.job}, 48*time.Hour, types.SeverityDegraded,
				JUnitProberSettings{
					HistoryRuns:         2,
					FailedRunsThreshold: 1,
					ArtifactURLStyle:    types.JUnitArtifactStyleGCS,
					GroupBy:             tt.groupBy,
				},
				&mockHTTPDoer{responses: responses})
			results := make(chan ProbeResult, 1)
			p.Probe(context.Background(), results)
			res := <-results
			if res.Error == nil || res.Status == types.StatusHealthy {
				t.Fatalf("status=%s err=%v, want error", res.Status, res.Error)
			}
		})
	}
}

func TestJUnitProber_MaxAgeDropsAfterHistory(t *testing.T) {
	const job = "periodic-build-farm-canary-build01"
	bucket := defaultGCSBucket
	failing := readJUnitFixture(t, "failing.xml")
	finishedBody := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	now := time.Now()
	responses := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {body: "3"},
		gcsListObjectURLTest(bucket, job):                    {body: gcsListBody(job, []string{"3", "2", "1"})},
	}
	add := func(id string, started time.Time) {
		responses[gcsProwObjectURL(bucket, job, id, prowObjectStarted)] = mockHTTPResponse{body: fmt.Sprintf(`{"timestamp":%d}`, started.Unix())}
		responses[gcsProwObjectURL(bucket, job, id, prowObjectFinished)] = mockHTTPResponse{body: finishedBody}
		responses[gcsProwObjectURL(bucket, job, id, "artifacts", "junit_canary.xml")] = mockHTTPResponse{body: failing}
	}
	add("3", now.Add(-5*time.Hour))
	add("2", now.Add(-6*time.Hour))
	add("1", now.Add(-10*time.Minute))
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, time.Hour, types.SeverityDegraded,
		JUnitProberSettings{
			HistoryRuns:         2,
			FailedRunsThreshold: 1,
			ArtifactURLStyle:    types.JUnitArtifactStyleGCS,
		},
		&mockHTTPDoer{responses: responses})
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	res := <-results
	if res.Error != nil || res.Status != types.StatusHealthy {
		t.Fatalf("status=%s err=%v reasons=%#v, want healthy", res.Status, res.Error, res.Reasons)
	}
}

func gcsListBody(job string, ids []string) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf(`"logs/%s/%s/"`, job, id)
	}
	return `{"prefixes":[` + strings.Join(parts, ",") + `]}`
}

func probeJUnit(t *testing.T, p *JUnitProber) ProbeResult {
	t.Helper()
	results := make(chan ProbeResult, 1)
	p.Probe(context.Background(), results)
	select {
	case res := <-results:
		return res
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for result")
	}
	return ProbeResult{}
}

func finishedJUnitResponses(bucket, job, build, xml string) map[string]mockHTTPResponse {
	return map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):                  {body: build},
		gcsProwObjectURL(bucket, job, build, prowObjectFinished):              {body: `{"timestamp":1,"result":"SUCCESS","passed":true}`},
		gcsProwObjectURL(bucket, job, build, "artifacts", "junit_canary.xml"): {body: xml},
	}
}

func TestJUnitProber_ProbeJobs(t *testing.T) {
	const jobA = "job-a"
	const jobB = "job-b"
	bucket := defaultGCSBucket
	okXML := readJUnitFixture(t, "ok_single.xml")
	failingXML := readJUnitFixture(t, "failing.xml")
	settings := JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS}

	merge := func(parts ...map[string]mockHTTPResponse) map[string]mockHTTPResponse {
		out := map[string]mockHTTPResponse{}
		for _, part := range parts {
			for k, v := range part {
				out[k] = v
			}
		}
		return out
	}
	unreadable := func(job string) map[string]mockHTTPResponse {
		return map[string]mockHTTPResponse{
			gcsProwObjectURL(bucket, job, prowObjectLatestBuild): {err: fmt.Errorf("network error")},
		}
	}
	probe := func(responses map[string]mockHTTPResponse) ProbeResult {
		t.Helper()
		p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{jobA, jobB}, 0, types.SeverityDown, settings, &mockHTTPDoer{responses: responses})
		return probeJUnit(t, p)
	}

	t.Run("failing job escalates past a passing job", func(t *testing.T) {
		res := probe(merge(finishedJUnitResponses(bucket, jobA, "11", okXML), finishedJUnitResponses(bucket, jobB, "22", failingXML)))
		if res.Error != nil {
			t.Fatalf("unexpected error: %v", res.Error)
		}
		if res.Status != types.StatusDown {
			t.Fatalf("status = %s, want Down", res.Status)
		}
		if len(res.Reasons) != 1 || res.Reasons[0].Check != jobB {
			t.Fatalf("reasons = %#v, want only %s", res.Reasons, jobB)
		}
		if !strings.Contains(res.Reasons[0].Results, "2/4 tests failed: git-clone, quay-pull") {
			t.Fatalf("reason = %q", res.Reasons[0].Results)
		}
	})

	t.Run("reasons from both failing jobs", func(t *testing.T) {
		res := probe(merge(finishedJUnitResponses(bucket, jobA, "11", failingXML), finishedJUnitResponses(bucket, jobB, "22", failingXML)))
		if res.Error != nil {
			t.Fatalf("unexpected error: %v", res.Error)
		}
		if res.Status != types.StatusDown {
			t.Fatalf("status = %s, want Down", res.Status)
		}
		if len(res.Reasons) != 2 || res.Reasons[0].Check != jobA || res.Reasons[1].Check != jobB {
			t.Fatalf("reasons = %#v", res.Reasons)
		}
	})

	t.Run("unreadable job does not hide an unhealthy sibling", func(t *testing.T) {
		res := probe(merge(unreadable(jobA), finishedJUnitResponses(bucket, jobB, "22", failingXML)))
		if res.Error != nil {
			t.Fatalf("unexpected error: %v", res.Error)
		}
		if res.Status != types.StatusDown {
			t.Fatalf("status = %s, want Down", res.Status)
		}
		if len(res.Reasons) != 1 || res.Reasons[0].Check != jobB {
			t.Fatalf("reasons = %#v", res.Reasons)
		}
	})

	t.Run("unreadable job with a healthy sibling returns error", func(t *testing.T) {
		res := probe(merge(unreadable(jobA), finishedJUnitResponses(bucket, jobB, "22", okXML)))
		if res.Error == nil {
			t.Fatal("expected error")
		}
		if res.Status == types.StatusHealthy {
			t.Fatalf("status = %s", res.Status)
		}
	})

	t.Run("all jobs unreadable returns error", func(t *testing.T) {
		res := probe(merge(unreadable(jobA), unreadable(jobB)))
		if res.Error == nil {
			t.Fatal("expected error")
		}
		if res.Status == types.StatusHealthy {
			t.Fatalf("status = %s", res.Status)
		}
	})
}

func TestJUnitProber_ProbeByGroup_AllUnreadable(t *testing.T) {
	bucket := defaultGCSBucket
	const jobA = "job-a"
	const jobB = "job-b"
	p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{jobA, jobB}, 0, types.SeverityDegraded,
		JUnitProberSettings{HistoryRuns: 1, ArtifactURLStyle: types.JUnitArtifactStyleGCS, GroupBy: types.JUnitGroupByLeaseResourcePrefix},
		&mockHTTPDoer{responses: map[string]mockHTTPResponse{
			gcsProwObjectURL(bucket, jobA, prowObjectLatestBuild): {err: fmt.Errorf("network error")},
			gcsProwObjectURL(bucket, jobB, prowObjectLatestBuild): {err: fmt.Errorf("network error")},
		}})
	res := probeJUnit(t, p)
	if res.Error == nil {
		t.Fatal("expected error")
	}
	if res.Status == types.StatusHealthy {
		t.Fatalf("status = %s", res.Status)
	}
}

type logCapture struct {
	mu      sync.Mutex
	entries []logrus.Fields
}

func (l *logCapture) Levels() []logrus.Level { return logrus.AllLevels }

func (l *logCapture) Fire(entry *logrus.Entry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	copied := logrus.Fields{}
	for k, v := range entry.Data {
		copied[k] = v
	}
	copied["msg"] = entry.Message
	l.entries = append(l.entries, copied)
	return nil
}

func (l *logCapture) find(outcome string) logrus.Fields {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e["msg"] == "junit artifact evaluation" && e["outcome"] == outcome {
			return e
		}
	}
	return nil
}

func TestJUnitProber_DebugLogsArtifact(t *testing.T) {
	oldLevel := logrus.GetLevel()
	oldHooks := logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.SetLevel(logrus.DebugLevel)
	hook := &logCapture{}
	logrus.AddHook(hook)
	t.Cleanup(func() {
		logrus.SetLevel(oldLevel)
		logrus.StandardLogger().ReplaceHooks(oldHooks)
	})

	const job = "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
	const build = "10"
	bucket := defaultGCSBucket
	artifact := "artifacts/e2e-aws-ovn/gather-must-gather/artifacts/junit_install.xml"
	body := `<?xml version="1.0"?><testsuite tests="3">
  <testcase name="install should succeed: infrastructure"/>
  <testcase name="install should succeed: overall"><failure/></testcase>
  <testcase name="unrelated"><failure/></testcase>
</testsuite>`
	finished := `{"timestamp":1,"result":"SUCCESS","passed":true}`
	xmlURL := gcsProwObjectURL(bucket, job, build, "artifacts", "e2e-aws-ovn", "gather-must-gather", "artifacts", "junit_install.xml")
	base := map[string]mockHTTPResponse{
		gcsProwObjectURL(bucket, job, prowObjectLatestBuild):     {body: build},
		gcsProwObjectURL(bucket, job, build, prowObjectFinished): {body: finished},
	}
	settings := JUnitProberSettings{
		HistoryRuns:      1,
		ArtifactPath:     artifact,
		TestcasePrefix:   "install should succeed:",
		ArtifactURLStyle: types.JUnitArtifactStyleGCS,
	}

	t.Run("found", func(t *testing.T) {
		hook.mu.Lock()
		hook.entries = nil
		hook.mu.Unlock()
		responses := map[string]mockHTTPResponse{xmlURL: {body: body}}
		for k, v := range base {
			responses[k] = v
		}
		p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 0, types.SeverityDegraded, settings, &mockHTTPDoer{responses: responses})
		res := probeJUnit(t, p)
		if res.Error != nil {
			t.Fatalf("unexpected error: %v", res.Error)
		}
		entry := hook.find("found")
		if entry == nil {
			t.Fatalf("missing found log, entries=%v", hook.entries)
		}
		if entry["artifact_path"] != artifact {
			t.Fatalf("artifact_path = %v", entry["artifact_path"])
		}
		url, _ := entry["url"].(string)
		if !strings.Contains(url, artifact) {
			t.Fatalf("url = %v", entry["url"])
		}
		names, _ := entry["matched_tests"].([]string)
		if diff := cmp.Diff([]string{"install should succeed: infrastructure", "install should succeed: overall"}, names); diff != "" {
			t.Fatalf("matched_tests mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("missing", func(t *testing.T) {
		hook.mu.Lock()
		hook.entries = nil
		hook.mu.Unlock()
		responses := map[string]mockHTTPResponse{xmlURL: {statusCode: 404, body: "not found"}}
		for k, v := range base {
			responses[k] = v
		}
		p := newJUnitProber(testComponentSlug, testSubComponentSlug, bucket, []string{job}, 0, types.SeverityDegraded, settings, &mockHTTPDoer{responses: responses})
		_ = probeJUnit(t, p)
		entry := hook.find("missing")
		if entry == nil {
			t.Fatalf("missing missing-artifact log, entries=%v", hook.entries)
		}
		url, _ := entry["url"].(string)
		if !strings.Contains(url, artifact) {
			t.Fatalf("url = %v", entry["url"])
		}
	})
}
