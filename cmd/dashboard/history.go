package main

import (
	"sort"
	"time"

	"ship-status-dash/pkg/types"
)

type timeInterval struct {
	start, end time.Time
}

// mergedDuration sums non-overlapping durations from a set of potentially overlapping intervals.
func mergedDuration(intervals []timeInterval) time.Duration {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].start.Before(intervals[j].start)
	})
	var total time.Duration
	curStart, curEnd := intervals[0].start, intervals[0].end
	for _, iv := range intervals[1:] {
		if !iv.start.After(curEnd) {
			if iv.end.After(curEnd) {
				curEnd = iv.end
			}
		} else {
			total += curEnd.Sub(curStart)
			curStart, curEnd = iv.start, iv.end
		}
	}
	return total + curEnd.Sub(curStart)
}

type dayBucket struct {
	dayStart, nextDay time.Time
	intervals         []timeInterval
	highestSeverity   *types.Severity
	count             int
}

// buildHistoryBuckets aggregates outages into one bucket per calendar day for the past `days` days.
func buildHistoryBuckets(outages []types.Outage, days int, now time.Time) []types.OutageDayBucket {
	y, m, d := now.Date()
	loc := now.Location()

	buckets := make([]dayBucket, days)
	for i := range buckets {
		offset := days - 1 - i
		dayStart := time.Date(y, m, d-offset, 0, 0, 0, 0, loc)
		buckets[i] = dayBucket{
			dayStart: dayStart,
			nextDay:  dayStart.AddDate(0, 0, 1),
		}
	}

	for _, o := range outages {
		start := o.StartTime
		var end time.Time
		if o.EndTime.Valid {
			end = o.EndTime.Time
		} else {
			end = now
		}

		level := types.GetSeverityLevel(o.Severity)

		for i := range buckets {
			b := &buckets[i]

			if !start.Before(b.nextDay) || !end.After(b.dayStart) {
				continue
			}

			b.count++
			if b.highestSeverity == nil || level > types.GetSeverityLevel(*b.highestSeverity) {
				sev := o.Severity
				b.highestSeverity = &sev
			}

			clippedStart := start
			if clippedStart.Before(b.dayStart) {
				clippedStart = b.dayStart
			}
			clippedEnd := end
			if clippedEnd.After(b.nextDay) {
				clippedEnd = b.nextDay
			}
			if clippedEnd.After(clippedStart) {
				b.intervals = append(b.intervals, timeInterval{clippedStart, clippedEnd})
			}
		}
	}

	result := make([]types.OutageDayBucket, days)
	for i, b := range buckets {
		var highestSeverity *string
		if b.highestSeverity != nil {
			s := string(*b.highestSeverity)
			highestSeverity = &s
		}
		result[i] = types.OutageDayBucket{
			Date:               b.dayStart.Format("2006-01-02"),
			HighestSeverity:    highestSeverity,
			TotalOutageMinutes: mergedDuration(b.intervals).Minutes(),
			OutageCount:        b.count,
		}
	}

	return result
}
