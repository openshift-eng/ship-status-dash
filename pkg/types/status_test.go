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

	tests := []struct {
		name      string
		confirmed []Outage
		suspected []Outage
		want      Status
	}{
		{name: "healthy when empty", want: StatusHealthy},
		{name: "confirmed only", confirmed: confirmed, want: StatusDown},
		{name: "confirmed takes precedence over suspected", confirmed: confirmed, suspected: suspected, want: StatusDown},
		{name: "suspected only", suspected: suspected, want: StatusSuspected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, StatusFromActiveOutages(tt.confirmed, tt.suspected))
		})
	}
}

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "Healthy", s: "Healthy", want: true},
		{name: "Degraded", s: "Degraded", want: true},
		{name: "Down", s: "Down", want: true},
		{name: "CapacityExhausted", s: "CapacityExhausted", want: true},
		{name: "Suspected", s: "Suspected", want: true},
		{name: "Partial", s: "Partial", want: true},
		{name: "Unknown", s: "Unknown", want: false},
		{name: "empty", s: "", want: false},
		{name: "lowercase down", s: "down", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidStatus(tt.s))
		})
	}
}

func TestIsValidSubComponentStatus(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "Healthy", s: "Healthy", want: true},
		{name: "Degraded", s: "Degraded", want: true},
		{name: "Down", s: "Down", want: true},
		{name: "CapacityExhausted", s: "CapacityExhausted", want: true},
		{name: "Suspected", s: "Suspected", want: true},
		{name: "Partial excluded", s: "Partial", want: false},
		{name: "Unknown", s: "Unknown", want: false},
		{name: "empty", s: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsValidSubComponentStatus(tt.s))
		})
	}
}
