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
	Confirmed      *bool         `json:"confirmed,omitempty"`
}

// TriageNoteBodyRequest represents the body of a request to add or update a triage note.
type TriageNoteBodyRequest struct {
	Body string `json:"body"`
}

// OutageLinkRequest represents the body of a request to add or update an outage link.
type OutageLinkRequest struct {
	URL         string `json:"url"`
	LinkType    string `json:"link_type,omitempty"`
	Description string `json:"description,omitempty"`
}

// ComponentMonitorReportRequest represents a report from a component monitor.
type ComponentMonitorReportRequest struct {
	ComponentMonitor string                                  `json:"component_monitor"`
	Statuses         []ComponentMonitorReportComponentStatus `json:"statuses"`
}

// ComponentMonitorReportComponentStatus represents the status of a component/sub-component in a monitor report.
type ComponentMonitorReportComponentStatus struct {
	ComponentSlug    string   `json:"component_name"`
	SubComponentSlug string   `json:"sub_component_name"`
	Status           Status   `json:"status"`
	Reasons          []Reason `json:"reasons"`
}

// SubComponentListItem is a sub-component with its parent component name for list API responses.
type SubComponentListItem struct {
	ComponentName string `json:"component_name"`
	SubComponent
}

// OutageDayBucket holds aggregated outage data for a single calendar day.
type OutageDayBucket struct {
	Date               string  `json:"date"`                 // YYYY-MM-DD
	HighestSeverity    *string `json:"highest_severity"`     // null when no outages that day
	TotalOutageMinutes float64 `json:"total_outage_minutes"` // merged, non-overlapping minutes
	OutageCount        int     `json:"outage_count"`
}
