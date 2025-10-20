package types

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestComponent_GetSubComponentBySlug(t *testing.T) {
	tests := []struct {
		name             string
		component        Component
		subComponentName string
		expectedResult   *SubComponent
	}{
		{
			name: "subcomponent found - single subcomponent",
			component: Component{
				Name: "Prow",
				Subcomponents: []SubComponent{
					{Name: "Tide", Slug: "tide", Description: "Merge bot"},
				},
			},
			subComponentName: "tide",
			expectedResult:   &SubComponent{Name: "Tide", Slug: "tide", Description: "Merge bot"},
		},
		{
			name: "subcomponent found - multiple subcomponents",
			component: Component{
				Name: "Prow",
				Subcomponents: []SubComponent{
					{Name: "Tide", Slug: "tide", Description: "Merge bot"},
					{Name: "Deck", Slug: "deck", Description: "Dashboard"},
					{Name: "Hook", Slug: "hook", Description: "Webhook handler"},
				},
			},
			subComponentName: "deck",
			expectedResult:   &SubComponent{Name: "Deck", Slug: "deck", Description: "Dashboard"},
		},
		{
			name: "subcomponent not found",
			component: Component{
				Name: "Prow",
				Subcomponents: []SubComponent{
					{Name: "Tide", Slug: "tide", Description: "Merge bot"},
					{Name: "Deck", Slug: "deck", Description: "Dashboard"},
				},
			},
			subComponentName: "nonexistent",
			expectedResult:   nil,
		},
		{
			name: "empty subcomponents list",
			component: Component{
				Name:          "Prow",
				Subcomponents: []SubComponent{},
			},
			subComponentName: "any-sub-component",
			expectedResult:   nil,
		},
		{
			name: "case sensitive matching",
			component: Component{
				Name: "Prow",
				Subcomponents: []SubComponent{
					{Name: "Tide", Slug: "tide", Description: "Merge bot"},
				},
			},
			subComponentName: "Tide", // name, not slug
			expectedResult:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.component.GetSubComponentBySlug(tt.subComponentName)

			if tt.expectedResult == nil {
				assert.Nil(t, result)
			} else {
				if result == nil {
					t.Fatalf("got nil result, want non-nil: %+v", tt.expectedResult)
				}
				if diff := cmp.Diff(*tt.expectedResult, *result); diff != "" {
					t.Errorf("GetSubComponentBySlug mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
