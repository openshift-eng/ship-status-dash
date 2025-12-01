package types

import (
	"database/sql"
	"time"
)

// UpsertOutageRequest represents the fields to create or update an outage.
type UpsertOutageRequest struct {
	Severity       *string       `json:"severity,omitempty"`
	StartTime      *time.Time    `json:"start_time,omitempty"`
	EndTime        *sql.NullTime `json:"end_time,omitempty"`
	Description    *string       `json:"description,omitempty"`
	DiscoveredFrom *string       `json:"discovered_from,omitempty"`
	// CreatedBy should not be passed by the frontend, this is only for use via the component-monitor
	CreatedBy *string `json:"created_by,omitempty"`
	// ResolvedBy should not be passed by the frontend, this is only for use via the component-monitor
	// the value will be obtained from the active user header otherwise
	ResolvedBy  *string `json:"resolved_by,omitempty"`
	Confirmed   *bool   `json:"confirmed,omitempty"`
	TriageNotes *string `json:"triage_notes,omitempty"`
}

// ComponentMonitorReportRequest represents a report from a component monitor.
type ComponentMonitorReportRequest struct {
	ComponentMonitor string                                  `json:"component_monitor"`
	Statuses         []ComponentMonitorReportComponentStatus `json:"statuses"`
}

// ComponentMonitorReportComponentStatus represents the status of a component/sub-component in a monitor report.
type ComponentMonitorReportComponentStatus struct {
	ComponentSlug    string `json:"component_name"`
	SubComponentSlug string `json:"sub_component_name"`
	Status           Status `json:"status"`
	Reason           Reason `json:"reason"`
}
