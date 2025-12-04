package main

import (
	"context"
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
func NewProbeOrchestrator(probers []Prober, frequency time.Duration, dashboardURL string, componentMonitorName string, log *logrus.Logger) *ProbeOrchestrator {
	return &ProbeOrchestrator{
		probers:      probers,
		results:      make(chan types.ComponentMonitorReportComponentStatus),
		errChan:      make(chan error),
		frequency:    frequency,
		reportClient: NewReportClient(dashboardURL, componentMonitorName),
		log:          log,
	}
}

// Run starts the probe orchestration loop.
func (o *ProbeOrchestrator) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			o.log.Warn("Context cancelled, exiting")
			return
		}

		o.drainChannels()

		startTime := time.Now()
		o.startProbes(ctx)
		results := o.collectProbeResults(ctx)
		if err := o.reportClient.SendReport(results); err != nil {
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

func (o *ProbeOrchestrator) startProbes(ctx context.Context) {
	o.log.Infof("Probing %d components...", len(o.probers))
	for _, prober := range o.probers {
		go prober.Probe(ctx, o.results, o.errChan)
	}
}

func (o *ProbeOrchestrator) collectProbeResults(ctx context.Context) []types.ComponentMonitorReportComponentStatus {
	results := []types.ComponentMonitorReportComponentStatus{}
	timeout := time.After(o.frequency)

	for len(results) < len(o.probers) {
		select {
		case result := <-o.results:
			o.log.WithFields(logrus.Fields{
				"component":     result.ComponentSlug,
				"sub_component": result.SubComponentSlug,
				"status":        result.Status,
				"results":       result.Reason.Results,
			}).Info("Component monitor probe result received")
			results = append(results, result)
		case err := <-o.errChan:
			o.log.Errorf("Error: %v", err)
			//TODO: I think this is okay, because we always add a result to the channel for each error, but verify
		case <-ctx.Done():
			o.log.Warn("Context cancelled during probe collection, exiting")
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
			o.log.Warn("Context cancelled during sleep, exiting")
			return false
		case <-time.After(sleepDuration):
		}
	}
	return true
}
