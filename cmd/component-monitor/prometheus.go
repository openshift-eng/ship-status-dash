package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	promapi "github.com/prometheus/client_golang/api"
	promclientv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"ship-status-dash/pkg/types"
)

// validatePrometheusLocations validates prometheusLocation values based on whether kubeconfigDir is provided:
func validatePrometheusLocations(components []types.MonitoringComponent, kubeconfigDir string) error {
	for _, component := range components {
		if component.PrometheusMonitor == nil {
			continue
		}

		location := component.PrometheusMonitor.PrometheusLocation
		if location == "" {
			return fmt.Errorf("prometheusLocation is required for component %s/%s", component.ComponentSlug, component.SubComponentSlug)
		}

		if kubeconfigDir != "" {
			// When kubeconfig-dir is provided, location must be a cluster name (not a URL)
			if isURL(location) {
				return fmt.Errorf("prometheusLocation must be a cluster name (not a URL) when --kubeconfig-dir is set, got: %s", location)
			}

			// Check if kubeconfig file exists for this cluster
			kubeconfigPath := filepath.Join(kubeconfigDir, location+".config")
			if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
				return fmt.Errorf("kubeconfig file not found for cluster %s at %s", location, kubeconfigPath)
			}
		} else {
			// When kubeconfig-dir is not provided, location must be a URL
			if !isURL(location) {
				return fmt.Errorf("prometheusLocation must be a URL when --kubeconfig-dir is not set, got: %s", location)
			}
		}
	}

	return nil
}

// isURL checks if a string is a valid URL
func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func createPrometheusClients(components []types.MonitoringComponent, kubeconfigDir string) (map[string]promclientv1.API, error) {
	clients := make(map[string]promclientv1.API)

	// Collect unique Prometheus locations
	prometheusLocations := make(map[string]bool)
	for _, component := range components {
		if component.PrometheusMonitor != nil {
			prometheusLocations[component.PrometheusMonitor.PrometheusLocation] = true
		}
	}

	if len(prometheusLocations) == 0 {
		return clients, nil
	}

	// If kubeconfigDir is not set, treat all locations as URLs (for e2e/local dev)
	if kubeconfigDir == "" {
		for location := range prometheusLocations {
			if !isURL(location) {
				return nil, fmt.Errorf("prometheusLocation must be a URL when --kubeconfig-dir is not set, got: %s", location)
			}
			client, err := promapi.NewClient(promapi.Config{
				Address: location,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create prometheus client for %s: %w", location, err)
			}
			prometheusAPI := promclientv1.NewAPI(client)
			clients[location] = prometheusAPI
		}
		return clients, nil
	}

	// kubeconfigDir is set - treat locations as cluster names
	clusterConfigs := make(map[string]*rest.Config)
	for location := range prometheusLocations {
		kubeconfigPath := filepath.Join(kubeconfigDir, location+".config")
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig for cluster %s: %w", location, err)
		}
		clusterConfigs[location] = config
	}

	for location := range prometheusLocations {
		config := clusterConfigs[location]

		roundTripper, err := rest.TransportFor(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport for cluster %s: %w", location, err)
		}

		prometheusURL, err := discoverPrometheusRoute(config)
		if err != nil {
			return nil, fmt.Errorf("failed to discover Prometheus route for cluster %s: %w", location, err)
		}

		client, err := promapi.NewClient(promapi.Config{
			Address:      prometheusURL,
			RoundTripper: roundTripper,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus client for cluster %s: %w", location, err)
		}
		prometheusAPI := promclientv1.NewAPI(client)
		clients[location] = prometheusAPI
	}

	return clients, nil
}

func discoverPrometheusRoute(config *rest.Config) (string, error) {
	routeClient, err := routeclientset.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create route client: %w", err)
	}

	route, err := routeClient.Routes("openshift-monitoring").Get(context.Background(), "prometheus-k8s", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get prometheus route: %w", err)
	}

	var addr string
	if route.Spec.TLS != nil {
		addr = "https://" + route.Spec.Host
	} else {
		addr = "http://" + route.Spec.Host
	}

	return addr, nil
}
