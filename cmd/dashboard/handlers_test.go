package main

import (
	"database/sql"
	"testing"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/stretchr/testify/assert"
)

func TestDetermineStatusFromSeverity(t *testing.T) {
	now := time.Now()
	confirmedTime := sql.NullTime{Time: now, Valid: true}
	unconfirmedTime := sql.NullTime{Valid: false}

	tests := []struct {
		name     string
		outages  []types.Outage
		expected types.Status
	}{
		{
			name: "single confirmed outage - down severity",
			outages: []types.Outage{
				{Severity: types.SeverityDown, ConfirmedAt: confirmedTime},
			},
			expected: types.StatusDown,
		},
		{
			name: "single unconfirmed outage - down severity",
			outages: []types.Outage{
				{Severity: types.SeverityDown, ConfirmedAt: unconfirmedTime},
			},
			expected: types.StatusSuspected,
		},
		{
			name: "multiple confirmed outages - highest severity wins",
			outages: []types.Outage{
				{Severity: types.SeveritySuspected, ConfirmedAt: confirmedTime},
				{Severity: types.SeverityDown, ConfirmedAt: confirmedTime},
				{Severity: types.SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: types.StatusDown,
		},
		{
			name: "mixed confirmed and unconfirmed - confirmed takes precedence",
			outages: []types.Outage{
				{Severity: types.SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: types.SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: types.StatusDegraded,
		},
		{
			name: "only unconfirmed outages - shows suspected",
			outages: []types.Outage{
				{Severity: types.SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: types.SeverityDegraded, ConfirmedAt: unconfirmedTime},
			},
			expected: types.StatusSuspected,
		},
		{
			name: "confirmed degraded with unconfirmed down - confirmed takes precedence",
			outages: []types.Outage{
				{Severity: types.SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: types.SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: types.StatusDegraded,
		},
		{
			name:     "empty outages slice",
			outages:  []types.Outage{},
			expected: types.StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatusFromSeverity(tt.outages)
			assert.Equal(t, tt.expected, result)
		})
	}
}
