package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
)

const (
	maxBuildLogScanBytes  = 8 * 1024 * 1024
	maxBuildLogTokenBytes = 1024 * 1024
)

// leaseLinePattern matches ci-tools leaseStep.acquireLeases. The line is not anchored:
// live logs wrap it in ANSI color and a logrus prefix.
var leaseLinePattern = regexp.MustCompile(`Acquired \d+ lease\(s\) for [^:]+: \[([^\]]*)\]`)

type leaseCacheEntry struct {
	account string
	ok      bool
}

func (p *JUnitProber) probeByGroup(ctx context.Context) ProbeResult {
	grouped := map[string][]junitRunEval{}
	var staleReasons []types.Reason
	var readErr error
	for _, job := range p.jobNames {
		builds, stale, err := p.jobBuilds(ctx, job)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"component":     p.componentSlug,
				"sub_component": p.subComponentSlug,
				"job":           job,
			}).Warn("skipping unreadable junit job")
			if readErr == nil {
				readErr = err
			}
			continue
		}
		if stale != nil {
			staleReasons = append(staleReasons, stale.Reasons...)
			continue
		}
		scored := 0
		sawUnscored := false
		var jobReadErr error
		for _, buildID := range builds {
			run, included, perr := p.evaluateIncludedBuild(ctx, job, buildID)
			if perr != nil {
				logrus.WithError(perr).WithFields(logrus.Fields{"job": job, "build": buildID}).Warn("skipping junit build")
				if jobReadErr == nil {
					jobReadErr = perr
				}
				continue
			}
			switch included {
			case includeUnscored:
				sawUnscored = true
				continue
			case includeDroppedAge:
				continue
			}
			key, keyOK, keyErr := p.groupKey(ctx, job, buildID)
			if keyErr != nil {
				logrus.WithError(keyErr).WithFields(logrus.Fields{"job": job, "build": buildID}).Warn("skipping junit build")
				if jobReadErr == nil {
					jobReadErr = keyErr
				}
				continue
			}
			if !keyOK {
				continue
			}
			scored++
			grouped[key] = append(grouped[key], run)
		}
		if scored == 0 && jobReadErr != nil && readErr == nil {
			readErr = jobReadErr
		}
		if scored == 0 && sawUnscored && readErr == nil {
			readErr = fmt.Errorf("no junit runs scored for %s on job %s", p.settings.ArtifactPath, job)
		}
	}

	var reasons []types.Reason
	reasons = append(reasons, staleReasons...)
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		reason, unhealthy := p.consecutiveStreakReason(key, grouped[key])
		if unhealthy {
			reasons = append(reasons, reason)
		}
	}
	return p.combineJobProbe(p.severity.ToStatus(), reasons, readErr)
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

func shortDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

func (p *JUnitProber) consecutiveStreakReason(check string, runs []junitRunEval) (types.Reason, bool) {
	threshold := p.settings.FailedRunsThreshold
	if threshold < 1 {
		threshold = 1
	}
	if p.settings.LatestRunGroupWindow > 0 {
		return p.latestGroupFailureReason(check, runs, threshold)
	}
	sortRunsNewestFirst(runs)
	var streak []junitRunEval
	for _, run := range runs {
		if !junitEvalUnhealthy(run.total, run.failed, run.artifactSig) {
			break
		}
		streak = append(streak, run)
	}
	if len(streak) < threshold {
		return types.Reason{}, false
	}
	return p.leaseFailureReason(check, streak, fmt.Sprintf("cloud-account: %s: %d consecutive failure(s) (threshold %d): %%s", check, len(streak), threshold)), true
}

func (p *JUnitProber) latestGroupFailureReason(check string, runs []junitRunEval, threshold int) (types.Reason, bool) {
	cohort := runsInLatestGroup(runs, p.settings.LatestRunGroupWindow)
	var failed []junitRunEval
	for _, run := range cohort {
		if junitEvalUnhealthy(run.total, run.failed, run.artifactSig) {
			failed = append(failed, run)
		}
	}
	if len(failed) < threshold {
		return types.Reason{}, false
	}
	return p.leaseFailureReason(check, failed, fmt.Sprintf(
		"cloud-account: %s: %d failure(s) in the latest %s (threshold %d): %%s",
		check, len(failed), shortDuration(p.settings.LatestRunGroupWindow), threshold,
	)), true
}

