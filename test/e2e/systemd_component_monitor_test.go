package e2e

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"ship-status-dash/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

const dbusSocketPath = "/run/dbus/system_bus_socket"

func skipIfNoDBus(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("systemd e2e tests require Linux (current OS: %s)", runtime.GOOS)
	}
	if _, err := os.Stat(dbusSocketPath); os.IsNotExist(err) {
		t.Skipf("systemd e2e tests require D-Bus socket at %s", dbusSocketPath)
	}
}

func TestE2E_SystemdComponentMonitor(t *testing.T) {
	skipIfNoDBus(t)

	serverURL := os.Getenv("TEST_SERVER_URL")
	require.NotEmpty(t, serverURL, "TEST_SERVER_URL is not set")
	mockOauthProxyURL := os.Getenv("TEST_MOCK_OAUTH_PROXY_URL")
	require.NotEmpty(t, mockOauthProxyURL, "TEST_MOCK_OAUTH_PROXY_URL is not set")

	client, err := NewTestHTTPClient(serverURL, mockOauthProxyURL)
	require.NoError(t, err)

	componentName := "Errata Reliability"
	subComponentName := "systemd-test"

	configPath := os.Getenv("TEST_COMPONENT_MONITOR_CONFIG_PATH")
	require.NotEmpty(t, configPath, "TEST_COMPONENT_MONITOR_CONFIG_PATH must be set")
	originalConfig := readConfig(t, configPath)
	defer restoreConfig(t, configPath, originalConfig)

	t.Run("ActiveUnitNoOutage", func(t *testing.T) {
		// dbus.service is guaranteed to be active since we passed the D-Bus socket check
		activeUnit := os.Getenv("TEST_SYSTEMD_ACTIVE_UNIT")
		if activeUnit == "" {
			activeUnit = "dbus.service"
		}

		cleanupActiveOutages(t, client, componentName, subComponentName)

		updateComponentMonitorConfig(t, func(config *types.ComponentMonitorConfig) {
			config.Components = append(config.Components, types.MonitoringComponent{
				ComponentSlug:    "errata-reliability",
				SubComponentSlug: subComponentName,
				SystemdMonitor: &types.SystemdMonitor{
					Unit:     activeUnit,
					Severity: "Down",
				},
			})
		})

		// Wait a few probe cycles and verify no outage is created
		ctx := context.Background()
		var outageFound bool
		_ = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			outages := getOutages(t, client, componentName, subComponentName)
			activeOutages := filterActiveOutages(outages)
			for _, o := range activeOutages {
				if o.DiscoveredFrom == "component-monitor" {
					outageFound = true
					return true, nil
				}
			}
			return false, nil
		})
		assert.False(t, outageFound, "Active systemd unit should not create an outage")

		restoreConfig(t, configPath, originalConfig)
		time.Sleep(1 * time.Second)
		cleanupActiveOutages(t, client, componentName, subComponentName)
	})

	t.Run("InactiveUnitCreatesOutage", func(t *testing.T) {
		cleanupActiveOutages(t, client, componentName, subComponentName)

		updateComponentMonitorConfig(t, func(config *types.ComponentMonitorConfig) {
			config.Components = append(config.Components, types.MonitoringComponent{
				ComponentSlug:    "errata-reliability",
				SubComponentSlug: subComponentName,
				SystemdMonitor: &types.SystemdMonitor{
					Unit:     "nonexistent-systemd-e2e-test-12345.service",
					Severity: "Down",
				},
			})
		})

		foundOutage := waitForOutageCreated(t, client, componentName, subComponentName, 60*time.Second)
		require.NotNil(t, foundOutage, "Outage should be created for inactive systemd unit")

		assert.Equal(t, string(types.SeverityDown), string(foundOutage.Severity))
		assert.Equal(t, "component-monitor", foundOutage.DiscoveredFrom)
		assert.Equal(t, "e2e-component-monitor", foundOutage.CreatedBy)
		require.Len(t, foundOutage.Reasons, 1)
		assert.Equal(t, types.CheckTypeSystemd, foundOutage.Reasons[0].Type)
		assert.Equal(t, "nonexistent-systemd-e2e-test-12345.service", foundOutage.Reasons[0].Check)
		assert.Equal(t, "ActiveState: inactive", foundOutage.Reasons[0].Results)

		restoreConfig(t, configPath, originalConfig)
		time.Sleep(1 * time.Second)
		cleanupActiveOutages(t, client, componentName, subComponentName)
	})

	t.Run("SeverityMapping", func(t *testing.T) {
		cleanupActiveOutages(t, client, componentName, subComponentName)

		updateComponentMonitorConfig(t, func(config *types.ComponentMonitorConfig) {
			config.Components = append(config.Components, types.MonitoringComponent{
				ComponentSlug:    "errata-reliability",
				SubComponentSlug: subComponentName,
				SystemdMonitor: &types.SystemdMonitor{
					Unit:     "nonexistent-systemd-e2e-severity-test.service",
					Severity: "Degraded",
				},
			})
		})

		foundOutage := waitForOutageCreated(t, client, componentName, subComponentName, 60*time.Second)
		require.NotNil(t, foundOutage, "Outage should be created for inactive systemd unit")
		assert.Equal(t, string(types.SeverityDegraded), string(foundOutage.Severity))

		restoreConfig(t, configPath, originalConfig)
		time.Sleep(1 * time.Second)
		cleanupActiveOutages(t, client, componentName, subComponentName)
	})
}
