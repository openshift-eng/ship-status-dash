package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
)

// getStatus is a helper function to get component status and do basic assertions
func getStatus(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) types.ComponentStatus {
	url := fmt.Sprintf("/api/status/%s", utils.Slugify(componentName))
	if subComponentName != "" {
		url += fmt.Sprintf("/%s", utils.Slugify(subComponentName))
	}

	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var status types.ComponentStatus
	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	return status
}

// getComponents is a helper function to get all components and do basic assertions
func getComponents(t *testing.T, client *TestHTTPClient) []types.Component {
	resp, err := client.Get("/api/components")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var components []types.Component
	err = json.NewDecoder(resp.Body).Decode(&components)
	require.NoError(t, err)

	return components
}

// getComponent is a helper function to get a specific component and do basic assertions
func getComponent(t *testing.T, client *TestHTTPClient, componentName string) types.Component {
	resp, err := client.Get(fmt.Sprintf("/api/components/%s", utils.Slugify(componentName)))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var component types.Component
	err = json.NewDecoder(resp.Body).Decode(&component)
	require.NoError(t, err)

	return component
}

// getOutages is a helper function to get outages for a component or sub-component
func getOutages(t *testing.T, client *TestHTTPClient, componentName, subComponentName string) []types.Outage {
	url := fmt.Sprintf("/api/components/%s", utils.Slugify(componentName))
	if subComponentName != "" {
		url += fmt.Sprintf("/%s", utils.Slugify(subComponentName))
	}
	url += "/outages"

	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var outages []types.Outage
	err = json.NewDecoder(resp.Body).Decode(&outages)
	require.NoError(t, err)

	return outages
}

// getAllComponentsStatus is a helper function to get all components status and do basic assertions
func getAllComponentsStatus(t *testing.T, client *TestHTTPClient) []types.ComponentStatus {
	resp, err := client.Get("/api/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var allStatuses []types.ComponentStatus
	err = json.NewDecoder(resp.Body).Decode(&allStatuses)
	require.NoError(t, err)

	return allStatuses
}

// expect404 is a helper function to make a GET request and expect a 404 response
func expect404(t *testing.T, client *TestHTTPClient, url string) {
	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
