package main

import (
	"sort"
	"time"

	"ship-status-dash/pkg/types"
)

// severityOrder defines severity levels in ascending priority (higher index = worse).
var severityOrder = []string{"Unknown", "Suspected", "Partial", "Degraded", "CapacityExhausted", "Down"}

func severityPriority(s string) int {
	for i, v := range severityOrder {
		if v == s {
			return i
		}
	}
	return -1
}

// mergedDuration sums non-overlapping durations from a set of potentially overlapping intervals.
func mergedDuration(intervals [][2]time.Time) time.Duration {
	if len(intervals) == 0 {
		return 0
	}
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i][0].Before(intervals[j][0])
	})
	total := time.Duration(0)
	curStart, curEnd := intervals[0][0], intervals[0][1]
	for _, iv := range intervals[1:] {
		if !iv[0].After(curEnd) {
			if iv[1].After(curEnd) {
				curEnd = iv[1]
			}
		} else {
			total += curEnd.Sub(curStart)
			curStart, curEnd = iv[0], iv[1]
		}
	}
	return total + curEnd.Sub(curStart)
}

// buildHistoryBuckets aggregates outages into one bucket per calendar day for the past `days` days.
func buildHistoryBuckets(outages []types.Outage, days int, now time.Time) []types.OutageDayBucket {
	buckets := make([]types.OutageDayBucket, 0, days)

	y, m, d := now.Date()
	loc := now.Location()

	for i := days - 1; i >= 0; i-- {
		dayStart := time.Date(y, m, d-i, 0, 0, 0, 0, loc)
		nextDayStart := dayStart.AddDate(0, 0, 1)

		var intervals [][2]time.Time
		highestPriority := -1
		var highestSeverity *string
		outageCount := 0

		for _, o := range outages {
			start := o.StartTime
			var end time.Time
			if o.EndTime.Valid {
				end = o.EndTime.Time
			} else {
				end = now
			}

			// Skip outages that don't overlap this day.
			if !start.Before(nextDayStart) || !end.After(dayStart) {
				continue
			}

			outageCount++

			sev := string(o.Severity)
			priority := severityPriority(sev)
			if priority > highestPriority {
				highestPriority = priority
				s := sev
				highestSeverity = &s
			} else if priority == -1 && highestSeverity == nil {
				s := sev
				highestSeverity = &s
			}

		// Clip interval to [dayStart, nextDayStart).
		clippedStart := start
		if clippedStart.Before(dayStart) {
			clippedStart = dayStart
		}
		clippedEnd := end
		if clippedEnd.After(nextDayStart) {
			clippedEnd = nextDayStart
		}
			if clippedEnd.After(clippedStart) {
				intervals = append(intervals, [2]time.Time{clippedStart, clippedEnd})
			}
		}

		buckets = append(buckets, types.OutageDayBucket{
			Date:               dayStart.Format("2006-01-02"),
			HighestSeverity:    highestSeverity,
			TotalOutageMinutes: mergedDuration(intervals).Minutes(),
			OutageCount:        outageCount,
		})
	}

	return buckets
}
