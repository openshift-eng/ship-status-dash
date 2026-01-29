package outage

import (
	"ship-status-dash/pkg/types"
)

// MockOutageManager is a mock implementation of the OutageManager interface for testing.
type MockOutageManager struct {
	// Mock data for queries
	ActiveOutagesCreatedBy      []types.Outage
	ActiveOutagesCreatedByError error

	// Captured data for assertions
	CreatedOutages []struct {
		Outage  *types.Outage
		Reasons []types.Reason
	}
	UpdatedOutages []*types.Outage

	// Call tracking
	GetActiveOutagesCreatedByCallCount int

	// Mock functions
	CreateOutageFn                   func(*types.Outage, []types.Reason) error
	UpdateOutageFn                   func(*types.Outage) error
	GetActiveOutagesCreatedByFn      func(string, string, string) ([]types.Outage, error)
	GetActiveOutagesDiscoveredFromFn func(string, string, string) ([]types.Outage, error)
}

// GetActiveOutagesCreatedBy returns mock active outages.
func (m *MockOutageManager) GetActiveOutagesCreatedBy(componentSlug, subComponentSlug, createdBy string) ([]types.Outage, error) {
	currentCallCount := m.GetActiveOutagesCreatedByCallCount
	m.GetActiveOutagesCreatedByCallCount++
	if m.GetActiveOutagesCreatedByFn != nil {
		// Temporarily restore the count so the function sees the pre-increment value
		originalCount := m.GetActiveOutagesCreatedByCallCount
		m.GetActiveOutagesCreatedByCallCount = currentCallCount
		result, err := m.GetActiveOutagesCreatedByFn(componentSlug, subComponentSlug, createdBy)
		m.GetActiveOutagesCreatedByCallCount = originalCount
		return result, err
	}
	if m.ActiveOutagesCreatedByError != nil {
		return nil, m.ActiveOutagesCreatedByError
	}
	return m.ActiveOutagesCreatedBy, nil
}

// CreateOutage captures the outage and reasons for assertions.
func (m *MockOutageManager) CreateOutage(outage *types.Outage, reasons []types.Reason) error {
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
func (m *MockOutageManager) UpdateOutage(outage *types.Outage) error {
	if m.UpdateOutageFn != nil {
		return m.UpdateOutageFn(outage)
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

// GetActiveOutagesForComponent is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) GetActiveOutagesForComponent(componentSlug string) ([]types.Outage, error) {
	return nil, nil
}

// GetActiveOutagesDiscoveredFrom returns mock active outages discovered from a specific source.
func (m *MockOutageManager) GetActiveOutagesDiscoveredFrom(componentSlug, subComponentSlug, discoveredFrom string) ([]types.Outage, error) {
	if m.GetActiveOutagesDiscoveredFromFn != nil {
		return m.GetActiveOutagesDiscoveredFromFn(componentSlug, subComponentSlug, discoveredFrom)
	}
	return []types.Outage{}, nil
}

// DeleteOutage is not used by ComponentMonitorReportProcessor but included for interface completeness.
func (m *MockOutageManager) DeleteOutage(outage *types.Outage) error {
	return nil
}
