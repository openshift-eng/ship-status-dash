//nolint:errcheck,unparam // Test helpers - error handling and unused parameters are acceptable in test code
package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	prowComponentName = "Prow"
)

func TestE2E_Dashboard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		t.Fatalf("TEST_SERVER_URL is not set")
	}
	mockOauthProxyURL := os.Getenv("TEST_MOCK_OAUTH_PROXY_URL")
	if mockOauthProxyURL == "" {
		t.Fatalf("TEST_MOCK_OAUTH_PROXY_URL is not set")
	}
	client, err := NewTestHTTPClient(serverURL, mockOauthProxyURL)
	require.NoError(t, err)

	t.Run("Health", testHealth(client))
	t.Run("Components", testComponents(client))
	t.Run("ComponentInfo", testComponentInfo(client))
	t.Run("Outages", testOutages(client))
	t.Run("UpdateOutage", testUpdateOutage(client))
	t.Run("DeleteOutage", testDeleteOutage(client))
	t.Run("GetOutage", testGetOutage(client))
	t.Run("SubComponentStatus", testSubComponentStatus(client))
	t.Run("ComponentStatus", testComponentStatus(client))
	t.Run("AllComponentsStatus", testAllComponentsStatus(client))
	t.Run("User", testUser(client))
}

func testHealth(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		resp, err := client.Get("/health", false)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)

		assert.Equal(t, "ok", health["status"])
		assert.NotEmpty(t, health["time"])
	}
}

func testComponents(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		components := getComponents(t, client)

		assert.Len(t, components, 2)
		assert.Equal(t, "Prow", components[0].Name)
		assert.Equal(t, "Backbone of the CI system", components[0].Description)
		assert.Equal(t, "TestPlatform", components[0].ShipTeam)
		assert.Equal(t, "#test-channel", components[0].SlackChannel)
		assert.Len(t, components[0].Subcomponents, 2)
		assert.Equal(t, "Tide", components[0].Subcomponents[0].Name)
		assert.Equal(t, "Deck", components[0].Subcomponents[1].Name)

		assert.Equal(t, "Build Farm", components[1].Name)
		assert.Equal(t, "Where the CI jobs are run", components[1].Description)
		assert.Equal(t, "DPTP", components[1].ShipTeam)
		assert.Equal(t, "#ops-testplatform", components[1].SlackChannel)
		assert.Len(t, components[1].Subcomponents, 2)
		assert.Equal(t, "Build01", components[1].Subcomponents[0].Name)
		assert.Equal(t, "Build02", components[1].Subcomponents[1].Name)
	}
}

func testComponentInfo(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET component info for existing component returns component details", func(t *testing.T) {
			component := getComponent(t, client, "Prow")

			assert.Equal(t, "Prow", component.Name)
			assert.Equal(t, "Backbone of the CI system", component.Description)
			assert.Equal(t, "TestPlatform", component.ShipTeam)
			assert.Equal(t, "#test-channel", component.SlackChannel)
			assert.Len(t, component.Subcomponents, 2)
			assert.Equal(t, "Tide", component.Subcomponents[0].Name)
			assert.Equal(t, "Deck", component.Subcomponents[1].Name)
		})

		t.Run("GET component info for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, "/api/components/"+utils.Slugify("NonExistentComponent"), false)
		})
	}
}

// createOutage is a helper function to create an outage for testing
func createOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) types.Outage {
	outagePayload := map[string]interface{}{
		"severity":        string(types.SeverityDown),
		"start_time":      time.Now().UTC().Format(time.RFC3339),
		"description":     "Test outage for " + subComponentName,
		"discovered_from": "e2e-test",
		"created_by":      "developer",
	}

	payloadBytes, err := json.Marshal(outagePayload)
	require.NoError(t, err)

	resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify(componentName), utils.Slugify(subComponentName)), payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var outage types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outage)
	require.NoError(t, err)

	// Verify that created_by is set to the user from X-Forwarded-User header
	assert.Equal(t, "developer", outage.CreatedBy, "created_by should be set to the user from X-Forwarded-User header")

	return outage
}

// deleteOutage is a helper function to delete an outage for cleanup
func deleteOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string, outageID uint) {
	resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify(componentName), utils.Slugify(subComponentName), outageID))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

