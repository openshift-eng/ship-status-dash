package outage

import (
	"time"

	"ship-status-dash/pkg/types"
)

// MockOutageManager is a mock implementation of the OutageManager interface for testing.
type MockOutageManager struct {
	// Mock data for queries
	ActiveOutagesCreatedBy      []types.Outage
	ActiveOutagesCreatedByError error
	RecentlyClosedOutages       []types.Outage

	// Captured data for assertions
	CreatedOutages []struct {
		Outage  *types.Outage
		Reasons []types.Reason
	}
	UpdatedOutages []*types.Outage

	// Mock functions
	CreateOutageFn                   func(*types.Outage, []types.Reason) error
	UpdateOutageFn                   func(*types.Outage, string) error
	GetActiveOutagesCreatedByFn      func(string, string, string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFromFn func(string, string, string) ([]types.Outage, error)
	GetActiveOutagesForComponentFn   func(string) ([]types.Outage, error)
	FindReopenableOutageFn           func(string, string, string, time.Time, []types.Reason) (*types.Outage, error)
	GetOutagesDuringFn               func(time.Time, time.Time, []types.SubComponentRef) ([]types.Outage, error)

	LastGetOutagesDuringQueryStart time.Time
	LastGetOutagesDuringQueryEnd   time.Time
	LastGetOutagesDuringRefs       []types.SubComponentRef
}

// GetActiveOutagesCreatedBy returns mock active outages.
func (m *MockOutageManager) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	if m.GetActiveOutagesCreatedByFn != nil {
		return m.GetActiveOutagesCreatedByFn(componentSlug, subComponentSlug, createdBy)
	}
	if m.ActiveOutagesCreatedByError != nil {
		return nil, m.ActiveOutagesCreatedByError
	}
	return m.ActiveOutagesCreatedBy, nil
}

// CreateOutage captures the outage and reasons for assertions.
func (m *MockOutageManager) CreateOutage(outage *types.Outage, reasons []types.Reason, user string) error {
	if m.CreateOutageFn != nil {
		return m.CreateOutageFn(outage, reasons)
	}
	// Capture the outage and reasons
	outageCopy := *outage
	reasonsCopy := make([]types.Reason, len(reasons))
	copy(reasonsCopy, reasons)
	m.CreatedOutages = append(m.CreatedOutages, struct {
		Outage  *types.Outage
		Reasons []types.Reason
	}{
		Outage:  &outageCopy,
		Reasons: reasonsCopy,
	})
	return nil
}

// UpdateOutage captures the outage for assertions.
func (m *MockOutageManager) UpdateOutage(outage *types.Outage, user string) error {
	if m.UpdateOutageFn != nil {
		return m.UpdateOutageFn(outage, user)
	}
	// Capture the outage
	outageCopy := *outage
	m.UpdatedOutages = append(m.UpdatedOutages, &outageCopy)
	return nil
}

// GetOutageByID is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) GetOutageByID(componentSlug, subComponentSlug string, outageID uint) (*types.Outage, error) {
	return nil, nil
}

// GetOutagesForSubComponent is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) GetOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return nil, nil
}

// GetOutagesForComponent is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) GetOutagesForComponent(componentSlug string, subComponentSlugs []string) ([]types.Outage, error) {
	return nil, nil
}

// GetActiveOutagesForSubComponent is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) GetActiveOutagesForSubComponent(componentSlug, subComponentSlug string) ([]types.Outage, error) {
	return nil, nil
}

// GetActiveOutagesForComponent returns mock active outages for a component.
func (m *MockOutageManager) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	if m.GetActiveOutagesForComponentFn != nil {
		return m.GetActiveOutagesForComponentFn(componentSlug)
	}
	return nil, nil
}

// GetActiveOutagesDiscoveredFrom returns mock active outages discovered from a specific source.
func (m *MockOutageManager) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	if m.GetActiveOutagesDiscoveredFromFn != nil {
		return m.GetActiveOutagesDiscoveredFromFn(componentSlug, subComponentSlug, discoveredFrom)
	}
	return []types.Outage{}, nil
}

func (m *MockOutageManager) AppendReasons(outageID uint, reasons []types.Reason) error {
	return nil
}

// FindReopenableOutage simulates the SQL join by searching RecentlyClosedOutages for the first
// outage whose reasons overlap with the incoming reasons.
func (m *MockOutageManager) FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error) {
	if m.FindReopenableOutageFn != nil {
		return m.FindReopenableOutageFn(componentSlug, subComponentSlug, createdBy, since, reasons)
	}
	for i := range m.RecentlyClosedOutages {
		for _, existing := range m.RecentlyClosedOutages[i].Reasons {
			for _, incoming := range reasons {
				if existing.Type == incoming.Type && existing.Check == incoming.Check {
					return &m.RecentlyClosedOutages[i], nil
				}
			}
		}
	}
	return nil, nil
}

// GetOutagesDuring records the last call and delegates to GetOutagesDuringFn when set.
func (m *MockOutageManager) GetOutagesDuring(queryStart, queryEnd time.Time, refs []types.SubComponentRef) ([]types.Outage, error) {
	m.LastGetOutagesDuringQueryStart = queryStart
	m.LastGetOutagesDuringQueryEnd = queryEnd
	m.LastGetOutagesDuringRefs = append([]types.SubComponentRef(nil), refs...)
	if m.GetOutagesDuringFn != nil {
		return m.GetOutagesDuringFn(queryStart, queryEnd, refs)
	}
	return []types.Outage{}, nil
}

// GetOutageAuditLogs is included for interface completeness.
func (m *MockOutageManager) GetOutageAuditLogs(outageID uint) ([]types.OutageAuditLog, error) {
	return nil, nil
}

// DeleteOutage is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) DeleteOutage(outage *types.Outage, user string) error {
	return nil
}
