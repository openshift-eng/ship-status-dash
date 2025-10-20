package main

import (
	"testing"

	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestDetermineStatusFromSeverity(t *testing.T) {
	tests := []struct {
		name     string
		outages  []types.Outage
		expected types.Status
	}{
		{
			name: "single outage - down severity",
			outages: []types.Outage{
				{Severity: types.SeverityDown},
			},
			expected: types.StatusDown,
		},
		{
			name: "multiple outages - highest severity wins",
			outages: []types.Outage{
				{Severity: types.SeveritySuspected},
				{Severity: types.SeverityDown},
				{Severity: types.SeverityDegraded},
			},
			expected: types.StatusDown,
		},
		{
			name: "multiple outages - degraded highest",
			outages: []types.Outage{
				{Severity: types.SeveritySuspected},
				{Severity: types.SeverityDegraded},
				{Severity: types.SeveritySuspected},
			},
			expected: types.StatusDegraded,
		},
		{
			name: "all same severity",
			outages: []types.Outage{
				{Severity: types.SeveritySuspected},
				{Severity: types.SeveritySuspected},
			},
			expected: types.StatusSuspected,
		},
		{
			name:     "empty outages slice",
			outages:  []types.Outage{},
			expected: types.StatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatusFromSeverity(tt.outages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetComponent(t *testing.T) {
	tests := []struct {
		name           string
		components     []*types.Component
		componentName  string
		expectedResult *types.Component
	}{
		{
			name: "component found - single component (slug)",
			components: []*types.Component{
				{Name: "Prow", Slug: "prow", Description: "CI/CD system"},
			},
			componentName:  "prow",
			expectedResult: &types.Component{Name: "Prow", Slug: "prow", Description: "CI/CD system"},
		},
		{
			name: "component found - multiple components (slug)",
			components: []*types.Component{
				{Name: "Prow", Slug: "prow", Description: "CI/CD system"},
				{Name: "Deck", Slug: "deck", Description: "Dashboard"},
				{Name: "Tide", Slug: "tide", Description: "Merge bot"},
			},
			componentName:  "deck",
			expectedResult: &types.Component{Name: "Deck", Slug: "deck", Description: "Dashboard"},
		},
		{
			name: "component not found",
			components: []*types.Component{
				{Name: "Prow", Slug: "prow", Description: "CI/CD system"},
				{Name: "Deck", Slug: "deck", Description: "Dashboard"},
			},
			componentName:  "nonexistent",
			expectedResult: nil,
		},
		{
			name:           "empty components list",
			components:     []*types.Component{},
			componentName:  "any-component",
			expectedResult: nil,
		},
		{
			name: "non-slug name is not accepted",
			components: []*types.Component{
				{Name: "Prow", Slug: "prow", Description: "CI/CD system"},
			},
			componentName:  "Prow", // name, not slug
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := &Handlers{
				config: &types.Config{Components: tt.components},
			}

			result := handlers.getComponent(tt.componentName)

			if tt.expectedResult == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				if diff := cmp.Diff(*tt.expectedResult, *result); diff != "" {
					t.Errorf("getComponent mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
