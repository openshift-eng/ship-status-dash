package types

type Status string

const (
	StatusHealthy   Status = "Healthy"
	StatusDegraded  Status = "Degraded"
	StatusDown      Status = "Down"
	StatusSuspected Status = "Suspected"
	StatusPartial   Status = "Partial" // Indicates that some sub-components are healthy, and some are degraded or down
)

// ToSeverity converts a Status to a Severity. Returns an empty string if the status cannot be converted to a severity.
func (s Status) ToSeverity() Severity {
	switch s {
	case StatusDown:
		return SeverityDown
	case StatusDegraded:
		return SeverityDegraded
	default:
		return ""
	}
}

type ComponentStatus struct {
	ComponentName string   `json:"component_name"`
	Status        Status   `json:"status"`
	ActiveOutages []Outage `json:"active_outages"`
}
