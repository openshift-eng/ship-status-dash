package junit

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	defaultGCSBucket      = "test-platform-results-public"
	prowObjectLatestBuild = "latest-build.txt"
	prowObjectStarted     = "started.json"
	prowObjectFinished    = "finished.json"
	prowObjectBuildLog    = "build-log.txt"
	maxTextBodyBytes      = 4 * 1024 * 1024
	defaultGCSWebBase     = types.JUnitDefaultGCSWebBase

	// History grouping: runs count toward the threshold when they share the
	// same normalized failure (sorted unique failing test names, a zero-test run,
	// or a missing JUnit file on a finished build).
	junitSignatureZero          = "zero_tests"
	junitSignatureFailedPfx     = "failed:" // + sorted, comma-joined test names
	junitSignatureMissingJUnit  = "missing_junit"
	junitSignatureNoPrefixMatch = "no_prefix_match"

	// defaultLastSuccessScanLimit is how many recent finished builds to scan for a
	// last-success Spyglass link when enriching a stale reason and HistoryRuns is 1.
	defaultLastSuccessScanLimit = 5
)

type httpStatusError struct {
	StatusCode int
	URL        string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

func isHTTP404(err error) bool {
	var httpErr *httpStatusError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

// junitHTTPClient is the HTTP client used to GET Prow and GCS resources. *http.Client implements it.
type junitHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

// ProbeResult is one JUnit reading. The component monitor adds the probe type when it reports.
type ProbeResult struct {
	types.ComponentMonitorReportComponentStatus
	Error error
}

// JUnitProberSettings holds JUnit prober options parsed from JUnitMonitor.
type JUnitProberSettings struct {
	ArtifactURLStyle         string
	GCSWebBaseURL            string
	HistoryRuns              int
	FailedRunsThreshold      int
	ArtifactPath             string
	TestcasePrefix           string
	AlertOnNoRecentRun       bool
	MissingArtifactUnhealthy *bool
	GroupBy                  string
	LatestRunGroupWindow     time.Duration
}

// JUnitProber fetches Prow JUnit XML from GCS or GCSweb and reports component health.
type JUnitProber struct {
	componentSlug    string
	subComponentSlug string
	bucket           string
	jobNames         []string
	maxAge           time.Duration
	severity         types.Severity
	client           junitHTTPClient
	settings         JUnitProberSettings

	mu         sync.Mutex
	evalCache  map[string]junitEvalCacheEntry
	leaseCache map[string]leaseCacheEntry
}

type junitEvalCacheEntry struct {
	total       int
	failed      []string
	artifactSig string
}

// NewJUnitProber returns a JUnitProber. It uses default bucket, severity, and artifact style when empty,
// enforces a minimum history of 1, and sets FailedRunsThreshold to 1 when a single run is configured.
// Job names are trimmed. ArtifactPath is required; config validation rejects an empty artifact_path.
func NewJUnitProber(componentSlug, subComponentSlug, bucket string, jobNames []string, maxAge time.Duration, severity types.Severity, settings JUnitProberSettings, client junitHTTPClient) *JUnitProber {
	names := make([]string, 0, len(jobNames))
	for _, name := range jobNames {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	if bucket == "" {
		bucket = defaultGCSBucket
	}
	if severity == "" {
		severity = types.SeverityDegraded
	}
	if settings.HistoryRuns < 1 {
		settings.HistoryRuns = 1
	}
	if settings.HistoryRuns == 1 {
		settings.FailedRunsThreshold = 1
	}
	if settings.ArtifactURLStyle == "" {
		settings.ArtifactURLStyle = types.JUnitArtifactStyleGCS
	}
	settings.ArtifactPath = strings.TrimSpace(settings.ArtifactPath)
	settings.TestcasePrefix = strings.TrimSpace(settings.TestcasePrefix)
	settings.GroupBy = strings.TrimSpace(settings.GroupBy)
	if settings.GroupBy == "" {
		settings.GroupBy = types.JUnitGroupByJob
	}
	return &JUnitProber{
		componentSlug:    componentSlug,
		subComponentSlug: subComponentSlug,
		bucket:           bucket,
		jobNames:         names,
		maxAge:           maxAge,
		severity:         severity,
		client:           client,
		settings:         settings,
		evalCache:        map[string]junitEvalCacheEntry{},
		leaseCache:       map[string]leaseCacheEntry{},
	}
}

func (p *JUnitProber) prowLogObjectURL(job string, segments ...string) string {
	tail := strings.Join(segments, "/")
	switch p.settings.ArtifactURLStyle {
	case types.JUnitArtifactStyleGCSWeb:
		base := strings.TrimRight(p.gcswebBase(), "/")
		return fmt.Sprintf("%s/gcs/%s/logs/%s/%s", base, p.bucket, job, tail)
	default:
		return fmt.Sprintf("https://storage.googleapis.com/%s/logs/%s/%s", p.bucket, job, tail)
	}
}

func (p *JUnitProber) gcswebBase() string {
	if strings.TrimSpace(p.settings.GCSWebBaseURL) != "" {
		return strings.TrimRight(p.settings.GCSWebBaseURL, "/")
	}
	return defaultGCSWebBase
}

func (p *JUnitProber) formatErrorResult(job string, err error) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
		},
		Error: fmt.Errorf("error running JUnit probe, for component: %s sub-component %s. job: %s. error: %w", p.componentSlug, p.subComponentSlug, job, err),
	}
}

