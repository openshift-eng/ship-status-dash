//nolint:errcheck,unparam // Test helpers - error handling and unused parameters are acceptable in test code
package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"

	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_ComponentMonitor(t *testing.T) {
	serverURL := os.Getenv("TEST_SERVER_URL")
	if serverURL == "" {
		t.Fatalf("TEST_SERVER_URL is not set")
	}
	mockOauthProxyURL := os.Getenv("TEST_MOCK_OAUTH_PROXY_URL")
	if mockOauthProxyURL == "" {
		t.Fatalf("TEST_MOCK_OAUTH_PROXY_URL is not set")
	}
	mockMonitoredComponentURL := os.Getenv("TEST_MOCK_MONITORED_COMPONENT_URL")
	if mockMonitoredComponentURL == "" {
		t.Fatalf("TEST_MOCK_MONITORED_COMPONENT_URL is not set")
	}

	client, err := NewTestHTTPClient(serverURL, mockOauthProxyURL)
	require.NoError(t, err)

	t.Run("HTTPComponentMonitorProbe", testHTTPComponentMonitorProbe(client, mockMonitoredComponentURL))
}

func testHTTPComponentMonitorProbe(client *TestHTTPClient, mockMonitoredComponentURL string) func(*testing.T) {
	return func(t *testing.T) {
		// Ensure service starts in healthy state
		resp, err := http.Get(mockMonitoredComponentURL + "/health")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Verify no active outages exist initially
		outages := getOutages(t, client, "Sippy", "Sippy")
		activeOutages := filterActiveOutages(outages)
		assert.Empty(t, activeOutages, "Should have no active outages initially")

		// Bring service down
		resp, err = http.Get(mockMonitoredComponentURL + "/down")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Wait for component-monitor to detect the failure
		// Frequency is 10s, retry_after is 2s, so wait at least 15s to be safe
		time.Sleep(15 * time.Second)

		// Verify outage was created
		outages = getOutages(t, client, "Sippy", "Sippy")
		activeOutages = filterActiveOutages(outages)
		require.NotEmpty(t, activeOutages, "Should have an active outage after service goes down")

		var foundOutage *types.Outage
		for i := range activeOutages {
			if activeOutages[i].DiscoveredFrom == "component-monitor" &&
				activeOutages[i].ComponentName == utils.Slugify("Sippy") &&
				activeOutages[i].SubComponentName == utils.Slugify("Sippy") {
				foundOutage = &activeOutages[i]
				break
			}
		}
		require.NotNil(t, foundOutage, "Should find outage created by component-monitor")

		// Verify outage properties
		assert.Equal(t, string(types.SeverityDown), string(foundOutage.Severity))
		assert.Equal(t, "component-monitor", foundOutage.DiscoveredFrom)
		assert.Equal(t, "e2e-component-monitor", foundOutage.CreatedBy)
		require.NotEmpty(t, foundOutage.Reasons, "Outage should have reasons")
		assert.Equal(t, types.CheckTypeHTTP, foundOutage.Reasons[0].Type)
		assert.Contains(t, foundOutage.Reasons[0].Check, "http://localhost:9000/health")
		assert.False(t, foundOutage.EndTime.Valid, "Outage should not be resolved yet")

		// Bring service back up
		resp, err = http.Get(mockMonitoredComponentURL + "/up")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Wait for component-monitor to detect the recovery
		// Frequency is 10s, retry_after is 2s, so wait at least 15s to be safe
		time.Sleep(15 * time.Second)

		// Verify outage was resolved
		outages = getOutages(t, client, "Sippy", "Sippy")
		activeOutages = filterActiveOutages(outages)
		assert.Empty(t, activeOutages, "Should have no active outages after service recovers")

		// Verify the previously found outage is now resolved
		// Get all outages and find the resolved one
		outages = getOutages(t, client, "Sippy", "Sippy")
		var resolvedOutage *types.Outage
		for i := range outages {
			if outages[i].ID == foundOutage.ID {
				resolvedOutage = &outages[i]
				break
			}
		}
		require.NotNil(t, resolvedOutage, "Should find the previously created outage")

		assert.True(t, resolvedOutage.EndTime.Valid, "Outage should be resolved with EndTime set")
		assert.NotNil(t, resolvedOutage.ResolvedBy, "Outage should have ResolvedBy set")
		assert.Equal(t, "e2e-component-monitor", *resolvedOutage.ResolvedBy)
	}
}

// filterActiveOutages filters outages to only return those that are currently active (not resolved)
// TODO: at some point we should have a filter for the endpoint that only returns active outages
func filterActiveOutages(outages []types.Outage) []types.Outage {
	var active []types.Outage
	for i := range outages {
		if !outages[i].EndTime.Valid {
			active = append(active, outages[i])
		}
	}
	return active
}
