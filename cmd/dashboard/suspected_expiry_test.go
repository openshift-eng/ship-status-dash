package main

import (
	"fmt"
	"testing"
	"time"

	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuspectedOutageExpiryChecker_expireStaleOutages(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name               string
		setupOutageManager func(*outage.MockOutageManager)
		verifyUpdates      func(*testing.T, *outage.MockOutageManager)
	}{
		{
			name: "no stale outages found",
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.GetStaleSuspectedOutagesFn = func(cutoff time.Time) ([]types.Outage, error) {
					return []types.Outage{}, nil
				}
			},
			verifyUpdates: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Empty(t, m.UpdatedOutages, "no outages should be updated")
			},
		},
		{
			name: "single stale outage is resolved",
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.GetStaleSuspectedOutagesFn = func(cutoff time.Time) ([]types.Outage, error) {
					return []types.Outage{
						{
							ComponentName:    "comp",
							SubComponentName: "sub",
							Severity:         types.SeveritySuspected,
							StartTime:        time.Now().Add(-48 * time.Hour),
						},
					}, nil
				}
			},
			verifyUpdates: func(t *testing.T, m *outage.MockOutageManager) {
				require.Len(t, m.UpdatedOutages, 1)
				assert.True(t, m.UpdatedOutages[0].EndTime.Valid, "outage should have EndTime set")
				assert.Equal(t, "comp", m.UpdatedOutages[0].ComponentName)
			},
		},
		{
			name: "multiple stale outages are all resolved",
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.GetStaleSuspectedOutagesFn = func(cutoff time.Time) ([]types.Outage, error) {
					return []types.Outage{
						{
							ComponentName:    "comp-a",
							SubComponentName: "sub-1",
							Severity:         types.SeveritySuspected,
							StartTime:        time.Now().Add(-48 * time.Hour),
						},
						{
							ComponentName:    "comp-b",
							SubComponentName: "sub-2",
							Severity:         types.SeveritySuspected,
							StartTime:        time.Now().Add(-72 * time.Hour),
						},
					}, nil
				}
			},
			verifyUpdates: func(t *testing.T, m *outage.MockOutageManager) {
				require.Len(t, m.UpdatedOutages, 2)
				for _, o := range m.UpdatedOutages {
					assert.True(t, o.EndTime.Valid, "outage %s/%s should have EndTime set", o.ComponentName, o.SubComponentName)
				}
			},
		},
		{
			name: "query error prevents any updates",
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.GetStaleSuspectedOutagesFn = func(cutoff time.Time) ([]types.Outage, error) {
					return nil, fmt.Errorf("database connection refused")
				}
			},
			verifyUpdates: func(t *testing.T, m *outage.MockOutageManager) {
				assert.Empty(t, m.UpdatedOutages, "no outages should be updated after query error")
			},
		},
		{
			name: "partial update failure continues resolving remaining outages",
			setupOutageManager: func(m *outage.MockOutageManager) {
				m.GetStaleSuspectedOutagesFn = func(cutoff time.Time) ([]types.Outage, error) {
					return []types.Outage{
						{
							ComponentName:    "comp",
							SubComponentName: "sub-fail",
							Severity:         types.SeveritySuspected,
							StartTime:        time.Now().Add(-48 * time.Hour),
						},
						{
							ComponentName:    "comp",
							SubComponentName: "sub-ok",
							Severity:         types.SeveritySuspected,
							StartTime:        time.Now().Add(-48 * time.Hour),
						},
					}, nil
				}
				callCount := 0
				m.UpdateOutageFn = func(o *types.Outage, user string) error {
					callCount++
					if callCount == 1 {
						return fmt.Errorf("update failed")
					}
					outageCopy := *o
					m.UpdatedOutages = append(m.UpdatedOutages, &outageCopy)
					return nil
				}
			},
			verifyUpdates: func(t *testing.T, m *outage.MockOutageManager) {
				require.Len(t, m.UpdatedOutages, 1, "only the second outage should be captured as updated")
				assert.Equal(t, "sub-ok", m.UpdatedOutages[0].SubComponentName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOutageManager := &outage.MockOutageManager{}
			tt.setupOutageManager(mockOutageManager)

			checker := NewSuspectedOutageExpiryChecker(mockOutageManager, 5*time.Minute, logger)
			checker.expireStaleOutages()

			tt.verifyUpdates(t, mockOutageManager)
		})
	}
}