func (p *JUnitProber) healthyResult() ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           types.StatusHealthy,
		},
	}
}

// checkBuildStaleness fetches started.json for the given build and checks
// whether it exceeds maxAge. Returns a non-nil *ProbeResult when the build
// is stale; returns (nil, nil) when fresh; returns (nil, err) on fetch/parse
// failures.
func (p *JUnitProber) checkBuildStaleness(ctx context.Context, job, buildID string) (*ProbeResult, error) {
	startedBody, err := p.fetchText(ctx, p.prowLogObjectURL(job, buildID, prowObjectStarted))
	if err != nil {
		if isHTTP404(err) {
			finished, finErr := p.isBuildFinished(ctx, job, buildID)
			if finErr != nil {
				return nil, finErr
			}
			if finished {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("fetching started.json for build %s: %w", buildID, err)
	}
	var started prowStarted
	if err := json.Unmarshal([]byte(startedBody), &started); err != nil {
		return nil, fmt.Errorf("parsing started.json for build %s: %w", buildID, err)
	}
	if started.Timestamp <= 0 {
		return nil, fmt.Errorf("invalid or missing timestamp in started.json for build %s", buildID)
	}
	if age := time.Since(time.Unix(started.Timestamp, 0)); age > p.maxAge {
		return &ProbeResult{
			ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    p.componentSlug,
				SubComponentSlug: p.subComponentSlug,
				Status:           p.severity.ToStatus(),
				Reasons: []types.Reason{{
					Type:    types.CheckTypeJUnit,
					Check:   job,
					Results: fmt.Sprintf("latest build %s started %s ago (max age %s)", buildID, age.Round(time.Minute), p.maxAge),
				}},
			},
		}, nil
	}
	return nil, nil
}

func (p *JUnitProber) isBuildFinished(ctx context.Context, job, buildID string) (bool, error) {
	_, err := p.fetchText(ctx, p.prowLogObjectURL(job, buildID, prowObjectFinished))
	if err == nil {
		return true, nil
	}
	if isHTTP404(err) {
		return false, nil
	}
	return false, err
}

func (p *JUnitProber) spyglassViewURL(job, buildID string) string {
	return fmt.Sprintf("%s/view/gs/%s/logs/%s/%s", types.JUnitDefaultProwSpyglassBase, p.bucket, job, buildID)
}

func (p *JUnitProber) lastSuccessScanLimit() int {
	if p.settings.HistoryRuns > 1 {
		return p.settings.HistoryRuns
	}
	return defaultLastSuccessScanLimit
}

// enrichStaleResult appends a Spyglass link for the stale build and, when possible,
// a link to the most recent successful finished run among recent history.
func (p *JUnitProber) enrichStaleResult(ctx context.Context, job string, stale *ProbeResult, staleBuildID string) {
	parts := []string{stale.Reasons[0].Results, p.spyglassViewURL(job, staleBuildID)}
	lastID, err := p.findLastSuccessfulBuild(ctx, job, staleBuildID)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"component":     p.componentSlug,
			"sub_component": p.subComponentSlug,
			"job":           job,
			"stale_build":   staleBuildID,
		}).Warn("failed to find last successful junit run for stale enrichment")
		parts = append(parts, "last success lookup failed")
	} else if lastID == "" {
		parts = append(parts, "no recent successful run found")
	} else {
		parts = append(parts, fmt.Sprintf("last success: %s %s", lastID, p.spyglassViewURL(job, lastID)))
	}
	stale.Reasons[0].Results = strings.Join(parts, "; ")
}

