package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"ship-status-dash/pkg/types"
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

			probers := make([]Prober, tt.numProbers)
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
					types.SeverityDown,
				)
			}

			orchestrator := NewProbeOrchestrator(
				probers,
				100*time.Millisecond,
				"http://test",
				"test-monitor",
				"",
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
				[]Prober{},
				100*time.Millisecond,
				"http://test",
				"test-monitor",
				"",
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
				[]Prober{},
				tt.frequency,
				"http://test",
				"test-monitor",
				"",
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

func TestMergeStatusesByComponent(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.ComponentMonitorReportComponentStatus
		expected []types.ComponentMonitorReportComponentStatus
	}{
		{
			name:     "empty input returns empty",
			input:    []types.ComponentMonitorReportComponentStatus{},
			expected: []types.ComponentMonitorReportComponentStatus{},
		},
		{
			name: "single status returns unchanged",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200"},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200"},
					},
				},
			},
		},
		{
			name: "multiple components remain separate",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://comp1.com"}},
				},
				{
					ComponentSlug:    "comp2",
					SubComponentSlug: "sub2",
					Status:           types.StatusDown,
					Reasons:          []types.Reason{{Type: types.CheckTypePrometheus, Check: "up"}},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://comp1.com"}},
				},
				{
					ComponentSlug:    "comp2",
					SubComponentSlug: "sub2",
					Status:           types.StatusDown,
					Reasons:          []types.Reason{{Type: types.CheckTypePrometheus, Check: "up"}},
				},
			},
		},
		{
			name: "HTTP and Prometheus probes merge - all healthy",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
					},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned successfully"},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
						{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned successfully"},
					},
				},
			},
		},
		{
			name: "HTTP and Prometheus probes merge - mixed statuses",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
					},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned unsuccessful"},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
						{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned unsuccessful"},
					},
				},
			},
		},
		{
			name: "status priority - most critical status wins",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusSuspected,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "query1", Results: "failed"},
					},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusCapacityExhausted,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "query2", Results: "failed"},
					},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDegraded,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 503 (expected 200)"},
					},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{Type: types.CheckTypeHTTP, Check: "http://example2.com", Results: "Status code 500 (expected 200)"},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "query1", Results: "failed"},
						{Type: types.CheckTypePrometheus, Check: "query2", Results: "failed"},
						{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 503 (expected 200)"},
						{Type: types.CheckTypeHTTP, Check: "http://example2.com", Results: "Status code 500 (expected 200)"},
					},
				},
			},
		},
		{
			name: "different sub-components remain separate",
			input: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub1.com"}},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub2",
					Status:           types.StatusDown,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub2.com"}},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub1.com"}},
				},
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub2",
					Status:           types.StatusDown,
					Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub2.com"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeStatusesByComponent(tt.input)
			diff := cmp.Diff(tt.expected, result)
			if diff != "" {
				t.Errorf("mergeStatusesByComponent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
