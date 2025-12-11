# Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.

## Overview

The component-monitor supports two types of monitoring:

1. **HTTP Monitoring**: Probes HTTP endpoints and checks for expected status codes
2. **Prometheus Monitoring**: Executes Prometheus queries to check component health

## Configuration

### Command-Line Flags

- `--config-path` (required): Path to the component monitor configuration file (YAML)
- `--dashboard-url` (default: `http://localhost:8080`): Base URL of the dashboard API
- `--name` (required): Name identifier for this component monitor instance
- `--kubeconfig-dir` (optional): Path to a directory containing kubeconfig files for different clusters

### Configuration File

The configuration file is a YAML file with the following structure:

```yaml
frequency: 30s
components:
  - component_slug: "prow"
    sub_component_slug: "deck"
    http_monitor:
      url: "https://prow.ci.openshift.org/"
      code: 200
      retry_after: 10s
  - component_slug: "prow"
    sub_component_slug: "tide"
    prometheus_monitor:
      prometheus_location: "app.ci"
      queries:
        - query: "up{job=\"tide\"} == 1"
          failure_query: "up{job=\"tide\"}"
```

#### Configuration Fields

- `frequency`: How often to probe all components (e.g., `30s`, `1m`)
- `components`: List of components to monitor
  - `component_slug`: Top-level component identifier
  - `sub_component_slug`: Sub-component identifier
  - `http_monitor`: HTTP monitoring configuration (optional)
    - `url`: HTTP endpoint to probe
    - `code`: Expected HTTP status code
    - `retry_after`: Duration to wait before retrying after a failure
  - `prometheus_monitor`: Prometheus monitoring configuration (optional)
    - `prometheus_location`: Either:
      - A URL (for e2e and local development), e.g., `http://localhost:9090`
      - A cluster name (when `--kubeconfig-dir` is provided), e.g., `app.ci`
        The cluster name must correspond to a kubeconfig file in the kubeconfig directory.
        When using a cluster name, the Prometheus route will be discovered automatically via OpenShift Routes.
    - `queries`: List of Prometheus queries to execute
      - `query`: The Prometheus query to run (must return results for healthy state)
      - `failure_query`: Optional query to run when the main query fails, providing additional context for the outage

## Kubeconfig Directory

When `--kubeconfig-dir` is provided:

- The `prometheus_location` field in the configuration must be a cluster name (not a URL)
- Each kubeconfig file in the directory should be named after the cluster with a `.config` suffix (e.g., `app.ci.config` for the app.ci cluster)
- The component-monitor will:
  1. Load the kubeconfig file for the specified cluster
  2. Use the kubeconfig's authentication (bearer token, TLS certificates)
  3. Discover the Prometheus route via OpenShift Routes API
  4. Create an authenticated Prometheus client

### Example Kubeconfig Directory Structure

```
/etc/kubeconfigs/
├── app.ci.config
└── build01.config
```

Each file is a standard kubeconfig file with cluster, user, and context information.

## Local Development and E2E Testing

For local development and e2e testing:

- Do not provide `--kubeconfig-dir`
- Use URLs in `prometheus_location` (e.g., `http://localhost:9090`)
- The component-monitor will connect to Prometheus without authentication

## How It Works

1. The component-monitor loads the configuration file
2. For each configured component, it creates appropriate probers (HTTP or Prometheus)
3. At the configured frequency, it runs all probes
4. Probe results are sent to the dashboard API via POST to `/api/component-monitor/report`
5. The dashboard API processes the reports and creates/resolves outages accordingly

## Status Reporting

The component-monitor reports one of three statuses for each component:

- `Healthy`: Component is functioning normally
- `Degraded`: Component is experiencing issues but still partially functional (currently, this means that some queries are failing while others are passing)
- `Down`: Component is completely unavailable

The status is determined by the probe results:
- HTTP monitors: Status code matches expected → Healthy, otherwise → Down
- Prometheus monitors: Query returns results → Healthy, no results → Down

## Error Handling

- If a probe fails to execute (network error, etc.), an error is logged but the probe continues
- If the dashboard API is unavailable, errors are logged and the component-monitor continues running
- Configuration validation errors cause the component-monitor to exit immediately