// findLastSuccessfulBuild returns the newest finished build (excluding staleBuildID)
// whose JUnit evaluation is healthy, scanning up to lastSuccessScanLimit runs.
// An empty string with a nil error means none were found. Per-build JUnit evaluation
// errors are skipped so a single bad artifact does not abort the scan.
func (p *JUnitProber) findLastSuccessfulBuild(ctx context.Context, job, staleBuildID string) (string, error) {
	limit := p.lastSuccessScanLimit()
	finished, err := p.listFinishedBuildIDsDesc(ctx, job, staleBuildID, limit+1)
	if err != nil {
		return "", err
	}
	scanned := 0
	for _, id := range finished {
		if id == staleBuildID {
			continue
		}
		if scanned >= limit {
			break
		}
		scanned++
		total, failed, artifactSig, evalErr := p.evaluateBuildJUnit(ctx, job, id)
		if evalErr != nil {
			continue
		}
		if !junitEvalUnhealthy(total, failed, artifactSig) {
			return id, nil
		}
	}
	return "", nil
}

func (p *JUnitProber) listFinishedBuildIDsDesc(ctx context.Context, job, latestFromFile string, maxFinished int) ([]string, error) {
	ids, err := p.listBuildIDPrefixes(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("listing recent builds: %w", err)
	}
	seen := sets.New(latestFromFile)
	seen.Insert(ids...)
	all := seen.UnsortedList()
	sortBuildIDsDesc(all)

	// A missing finished.json is a gap, not a probe failure, while the build
	// is still inside max_age. The newest id is the common in-progress case.
	// An older gap is skipped the same way so a finished build further back
	// can still be scored. Once a gap is outside max_age, older ids are too,
	// so the scan stops.
	logFields := logrus.Fields{
		"component":     p.componentSlug,
		"sub_component": p.subComponentSlug,
		"job":           job,
	}
	var finished []string
	for i, id := range all {
		done, fErr := p.isBuildFinished(ctx, job, id)
		if fErr != nil {
			return nil, fmt.Errorf("checking finished.json for build %s: %w", id, fErr)
		}
		if done {
			finished = append(finished, id)
			if maxFinished > 0 && len(finished) >= maxFinished {
				break
			}
			continue
		}
		entry := logrus.WithFields(logFields).WithField("build", id)
		if i == 0 {
			entry.Info("Excluding unfinished latest build from finished build list")
			continue
		}
		within, ageErr := p.buildWithinMaxAge(ctx, job, id)
		if ageErr != nil {
			return nil, fmt.Errorf("checking max age for unfinished build %s: %w", id, ageErr)
		}
		entry.Warn("Excluding unfinished build from finished build list")
		if !within {
			break
		}
	}
	return finished, nil
}

func (p *JUnitProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	if len(p.jobNames) == 0 {
		results <- p.formatErrorResult("", fmt.Errorf("no job_names configured"))
		return
	}
	switch p.settings.GroupBy {
	case types.JUnitGroupByJob:
		if len(p.jobNames) == 1 {
			results <- p.probeJob(ctx, p.jobNames[0])
			return
		}
		results <- p.probeJobs(ctx)
	case types.JUnitGroupByLeaseResourcePrefix:
		results <- p.probeByGroup(ctx)
	default:
		results <- p.formatErrorResult(strings.Join(p.jobNames, ","), fmt.Errorf("invalid group_by %q", p.settings.GroupBy))
	}
}

// combineJobProbe reports an unhealthy reading when one exists. A read error is returned only when
// no readable job is unhealthy, so mergeStatuses can omit the sub-component instead of reporting Healthy.
func (p *JUnitProber) combineJobProbe(status types.Status, reasons []types.Reason, readErr error) ProbeResult {
	if len(reasons) > 0 {
		if status == "" || status == types.StatusHealthy {
			status = p.severity.ToStatus()
		}
		return ProbeResult{
			ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
				ComponentSlug:    p.componentSlug,
				SubComponentSlug: p.subComponentSlug,
				Status:           status,
				Reasons:          reasons,
			},
		}
	}
	if readErr != nil {
		return p.formatErrorResult(strings.Join(p.jobNames, ","), readErr)
	}
	return p.healthyResult()
}