// updateOutage is a helper function to update an outage
func updateOutage(t *testing.T, client *TestHTTPClient, componentName, subComponentName string, outageID uint, payload map[string]interface{}) {
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	updateURL := fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify(componentName), utils.Slugify(subComponentName), outageID)
	resp, err := client.Patch(updateURL, payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func testOutages(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("POST to sub-component succeeds", func(t *testing.T) {
			outage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", outage.ID)

			assert.NotZero(t, outage.ID)
			assert.Equal(t, utils.Slugify("Prow"), outage.ComponentName)
			assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			assert.Equal(t, "e2e-test", outage.DiscoveredFrom)
		})

		t.Run("POST to non-existent sub-component fails", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        string(types.SeverityDown),
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage for non-existent sub-component",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), payloadBytes)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("POST with invalid severity fails", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        "InvalidSeverity",
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage with invalid severity",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("Deck")), payloadBytes)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

			var errorResponse map[string]string
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			require.NoError(t, err)
			assert.Contains(t, errorResponse["error"], "Invalid severity")
		})

		t.Run("GET on top-level component aggregates sub-components", func(t *testing.T) {
			// Create outages for different sub-components
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			outages := getOutages(t, client, "Prow", "")

			// Should have exactly our 2 outages since we clean up after ourselves
			assert.Len(t, outages, 2)

			// Verify our specific outages are present
			outageIDs := make(map[uint]bool)
			for _, outage := range outages {
				outageIDs[outage.ID] = true
			}
			assert.True(t, outageIDs[tideOutage.ID], "Tide outage should be present")
			assert.True(t, outageIDs[deckOutage.ID], "Deck outage should be present")
		})

		t.Run("GET on sub-component returns only that sub-component's outages", func(t *testing.T) {
			// Create outages for different sub-components
			tideOutage1 := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage1.ID)
			tideOutage2 := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage2.ID)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			outages := getOutages(t, client, "Prow", "Tide")

			// Should have exactly our 2 Tide outages since we clean up after ourselves
			assert.Len(t, outages, 2)

			// All outages should be for Tide only
			for _, outage := range outages {
				assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			}

			// Verify our specific outages are present
			outageIDs := make(map[uint]bool)
			for _, outage := range outages {
				outageIDs[outage.ID] = true
			}
			assert.True(t, outageIDs[tideOutage1.ID], "First Tide outage should be present")
			assert.True(t, outageIDs[tideOutage2.ID], "Second Tide outage should be present")
			assert.False(t, outageIDs[deckOutage.ID], "Deck outage should not be included")
		})

		t.Run("GET on non-existent sub-component fails", func(t *testing.T) {
			// This test doesn't need any setup - it should fail regardless of existing data
			expect404(t, client, fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), false)
		})

		t.Run("POST to unauthorized component returns 403", func(t *testing.T) {
			outagePayload := map[string]interface{}{
				"severity":        string(types.SeverityDown),
				"start_time":      time.Now().UTC().Format(time.RFC3339),
				"description":     "Test outage for unauthorized component",
				"discovered_from": "e2e-test",
				"created_by":      "developer",
			}

			payloadBytes, err := json.Marshal(outagePayload)
			require.NoError(t, err)

			expect403(t, client, "POST", fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Build Farm"), utils.Slugify("Build01")), payloadBytes)
		})
	}
}

func testUpdateOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		// Create an outage to update
		createdOutage := createOutage(t, client, "Prow", "Tide")
		defer deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

		// Now update the outage
		updatePayload := map[string]interface{}{
			"severity":     string(types.SeverityDegraded),
			"description":  "Updated description",
			"resolved_by":  "test-resolver",
			"triage_notes": "Updated triage notes",
		}

		updateBytes, err := json.Marshal(updatePayload)
		require.NoError(t, err)

		updateURL := fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID)
		t.Logf("Making PATCH request to: %s", updateURL)

		updateResp, err := client.Patch(updateURL, updateBytes)
		require.NoError(t, err)
		defer updateResp.Body.Close()

		if updateResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(updateResp.Body)
			t.Logf("Unexpected status %d, body: %s", updateResp.StatusCode, string(body))
		}

		assert.Equal(t, http.StatusOK, updateResp.StatusCode)
		assert.Equal(t, "application/json", updateResp.Header.Get("Content-Type"))

		var updatedOutage types.Outage
		err = json.NewDecoder(updateResp.Body).Decode(&updatedOutage)
		require.NoError(t, err)

		assert.Equal(t, createdOutage.ID, updatedOutage.ID)
		assert.Equal(t, string(types.SeverityDegraded), string(updatedOutage.Severity))
		assert.Equal(t, "Updated description", updatedOutage.Description)
		assert.Equal(t, "test-resolver", *updatedOutage.ResolvedBy)
		assert.Equal(t, "Updated triage notes", *updatedOutage.TriageNotes)
		assert.WithinDuration(t, createdOutage.StartTime.UTC(), updatedOutage.StartTime.UTC(), time.Second) // Should remain unchanged
		assert.Equal(t, createdOutage.CreatedBy, updatedOutage.CreatedBy)                                   // Should remain unchanged

		// Test updating non-existent outage
		nonExistentResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")), updateBytes)
		require.NoError(t, err)
		defer nonExistentResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, nonExistentResp.StatusCode)

		// Test updating with invalid component
		invalidComponentResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide"), createdOutage.ID), updateBytes)
		require.NoError(t, err)
		defer invalidComponentResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, invalidComponentResp.StatusCode)

		// Test updating with invalid severity
		invalidSeverityUpdate := map[string]interface{}{
			"severity": "InvalidSeverity",
		}
		invalidSeverityBytes, err := json.Marshal(invalidSeverityUpdate)
		require.NoError(t, err)

		invalidSeverityResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), invalidSeverityBytes)
		require.NoError(t, err)
		defer invalidSeverityResp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, invalidSeverityResp.StatusCode)

		var errorResponse map[string]string
		err = json.NewDecoder(invalidSeverityResp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Contains(t, errorResponse["error"], "Invalid severity")

		// Test confirming an outage
		confirmPayload := map[string]interface{}{
			"confirmed": true,
		}
		confirmBytes, err := json.Marshal(confirmPayload)
		require.NoError(t, err)

		confirmResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), confirmBytes)
		require.NoError(t, err)
		defer confirmResp.Body.Close()

		assert.Equal(t, http.StatusOK, confirmResp.StatusCode)

		var confirmedOutage types.Outage
		err = json.NewDecoder(confirmResp.Body).Decode(&confirmedOutage)
		require.NoError(t, err)

		// Verify that confirmed_by is set to the user from X-Forwarded-User header
		assert.NotNil(t, confirmedOutage.ConfirmedBy, "confirmed_by should be set when confirmed is true")
		assert.Equal(t, "developer", *confirmedOutage.ConfirmedBy, "confirmed_by should be set to the user from X-Forwarded-User header")
		assert.True(t, confirmedOutage.ConfirmedAt.Valid, "confirmed_at should be set when confirmed is true")

		t.Run("PATCH to unauthorized component returns 403", func(t *testing.T) {
			updatePayload := map[string]interface{}{
				"severity": string(types.SeverityDegraded),
			}

			updateBytes, err := json.Marshal(updatePayload)
			require.NoError(t, err)

			expect403(t, client, "PATCH", fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Build Farm"), utils.Slugify("Build01")), updateBytes)
		})
	}
}

func testDeleteOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("DELETE existing outage succeeds", func(t *testing.T) {
			// Create an outage to delete
			createdOutage := createOutage(t, client, "Prow", "Tide")

			// Delete the outage
			deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			// Verify the outage is deleted by trying to get it
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify("Prow"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			var outages []types.Outage
			err = json.NewDecoder(resp.Body).Decode(&outages)
			require.NoError(t, err)

			// The deleted outage should not be in the list
			for _, outage := range outages {
				assert.NotEqual(t, createdOutage.ID, outage.ID, "Deleted outage should not be present")
			}
		})

		t.Run("DELETE non-existent outage returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from non-existent component returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from non-existent sub-component returns 404", func(t *testing.T) {
			resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("DELETE outage from unauthorized component returns 403", func(t *testing.T) {
			expect403(t, client, "DELETE", fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Build Farm"), utils.Slugify("Build01")), nil)
		})
	}
}

func testGetOutage(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET existing outage succeeds", func(t *testing.T) {
			// Create an outage to retrieve
			createdOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", createdOutage.ID)

			// Get the outage
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Tide"), createdOutage.ID), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			var outage types.Outage
			err = json.NewDecoder(resp.Body).Decode(&outage)
			require.NoError(t, err)

			assert.Equal(t, createdOutage.ID, outage.ID)
			assert.Equal(t, utils.Slugify("Tide"), outage.SubComponentName)
			assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			assert.Equal(t, "e2e-test", outage.DiscoveredFrom)
			assert.Equal(t, "developer", outage.CreatedBy)
		})

		t.Run("GET non-existent outage returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/99999", utils.Slugify("Prow"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage from non-existent component returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("NonExistentComponent"), utils.Slugify("Tide")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage from non-existent sub-component returns 404", func(t *testing.T) {
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/1", utils.Slugify("Prow"), utils.Slugify("NonExistentSub")), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})

		t.Run("GET outage with wrong sub-component returns 404", func(t *testing.T) {
			// Create an outage for Tide
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			// Try to get it as if it were a Deck outage
			resp, err := client.Get(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Deck"), tideOutage.ID), false)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		})
	}
}

func testSubComponentStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for healthy sub-component returns Healthy", func(t *testing.T) {
			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusHealthy, status.Status)
			assert.Empty(t, status.ActiveOutages)
		})

		t.Run("GET status for sub-component with active outage returns outage severity", func(t *testing.T) {
			// Create an outage for Deck (should be auto-confirmed)
			outage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", outage.ID)

			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDown), string(status.ActiveOutages[0].Severity))
		})

		t.Run("GET status for sub-component with multiple outages returns most critical", func(t *testing.T) {
			// Create a Degraded outage for Tide
			degradedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", degradedOutage.ID)

			// Create a Down outage for Tide
			downOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", downOutage.ID)

			// Confirm both outages to test most critical logic
			updateOutage(t, client, "Prow", "Tide", degradedOutage.ID, map[string]interface{}{
				"confirmed": true,
			})
			updateOutage(t, client, "Prow", "Tide", downOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "Tide")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
		})

		t.Run("GET status for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, fmt.Sprintf("/api/status/%s/%s", utils.Slugify("NonExistent"), utils.Slugify("Deck")), false)
		})

		t.Run("GET status for non-existent sub-component returns 404", func(t *testing.T) {
			expect404(t, client, fmt.Sprintf("/api/status/%s/%s", utils.Slugify("Prow"), utils.Slugify("NonExistent")), false)
		})

		t.Run("GET status for sub-component with future end_time still considers outage active", func(t *testing.T) {
			// Create an outage first (should be auto-confirmed)
			outage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", outage.ID)

			// Update the outage to have a future end_time
			futureTime := time.Now().Add(24 * time.Hour) // 24 hours in the future
			updatePayload := map[string]interface{}{
				"end_time": map[string]interface{}{
					"Time":  futureTime.UTC().Format(time.RFC3339),
					"Valid": true,
				},
			}

			updateBytes, err := json.Marshal(updatePayload)
			require.NoError(t, err)

			updateResp, err := client.Patch(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify("Prow"), utils.Slugify("Deck"), outage.ID), updateBytes)
			require.NoError(t, err)
			defer updateResp.Body.Close()

			assert.Equal(t, http.StatusOK, updateResp.StatusCode)

			// Verify that resolved_by is set to the user from X-Forwarded-User header when end_time is set
			var updatedOutage types.Outage
			err = json.NewDecoder(updateResp.Body).Decode(&updatedOutage)
			require.NoError(t, err)
			assert.NotNil(t, updatedOutage.ResolvedBy, "resolved_by should be set when end_time is provided")
			assert.Equal(t, "developer", *updatedOutage.ResolvedBy, "resolved_by should be set to the user from X-Forwarded-User header")

			// Check that the status endpoint still considers this outage active
			status := getStatus(t, client, "Prow", "Deck")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, outage.ID, status.ActiveOutages[0].ID)
		})

		t.Run("GET status for sub-component with unconfirmed outage returns Suspected", func(t *testing.T) {
			// Create an unconfirmed outage for Tide (which has requires_confirmation: true)
			outage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", outage.ID)

			status := getStatus(t, client, "Prow", "Tide")

			assert.Equal(t, types.StatusSuspected, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.False(t, status.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for sub-component with mixed confirmed/unconfirmed outages returns confirmed severity", func(t *testing.T) {
			// Create confirmed degraded outage
			confirmedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", confirmedOutage.ID)

			// Confirm the degraded outage
			updateOutage(t, client, "Prow", "Tide", confirmedOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			// Create unconfirmed down outage
			unconfirmedOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", unconfirmedOutage.ID)

			status := getStatus(t, client, "Prow", "Tide")

			// Should return Degraded (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDegraded, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
		})
	}
}

func testComponentStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for healthy component returns Healthy", func(t *testing.T) {
			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusHealthy, status.Status)
			assert.Empty(t, status.ActiveOutages)
		})

		t.Run("GET status for component with one degraded sub-component returns Partial", func(t *testing.T) {
			// Create a degraded outage for Deck (doesn't require confirmation)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusPartial, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDegraded), string(status.ActiveOutages[0].Severity))
		})

		t.Run("GET status for component with all sub-components down returns Down", func(t *testing.T) {
			// Create Down outages for both sub-components
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			// Confirm Tide outage (Deck should be auto-confirmed)
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
			for _, outage := range status.ActiveOutages {
				assert.Equal(t, string(types.SeverityDown), string(outage.Severity))
			}
		})

		t.Run("GET status for component with mixed severity outages returns most severe", func(t *testing.T) {
			// Create outages with different severities
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			// Confirm the Tide outage to test most severe logic
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
			// Verify we have both severities present
			severities := make(map[string]bool)
			for _, outage := range status.ActiveOutages {
				severities[string(outage.Severity)] = true
			}
			assert.True(t, severities[string(types.SeverityDown)])
			assert.True(t, severities[string(types.SeverityDegraded)])
		})

		t.Run("GET status for component with unconfirmed outages on one sub-component returns Partial", func(t *testing.T) {
			// Create unconfirmed outages for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			status := getStatus(t, client, "Prow", "")

			assert.Equal(t, types.StatusPartial, status.Status)
			assert.Len(t, status.ActiveOutages, 1)
			assert.False(t, status.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for component with mixed confirmed/unconfirmed outages shows confirmed severity", func(t *testing.T) {
			// Create outage for Deck (should be auto-confirmed)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			status := getStatus(t, client, "Prow", "")

			// Should return Down (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDown, status.Status)
			assert.Len(t, status.ActiveOutages, 2)
		})

		t.Run("GET status for non-existent component returns 404", func(t *testing.T) {
			expect404(t, client, "/api/status/"+utils.Slugify("NonExistent"), false)
		})
	}
}

