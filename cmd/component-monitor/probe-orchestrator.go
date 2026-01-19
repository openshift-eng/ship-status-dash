package main

import (
	"context"
	"sort"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
)

// Prober is an interface for component probes.
type Prober interface {
	Probe(ctx context.Context, results chan<- types.ComponentMonitorReportComponentStatus, errChan chan<- error)
}

// ProbeOrchestrator manages the execution of component probes.
type ProbeOrchestrator struct {
	probers      []Prober
	results      chan types.ComponentMonitorReportComponentStatus
	errChan      chan error
	frequency    time.Duration
	reportClient *ReportClient
	log          *logrus.Logger
}

// NewProbeOrchestrator creates a new ProbeOrchestrator.
func NewProbeOrchestrator(probers []Prober, frequency time.Duration, dashboardURL string, componentMonitorName string, authToken string, log *logrus.Logger) *ProbeOrchestrator {
	return &ProbeOrchestrator{
		probers:      probers,
		results:      make(chan types.ComponentMonitorReportComponentStatus),
		errChan:      make(chan error),
		frequency:    frequency,
		reportClient: NewReportClient(dashboardURL, componentMonitorName, authToken),
		log:          log,
	}
}

// Run starts the probe orchestration loop.
func (o *ProbeOrchestrator) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			o.log.Warn("Context canceled, exiting")
			return
		}

		o.drainChannels()

		startTime := time.Now()
		o.startProbes(ctx)
		results := o.collectProbeResults(ctx)
		mergedResults := mergeStatusesByComponent(results)
		if err := o.reportClient.SendReport(mergedResults); err != nil {
			o.log.Errorf("Error sending report: %v", err)
		} else {
			o.log.Infof("Report sent successfully")
		}
		elapsed := time.Since(startTime)
		o.log.Infof("Probing completed in %s", elapsed)
		if !o.waitForNextCycle(ctx, elapsed) {
			return
		}
	}
}

// DryRun runs probes once and outputs the report as JSON to stdout.
func (o *ProbeOrchestrator) DryRun(ctx context.Context) {
	o.startProbes(ctx)
	results := o.collectProbeResults(ctx)
	mergedResults := mergeStatusesByComponent(results)
	if err := o.reportClient.PrintReport(mergedResults); err != nil {
		o.log.Errorf("Error outputting report: %v", err)
	}
}

func (o *ProbeOrchestrator) startProbes(ctx context.Context) {
	o.log.Infof("Probing %d components...", len(o.probers))
	for _, prober := range o.probers {
		go prober.Probe(ctx, o.results, o.errChan)
	}
}

func (o *ProbeOrchestrator) collectProbeResults(ctx context.Context) []types.ComponentMonitorReportComponentStatus {
	probesCompleted := 0
	results := []types.ComponentMonitorReportComponentStatus{}
	timeout := time.After(o.frequency)

	for probesCompleted < len(o.probers) {
		select {
		case result := <-o.results:
			o.log.WithFields(logrus.Fields{
				"component":     result.ComponentSlug,
				"sub_component": result.SubComponentSlug,
				"status":        result.Status,
			}).Info("Component monitor probe result received")
			results = append(results, result)
			probesCompleted++
		case err := <-o.errChan:
			o.log.Errorf("Error: %v", err)
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
		case result := <-o.results:
			o.log.Warnf("Discarding old result for component %s sub-component %s", result.ComponentSlug, result.SubComponentSlug)
		case err := <-o.errChan:
			o.log.Warnf("Discarding old error: %v", err)
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

// mergeStatusesByComponent merges multiple status reports for the same component/sub-component
// into a single unified status. It groups by (ComponentSlug, SubComponentSlug), combines all
// reasons, and determines the most critical status when multiple probes report different statuses.
func mergeStatusesByComponent(statuses []types.ComponentMonitorReportComponentStatus) []types.ComponentMonitorReportComponentStatus {
	if len(statuses) == 0 {
		return statuses
	}

	type componentKey struct {
		component    string
		subComponent string
	}

	grouped := make(map[componentKey][]types.ComponentMonitorReportComponentStatus)
	for _, status := range statuses {
		key := componentKey{
			component:    status.ComponentSlug,
			subComponent: status.SubComponentSlug,
		}
		grouped[key] = append(grouped[key], status)
	}

	merged := make([]types.ComponentMonitorReportComponentStatus, 0, len(grouped))
	for key, group := range grouped {
		if len(group) == 1 {
			merged = append(merged, group[0])
			continue
		}

		var allReasons []types.Reason
		var mostCriticalStatus types.Status

		for i, status := range group {
			allReasons = append(allReasons, status.Reasons...)
			if i == 0 {
				mostCriticalStatus = status.Status
			} else {
				currentLevel := types.GetSeverityLevel(status.Status.ToSeverity())
				mostCriticalLevel := types.GetSeverityLevel(mostCriticalStatus.ToSeverity())
				if currentLevel > mostCriticalLevel {
					mostCriticalStatus = status.Status
				}
			}
		}

		merged = append(merged, types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    key.component,
			SubComponentSlug: key.subComponent,
			Status:           mostCriticalStatus,
			Reasons:          allReasons,
		})
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