type junitRunEval struct {
	job         string
	buildID     string
	started     time.Time
	total       int
	failed      []string
	artifactSig string
}

func (p *JUnitProber) buildWithinMaxAge(ctx context.Context, job, buildID string) (bool, error) {
	if p.maxAge <= 0 {
		return true, nil
	}
	started, err := p.fetchBuildStarted(ctx, job, buildID)
	if isHTTP404(err) || (err == nil && started.IsZero()) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if time.Since(started) > p.maxAge {
		return false, nil
	}
	return true, nil
}

type includeResult int

const (
	includeDroppedAge includeResult = iota
	includeUnscored
	includeScored
)

func (p *JUnitProber) evaluateIncludedBuild(ctx context.Context, job, buildID string) (junitRunEval, includeResult, error) {
	var started time.Time
	if p.maxAge > 0 || p.settings.LatestRunGroupWindow > 0 || p.settings.GroupBy == types.JUnitGroupByLeaseResourcePrefix {
		var err error
		started, err = p.fetchBuildStarted(ctx, job, buildID)
		if isHTTP404(err) {
			started, err = time.Time{}, nil
		}
		if err != nil {
			return junitRunEval{}, includeDroppedAge, err
		}
		if p.maxAge > 0 && !started.IsZero() && time.Since(started) > p.maxAge {
			return junitRunEval{}, includeDroppedAge, nil
		}
	}
	total, failed, artifactSig, perr := p.evaluateBuildJUnit(ctx, job, buildID)
	if perr != nil {
		return junitRunEval{}, includeDroppedAge, perr
	}
	if p.unscoredArtifact(artifactSig) {
		return junitRunEval{}, includeUnscored, nil
	}
	return junitRunEval{
		job: job, buildID: buildID, started: started, total: total, failed: failed, artifactSig: artifactSig,
	}, includeScored, nil
}

func (p *JUnitProber) unscoredArtifact(artifactSig string) bool {
	if artifactSig == junitSignatureNoPrefixMatch {
		return true
	}
	return artifactSig == junitSignatureMissingJUnit && !p.countMissingArtifact()
}

func (p *JUnitProber) fetchBuildStarted(ctx context.Context, job, buildID string) (time.Time, error) {
	body, err := p.fetchText(ctx, p.prowLogObjectURL(job, buildID, prowObjectStarted))
	if err != nil {
		return time.Time{}, err
	}
	var started prowStarted
	if err := json.Unmarshal([]byte(body), &started); err != nil {
		return time.Time{}, fmt.Errorf("parsing started.json for build %s: %w", buildID, err)
	}
	if started.Timestamp <= 0 {
		return time.Time{}, nil
	}
	return time.Unix(started.Timestamp, 0), nil
}

func (p *JUnitProber) countMissingArtifact() bool {
	if p.settings.MissingArtifactUnhealthy == nil {
		return true
	}
	return *p.settings.MissingArtifactUnhealthy
}

func (p *JUnitProber) jobBuilds(ctx context.Context, job string) ([]string, *ProbeResult, error) {
	latest, err := p.fetchText(ctx, p.prowLogObjectURL(job, prowObjectLatestBuild))
	if err != nil {
		return nil, nil, fmt.Errorf("fetching latest build ID: %w", err)
	}
	latest = strings.TrimSpace(latest)
	if latest == "" {
		return nil, nil, fmt.Errorf("empty latest build ID from %s", prowObjectLatestBuild)
	}

	finished, finErr := p.isBuildFinished(ctx, job, latest)
	if finErr != nil {
		return nil, nil, fmt.Errorf("checking finished.json for build %s: %w", latest, finErr)
	}

	unfinishedBuild := ""
	if !finished {
		logrus.WithFields(logrus.Fields{
			"component":     p.componentSlug,
			"sub_component": p.subComponentSlug,
			"job":           job,
			"build":         latest,
		}).Info("Latest build not finished, falling back to previous build")
		unfinishedBuild = latest
		prevID, findErr := p.findPreviousBuildID(ctx, job, latest)
		if findErr != nil {
			return nil, nil, fmt.Errorf("latest build %s not finished and %w", latest, findErr)
		}
		latest = prevID
	}

	if stale, err := p.noteStale(ctx, job, latest); err != nil || stale != nil {
		return nil, stale, err
	}

	builds, err := p.resolveBuildIDsToEvaluate(ctx, job, latest)
	if err != nil {
		return nil, nil, err
	}
	if unfinishedBuild != "" {
		filtered := builds[:0]
		for _, b := range builds {
			if b != unfinishedBuild {
				filtered = append(filtered, b)
			}
		}
		builds = filtered
	}
	return builds, nil, nil
}

