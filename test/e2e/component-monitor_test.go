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
	t.Run("PrometheusComponentMonitorProbe_SingleQuery", testPrometheusComponentMonitorProbe_SingleQuery(client, mockMonitoredComponentURL, prometheusURL))
	t.Run("PrometheusComponentMonitorProbe_MultipleQueries", testPrometheusComponentMonitorProbe_MultipleQueries(client, mockMonitoredComponentURL, prometheusURL))
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
		require.Equal(t, len(foundOutage.Reasons), 1, "Outage should have one reason")
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

func testPrometheusComponentMonitorProbe_SingleQuery(client *TestHTTPClient, mockMonitoredComponentURL, prometheusURL string) func(*testing.T) {
	return func(t *testing.T) {
		componentName := "Sippy"
		subComponentName := "api"
		expectedQueryCount := 1

		// Clean up any existing active outages from previous test runs
		cleanupActiveOutages(t, client, componentName, subComponentName)

		healthyMetrics := map[string]interface{}{
			"success_rate": 1.0, // >= 0.9, so query succeeds
		}
		unhealthyMetrics := map[string]interface{}{
			"success_rate": 0.5, // < 0.9, so query returns empty vector (fails)
		}

		setupPrometheusTestMetrics(t, mockMonitoredComponentURL, healthyMetrics)
		verifyNoActiveOutages(t, client, componentName, subComponentName, "initially")
		waitAndVerifySuccessfulProbe(t, client, componentName, subComponentName)

		setUnhealthyMetricsAndWait(t, mockMonitoredComponentURL, unhealthyMetrics)
		foundOutage := verifyOutageCreated(t, client, componentName, subComponentName)
		verifyOutageReasons(t, foundOutage, expectedQueryCount)

		restoreMetricsAndVerifyRecovery(t, mockMonitoredComponentURL, healthyMetrics, client, componentName, subComponentName)
	}
}

func testPrometheusComponentMonitorProbe_MultipleQueries(client *TestHTTPClient, mockMonitoredComponentURL, prometheusURL string) func(*testing.T) {
	return func(t *testing.T) {
		componentName := "Sippy"
		subComponentName := "data-load"
		expectedQueryCount := 3

		// Clean up any existing active outages from previous test runs
		cleanupActiveOutages(t, client, componentName, subComponentName)

		healthyMetrics := map[string]interface{}{
			"data_load_failure": map[string]float64{
				"api":   0.0, // < 1, so query succeeds
				"db":    0.0, // < 1, so query succeeds
				"cache": 0.0, // < 1, so query succeeds
			},
		}
		unhealthyMetrics := map[string]interface{}{
			"data_load_failure": map[string]float64{
				"api":   1.0, // >= 1, so query returns empty vector (fails)
				"db":    1.0, // >= 1, so query returns empty vector (fails)
				"cache": 1.0, // >= 1, so query returns empty vector (fails)
			},
		}

		setupPrometheusTestMetrics(t, mockMonitoredComponentURL, healthyMetrics)
		verifyNoActiveOutages(t, client, componentName, subComponentName, "initially")
		waitAndVerifySuccessfulProbe(t, client, componentName, subComponentName)

		setUnhealthyMetricsAndWait(t, mockMonitoredComponentURL, unhealthyMetrics)
		foundOutage := verifyOutageCreated(t, client, componentName, subComponentName)
		verifyOutageReasons(t, foundOutage, expectedQueryCount)

		restoreMetricsAndVerifyRecovery(t, mockMonitoredComponentURL, healthyMetrics, client, componentName, subComponentName)
	}
}

func setupPrometheusTestMetrics(t *testing.T, mockMonitoredComponentURL string, metrics map[string]interface{}) {
	updateMetrics(t, mockMonitoredComponentURL, metrics)
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

func verifyOutageReasons(t *testing.T, foundOutage *types.Outage, expectedQueryCount int) {
	require.Equal(t, expectedQueryCount, len(foundOutage.Reasons), "Outage should have exactly %d reasons (one per failed query)", expectedQueryCount)

	for i, reason := range foundOutage.Reasons {
		assert.Equal(t, types.CheckTypePrometheus, reason.Type, "Reason %d should be Prometheus type", i)
		assert.NotEmpty(t, reason.Check, "Reason %d should have a query", i)
		assert.NotEmpty(t, reason.Results, "Reason %d should have results", i)
	}
}

func restoreMetricsAndVerifyRecovery(t *testing.T, mockMonitoredComponentURL string, metrics map[string]interface{}, client *TestHTTPClient, componentName, subComponentName string) {
	updateMetrics(t, mockMonitoredComponentURL, metrics)
	time.Sleep(15 * time.Second) // Wait for component-monitor to detect recovery
	verifyNoActiveOutages(t, client, componentName, subComponentName, "after Prometheus queries recover")
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
