package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/types"
)

func TestMergedDuration(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		intervals []timeInterval
		want      time.Duration
	}{
		{
			name:      "empty",
			intervals: nil,
			want:      0,
		},
		{
			name:      "single interval",
			intervals: []timeInterval{{t0, t0.Add(30 * time.Minute)}},
			want:      30 * time.Minute,
		},
		{
			name: "non-overlapping",
			intervals: []timeInterval{
				{t0, t0.Add(10 * time.Minute)},
				{t0.Add(20 * time.Minute), t0.Add(30 * time.Minute)},
			},
			want: 20 * time.Minute,
		},
		{
			name: "overlapping",
			intervals: []timeInterval{
				{t0, t0.Add(30 * time.Minute)},
				{t0.Add(15 * time.Minute), t0.Add(45 * time.Minute)},
			},
			want: 45 * time.Minute,
		},
		{
			name: "contained",
			intervals: []timeInterval{
				{t0, t0.Add(60 * time.Minute)},
				{t0.Add(10 * time.Minute), t0.Add(20 * time.Minute)},
			},
			want: 60 * time.Minute,
		},
		{
			name: "adjacent",
			intervals: []timeInterval{
				{t0, t0.Add(10 * time.Minute)},
				{t0.Add(10 * time.Minute), t0.Add(20 * time.Minute)},
			},
			want: 20 * time.Minute,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, mergedDuration(tc.intervals))
		})
	}
}

func TestBuildHistoryBuckets(t *testing.T) {
	now := time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)
	days := 3

	t.Run("no outages produces all healthy buckets", func(t *testing.T) {
		buckets := buildHistoryBuckets(nil, days, now)
		require.Len(t, buckets, days)
		for _, b := range buckets {
			assert.Equal(t, 0, b.OutageCount)
			assert.Nil(t, b.HighestSeverity)
			assert.Equal(t, 0.0, b.TotalOutageMinutes)
		}
	})

	t.Run("outage on one day appears in correct bucket", func(t *testing.T) {
		// Outage on Jan 9 (second-to-last day), fully contained.
		start := time.Date(2024, 1, 9, 2, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 9, 4, 0, 0, 0, time.UTC)
		outages := []types.Outage{
			{
				Severity:  types.SeverityDown,
				StartTime: start,
				EndTime:   sql.NullTime{Time: end, Valid: true},
			},
		}

		buckets := buildHistoryBuckets(outages, days, now)
		require.Len(t, buckets, days)

		assert.Equal(t, "2024-01-08", buckets[0].Date)
		assert.Equal(t, 0, buckets[0].OutageCount)

		assert.Equal(t, "2024-01-09", buckets[1].Date)
		assert.Equal(t, 1, buckets[1].OutageCount)
		require.NotNil(t, buckets[1].HighestSeverity)
		assert.Equal(t, "Down", *buckets[1].HighestSeverity)
		assert.InDelta(t, 120.0, buckets[1].TotalOutageMinutes, 0.01)

		assert.Equal(t, "2024-01-10", buckets[2].Date)
		assert.Equal(t, 0, buckets[2].OutageCount)
	})

	t.Run("ongoing outage contributes up to now", func(t *testing.T) {
		// Outage started 30 min ago and is still ongoing.
		start := now.Add(-30 * time.Minute)
		outages := []types.Outage{
			{
				Severity:  types.SeverityDegraded,
				StartTime: start,
				EndTime:   sql.NullTime{Valid: false},
			},
		}

		buckets := buildHistoryBuckets(outages, days, now)
		last := buckets[len(buckets)-1]
		assert.Equal(t, 1, last.OutageCount)
		assert.InDelta(t, 30.0, last.TotalOutageMinutes, 0.01)
	})

	t.Run("outage spanning midnight is split across two buckets", func(t *testing.T) {
		// Starts 1h before midnight Jan 8→9, ends 2h into Jan 9: 1h in Jan 8, 2h in Jan 9.
		start := time.Date(2024, 1, 8, 23, 0, 0, 0, time.UTC)
		end := time.Date(2024, 1, 9, 2, 0, 0, 0, time.UTC)
		outages := []types.Outage{
			{
				Severity:  types.SeverityDown,
				StartTime: start,
				EndTime:   sql.NullTime{Time: end, Valid: true},
			},
		}

		buckets := buildHistoryBuckets(outages, days, now)
		require.Len(t, buckets, days)

		jan8 := buckets[0]
		assert.Equal(t, "2024-01-08", jan8.Date)
		assert.Equal(t, 1, jan8.OutageCount)
		assert.InDelta(t, 60.0, jan8.TotalOutageMinutes, 0.01)

		jan9 := buckets[1]
		assert.Equal(t, "2024-01-09", jan9.Date)
		assert.Equal(t, 1, jan9.OutageCount)
		assert.InDelta(t, 120.0, jan9.TotalOutageMinutes, 0.01)
	})

	t.Run("overlapping outages on same day are merged", func(t *testing.T) {
		// Two outages: 00:00-01:00 and 00:30-02:00 → merged = 2h = 120 min.
		day := time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC)
		outages := []types.Outage{
			{
				Severity:  types.SeverityDown,
				StartTime: day,
				EndTime:   sql.NullTime{Time: day.Add(60 * time.Minute), Valid: true},
			},
			{
				Severity:  types.SeverityDegraded,
				StartTime: day.Add(30 * time.Minute),
				EndTime:   sql.NullTime{Time: day.Add(120 * time.Minute), Valid: true},
			},
		}

		buckets := buildHistoryBuckets(outages, days, now)
		b := buckets[1] // Jan 9
		assert.Equal(t, 2, b.OutageCount)
		assert.InDelta(t, 120.0, b.TotalOutageMinutes, 0.01)
		require.NotNil(t, b.HighestSeverity)
		assert.Equal(t, "Down", *b.HighestSeverity, "highest severity should be Down")
	})
}
