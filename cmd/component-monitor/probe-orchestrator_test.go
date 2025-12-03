package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"ship-status-dash/pkg/types"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestProbeOrchestrator_collectProbeResults(t *testing.T) {
	tests := []struct {
		name              string
		numProbers        int
		probeResults      []types.ComponentMonitorReportComponentStatus
		probeErrors       []error
		cancelContext     bool
		timeout           bool
		expectedResultLen int
	}{
		{
			name:       "collect all results successfully",
			numProbers: 2,
			probeResults: []types.ComponentMonitorReportComponentStatus{
				{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy},
				{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown},
			},
			expectedResultLen: 2,
		},
		{
			name:       "collect results with errors",
			numProbers: 2,
			probeResults: []types.ComponentMonitorReportComponentStatus{
				{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy},
				{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown},
			},
			probeErrors:       []error{assert.AnError},
			expectedResultLen: 2,
		},
		{
			name:              "context cancellation during collection",
			numProbers:        3,
			cancelContext:     true,
			expectedResultLen: 0,
		},
		{
			name:              "timeout waiting for results",
			numProbers:        2,
			timeout:           true,
			expectedResultLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			probers := make([]*HTTPProber, tt.numProbers)
			for i := 0; i < tt.numProbers; i++ {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()

				probers[i] = NewHTTPProber(
					"comp",
					"sub",
					server.URL,
					http.StatusOK,
					10*time.Millisecond,
				)
			}

			orchestrator := NewProbeOrchestrator(
				probers,
				100*time.Millisecond,
				"http://test",
				"test-monitor",
				log,
			)

			ctx := context.Background()
			if tt.cancelContext {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			}

			if tt.timeout {
				orchestrator.frequency = 10 * time.Millisecond
			}

			go func() {
				time.Sleep(20 * time.Millisecond)
				for _, result := range tt.probeResults {
					orchestrator.results <- result
				}
				for _, err := range tt.probeErrors {
					orchestrator.errChan <- err
				}
			}()

			results := orchestrator.collectProbeResults(ctx)

			assert.Equal(t, tt.expectedResultLen, len(results))
		})
	}
}

func TestProbeOrchestrator_drainChannels(t *testing.T) {
	tests := []struct {
		name        string
		results     []types.ComponentMonitorReportComponentStatus
		errors      []error
		expectDrain bool
	}{
		{
			name: "drain old results",
			results: []types.ComponentMonitorReportComponentStatus{
				{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy},
				{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown},
			},
			expectDrain: true,
		},
		{
			name:        "drain old errors",
			errors:      []error{assert.AnError},
			expectDrain: true,
		},
		{
			name:        "drain mixed results and errors",
			results:     []types.ComponentMonitorReportComponentStatus{{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy}},
			errors:      []error{assert.AnError},
			expectDrain: true,
		},
		{
			name:        "no items to drain",
			expectDrain: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			orchestrator := NewProbeOrchestrator(
				[]*HTTPProber{},
				100*time.Millisecond,
				"http://test",
				"test-monitor",
				log,
			)

			go func() {
				for _, result := range tt.results {
					orchestrator.results <- result
				}
				for _, err := range tt.errors {
					orchestrator.errChan <- err
				}
			}()

			time.Sleep(10 * time.Millisecond)
			orchestrator.drainChannels()

			select {
			case <-orchestrator.results:
				t.Error("results channel should be empty after draining")
			case <-orchestrator.errChan:
				t.Error("error channel should be empty after draining")
			default:
			}
		})
	}
}

func TestProbeOrchestrator_waitForNextCycle(t *testing.T) {
	tests := []struct {
		name           string
		frequency      time.Duration
		elapsed        time.Duration
		cancelContext  bool
		expectContinue bool
	}{
		{
			name:           "wait for next cycle when elapsed < frequency",
			frequency:      100 * time.Millisecond,
			elapsed:        50 * time.Millisecond,
			expectContinue: true,
		},
		{
			name:           "immediate next cycle when elapsed >= frequency",
			frequency:      50 * time.Millisecond,
			elapsed:        100 * time.Millisecond,
			expectContinue: true,
		},
		{
			name:           "context cancellation during wait",
			frequency:      100 * time.Millisecond,
			elapsed:        50 * time.Millisecond,
			cancelContext:  true,
			expectContinue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			orchestrator := NewProbeOrchestrator(
				[]*HTTPProber{},
				tt.frequency,
				"http://test",
				"test-monitor",
				log,
			)

			ctx := context.Background()
			if tt.cancelContext {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				go func() {
					time.Sleep(10 * time.Millisecond)
					cancel()
				}()
			}

			result := orchestrator.waitForNextCycle(ctx, tt.elapsed)
			assert.Equal(t, tt.expectContinue, result)
		})
	}
}