func (p *JUnitProber) noteStale(ctx context.Context, job, buildID string) (*ProbeResult, error) {
	if !p.settings.AlertOnNoRecentRun {
		return nil, nil
	}
	stale, err := p.checkBuildStaleness(ctx, job, buildID)
	if err != nil || stale == nil {
		return stale, err
	}
	p.enrichStaleResult(ctx, job, stale, buildID)
	return stale, nil
}

func (p *JUnitProber) resolveBuildIDsToEvaluate(ctx context.Context, job, latestFromFile string) ([]string, error) {
	if p.settings.HistoryRuns <= 1 {
		return []string{latestFromFile}, nil
	}
	finished, err := p.listFinishedBuildIDsDesc(ctx, job, latestFromFile, p.settings.HistoryRuns)
	if err != nil {
		return nil, err
	}
	if len(finished) == 0 {
		return nil, fmt.Errorf("no finished builds found")
	}

	n := p.settings.HistoryRuns
	if len(finished) < n {
		return finished, nil
	}
	return finished[:n], nil
}

func (p *JUnitProber) findPreviousBuildID(ctx context.Context, job, currentBuildID string) (string, error) {
	ids, err := p.listBuildIDPrefixes(ctx, job)
	if err != nil {
		return "", fmt.Errorf("listing builds for fallback: %w", err)
	}
	seen := sets.New(currentBuildID)
	seen.Insert(ids...)
	all := seen.UnsortedList()
	sortBuildIDsDesc(all)
	for i, id := range all {
		if id == currentBuildID && i+1 < len(all) {
			return all[i+1], nil
		}
	}
	return "", fmt.Errorf("no previous build found before %s", currentBuildID)
}

func (p *JUnitProber) listBuildIDPrefixes(ctx context.Context, job string) ([]string, error) {
	jobPrefix := "logs/" + job + "/"
	unique := sets.New[string]()
	for page, token := 0, ""; page < 50; page++ {
		body, err := p.fetchGCSListPage(ctx, jobPrefix, token)
		if err != nil {
			return nil, err
		}
		var r gcsListObjectResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			return nil, fmt.Errorf("parsing GCS list: %w", err)
		}
		for _, pr := range r.Prefixes {
			id, ok := buildIDFromPrefixPath(pr, job)
			if ok {
				unique.Insert(id)
			}
		}
		if r.NextPageToken == "" {
			break
		}
		token = r.NextPageToken
	}
	return unique.UnsortedList(), nil
}

func (p *JUnitProber) fetchGCSListPage(ctx context.Context, prefix, pageToken string) (string, error) {
	const pageSize = 1000
	v := url.Values{}
	v.Set("prefix", prefix)
	v.Set("delimiter", "/")
	v.Set("maxResults", fmt.Sprint(pageSize))
	v.Set("fields", "prefixes,nextPageToken")
	if pageToken != "" {
		v.Set("pageToken", pageToken)
	}
	escapedBucket := url.PathEscape(p.bucket)
	u := fmt.Sprintf("https://www.googleapis.com/storage/v1/b/%s/o?%s", escapedBucket, v.Encode())
	return p.fetchText(ctx, u)
}

type gcsListObjectResponse struct {
	Prefixes      []string `json:"prefixes"`
	NextPageToken string   `json:"nextPageToken"`
}

func buildIDFromPrefixPath(prefix, jobName string) (string, bool) {
	trim := strings.TrimLeft(prefix, "/")
	expected := "logs/" + jobName + "/"
	if !strings.HasPrefix(trim, expected) {
		return "", false
	}
	rest := strings.TrimPrefix(trim, expected)
	if rest == "" {
		return "", false
	}
	id := strings.SplitN(rest, "/", 2)[0]
	if id == "" {
		return "", false
	}
	return id, true
}

func sortBuildIDsDesc(ids []string) {
	sort.Slice(ids, func(i, j int) bool { return buildIDGreater(ids[i], ids[j]) })
}

