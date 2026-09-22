package main

import (
	"context"
	"errors"
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

			orchestrator := NewProbeOrchestrator(
				nil,
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

			outcomes := make(chan probeOutcome, len(tt.probeResults))
			go func() {
				time.Sleep(20 * time.Millisecond)
				for _, result := range tt.probeResults {
					outcomes <- probeOutcome{result: result}
				}
			}()

			results := orchestrator.collectProbeResults(ctx, outcomes, expected)
			if diff := cmp.Diff(tt.probeResults, results, testhelper.EquateErrorMessage); diff != "" {
				t.Errorf("collectProbeResults() mismatch (-want +got):\n%s", diff)
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
	delay  time.Duration
}

func (p *countingProber) Probe(ctx context.Context, results chan<- ProbeResult) {
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
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
		lastOKAgo time.Duration // 0 means never succeeded
		setLastOK bool
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
			setLastOK: true,
			lastOKAgo: 10 * time.Second,
			want:      false,
		},
		{
			name:      "success older than frequency is due",
			frequency: time.Minute,
			setLastOK: true,
			lastOKAgo: 2 * time.Minute,
			want:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &scheduledProber{frequency: tt.frequency}
			if tt.setLastOK {
				s.lastOK = now.Add(-tt.lastOKAgo)
			}
			if got := s.due(now); got != tt.want {
				t.Errorf("due() = %v, want %v", got, tt.want)
			}
		})
	}
}

type runOnceProbeSpec struct {
	frequency   time.Duration
	recentlyRan bool
	delay       time.Duration
	err         bool
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

	tests := []struct {
		name            string
		tick            time.Duration
		probes          []runOnceProbeSpec
		runs            int
		waitForNextTick bool
		wantCalls       []int32
		wantReports     int
	}{
		{
			name: "override frequency probe is not invoked until due",
			tick: 10 * time.Millisecond,
			probes: []runOnceProbeSpec{
				{frequency: 10 * time.Millisecond},
				{frequency: time.Hour, recentlyRan: true},
			},
			runs:        1,
			wantCalls:   []int32{1, 0},
			wantReports: 1,
		},
		{
			name: "success defers until frequency elapses",
			tick: 10 * time.Millisecond,
			probes: []runOnceProbeSpec{
				{frequency: time.Hour},
			},
			runs:        2,
			wantCalls:   []int32{1},
			wantReports: 1,
		},
		{
			name: "error does not advance lastOK",
			tick: 10 * time.Millisecond,
			probes: []runOnceProbeSpec{
				{frequency: time.Hour, err: true},
			},
			runs:        2,
			wantCalls:   []int32{2},
			wantReports: 0,
		},
		{
			name: "success is due on the next tick even if the probe ran long",
			tick: 40 * time.Millisecond,
			probes: []runOnceProbeSpec{
				{frequency: 40 * time.Millisecond, delay: 15 * time.Millisecond},
			},
			runs:            2,
			waitForNextTick: true,
			wantCalls:       []int32{2},
			wantReports:     2,
		},
		{
			name: "empty due set does not send report",
			tick: 10 * time.Millisecond,
			probes: []runOnceProbeSpec{
				{frequency: time.Hour, recentlyRan: true},
			},
			runs:        1,
			wantCalls:   []int32{0},
			wantReports: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counters := make([]atomic.Int32, len(tt.probes))
			schedule := make([]scheduledProber, len(tt.probes))
			for i, spec := range tt.probes {
				result := healthy
				if spec.err {
					result = errored
				}
				entry := scheduledProber{
					prober:    &countingProber{calls: &counters[i], result: result, delay: spec.delay},
					frequency: spec.frequency,
				}
				if spec.recentlyRan {
					entry.lastOK = time.Now()
				}
				schedule[i] = entry
			}

			reporter := &fakeReporter{}
			log := logrus.New()
			log.SetLevel(logrus.ErrorLevel)
			o := NewProbeOrchestrator(schedule, tt.tick, "http://test", "test-monitor", "", log)
			o.reportClient = reporter

			ctx := context.Background()
			cycleStart := time.Now()
			for run := 0; run < tt.runs; run++ {
				if run > 0 && tt.waitForNextTick {
					if remaining := time.Until(cycleStart.Add(tt.tick)); remaining > 0 {
						time.Sleep(remaining)
					}
				}
				o.runOnce(ctx)
			}

			for i, want := range tt.wantCalls {
				if got := counters[i].Load(); got != want {
					t.Errorf("probe %d calls = %d, want %d", i, got, want)
				}
			}
			if got := reporter.reportCount(); got != tt.wantReports {
				t.Errorf("reports = %d, want %d", got, tt.wantReports)
			}
		})
	}
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