func createOutageWithSeverity(t *testing.T, client *TestHTTPClient, componentName, subComponentName, severity string) types.Outage {
	outagePayload := map[string]interface{}{
		"severity":        severity,
		"start_time":      time.Now().UTC().Format(time.RFC3339),
		"description":     fmt.Sprintf("Test outage with %s severity", severity),
		"discovered_from": "e2e-test",
		"created_by":      "developer",
	}

	payloadBytes, err := json.Marshal(outagePayload)
	require.NoError(t, err)

	resp, err := client.Post(fmt.Sprintf("/api/components/%s/%s/outages", utils.Slugify(componentName), utils.Slugify(subComponentName)), payloadBytes)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var outage types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outage)
	require.NoError(t, err)

	// Verify that created_by is set to the user from X-Forwarded-User header
	assert.Equal(t, "developer", outage.CreatedBy, "created_by should be set to the user from X-Forwarded-User header")

	return outage
}
func testAllComponentsStatus(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET status for all components returns all components with their status", func(t *testing.T) {
			allStatuses := getAllComponentsStatus(t, client)

			// Should have exactly 2 components (Prow and Build Farm) based on test config
			assert.Len(t, allStatuses, 2)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			var buildFarmStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
				}
				if allStatuses[i].ComponentName == "Build Farm" {
					buildFarmStatus = &allStatuses[i]
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			require.NotNil(t, buildFarmStatus, "Build Farm component should be present")
			assert.Equal(t, types.StatusHealthy, prowStatus.Status)
			assert.Empty(t, prowStatus.ActiveOutages)
			assert.Equal(t, types.StatusHealthy, buildFarmStatus.Status)
			assert.Empty(t, buildFarmStatus.ActiveOutages)
		})

		t.Run("GET status for all components with outages shows correct statuses", func(t *testing.T) {
			// Create outages for different sub-components
			tideOutage := createOutageWithSeverity(t, client, "Prow", "Tide", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDown))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			// Confirm Tide outage (Deck should be auto-confirmed)
			updateOutage(t, client, "Prow", "Tide", tideOutage.ID, map[string]interface{}{
				"confirmed": true,
			})

			allStatuses := getAllComponentsStatus(t, client)

			// Should have exactly 2 components (Prow and Build Farm)
			assert.Len(t, allStatuses, 2)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusDown, prowStatus.Status) // Most severe status
			assert.Len(t, prowStatus.ActiveOutages, 2)

			// Verify we have both severities present
			severities := make(map[string]bool)
			for _, outage := range prowStatus.ActiveOutages {
				severities[string(outage.Severity)] = true
			}
			assert.True(t, severities[string(types.SeverityDown)])
			assert.True(t, severities[string(types.SeverityDegraded)])
		})

		t.Run("GET status for all components with partial outages shows Partial status", func(t *testing.T) {
			// Create outage for only one sub-component (Deck doesn't require confirmation)
			deckOutage := createOutageWithSeverity(t, client, "Prow", "Deck", string(types.SeverityDegraded))
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			// Should have exactly 2 components (Prow and Build Farm)
			assert.Len(t, allStatuses, 2)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusPartial, prowStatus.Status) // Only one sub-component affected
			assert.Len(t, prowStatus.ActiveOutages, 1)
			assert.Equal(t, string(types.SeverityDegraded), string(prowStatus.ActiveOutages[0].Severity))
		})

		t.Run("GET status for all components with unconfirmed outages shows Partial", func(t *testing.T) {
			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			// Should have exactly 2 components (Prow and Build Farm)
			assert.Len(t, allStatuses, 2)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			assert.Equal(t, types.StatusPartial, prowStatus.Status)
			assert.Len(t, prowStatus.ActiveOutages, 1)
			assert.False(t, prowStatus.ActiveOutages[0].ConfirmedAt.Valid)
		})

		t.Run("GET status for all components with mixed confirmed/unconfirmed outages shows confirmed severity", func(t *testing.T) {
			// Create outage for Deck (should be auto-confirmed)
			deckOutage := createOutage(t, client, "Prow", "Deck")
			defer deleteOutage(t, client, "Prow", "Deck", deckOutage.ID)

			// Create unconfirmed outage for Tide (requires_confirmation: true)
			tideOutage := createOutage(t, client, "Prow", "Tide")
			defer deleteOutage(t, client, "Prow", "Tide", tideOutage.ID)

			allStatuses := getAllComponentsStatus(t, client)

			// Should have exactly 2 components (Prow and Build Farm)
			assert.Len(t, allStatuses, 2)
			// Find Prow component
			var prowStatus *types.ComponentStatus
			for i := range allStatuses {
				if allStatuses[i].ComponentName == prowComponentName {
					prowStatus = &allStatuses[i]
					break
				}
			}
			require.NotNil(t, prowStatus, "Prow component should be present")
			assert.Equal(t, "Prow", prowStatus.ComponentName)
			// Should return Down (confirmed) not Suspected (unconfirmed)
			assert.Equal(t, types.StatusDown, prowStatus.Status)
			assert.Len(t, prowStatus.ActiveOutages, 2)
		})
	}
}

func testUser(client *TestHTTPClient) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("GET /api/user returns authenticated user", func(t *testing.T) {
			resp, err := client.Get("/api/user", true)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			var userResponse struct {
				Username   string   `json:"username"`
				Components []string `json:"components"`
			}
			err = json.NewDecoder(resp.Body).Decode(&userResponse)
			require.NoError(t, err)

			assert.Equal(t, "developer", userResponse.Username)
			// Components should be a slice (can be empty)
			assert.NotNil(t, userResponse.Components)
			// Developer should only have access to Prow, not Build Farm
			assert.Contains(t, userResponse.Components, utils.Slugify("Prow"), "developer should have access to Prow")
			assert.NotContains(t, userResponse.Components, utils.Slugify("Build Farm"), "developer should not have access to Build Farm")
		})
	}
}