func buildIDGreater(a, b string) bool {
	ai, ok1 := new(big.Int).SetString(a, 10)
	bi, ok2 := new(big.Int).SetString(b, 10)
	if ok1 && ok2 {
		return ai.Cmp(bi) > 0
	}
	return a > b
}

// evaluateBuildJUnit returns JUnit stats for a build. When the build is finished but
// the configured artifact is missing, artifactSig is set instead of returning an error so
// the sub-component is reported unhealthy. Finished builds are cached so later ticks skip them.
func (p *JUnitProber) evaluateBuildJUnit(ctx context.Context, job, buildID string) (total int, failed []string, artifactSig string, err error) {
	if total, failed, artifactSig, ok := p.loadEvalCache(job, buildID); ok {
		return total, failed, artifactSig, nil
	}
	total, failed, artifactSig, err = p.fetchBuildJUnit(ctx, job, buildID)
	if err != nil {
		return 0, nil, "", err
	}
	p.storeEvalCache(job, buildID, total, failed, artifactSig)
	return total, slices.Clone(failed), artifactSig, nil
}

func junitEvalCacheKey(job, buildID string) string {
	return job + "\x00" + buildID
}

func (p *JUnitProber) loadEvalCache(job, buildID string) (int, []string, string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.evalCache[junitEvalCacheKey(job, buildID)]
	if !ok {
		return 0, nil, "", false
	}
	return entry.total, slices.Clone(entry.failed), entry.artifactSig, true
}

func (p *JUnitProber) storeEvalCache(job, buildID string, total int, failed []string, artifactSig string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.evalCache[junitEvalCacheKey(job, buildID)] = junitEvalCacheEntry{
		total:       total,
		failed:      slices.Clone(failed),
		artifactSig: artifactSig,
	}
}

func (p *JUnitProber) fetchBuildJUnit(ctx context.Context, job, buildID string) (total int, failed []string, artifactSig string, err error) {
	artifactURL := p.prowLogObjectURL(job, artifactObjectSegments(buildID, p.settings.ArtifactPath)...)
	logger := logrus.WithFields(logrus.Fields{
		"component":       p.componentSlug,
		"sub_component":   p.subComponentSlug,
		"job":             job,
		"build":           buildID,
		"artifact_path":   p.settings.ArtifactPath,
		"url":             artifactURL,
		"testcase_prefix": p.settings.TestcasePrefix,
	})
	xmlBody, err := p.fetchText(ctx, artifactURL)
	if err == nil {
		suites, parseErr := parseSuitesFromJunitBody(xmlBody)
		if parseErr != nil {
			logger.WithError(parseErr).WithField("outcome", "error").Debug("junit artifact evaluation")
			return 0, nil, "", fmt.Errorf("parsing junit XML for build %s: %w", buildID, parseErr)
		}
		if p.settings.TestcasePrefix != "" {
			t, f := aggregatePrefixedJunit(suites, p.settings.TestcasePrefix)
			if t == 0 {
				logger.WithFields(logrus.Fields{"outcome": "no_prefix_match", "matched": 0}).Debug("junit artifact evaluation")
				return 0, nil, junitSignatureNoPrefixMatch, nil
			}
			logger.WithFields(logrus.Fields{
				"outcome":       "found",
				"matched":       t,
				"failed":        f,
				"matched_tests": prefixedTestcaseNames(suites, p.settings.TestcasePrefix),
			}).Debug("junit artifact evaluation")
			return t, f, "", nil
		}
		t, f := aggregateJunitFromSuites(suites)
		logger.WithFields(logrus.Fields{"outcome": "found", "matched": t, "failed": f}).Debug("junit artifact evaluation")
		return t, f, "", nil
	}
	if !isHTTP404(err) {
		logger.WithError(err).WithField("outcome", "error").Debug("junit artifact evaluation")
		return 0, nil, "", err
	}
	finished, finErr := p.isBuildFinished(ctx, job, buildID)
	if finErr != nil {
		logger.WithError(finErr).WithField("outcome", "error").Debug("junit artifact evaluation")
		return 0, nil, "", fmt.Errorf("checking finished.json for build %s: %w", buildID, finErr)
	}
	if !finished {
		logger.WithField("outcome", "missing").Debug("junit artifact evaluation")
		return 0, nil, "", fmt.Errorf("build %s has no %s and is not finished", buildID, p.settings.ArtifactPath)
	}
	logger.WithField("outcome", "missing").Debug("junit artifact evaluation")
	return 0, nil, junitSignatureMissingJUnit, nil
}

