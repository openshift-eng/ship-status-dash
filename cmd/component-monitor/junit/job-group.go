package junit

import (
	"context"
	"fmt"
	"strings"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
)

// probeJobs scores each Prow job on its own. Reason.Check is the job name.
func (p *JUnitProber) probeJobs(ctx context.Context) ProbeResult {
	var reasons []types.Reason
	status := types.StatusHealthy
	var firstErr *ProbeResult
	for _, job := range p.jobNames {
		result := p.probeJob(ctx, job)
		if result.Error != nil {
			logrus.WithError(result.Error).WithFields(logrus.Fields{
				"component":     p.componentSlug,
				"sub_component": p.subComponentSlug,
				"job":           job,
			}).Warn("skipping unreadable junit job")
			if firstErr == nil {
				copied := result
				firstErr = &copied
			}
			continue
		}
		if result.Status == types.StatusHealthy {
			continue
		}
		if types.GetSeverityLevel(result.Status.ToSeverity()) > types.GetSeverityLevel(status.ToSeverity()) {
			status = result.Status
		}
		reasons = append(reasons, result.Reasons...)
	}
	if len(reasons) > 0 {
		return p.combineJobProbe(status, reasons, nil)
	}
	if firstErr != nil {
		return *firstErr
	}
	return p.healthyResult()
}

func (p *JUnitProber) probeJob(ctx context.Context, job string) ProbeResult {
	builds, stale, err := p.jobBuilds(ctx, job)
	if err != nil {
		return p.formatErrorResult(job, err)
	}
	if stale != nil {
		return *stale
	}

	if p.settings.HistoryRuns == 1 {
		ok, err := p.buildWithinMaxAge(ctx, job, builds[0])
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"component":     p.componentSlug,
				"sub_component": p.subComponentSlug,
				"job":           job,
				"build":         builds[0],
			}).Warn("junit build unreadable")
			return p.formatErrorResult(job, err)
		}
		if !ok {
			return p.healthyResult()
		}
		r, perr := p.probeJunitForBuildID(ctx, job, builds[0])
		if perr != nil {
			logrus.WithError(perr).WithFields(logrus.Fields{
				"component":     p.componentSlug,
				"sub_component": p.subComponentSlug,
				"job":           job,
				"build":         builds[0],
			}).Warn("junit build unreadable")
			return p.formatErrorResult(job, perr)
		}
		return r
	}

	var runs []junitRunEval
	var readErr error
	sawUnscored := false
	for _, b := range builds {
		run, included, perr := p.evaluateIncludedBuild(ctx, job, b)
		if perr != nil {
			logrus.WithError(perr).WithFields(logrus.Fields{"job": job, "build": b}).Warn("skipping junit build")
			if readErr == nil {
				readErr = perr
			}
			continue
		}
		switch included {
		case includeUnscored:
			sawUnscored = true
			continue
		case includeScored:
			runs = append(runs, run)
		}
	}
	if len(runs) == 0 && readErr != nil {
		return p.formatErrorResult(job, readErr)
	}
	if len(runs) == 0 && sawUnscored {
		return p.formatErrorResult(job, fmt.Errorf("no junit runs scored for %s", p.settings.ArtifactPath))
	}
	if p.settings.LatestRunGroupWindow > 0 {
		runs = runsInLatestGroup(runs, p.settings.LatestRunGroupWindow)
	}
	reason, unhealthy := p.failureSignatureReason(job, runs)
	if !unhealthy {
		return p.healthyResult()
	}
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           p.severity.ToStatus(),
			Reasons:          []types.Reason{reason},
		},
	}
}

func (p *JUnitProber) failureSignatureReason(check string, runs []junitRunEval) (types.Reason, bool) {
	threshold := p.settings.FailedRunsThreshold
	if threshold < 1 {
		threshold = 1
	}
	signatureCount := make(map[string]int)
	summaries := make([]string, 0, len(runs))
	for _, run := range runs {
		label := fmt.Sprintf("build %s", run.buildID)
		if run.job != check {
			label = fmt.Sprintf("%s/%s", run.job, run.buildID)
		}
		summaries = append(summaries, fmt.Sprintf("%s: %s; %s", label, formatJunitBuildSummary(run.total, run.failed, run.artifactSig, p.settings.ArtifactPath), p.spyglassViewURL(run.job, run.buildID)))
		if !junitEvalUnhealthy(run.total, run.failed, run.artifactSig) {
			continue
		}
		signatureCount[junitEvalSignature(run.total, run.failed, run.artifactSig)]++
	}
	highestSig, highestCount := "", 0
	for sig, n := range signatureCount {
		if n > highestCount {
			highestCount, highestSig = n, sig
		} else if n == highestCount && n > 0 && (highestSig == "" || sig < highestSig) {
			highestSig = sig
		}
	}
	if highestCount < threshold {
		return types.Reason{}, false
	}
	return types.Reason{
		Type:  types.CheckTypeJUnit,
		Check: check,
		Results: fmt.Sprintf(
			"junit: %d of the last %d run(s) share the same failure pattern: %s (threshold %d) — %s",
			highestCount, len(runs), formatJunitSignatureShort(highestSig, p.settings.ArtifactPath), threshold, strings.Join(summaries, " | "),
		),
	}, true
}

