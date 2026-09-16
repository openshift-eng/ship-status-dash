package main

import (
	"context"
	"sort"
	"sync"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
)

// Prober is an interface for component probes.
type Prober interface {
	Probe(ctx context.Context, results chan<- ProbeResult)
}

type reportSender interface {
	SendReport(results []types.ComponentMonitorReportComponentStatus) error
	PrintReport(results []types.ComponentMonitorReportComponentStatus) error
}

const (
	ProbeTypeHTTP       = "http"
	ProbeTypeJUnit      = "junit"
	ProbeTypePrometheus = "prometheus"
	ProbeTypeSystemd    = "systemd"
	ProbeTypeJira       = "jira"
)

type ProbeResult struct {
	types.ComponentMonitorReportComponentStatus
	ProbeType string
	Error     error
}

type scheduledProber struct {
	prober    Prober
	frequency time.Duration

	mu     sync.Mutex
	lastOK time.Time
	hasOK  bool
}

func (s *scheduledProber) due(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.hasOK || now.Sub(s.lastOK) >= s.frequency
}

func (s *scheduledProber) recordSuccess(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasOK = true
	s.lastOK = at
}

// ProbeOrchestrator manages the execution of component probes.
type ProbeOrchestrator struct {
	schedule     []scheduledProber
	results      chan ProbeResult
	frequency    time.Duration
	reportClient reportSender
	log          *logrus.Logger
}

// NewProbeOrchestrator creates a new ProbeOrchestrator.
func NewProbeOrchestrator(schedule []scheduledProber, frequency time.Duration, dashboardURL string, componentMonitorName string, authToken string, log *logrus.Logger) *ProbeOrchestrator {
	return &ProbeOrchestrator{
		schedule:     schedule,
		results:      make(chan ProbeResult),
		frequency:    frequency,
		reportClient: NewReportClient(dashboardURL, componentMonitorName, authToken),
		log:          log,
	}
}

func scheduleAtFrequency(probers []Prober, frequency time.Duration) []scheduledProber {
	schedule := make([]scheduledProber, len(probers))
	for i, p := range probers {
		schedule[i] = scheduledProber{prober: p, frequency: frequency}
	}
	return schedule
}

// Run starts the probe orchestration loop.
func (o *ProbeOrchestrator) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			o.log.Warn("Context canceled, exiting")
			return
		}

		startTime := time.Now()
		o.runOnce(ctx)
		elapsed := time.Since(startTime)
		o.log.Infof("Probing completed in %s", elapsed)
		if !o.waitForNextCycle(ctx, elapsed) {
			return
		}
	}
}

func (o *ProbeOrchestrator) runOnce(ctx context.Context) {
	o.drainChannels()

	due := o.dueProbers(time.Now())
	if len(due) == 0 {
		o.log.Info("No probes due this cycle")
		return
	}

	o.startProbes(ctx, due)
	results := o.collectProbeResults(ctx, len(due))
	mergedResults := mergeStatuses(results)
	if err := o.reportClient.SendReport(mergedResults); err != nil {
		o.log.Errorf("Error sending report: %v", err)
	} else {
		o.log.Infof("Report sent successfully")
	}
}

// DryRun runs probes once and outputs the report as JSON to stdout.
func (o *ProbeOrchestrator) DryRun(ctx context.Context) {
	due := make([]*scheduledProber, len(o.schedule))
	for i := range o.schedule {
		due[i] = &o.schedule[i]
	}
	o.startProbes(ctx, due)
	results := o.collectProbeResults(ctx, len(due))
	mergedResults := mergeStatuses(results)
	if err := o.reportClient.PrintReport(mergedResults); err != nil {
		o.log.Errorf("Error outputting report: %v", err)
	}
}

func (o *ProbeOrchestrator) dueProbers(now time.Time) []*scheduledProber {
	var due []*scheduledProber
	for i := range o.schedule {
		s := &o.schedule[i]
		if s.due(now) {
			due = append(due, s)
		}
	}
	return due
}

func (o *ProbeOrchestrator) startProbes(ctx context.Context, due []*scheduledProber) {
	o.log.Infof("Probing %d of %d components...", len(due), len(o.schedule))
	for _, s := range due {
		s := s
		go func() {
			ch := make(chan ProbeResult, 1)
			s.prober.Probe(ctx, ch)
			r := <-ch
			if r.Error == nil {
				s.recordSuccess(time.Now())
			}
			o.results <- r
		}()
	}
}

func (o *ProbeOrchestrator) collectProbeResults(ctx context.Context, expected int) []ProbeResult {
	probesCompleted := 0
	results := []ProbeResult{}
	if expected == 0 {
		return results
	}
	timeout := time.After(o.frequency)

	for probesCompleted < expected {
		select {
		case probeResult := <-o.results:
			resultLog := o.log.WithFields(logrus.Fields{
				"component":     probeResult.ComponentSlug,
				"sub_component": probeResult.SubComponentSlug,
				"status":        probeResult.Status,
				"probe_type":    probeResult.ProbeType,
			})
			if probeResult.Error != nil {
				resultLog.Errorf("Error: %v", probeResult.Error)
			} else {
				resultLog.Info("Component monitor probe result received")
			}
			results = append(results, probeResult)
			probesCompleted++
		case <-ctx.Done():
			o.log.Warn("Context canceled during probe collection, exiting")
			return results
		case <-timeout:
			o.log.Warnf("Timeout waiting for probe results after %s, restarting probe cycle", o.frequency)
			return results
		}
	}

	return results
}

