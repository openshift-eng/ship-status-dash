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

// FindReopenableOutage simulates the SQL join by filtering RecentlyClosedOutages on
// component, sub-component, creator, flap window, and reason overlap.
func (m *MockOutageManager) FindReopenableOutage(componentSlug, subComponentSlug, createdBy string, since time.Time, reasons []types.Reason) (*types.Outage, error) {
	if m.FindReopenableOutageFn != nil {
		return m.FindReopenableOutageFn(componentSlug, subComponentSlug, createdBy, since, reasons)
	}
	for i := range m.RecentlyClosedOutages {
		outage := &m.RecentlyClosedOutages[i]
		if outage.ComponentName != componentSlug || outage.SubComponentName != subComponentSlug || outage.CreatedBy != createdBy {
			continue
		}
		if !outage.EndTime.Valid || outage.EndTime.Time.Before(since) {
			continue
		}
		for _, existing := range outage.Reasons {
			for _, incoming := range reasons {
				if existing.Type == incoming.Type && existing.Check == incoming.Check {
					matched := *outage
					return &matched, nil
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

func (m *MockOutageManager) AddTriageNote(note *types.TriageNote) error {
	return nil
}

func (m *MockOutageManager) UpdateTriageNote(noteID, outageID uint, body, user string) (*types.TriageNote, error) {
	return nil, nil
}

func (m *MockOutageManager) DeleteTriageNote(noteID, outageID uint, user string) error {
	return nil
}

func (m *MockOutageManager) AddOutageLink(link *types.OutageLink) error {
	return nil
}

func (m *MockOutageManager) UpdateOutageLink(linkID, outageID uint, url string, linkType types.LinkType, description, user string) (*types.OutageLink, error) {
	return nil, nil
}

func (m *MockOutageManager) DeleteOutageLink(outageID, linkID uint, user string) error {
	return nil
}
