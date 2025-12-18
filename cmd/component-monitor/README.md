# Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.

## Overview

The component-monitor supports two types of monitoring:

1. **HTTP Monitoring**: Probes HTTP endpoints and checks for expected status codes
2. **Prometheus Monitoring**: Executes Prometheus queries (both instant and range queries) to check component health

## Architecture

The component-monitor runs as a standalone service that:
- Loads configuration from a YAML file
- Creates probers for each configured component/sub-component
- Periodically executes probes at a configured frequency
- Sends probe results to the dashboard API via HTTP POST requests
- Does not expose any HTTP endpoints itself (only makes outbound requests)

## Configuration

The component-monitor is configured via command-line flags and a YAML configuration file:

**Command-Line Flags:**
- `--config-path` (required): Path to the component monitor configuration file (YAML)
- `--dashboard-url`: Base URL of the dashboard API
- `--name` (required): Name identifier for this component monitor instance
- `--kubeconfig-dir` (optional): Path to a directory containing kubeconfig files for different clusters
- `--report-auth-token-file` (required): Path to file containing bearer token for authenticating report requests to the dashboard API

**Configuration File Structure:**
```yaml
frequency: 5m
components:
  - component_slug: "prow"
    sub_component_slug: "deck"
    http_monitor:
      url: "https://prow.ci.openshift.org/"
      code: 200
      retry_after: 4m
    prometheus_monitor:
      prometheus_location: "app.ci"
      queries:
        - query: "up{job=\"deck\"} == 1"
          failure_query: "up{job=\"deck\"}"
          duration: "5m"
          step: "30s"
```

**Prometheus Query Configuration:**
- `query`: The Prometheus query to run (must return results for healthy state)
- `failure_query`: Optional query to run when the main query fails, providing additional context
- `duration`: Optional duration string (e.g., `"5m"`, `"30s"`). If provided, the query will be executed as a range query
- `step`: Optional resolution for range queries (e.g., `"30s"`, `"15s"`). If not provided, a default step is calculated based on the duration

## Prometheus Location Configuration

The `prometheus_location` field can be configured in two ways:

**1. URL (for local development and e2e testing):**
- Set `prometheus_location` to a URL (e.g., `http://localhost:9090`)
- Do not provide `--kubeconfig-dir` flag
- The component-monitor connects directly to Prometheus without authentication

**2. Cluster Name (for production deployments):**
- Set `prometheus_location` to a cluster name (e.g., `app.ci`)
- Provide `--kubeconfig-dir` flag pointing to a directory with kubeconfig files
- Each kubeconfig file should be named after the cluster with a `.config` suffix (e.g., `app.ci.config`)
- The component-monitor will:
  1. Load the kubeconfig file for the specified cluster
  2. Use the kubeconfig's authentication (bearer token, TLS certificates)
  3. Discover the Prometheus route via OpenShift Routes API
  4. Create an authenticated Prometheus client

## Service Account Authentication

The component-monitor authenticates to the dashboard API using OpenShift ServiceAccount bearer tokens:

1. **Token Configuration**: The component-monitor reads a bearer token from a file specified via the `--report-auth-token-file` command-line flag
2. **Request Authentication**: When sending reports to the dashboard API, the component-monitor includes the token in the `Authorization` header as `Bearer <token>`
3. **OAuth Proxy Processing**: In production, requests go through the OAuth proxy which:
   - Validates the bearer token
   - Extracts the service account name (e.g., `system:serviceaccount:ship-status:component-monitor`)
   - Sets the `X-Forwarded-User` header to the service account name
   - Signs the request with HMAC and adds the `GAP-Signature` header
4. **Dashboard Authorization**: The dashboard validates that:
   - The HMAC signature is valid
   - The service account (from `X-Forwarded-User`) is listed as an owner of the component in the dashboard configuration
   - Only service accounts that are owners of a component can report status for that component's sub-components

**Component Configuration**: Components must have the service account listed in their `owners` section with a `service_account` field. For example, in the Dashboard configuration:
```yaml
components:
  - slug: "prow"
    owners:
      - service_account: "system:serviceaccount:ship-status:component-monitor"
```

## How It Works

1. The component-monitor loads the configuration file and validates all settings
2. For each configured component, it creates appropriate probers (HTTP or Prometheus)
3. At the configured frequency, it runs all probes concurrently
4. Probe results are aggregated and sent to the dashboard API via POST to `/api/component-monitor/report` with bearer token authentication
5. The dashboard API processes the reports and creates/resolves outages accordingly

## Status Reporting

The component-monitor reports one of three statuses for each component:

- **Healthy**: All queries/probes are successful
- **Degraded**: Some queries/probes are failing while others are passing
- **Down**: All queries/probes are failing

The status is determined by the probe results:
- **HTTP monitors**: Status code matches expected → Healthy, otherwise → Down
- **Prometheus monitors**: Query returns results → Healthy, no results → Down

## Range Queries

When a `duration` is specified for a Prometheus query, the component-monitor executes it as a range query:
- The query looks back over the specified duration from the current time
- The `step` parameter controls the resolution (time between data points)
- If `step` is not provided, a default is calculated:
  - For durations ≤ 1 hour: 15 seconds
  - For longer durations: duration / 250
- Range queries return a `Matrix` type, which is evaluated by checking if any time series have data points

## Error Handling

- If a probe fails to execute (network error, etc.), an error is logged but the probe continues
- If the dashboard API is unavailable, errors are logged and the component-monitor continues running
- Configuration validation errors (invalid durations, steps, or prometheus locations) cause the component-monitor to exit immediately

## Deployment

For deployment examples, see [`deploy/component-monitor/`](../../deploy/component-monitor/README.md).
