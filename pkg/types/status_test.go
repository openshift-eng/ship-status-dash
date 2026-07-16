package types

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatusFromOutages(t *testing.T) {
	now := time.Now()
	confirmedTime := sql.NullTime{Time: now, Valid: true}
	unconfirmedTime := sql.NullTime{Valid: false}

	tests := []struct {
		name     string
		outages  []Outage
		expected Status
	}{
		{
			name: "single confirmed outage - down severity",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: confirmedTime},
			},
			expected: StatusDown,
		},
		{
			name: "single unconfirmed outage - down severity shows suspected",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
			},
			expected: StatusSuspected,
		},
		{
			name: "multiple confirmed outages - highest severity wins",
			outages: []Outage{
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
				{Severity: SeverityDown, ConfirmedAt: confirmedTime},
			},
			expected: StatusDown,
		},
		{
			name: "mixed confirmed and unconfirmed - confirmed takes precedence",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: StatusDegraded,
		},
		{
			name: "only unconfirmed non-degraded outages - shows suspected",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
			},
			expected: StatusSuspected,
		},
		{
			name: "unconfirmed degraded outage shows degraded not suspected",
			outages: []Outage{
				{Severity: SeverityDegraded, ConfirmedAt: unconfirmedTime},
			},
			expected: StatusDegraded,
		},
		{
			name: "confirmed degraded with unconfirmed down - confirmed takes precedence",
			outages: []Outage{
				{Severity: SeverityDown, ConfirmedAt: unconfirmedTime},
				{Severity: SeverityDegraded, ConfirmedAt: confirmedTime},
			},
			expected: StatusDegraded,
		},
		{
			name:     "empty outages slice",
			outages:  []Outage{},
			expected: StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StatusFromOutages(tt.outages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStatusFromActiveOutages(t *testing.T) {
	now := time.Now()
	confirmed := []Outage{{Severity: SeverityDown, ConfirmedAt: sql.NullTime{Time: now, Valid: true}}}
	suspected := []Outage{{Severity: SeveritySuspected}}

	assert.Equal(t, StatusHealthy, StatusFromActiveOutages(nil, nil))
	assert.Equal(t, StatusDown, StatusFromActiveOutages(confirmed, nil))
	assert.Equal(t, StatusDown, StatusFromActiveOutages(confirmed, suspected))
	assert.Equal(t, StatusSuspected, StatusFromActiveOutages(nil, suspected))
}

func TestIsValidStatus(t *testing.T) {
	assert.True(t, IsValidStatus("Healthy"))
	assert.True(t, IsValidStatus("Degraded"))
	assert.True(t, IsValidStatus("Down"))
	assert.True(t, IsValidStatus("CapacityExhausted"))
	assert.True(t, IsValidStatus("Suspected"))
	assert.True(t, IsValidStatus("Partial"))
	assert.False(t, IsValidStatus("Unknown"))
	assert.False(t, IsValidStatus(""))
	assert.False(t, IsValidStatus("down"))
}
