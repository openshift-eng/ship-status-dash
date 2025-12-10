//nolint:errcheck,unparam // Test helpers - error handling and unused parameters are acceptable in test code
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allHealthyMetrics = map[string]interface{}{
	"success_rate": 1.0, // >= 0.9, so query succeeds for api sub-component
	"data_load_failure": map[string]float64{
		"api":   0.0, // < 1, so query succeeds for data-load sub-component
		"db":    0.0, // < 1, so query succeeds
		"cache": 0.0, // < 1, so query succeeds
	},
}

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

	prometheusURL := os.Getenv("TEST_PROMETHEUS_URL")
	if prometheusURL == "" {
		t.Fatalf("TEST_PROMETHEUS_URL is not set")
	}

	t.Run("HTTPComponentMonitorProbe", testHTTPComponentMonitorProbe(client, mockMonitoredComponentURL))
	t.Run("PrometheusComponentMonitorProbe", testPrometheusComponentMonitorProbe(client, mockMonitoredComponentURL, prometheusURL))
}

func testHTTPComponentMonitorProbe(client *TestHTTPClient, mockMonitoredComponentURL string) func(*testing.T) {
	return func(t *testing.T) {
		// Clean up any existing outages first (component-monitor may have created them)
		cleanupActiveOutages(t, client, "Sippy", "Sippy")

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
		require.Equal(t, len(foundOutage.Reasons), 1, "Outage should have one reason")
		assert.Equal(t, types.CheckTypeHTTP, foundOutage.Reasons[0].Type)
		assert.Equal(t, foundOutage.Reasons[0].Type, types.CheckTypeHTTP)
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

func testPrometheusComponentMonitorProbe(client *TestHTTPClient, mockMonitoredComponentURL, prometheusURL string) func(*testing.T) {
	return func(t *testing.T) {
		componentName := "Sippy"

		// Clean up any existing outages first (component-monitor may have created them before Prometheus had metrics)
		cleanupActiveOutages(t, client, componentName, "api")
		cleanupActiveOutages(t, client, componentName, "data-load")

		// Set up all healthy metrics once at the beginning
		setHealthyMetricsAndWait(t, mockMonitoredComponentURL)

		// Test sub-component with single query (api)
		t.Run("SingleQuery", func(t *testing.T) {
			subComponentName := "api"
			expectedReasons := []types.Reason{
				{
					Type:    types.CheckTypePrometheus,
					Check:   "success_rate >= 0.9",
					Results: "0.5",
				},
			}

			cleanupActiveOutages(t, client, componentName, subComponentName)
			verifyNoActiveOutages(t, client, componentName, subComponentName, "initially")
			waitAndVerifySuccessfulProbe(t, client, componentName, subComponentName)

			unhealthyMetrics := map[string]interface{}{
				"success_rate": 0.5, // < 0.9, so query returns empty vector (fails)
			}
			setUnhealthyMetricsAndWait(t, mockMonitoredComponentURL, unhealthyMetrics)
			foundOutage := verifyOutageCreated(t, client, componentName, subComponentName)
			verifyOutageReasons(t, foundOutage, expectedReasons)

			restoreHealthyMetricsAndVerifyRecovery(t, mockMonitoredComponentURL, client, componentName, subComponentName, foundOutage.ID)
		})

		// Test sub-component with multiple queries (data-load)
		t.Run("MultipleQueries", func(t *testing.T) {
			subComponentName := "data-load"
			expectedReasons := []types.Reason{
				{
					Type:    types.CheckTypePrometheus,
					Check:   "data_load_failure{component=\"api\"} < 1",
					Results: "1",
				},
				{
					Type:    types.CheckTypePrometheus,
					Check:   "data_load_failure{component=\"db\"} < 1",
					Results: "1",
				},
				{
					Type:    types.CheckTypePrometheus,
					Check:   "data_load_failure{component=\"cache\"} < 1",
					Results: "1",
				},
			}

			cleanupActiveOutages(t, client, componentName, subComponentName)
			verifyNoActiveOutages(t, client, componentName, subComponentName, "initially")
			waitAndVerifySuccessfulProbe(t, client, componentName, subComponentName)

			unhealthyMetrics := map[string]interface{}{
				"data_load_failure": map[string]float64{
					"api":   1.0, // >= 1, so query returns empty vector (fails)
					"db":    1.0, // >= 1, so query returns empty vector (fails)
					"cache": 1.0, // >= 1, so query returns empty vector (fails)
				},
			}
			setUnhealthyMetricsAndWait(t, mockMonitoredComponentURL, unhealthyMetrics)
			foundOutage := verifyOutageCreated(t, client, componentName, subComponentName)
			verifyOutageReasons(t, foundOutage, expectedReasons)

			restoreHealthyMetricsAndVerifyRecovery(t, mockMonitoredComponentURL, client, componentName, subComponentName, foundOutage.ID)
		})
	}
}

func setHealthyMetricsAndWait(t *testing.T, mockMonitoredComponentURL string) {
	updateMetrics(t, mockMonitoredComponentURL, allHealthyMetrics)
	// Wait for Prometheus to scrape and component-monitor to process healthy state
	time.Sleep(10 * time.Second) // Wait for Prometheus to scrape
	time.Sleep(15 * time.Second) // Wait for component-monitor to detect healthy state
}

func verifyNoActiveOutages(t *testing.T, client *TestHTTPClient, componentName, subComponentName, context string) {
	outages := getOutages(t, client, componentName, subComponentName)
	activeOutages := filterActiveOutages(outages)
	assert.Empty(t, activeOutages, "Should have no active outages %s", context)
}

func waitAndVerifySuccessfulProbe(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) {
	// Already waited in the test, just verify
	verifyNoActiveOutages(t, client, componentName, subComponentName, "when Prometheus queries succeed")
}

func setUnhealthyMetricsAndWait(t *testing.T, mockMonitoredComponentURL string, metrics map[string]interface{}) {
	updateMetrics(t, mockMonitoredComponentURL, metrics)
	time.Sleep(10 * time.Second) // Wait for Prometheus to scrape
	time.Sleep(15 * time.Second) // Wait for component-monitor to detect failure
}

func verifyOutageCreated(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) *types.Outage {
	outages := getOutages(t, client, componentName, subComponentName)
	activeOutages := filterActiveOutages(outages)
	require.NotEmpty(t, activeOutages, "Should have an active outage after Prometheus queries fail")

	var foundOutage *types.Outage
	for i := range activeOutages {
		if activeOutages[i].DiscoveredFrom == "component-monitor" &&
			activeOutages[i].ComponentName == utils.Slugify(componentName) &&
			activeOutages[i].SubComponentName == utils.Slugify(subComponentName) {
			foundOutage = &activeOutages[i]
			break
		}
	}
	require.NotNil(t, foundOutage, "Should find outage created by component-monitor")

	assert.Equal(t, "component-monitor", foundOutage.DiscoveredFrom)
	assert.Equal(t, "e2e-component-monitor", foundOutage.CreatedBy)

	return foundOutage
}

func verifyOutageReasons(t *testing.T, foundOutage *types.Outage, expectedReasons []types.Reason) {
	require.Equal(t, len(expectedReasons), len(foundOutage.Reasons), "Outage should have exactly %d reasons (one per failed query)", len(expectedReasons))

	// Create a map to track which expected reasons have been matched
	matched := make(map[int]bool, len(expectedReasons))

	// For each actual reason, find a matching expected reason
	for i, reason := range foundOutage.Reasons {
		assert.Equal(t, types.CheckTypePrometheus, reason.Type, "Reason %d should be Prometheus type", i)
		assert.NotEmpty(t, reason.Check, "Reason %d should have a query", i)
		assert.NotEmpty(t, reason.Results, "Reason %d should have results", i)

		// Find a matching expected reason
		found := false
		for j, expected := range expectedReasons {
			if matched[j] {
				continue
			}
			if expected.Type == reason.Type && expected.Check == reason.Check && expected.Results == reason.Results {
				matched[j] = true
				found = true
				break
			}
		}
		assert.True(t, found, "Reason %d should match one of the expected reasons", i)
	}

	// Verify all expected reasons were matched
	for i, expected := range expectedReasons {
		assert.True(t, matched[i], "Expected reason %d (Type: %s, Check: %s, Results: %s) should be present", i, expected.Type, expected.Check, expected.Results)
	}
}

func restoreHealthyMetricsAndVerifyRecovery(t *testing.T, mockMonitoredComponentURL string, client *TestHTTPClient, componentName, subComponentName string, outageId uint) {
	updateMetrics(t, mockMonitoredComponentURL, allHealthyMetrics)
	time.Sleep(10 * time.Second) // Wait for Prometheus to scrape
	time.Sleep(15 * time.Second) // Wait for component-monitor to detect recovery

	verifyNoActiveOutages(t, client, componentName, subComponentName, "after Prometheus queries recover")

	// Verify the previously found outage is now resolved
	outages := getOutages(t, client, componentName, subComponentName)
	var resolvedOutage *types.Outage
	for i := range outages {
		if outages[i].ID == outageId {
			resolvedOutage = &outages[i]
			break
		}
	}
	require.NotNil(t, resolvedOutage, "Should find the previously created outage")
	assert.True(t, resolvedOutage.EndTime.Valid, "Outage should be resolved with EndTime set")
	require.NotNil(t, resolvedOutage.ResolvedBy, "Outage should have ResolvedBy set")
	assert.Equal(t, "e2e-component-monitor", *resolvedOutage.ResolvedBy)
}

func updateMetrics(t *testing.T, baseURL string, metrics map[string]interface{}) {
	jsonData, err := json.Marshal(metrics)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/update-metrics", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
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

// cleanupActiveOutages deletes any existing active outages for a component/subcomponent
func cleanupActiveOutages(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) {
	outages := getOutages(t, client, componentName, subComponentName)
	activeOutages := filterActiveOutages(outages)
	for _, outage := range activeOutages {
		resp, err := client.Delete(fmt.Sprintf("/api/components/%s/%s/outages/%d", utils.Slugify(componentName), utils.Slugify(subComponentName), outage.ID))
		if err == nil && resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
		}
	}
	// Wait a bit after cleanup to ensure the deletion is processed
	if len(activeOutages) > 0 {
		time.Sleep(2 * time.Second)
	}
}