func (p *JUnitProber) probeJunitForBuildID(ctx context.Context, job, buildID string) (ProbeResult, error) {
	total, failed, artifactSig, err := p.evaluateBuildJUnit(ctx, job, buildID)
	if err != nil {
		return ProbeResult{}, err
	}
	if artifactSig != "" {
		if p.unscoredArtifact(artifactSig) {
			return ProbeResult{}, fmt.Errorf("no junit runs scored for %s on build %s", p.settings.ArtifactPath, buildID)
		}
		if artifactSig == junitSignatureMissingJUnit {
			return p.makeMissingJUnitResult(job, buildID), nil
		}
	}
	return p.makeStatusFromAggregates(job, buildID, total, failed), nil
}

func (p *JUnitProber) makeMissingJUnitResult(job, buildID string) ProbeResult {
	return ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    p.componentSlug,
			SubComponentSlug: p.subComponentSlug,
			Status:           p.severity.ToStatus(),
			Reasons: []types.Reason{{
				Type:    types.CheckTypeJUnit,
				Check:   job,
				Results: fmt.Sprintf("build %s: missing %s; %s", buildID, p.settings.ArtifactPath, p.spyglassViewURL(job, buildID)),
			}},
		},
	}
}

// junitFailureSignature is a key for the same failure "reason" across runs. Only call when junitUnhealthy.
// totalJUnitCases is the sum of suite tests attributes from JUnit XML (same as aggregateJunitFromSuites).
func junitFailureSignature(totalJUnitCases int, failed []string) string {
	if totalJUnitCases == 0 {
		return junitSignatureZero
	}
	// Unhealthy, totalJUnitCases>0, non-empty failed (sorted unique) per aggregateJunitFromSuites.
	if len(failed) == 0 {
		return ""
	}
	return junitSignatureFailedPfx + strings.Join(failed, ",")
}

func junitEvalSignature(total int, failed []string, artifactSig string) string {
	if artifactSig != "" {
		return artifactSig
	}
	return junitFailureSignature(total, failed)
}

func formatJunitSignatureShort(sig, artifactPath string) string {
	switch sig {
	case junitSignatureZero:
		return "zero tests (no JUnit test cases, or total tests=0)"
	case junitSignatureMissingJUnit:
		return fmt.Sprintf("missing %s", artifactPath)
	}
	if !strings.HasPrefix(sig, junitSignatureFailedPfx) {
		return sig
	}
	names := strings.Split(strings.TrimPrefix(sig, junitSignatureFailedPfx), ",")
	if len(names) == 0 {
		return "failed: (unknown cases)"
	}
	return "failed: " + strings.Join(names, ", ")
}

func formatJunitBuildSummary(total int, failed []string, artifactSig, artifactPath string) string {
	if artifactSig == junitSignatureMissingJUnit {
		return fmt.Sprintf("missing %s", artifactPath)
	}
	switch {
	case total == 0:
		return "zero tests found"
	case len(failed) == 0:
		return fmt.Sprintf("all %d tests passed", total)
	default:
		return fmt.Sprintf("%d/%d tests failed: %s", len(failed), total, strings.Join(failed, ", "))
	}
}

func (p *JUnitProber) makeStatusFromAggregates(job, buildID string, total int, failed []string) ProbeResult {
	var status types.Status
	var reasons []types.Reason
	spyglass := p.spyglassViewURL(job, buildID)
	switch {
	case total == 0:
		status = p.severity.ToStatus()
		reasons = []types.Reason{{
			Type:    types.CheckTypeJUnit,
			Check:   job,
			Results: fmt.Sprintf("build %s: zero tests found; %s", buildID, spyglass),
		}}
	case len(failed) == 0:
		status = types.StatusHealthy
		// Omit Reasons for Healthy — mergeStatuses strips them in the report anyway.
		reasons = nil
	default:
		status = p.severity.ToStatus()
		reasons = []types.Reason{{
			Type:    types.CheckTypeJUnit,
			Check:   job,
			Results: fmt.Sprintf("build %s: %d/%d tests failed: %s; %s", buildID, len(failed), total, strings.Join(failed, ", "), spyglass),
		}}
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
