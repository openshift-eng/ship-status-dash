# Ship Status Dashboard

SHIP Status and Availability Dashboard monitor

## Project Structure

This project consists of multiple components:

- **Dashboard**: Web application for viewing and managing component status, availability, and outages
  - Backend: Go server (`cmd/dashboard`)
  - Frontend: React application (`frontend/`)
- **Component Monitor**: Monitoring service that periodically probes components and reports their status to the dashboard
  - Go service (`cmd/component-monitor`)
  - Supports HTTP and Prometheus monitoring

For local development setup, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Dashboard Component

The dashboard is a web application for viewing and managing component status, availability, and outages. It consists of:
- Backend: Go server (`cmd/dashboard`)
- Frontend: React application (`frontend/`)

### Production Deployment

#### Authentication Architecture

The production deployment uses a dual ingress architecture with a single backing service:

- **Public Route**: `ship-status.ci.openshift.org` → Service port `8080` → Dashboard container
- **Protected Route**: `protected.ship-status.ci.openshift.org` → Service port `8443` → OAuth proxy container

Both routes point to the same Kubernetes Service (`dashboard`), which exposes two ports:
- Port `8080`: Direct access to the dashboard container (public routes, no authentication)
- Port `8443`: Access through the oauth-proxy container (protected routes, requires authentication)

#### Pod Architecture

Each deployment pod contains two containers:

1. **Dashboard Container** (port 8080)
   - Serves public API endpoints (read-only status endpoints)
   - Validates HMAC signatures for protected endpoints
   - Expects `X-Forwarded-User` header and `GAP-Signature` header from oauth-proxy

2. **OAuth Proxy Container** (port 8443)
   - Handles OpenShift OAuth authentication
   - Proxies authenticated requests to `localhost:8080` (dashboard container)
   - Adds authentication headers:
     - `X-Forwarded-User`: Authenticated username
     - `X-Forwarded-Access-Token`: OAuth access token
     - Other headers that we don't currently care about
   - Signs requests with HMAC using shared secret
   - Adds `GAP-Signature` header for request verification

#### Authentication Flow

**Public Routes** (no authentication):
```
Client → Ingress (ship-status.ci.openshift.org) → Service:8080 → Dashboard Container
```

**Protected Routes** (authentication required):
```
Client → Ingress (protected.ship-status.ci.openshift.org) → Service:8443 → OAuth Proxy
  → Dashboard Container (localhost:8080)
```

The dashboard container validates protected requests by:
1. Checking for `X-Forwarded-User` header
2. Verifying the `GAP-Signature` HMAC signature using the shared secret
3. Extracting user identity from headers for authorization checks

#### HMAC Signature

Both oauth-proxy and dashboard share the same HMAC secret. The signature includes:
- Content-Length, Content-MD5, Content-Type
- Date
- Authorization
- X-Forwarded-User, X-Forwarded-Email, X-Forwarded-Access-Token
- Cookie, Gap-Auth

Each of these headers are included when the OpenShift Oauth Proxy creates it's signature, and we must provide complete parity.
See [SignatureHeaders](https://github.com/openshift/oauth-proxy/blob/master/oauthproxy.go).

## Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.

### Overview

The component-monitor supports two types of monitoring:

1. **HTTP Monitoring**: Probes HTTP endpoints and checks for expected status codes
2. **Prometheus Monitoring**: Executes Prometheus queries (both instant and range queries) to check component health

### Architecture

The component-monitor runs as a standalone service that:
- Loads configuration from a YAML file
- Creates probers for each configured component/sub-component
- Periodically executes probes at a configured frequency
- Sends probe results to the dashboard API via HTTP POST requests
- Does not expose any HTTP endpoints itself (only makes outbound requests)

### Configuration

The component-monitor is configured via command-line flags and a YAML configuration file:

**Command-Line Flags:**
- `--config-path` (required): Path to the component monitor configuration file (YAML)
- `--dashboard-url`: Base URL of the dashboard API
- `--name` (required): Name identifier for this component monitor instance
- `--kubeconfig-dir` (optional): Path to a directory containing kubeconfig files for different clusters

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

### Prometheus Location Configuration

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

### How It Works

1. The component-monitor loads the configuration file and validates all settings
2. For each configured component, it creates appropriate probers (HTTP or Prometheus)
3. At the configured frequency, it runs all probes concurrently
4. Probe results are aggregated and sent to the dashboard API via POST to `/api/component-monitor/report`
5. The dashboard API processes the reports and creates/resolves outages accordingly

### Status Reporting

The component-monitor reports one of three statuses for each component:

- **Healthy**: All queries/probes are successful
- **Degraded**: Some queries/probes are failing while others are passing
- **Down**: All queries/probes are failing

The status is determined by the probe results:
- **HTTP monitors**: Status code matches expected → Healthy, otherwise → Down
- **Prometheus monitors**: Query returns results → Healthy, no results → Down

### Range Queries

When a `duration` is specified for a Prometheus query, the component-monitor executes it as a range query:
- The query looks back over the specified duration from the current time
- The `step` parameter controls the resolution (time between data points)
- If `step` is not provided, a default is calculated:
  - For durations ≤ 1 hour: 15 seconds
  - For longer durations: duration / 250
- Range queries return a `Matrix` type, which is evaluated by checking if any time series have data points

### Error Handling

- If a probe fails to execute (network error, etc.), an error is logged but the probe continues
- If the dashboard API is unavailable, errors are logged and the component-monitor continues running
- Configuration validation errors (invalid durations, steps, or prometheus locations) cause the component-monitor to exit immediately

### Deployment

For deployment examples, see [`deploy/component-monitor/`](deploy/component-monitor/README.md).