func (o *ProbeOrchestrator) drainChannels() {
	o.log.Infof("Draining channels before next cycle...")
	for {
		select {
		case probeResult := <-o.results:
			if probeResult.Error != nil {
				o.log.Warnf("Discarding old error for component %s sub-component %s: %v", probeResult.ComponentSlug, probeResult.SubComponentSlug, probeResult.Error)
			} else {
				o.log.Warnf("Discarding old result for component %s sub-component %s", probeResult.ComponentSlug, probeResult.SubComponentSlug)
			}
		default:
			o.log.Infof("Channels drained")
			return
		}
	}
}

func (o *ProbeOrchestrator) waitForNextCycle(ctx context.Context, elapsed time.Duration) bool {
	if elapsed < o.frequency {
		sleepDuration := o.frequency - elapsed
		o.log.Infof("Will probe again in %s", sleepDuration)
		select {
		case <-ctx.Done():
			o.log.Warn("Context canceled during sleep, exiting")
			return false
		case <-time.After(sleepDuration):
		}
	}
	return true
}

// mergeStatuses merges multiple status reports for the same component/sub-component
// into a single unified status. It groups by (ComponentSlug, SubComponentSlug), combines all
// reasons from unhealthy probes, and determines the most critical status when multiple probes report different statuses.
// If there are any errored statuses and all non-errored statuses are Healthy, the component/sub-component
// is omitted from the report.
func mergeStatuses(probeResults []ProbeResult) []types.ComponentMonitorReportComponentStatus {
	if len(probeResults) == 0 {
		return []types.ComponentMonitorReportComponentStatus{}
	}

	type componentKey struct {
		component    string
		subComponent string
	}

	grouped := make(map[componentKey][]ProbeResult)
	for _, probeResult := range probeResults {
		key := componentKey{
			component:    probeResult.ComponentSlug,
			subComponent: probeResult.SubComponentSlug,
		}
		grouped[key] = append(grouped[key], probeResult)
	}

	merged := make([]types.ComponentMonitorReportComponentStatus, 0, len(grouped))
	for key, group := range grouped {
		if result := mergeStatusesForSubComponent(key.component, key.subComponent, group); result != nil {
			merged = append(merged, *result)
		}
	}

	// Sort results for deterministic output
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].ComponentSlug != merged[j].ComponentSlug {
			return merged[i].ComponentSlug < merged[j].ComponentSlug
		}
		return merged[i].SubComponentSlug < merged[j].SubComponentSlug
	})

	return merged
}

// mergeStatusesForSubComponent merges probe results for a single component/sub-component.
// Returns nil if the component/sub-component should be omitted from the report.
func mergeStatusesForSubComponent(componentSlug, subComponentSlug string, group []ProbeResult) *types.ComponentMonitorReportComponentStatus {
	hasError := false
	var nonErroredStatuses []types.ComponentMonitorReportComponentStatus

	for _, probeResult := range group {
		if probeResult.Error != nil {
			hasError = true
		} else {
			nonErroredStatuses = append(nonErroredStatuses, probeResult.ComponentMonitorReportComponentStatus)
		}
	}

	if hasError {
		allHealthy := true
		for _, status := range nonErroredStatuses {
			if status.Status != types.StatusHealthy {
				allHealthy = false
				break
			}
		}
		// If we have an error in a probe, and the sub-component would otherwise be healthy, we omit the status from the report so that the absent-report-checker can create an outage if it continues to error.
		// This allows an admin to look into the error with the probe.
		if allHealthy {
			return nil
		}
	}

	if len(nonErroredStatuses) == 0 {
		return nil
	}

	var allFailedReasons []types.Reason
	mostCriticalStatus := types.StatusHealthy

	for _, status := range nonErroredStatuses {
		if status.Status != types.StatusHealthy {
			allFailedReasons = append(allFailedReasons, status.Reasons...)
		}
		currentLevel := types.GetSeverityLevel(status.Status.ToSeverity())
		mostCriticalLevel := types.GetSeverityLevel(mostCriticalStatus.ToSeverity())
		if currentLevel > mostCriticalLevel {
			mostCriticalStatus = status.Status
		}
	}

	result := &types.ComponentMonitorReportComponentStatus{
		ComponentSlug:    componentSlug,
		SubComponentSlug: subComponentSlug,
		Status:           mostCriticalStatus,
		Reasons:          allFailedReasons,
	}
	if mostCriticalStatus == types.StatusHealthy {
		result.Reasons = nil
	}
	return result
}
