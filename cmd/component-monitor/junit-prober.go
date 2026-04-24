package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ship-status-dash/pkg/types"
)

const defaultGCSBucket = "test-platform-results"

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// junitDoc decodes both <testsuites> and bare <testsuite> root elements.
// When the root is <testsuites>, Suites is populated; when it is <testsuite>,
// Tests and TestCases are populated directly.
type junitDoc struct {
	Suites    []junitSuite `xml:"testsuite"`
	Tests     int          `xml:"tests,attr"`
	TestCases []junitCase  `xml:"testcase"`
}

type junitSuite struct {
	Tests     int         `xml:"tests,attr"`
	TestCases []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name    string    `xml:"name,attr"`
	Failure *struct{} `xml:"failure"`
}

type prowStarted struct {
	Timestamp int64 `json:"timestamp"`
}

// JUnitProber reads JUnit XML artifacts from GCS produced by a Prow periodic job and reports component status.
type JUnitProber struct {
	componentSlug    string
	subComponentSlug string
	bucket           string
	jobName          string
	maxAge           time.Duration
	severity         types.Severity
	client           httpDoer
}

// NewJUnitProber returns a JUnitProber for the given job and GCS bucket.
func NewJUnitProber(componentSlug, subComponentSlug, bucket, jobName string, maxAge time.Duration, severity types.Severity, client httpDoer) *JUnitProber {
	if bucket == "" {
		bucket = defaultGCSBucket
	}
	if severity == "" {
		severity = types.SeverityDegraded
	}
	return &JUnitProber{
		componentSlug:    componentSlug,
		subComponentSlug: subComponentSlug,
		bucket:           bucket,
		jobName:          jobName,
		maxAge:           maxAge,
		severity:         severity,
		client:           client,
	}
}

func (p *JUnitProber) gcsURL(parts ...string) string {
	return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s/%s", p.bucket, p.jobName, strings.Join(parts, "/"))
}

func (p *JUnitProber) formatErrorResult(err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		Error: fmt.Errorf("error running JUnit probe, for component: %s sub-component %s. job: %s. error: %w", p.componentSlug, p.subComponentSlug, p.jobName, err),
	}
}

func (p *JUnitProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	buildID, err := p.fetchText(ctx, p.gcsURL("latest-build.txt"))
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("fetching latest build ID: %w", err))
		return
	}
	buildID = strings.TrimSpace(buildID)

	startedBody, err := p.fetchText(ctx, p.gcsURL(buildID, "started.json"))
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("fetching started.json for build %s: %w", buildID, err))
		return
	}

	var started prowStarted
	if err := json.Unmarshal([]byte(startedBody), &started); err != nil {
		results <- p.formatErrorResult(fmt.Errorf("parsing started.json for build %s: %w", buildID, err))
		return
	}
	if started.Timestamp <= 0 {
		results <- p.formatErrorResult(fmt.Errorf("invalid or missing timestamp in started.json for build %s", buildID))
		return
	}

	if age := time.Since(time.Unix(started.Timestamp, 0)); age > p.maxAge {
		results <- ProbeResult{
			ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    p.componentSlug,
				SubComponentSlug: p.subComponentSlug,
				Status:           p.severity.ToStatus(),
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   p.jobName,
					Results: fmt.Sprintf("latest build %s started %s ago (max age %s)", buildID, age.Round(time.Minute), p.maxAge),
				}},
			},
		}
		return
	}

	xmlBody, err := p.fetchText(ctx, p.gcsURL(buildID, "artifacts/junit_canary.xml"))
	if err != nil {
		results <- p.formatErrorResult(fmt.Errorf("fetching junit XML for build %s: %w", buildID, err))
		return
	}

	var doc junitDoc
	if err := xml.NewDecoder(strings.NewReader(xmlBody)).Decode(&doc); err != nil {
		results <- p.formatErrorResult(fmt.Errorf("parsing junit XML for build %s: %w", buildID, err))
		return
	}

	suites := doc.Suites
	if len(suites) == 0 {
		suites = []junitSuite{{Tests: doc.Tests, TestCases: doc.TestCases}}
	}

	results <- p.makeStatus(buildID, suites)
}

func (p *JUnitProber) makeStatus(buildID string, suites []junitSuite) ProbeResult {
	var failed []string
	var totalTests int
	for _, s := range suites {
		totalTests += s.Tests
		for _, tc := range s.TestCases {
			if tc.Failure != nil {
				failed = append(failed, tc.Name)
			}
		}
	}

	var status types.Status
	var reason string
	switch {
	case totalTests == 0:
		status = p.severity.ToStatus()
		reason = fmt.Sprintf("build %s: zero tests found", buildID)
	case len(failed) == 0:
		status = types.StatusHealthy
		reason = fmt.Sprintf("build %s: all %d tests passed", buildID, totalTests)
	default:
		status = p.severity.ToStatus()
		reason = fmt.Sprintf("build %s: %d/%d tests failed: %s", buildID, len(failed), totalTests, strings.Join(failed, ", "))
	}

	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           status,
			Reasons: []types.Reason{{
				Type:    types.CheckTypeJUnit,
				Check:   p.jobName,
				Results: reason,
			}},
		},
	}
}

func (p *JUnitProber) fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
