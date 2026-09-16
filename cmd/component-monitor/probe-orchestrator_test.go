package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"ship-status-dash/pkg/testhelper"
	"ship-status-dash/pkg/types"
)

func TestProbeOrchestrator_collectProbeResults(t *testing.T) {
	tests := []struct {
		name          string
		probeResults  []ProbeResult
		cancelContext bool
		timeout       bool
	}{
		{
			name: "collect all results successfully",
			probeResults: []ProbeResult{
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy}},
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown}},
			},
		},
		{
			name: "collect results with errors",
			probeResults: []ProbeResult{
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy}},
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown}},
				{Error: errors.New("probe error"), ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp1", SubComponentSlug: "sub1"}},
			},
		},
		{
			name:          "context cancellation during collection",
			cancelContext: true,
			probeResults:  []ProbeResult{},
		},
		{
			name:         "timeout waiting for results",
			timeout:      true,
			probeResults: []ProbeResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)

			probers := make([]Prober, len(tt.probeResults))
			for i := 0; i < len(tt.probeResults); i++ {
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
				scheduleAtFrequency(probers, 100*time.Millisecond),
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

			expected := len(tt.probeResults)
			if tt.timeout {
				orchestrator.frequency = 10 * time.Millisecond
				expected = 1
			}
			if tt.cancelContext {
				expected = 1
			}

			go func() {
				time.Sleep(20 * time.Millisecond)
				for _, result := range tt.probeResults {
					orchestrator.results <- result
				}
			}()

			results := orchestrator.collectProbeResults(ctx, expected)
			if diff := cmp.Diff(tt.probeResults, results, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("collectProbeResults() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProbeOrchestrator_drainChannels(t *testing.T) {
	tests := []struct {
		name         string
		probeResults []ProbeResult
		expectDrain  bool
	}{
		{
			name: "drain old results",
			probeResults: []ProbeResult{
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy}},
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp2", SubComponentSlug: "sub2", Status: types.StatusDown}},
			},
			expectDrain: true,
		},
		{
			name: "drain old errors",
			probeResults: []ProbeResult{
				{Error: assert.AnError},
			},
			expectDrain: true,
		},
		{
			name: "drain mixed results and errors",
			probeResults: []ProbeResult{
				{ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{ComponentSlug: "comp1", SubComponentSlug: "sub1", Status: types.StatusHealthy}},
				{Error: assert.AnError},
			},
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
				scheduleAtFrequency([]Prober{}, 100*time.Millisecond),
				100*time.Millisecond,
				"http://test",
				"test-monitor",
				"",
				log,
			)

			go func() {
				for _, result := range tt.probeResults {
					orchestrator.results <- result
				}
			}()

			time.Sleep(10 * time.Millisecond)
			orchestrator.drainChannels()

			select {
			case <-orchestrator.results:
				t.Error("results channel should be empty after draining")
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
				scheduleAtFrequency([]Prober{}, tt.frequency),
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
		input    []ProbeResult
		expected []types.ComponentMonitorReportComponentStatus
	}{
		{
			name:     "empty input returns empty",
			input:    []ProbeResult{},
			expected: []types.ComponentMonitorReportComponentStatus{},
		},
		{
			name: "single status returns unchanged",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200"},
						},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          nil,
				},
			},
		},
		{
			name: "multiple components remain separate",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://comp1.com"}},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp2",
						SubComponentSlug: "sub2",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypePrometheus, Check: "up"}},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          nil,
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
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
						},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned successfully"},
						},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          nil,
				},
			},
		},
		{
			name: "HTTP and Prometheus probes merge - mixed statuses",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{
							{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 200 (expected 200)"},
						},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned unsuccessful"},
						},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDown,
					Reasons: []types.Reason{
						{Type: types.CheckTypePrometheus, Check: "up", Results: "query returned unsuccessful"},
					},
				},
			},
		},
		{
			name: "status priority - most critical status wins",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusSuspected,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "query1", Results: "failed"},
						},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusCapacityExhausted,
						Reasons: []types.Reason{
							{Type: types.CheckTypePrometheus, Check: "query2", Results: "failed"},
						},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusDegraded,
						Reasons: []types.Reason{
							{Type: types.CheckTypeHTTP, Check: "http://example.com", Results: "Status code 503 (expected 200)"},
						},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusDown,
						Reasons: []types.Reason{
							{Type: types.CheckTypeHTTP, Check: "http://example2.com", Results: "Status code 500 (expected 200)"},
						},
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
			name: "junit unhealthy and prometheus healthy merge to unhealthy report",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusDegraded,
						Reasons: []types.Reason{{
							Type:    types.CheckTypeJUnit,
							Check:   "periodic-build-farm-canary-build03",
							Results: "build 123: missing artifacts/junit_canary.xml; https://prow.ci.openshift.org/view/gs/test-platform-results-public/logs/periodic-build-farm-canary-build03/123",
						}},
					},
					ProbeType: ProbeTypeJUnit,
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons: []types.Reason{{
							Type:    types.CheckTypePrometheus,
							Check:   "up",
							Results: "query returned successfully",
						}},
					},
					ProbeType: ProbeTypePrometheus,
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusDegraded,
					Reasons: []types.Reason{{
						Type:    types.CheckTypeJUnit,
						Check:   "periodic-build-farm-canary-build03",
						Results: "build 123: missing artifacts/junit_canary.xml; https://prow.ci.openshift.org/view/gs/test-platform-results-public/logs/periodic-build-farm-canary-build03/123",
					}},
				},
			},
		},
		{
			name: "different sub-components remain separate",
			input: []ProbeResult{
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub1",
						Status:           types.StatusHealthy,
						Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub1.com"}},
					},
				},
				{
					ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
						ComponentSlug:    "comp1",
						SubComponentSlug: "sub2",
						Status:           types.StatusDown,
						Reasons:          []types.Reason{{Type: types.CheckTypeHTTP, Check: "http://sub2.com"}},
					},
				},
			},
			expected: []types.ComponentMonitorReportComponentStatus{
				{
					ComponentSlug:    "comp1",
					SubComponentSlug: "sub1",
					Status:           types.StatusHealthy,
					Reasons:          nil,
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
			result := mergeStatuses(tt.input)
			diff := cmp.Diff(tt.expected, result)
			if diff != "" {
				t.Errorf("mergeStatusesByComponent() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type countingProber struct {
	calls  *atomic.Int32
	result ProbeResult
}

func (p *countingProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	p.calls.Add(1)
	results <- p.result
}

type fakeReporter struct {
	mu      sync.Mutex
	reports [][]types.ComponentMonitorReportComponentStatus
	prints  int
}

func (f *fakeReporter) SendReport(results []types.ComponentMonitorReportComponentStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := append([]types.ComponentMonitorReportComponentStatus(nil), results...)
	f.reports = append(f.reports, copied)
	return nil
}

func (f *fakeReporter) PrintReport(results []types.ComponentMonitorReportComponentStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prints++
	return nil
}

func (f *fakeReporter) reportCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reports)
}

func TestScheduledProberDue(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		frequency time.Duration
		hasOK     bool
		lastOKAgo time.Duration
		want      bool
	}{
		{
			name:      "never succeeded is due",
			frequency: time.Minute,
			want:      true,
		},
		{
			name:      "success within frequency is not due",
			frequency: time.Minute,
			hasOK:     true,
			lastOKAgo: 10 * time.Second,
			want:      false,
		},
		{
			name:      "success older than frequency is due",
			frequency: time.Minute,
			hasOK:     true,
			lastOKAgo: 2 * time.Minute,
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scheduledProber{frequency: tt.frequency, hasOK: tt.hasOK}
			if tt.hasOK {
				s.lastOK = now.Add(-tt.lastOKAgo)
			}
			if got := s.due(now); got != tt.want {
				t.Errorf("due() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProbeOrchestrator_runOnce(t *testing.T) {
	healthy := ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    "comp",
			SubComponentSlug: "sub",
			Status:           types.StatusHealthy,
		},
		ProbeType: ProbeTypeHTTP,
	}
	errored := ProbeResult{
		ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
			ComponentSlug:    "comp",
			SubComponentSlug: "slow",
		},
		ProbeType: ProbeTypeJira,
		Error:     errors.New("search failed"),
	}

	t.Run("override frequency probe is not invoked until due", func(t *testing.T) {
		var defaultCalls, overrideCalls atomic.Int32
		reporter := &fakeReporter{}
		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)
		o := NewProbeOrchestrator(
			[]scheduledProber{
				{prober: &countingProber{calls: &defaultCalls, result: healthy}, frequency: 10 * time.Millisecond},
				{prober: &countingProber{calls: &overrideCalls, result: healthy}, frequency: time.Hour, hasOK: true, lastOK: time.Now()},
			},
			10*time.Millisecond,
			"http://test",
			"test-monitor",
			"",
			log,
		)
		o.reportClient = reporter

		o.runOnce(context.Background())

		if defaultCalls.Load() != 1 {
			t.Errorf("default probe calls = %d, want 1", defaultCalls.Load())
		}
		if overrideCalls.Load() != 0 {
			t.Errorf("override probe calls = %d, want 0", overrideCalls.Load())
		}
		if reporter.reportCount() != 1 {
			t.Errorf("reports = %d, want 1", reporter.reportCount())
		}
	})

	t.Run("success defers until frequency elapses", func(t *testing.T) {
		var calls atomic.Int32
		reporter := &fakeReporter{}
		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)
		o := NewProbeOrchestrator(
			[]scheduledProber{
				{prober: &countingProber{calls: &calls, result: healthy}, frequency: time.Hour},
			},
			10*time.Millisecond,
			"http://test",
			"test-monitor",
			"",
			log,
		)
		o.reportClient = reporter

		o.runOnce(context.Background())
		o.runOnce(context.Background())

		if calls.Load() != 1 {
			t.Errorf("probe calls = %d, want 1", calls.Load())
		}
		if reporter.reportCount() != 1 {
			t.Errorf("reports = %d, want 1", reporter.reportCount())
		}
	})

	t.Run("error does not advance lastOK", func(t *testing.T) {
		var calls atomic.Int32
		reporter := &fakeReporter{}
		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)
		o := NewProbeOrchestrator(
			[]scheduledProber{
				{prober: &countingProber{calls: &calls, result: errored}, frequency: time.Hour},
			},
			10*time.Millisecond,
			"http://test",
			"test-monitor",
			"",
			log,
		)
		o.reportClient = reporter

		o.runOnce(context.Background())
		o.runOnce(context.Background())

		if calls.Load() != 2 {
			t.Errorf("erroring probe calls = %d, want 2", calls.Load())
		}
	})

	t.Run("empty due set does not send report", func(t *testing.T) {
		var calls atomic.Int32
		reporter := &fakeReporter{}
		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)
		o := NewProbeOrchestrator(
			[]scheduledProber{
				{prober: &countingProber{calls: &calls, result: healthy}, frequency: time.Hour, hasOK: true, lastOK: time.Now()},
			},
			10*time.Millisecond,
			"http://test",
			"test-monitor",
			"",
			log,
		)
		o.reportClient = reporter

		o.runOnce(context.Background())

		if calls.Load() != 0 {
			t.Errorf("probe calls = %d, want 0", calls.Load())
		}
		if reporter.reportCount() != 0 {
			t.Errorf("reports = %d, want 0", reporter.reportCount())
		}
	})
}

func TestProbeOrchestrator_DryRunRunsAll(t *testing.T) {
	var calls atomic.Int32
	reporter := &fakeReporter{}
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)
	o := NewProbeOrchestrator(
		[]scheduledProber{
			{
				prober: &countingProber{
					calls: &calls,
					result: ProbeResult{
						ComponentMonitorReportComponentStatus: types.ComponentMonitorReportComponentStatus{
							ComponentSlug:    "comp",
							SubComponentSlug: "sub",
							Status:           types.StatusHealthy,
						},
					},
				},
				frequency: time.Hour,
				hasOK:     true,
				lastOK:    time.Now(),
			},
		},
		10*time.Millisecond,
		"http://test",
		"test-monitor",
		"",
		log,
	)
	o.reportClient = reporter

	o.DryRun(context.Background())

	if calls.Load() != 1 {
		t.Errorf("dry-run probe calls = %d, want 1", calls.Load())
	}
	if reporter.prints != 1 {
		t.Errorf("print reports = %d, want 1", reporter.prints)
	}
}
