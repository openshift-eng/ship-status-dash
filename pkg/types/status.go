package types

import "time"

type Status string

const (
	StatusHealthy           Status = "Healthy"
	StatusDegraded          Status = "Degraded"
	StatusDown              Status = "Down"
	StatusCapacityExhausted Status = "CapacityExhausted"
	StatusSuspected         Status = "Suspected"
	StatusPartial           Status = "Partial" // Indicates that some sub-components are healthy, and some are degraded or down
)

// ToSeverity converts a Status to a Severity. Returns an empty string if the status cannot be converted to a severity.
func (s Status) ToSeverity() Severity {
	switch s {
	case StatusDown:
		return SeverityDown
	case StatusDegraded:
		return SeverityDegraded
	case StatusCapacityExhausted:
		return SeverityCapacityExhausted
	case StatusSuspected:
		return SeveritySuspected
	default:
		return ""
	}
}

// SuspectedOutageInfo exposes community-reported suspected outage data in status responses.
type SuspectedOutageInfo struct {
	OutageID    uint      `json:"outage_id"`
	ReportCount int64     `json:"report_count"`
	Description string    `json:"description,omitempty"`
	StartTime   time.Time `json:"start_time"`
	Reporters   []string  `json:"reporters"`
}

type ComponentStatus struct {
	ComponentName        string               `json:"component_name"`
	Status               Status               `json:"status"`
	ActiveOutages        []Outage             `json:"active_outages"`
	LastPingTime         *time.Time           `json:"last_ping_time,omitempty"`
	SubComponentStatuses map[string]Status    `json:"sub_component_statuses,omitempty"`
	SuspectedOutage      *SuspectedOutageInfo `json:"suspected_outage,omitempty"`
}

// StatusFromOutages returns the roll-up status from active outages. Suspected-severity
// outages are filtered out upstream by the excludeSuspected repository scope.
// Unconfirmed outages whose severity is not Degraded are treated as Suspected
// (admin-created outages on requires_confirmation sub-components). Unconfirmed Degraded
// outages are treated as Degraded (community-reported outages that reached the report threshold
// but have not yet been admin-confirmed).
func StatusFromOutages(outages []Outage) Status {
	if len(outages) == 0 {
		return StatusHealthy
	}

	confirmedOutages := make([]Outage, 0)
	hasUnconfirmedNonDegraded := false

	for _, outage := range outages {
		if outage.ConfirmedAt.Valid || outage.Severity == SeverityDegraded {
			confirmedOutages = append(confirmedOutages, outage)
		} else {
			hasUnconfirmedNonDegraded = true
		}
	}

	if len(confirmedOutages) > 0 {
		mostCriticalSeverity := confirmedOutages[0].Severity
		highestLevel := GetSeverityLevel(mostCriticalSeverity)

		for _, outage := range confirmedOutages[1:] {
			level := GetSeverityLevel(outage.Severity)
			if level > highestLevel {
				highestLevel = level
				mostCriticalSeverity = outage.Severity
			}
		}
		return mostCriticalSeverity.ToStatus()
	}

	if hasUnconfirmedNonDegraded {
		return StatusSuspected
	}

	return StatusHealthy
}