func prefixedTestcaseNames(suites []junitSuite, prefix string) []string {
	seen := sets.New[string]()
	for _, s := range suites {
		for _, tc := range s.TestCases {
			if !strings.HasPrefix(tc.Name, prefix) {
				continue
			}
			name := tc.Name
			if name == "" {
				name = "(unnamed)"
			}
			seen.Insert(name)
		}
	}
	if seen.Len() == 0 {
		return nil
	}
	return sets.List(seen)
}

// parseSuitesFromJunitBody decodes a JUnit XML body into one or more suites
// (testsuite root or a single synthetic suite from a testsuite payload).
func parseSuitesFromJunitBody(xmlBody string) ([]junitSuite, error) {
	if strings.TrimSpace(xmlBody) == "" {
		return nil, fmt.Errorf("empty JUnit body")
	}
	var doc junitDoc
	if err := xml.NewDecoder(strings.NewReader(xmlBody)).Decode(&doc); err != nil {
		return nil, err
	}
	suites := doc.Suites
	if len(suites) == 0 {
		suites = []junitSuite{{Tests: doc.Tests, TestCases: doc.TestCases}}
	}
	return suites, nil
}

func artifactObjectSegments(buildID, artifactPath string) []string {
	parts := strings.Split(artifactPath, "/")
	segs := make([]string, 0, 1+len(parts))
	segs = append(segs, buildID)
	for _, part := range parts {
		if part != "" {
			segs = append(segs, part)
		}
	}
	return segs
}

func aggregatePrefixedJunit(suites []junitSuite, prefix string) (total int, failedSorted []string) {
	seen := sets.New[string]()
	for _, s := range suites {
		for _, tc := range s.TestCases {
			if !strings.HasPrefix(tc.Name, prefix) {
				continue
			}
			total++
			if tc.Failure == nil && tc.Error == nil {
				continue
			}
			name := tc.Name
			if name == "" {
				name = "(unnamed)"
			}
			seen.Insert(name)
		}
	}
	if seen.Len() == 0 {
		return total, nil
	}
	return total, sets.List(seen)
}

func aggregateJunitFromSuites(suites []junitSuite) (total int, failedSorted []string) {
	seen := sets.New[string]()
	for _, s := range suites {
		total += s.Tests
		for _, tc := range s.TestCases {
			if tc.Failure == nil && tc.Error == nil {
				continue
			}
			if tc.Name != "" {
				seen.Insert(tc.Name)
			}
		}
	}
	if seen.Len() == 0 {
		return total, nil
	}
	return total, sets.List(seen)
}

// junitUnhealthy is true for zero total tests (no coverage) or any failed testcase.
func junitUnhealthy(total int, failed []string) bool {
	return total == 0 || len(failed) > 0
}

func junitEvalUnhealthy(total int, failed []string, artifactSig string) bool {
	if artifactSig != "" {
		return true
	}
	return junitUnhealthy(total, failed)
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
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", &httpStatusError{StatusCode: resp.StatusCode, URL: url}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTextBodyBytes))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// junitDoc unmarshals Prow JUnit with either a testsuite or testsuites root element.
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
	Error   *struct{} `xml:"error"`
}

type prowStarted struct {
	Timestamp int64 `json:"timestamp"`
}

func sortRunsNewestFirst(runs []junitRunEval) {
	sort.SliceStable(runs, func(i, j int) bool {
		if !runs[i].started.Equal(runs[j].started) {
			return runs[i].started.After(runs[j].started)
		}
		if runs[i].job != runs[j].job {
			return runs[i].job < runs[j].job
		}
		return runs[i].buildID > runs[j].buildID
	})
}

// runsInLatestGroup keeps runs that started within window of the newest run.
// The newest run is ordered by started.json, then job name, then build id.
func runsInLatestGroup(runs []junitRunEval, window time.Duration) []junitRunEval {
	if len(runs) == 0 {
		return runs
	}
	sorted := append([]junitRunEval(nil), runs...)
	sortRunsNewestFirst(sorted)
	if window <= 0 {
		return sorted
	}
	newest := sorted[0].started
	kept := make([]junitRunEval, 0, len(sorted))
	for _, run := range sorted {
		if !newest.IsZero() && !run.started.IsZero() && newest.Sub(run.started) > window {
			break
		}
		kept = append(kept, run)
	}
	return kept
}