func (p *JUnitProber) leaseFailureReason(check string, runs []junitRunEval, format string) types.Reason {
	parts := make([]string, 0, len(runs))
	for _, run := range runs {
		parts = append(parts, fmt.Sprintf("%s/%s (%s) %s", run.job, run.buildID, strings.Join(run.failed, ", "), p.spyglassViewURL(run.job, run.buildID)))
	}
	return types.Reason{
		Type:    types.CheckTypeJUnit,
		Check:   check,
		Results: fmt.Sprintf(format, strings.Join(parts, "; ")),
	}
}

func (p *JUnitProber) groupKey(ctx context.Context, job, buildID string) (string, bool, error) {
	switch p.settings.GroupBy {
	case types.JUnitGroupByJob:
		return job, true, nil
	case types.JUnitGroupByLeaseResourcePrefix:
		return p.leaseResourcePrefix(ctx, job, buildID)
	default:
		return "", false, fmt.Errorf("invalid group_by %q", p.settings.GroupBy)
	}
}

func (p *JUnitProber) leaseResourcePrefix(ctx context.Context, job, buildID string) (string, bool, error) {
	if account, ok, hit := p.loadLeaseCache(job, buildID); hit {
		return account, ok, nil
	}
	account, matched, err := p.readLeaseAccount(ctx, job, buildID)
	if err != nil {
		if isHTTP404(err) {
			p.storeLeaseCache(job, buildID, "", false)
			return "", false, nil
		}
		return "", false, err
	}
	if !matched || account == "" {
		p.storeLeaseCache(job, buildID, "", false)
		return "", false, nil
	}
	p.storeLeaseCache(job, buildID, account, true)
	return account, true, nil
}

func (p *JUnitProber) loadLeaseCache(job, buildID string) (string, bool, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.leaseCache[junitEvalCacheKey(job, buildID)]
	if !ok {
		return "", false, false
	}
	return entry.account, entry.ok, true
}

func (p *JUnitProber) storeLeaseCache(job, buildID, account string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.leaseCache[junitEvalCacheKey(job, buildID)] = leaseCacheEntry{account: account, ok: ok}
}

// readLeaseAccount streams build-log.txt until the first lease line or 8 MiB.
// Prow stores the object with gzip content-encoding, so this does not send a Range header.
func (p *JUnitProber) readLeaseAccount(ctx context.Context, job, buildID string) (string, bool, error) {
	u := p.prowLogObjectURL(job, buildID, prowObjectBuildLog)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", false, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", false, &httpStatusError{StatusCode: resp.StatusCode, URL: u}
	}
	reader, closeReader, err := decompressedLogReader(resp.Body)
	if err != nil {
		return "", false, err
	}
	defer closeReader()

	limited := io.LimitReader(reader, maxBuildLogScanBytes)
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 64*1024), maxBuildLogTokenBytes)
	for scanner.Scan() {
		account, matched := accountFromLeaseLine(scanner.Text())
		if matched {
			return account, true, nil
		}
	}
	if err := scanner.Err(); err != nil && err != bufio.ErrTooLong {
		return "", false, err
	}
	return "", false, nil
}

func accountFromResourceName(name string) (string, bool) {
	parts := strings.Split(name, "--")
	if len(parts) < 3 || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

func accountFromLeaseLine(line string) (string, bool) {
	match := leaseLinePattern.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	for _, name := range strings.Fields(match[1]) {
		if account, ok := accountFromResourceName(name); ok {
			return account, true
		}
	}
	return "", true
}

func decompressedLogReader(body io.Reader) (io.Reader, func(), error) {
	buffered := bufio.NewReader(body)
	magic, err := buffered.Peek(2)
	if err != nil && err != io.EOF {
		return nil, func() {}, err
	}
	if len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, func() {}, err
		}
		return gz, func() { _ = gz.Close() }, nil
	}
	return buffered, func() {}, nil
}
