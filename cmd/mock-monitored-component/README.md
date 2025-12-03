# Mock Monitored Component

A simple HTTP server that mocks a monitored component for end-to-end (e2e) testing. It provides endpoints for health checks and exposes Prometheus metrics that can be scraped and queried to test different Prometheus result types (Scalar, Vector, and Matrix).

## Overview

This mock component is used in e2e tests to simulate a real monitored component. It allows tests to:
- Control the health status of a component
- Expose Prometheus metrics that can be queried
- Test different Prometheus query result types

## Endpoints

### `/health`

Returns the current health status of the component.

- **Method**: GET
- **Response**: 
  - Status code: 200 (healthy) or 500 (unhealthy)
  - Body: `Status: <code>\n`

The status code can be controlled via the `/up` and `/down` endpoints.

### `/up`

Sets the component to a healthy state (HTTP 200).

- **Method**: GET
- **Response**: 
  - Status code: 200
  - Body: `Status set to 200\n`

### `/down`

Sets the component to an unhealthy state (HTTP 500).

- **Method**: GET
- **Response**: 
  - Status code: 200
  - Body: `Status set to 500\n`

### `/metrics`

Exposes Prometheus metrics in the standard Prometheus format. This endpoint can be scraped by Prometheus or queried directly.

- **Method**: GET
- **Response**: Prometheus metrics in text format

The following metrics are exposed:

1. **`success_rate`** (Gauge) - A simple gauge without labels
   - Used for testing **Scalar** results when queried with `scalar()`
   - Returns **Vector** on instant queries
   - Returns **Matrix** on range queries
   - Default value: 1.0

2. **`data_load_failure`** (GaugeVec) - A gauge with labels
   - Label: `component`
   - Used for testing **Vector** results (multiple time series with labels)
   - Returns **Matrix** on range queries
   - Default: `data_load_failure{component="api"} 0.0`

3. **`request_count`** (Gauge) - A gauge for range queries
   - Used for testing **Matrix** results (time series over a range)
   - Returns **Vector** on instant queries
   - Default value: 0.0

### `/update-metrics`

Updates the values of the Prometheus metrics. This endpoint allows e2e tests to set specific metric values for testing different scenarios.

- **Method**: POST
- **Content-Type**: application/json
- **Request Body**:
```json
{
  "success_rate": 0.95,
  "data_load_failure": {
    "api": 1.0,
    "db": 0.5,
    "cache": 0.0
  },
  "request_count": 100.0
}
```

All fields are optional. Only provided fields will be updated.

- **Response**: 
  - Status code: 200
  - Body: `Metrics updated\n`

**Example**:
```bash
curl -X POST http://localhost:9000/update-metrics \
  -H "Content-Type: application/json" \
  -d '{"success_rate": 0.85, "data_load_failure": {"api": 1.0}}'
```

## Usage in E2E Testing

The mock-monitored-component is used in the e2e test suite to:

1. **Test HTTP health monitoring**: The component-monitor can be configured to probe the `/health` endpoint and detect when the component goes down (via `/down`) or comes back up (via `/up`).

2. **Test Prometheus metric queries**: The component-monitor can query Prometheus metrics exposed at `/metrics` to test handling of different result types:
   - **Scalar**: Query `scalar(success_rate)` to get a single numeric value
   - **Vector**: Query `data_load_failure{component="api"}` to get a vector of time series with labels
   - **Matrix**: Query `request_count[5m]` to get a matrix of time series over a range

3. **Test metric-based outage detection**: By updating metrics via `/update-metrics`, tests can simulate different failure scenarios and verify that the component-monitor correctly detects and reports outages based on metric thresholds.

### Example E2E Test Flow

1. Start the mock-monitored-component:
```bash
go run ./cmd/mock-monitored-component --port 9000
```

2. Configure component-monitor to monitor it (see `test/e2e/scripts/component-monitor-config.yaml`)

3. In tests:
   - Verify initial healthy state: `GET /health` returns 200
   - Simulate failure: `GET /down` to set status to 500
   - Wait for component-monitor to detect and create outage
   - Restore health: `GET /up` to set status back to 200
   - Update metrics: `POST /update-metrics` with test values
   - Query metrics: `GET /metrics` to verify Prometheus format

## Running

### Standalone

```bash
go run ./cmd/mock-monitored-component --port 8080
```

The `--port` flag specifies the port to listen on (default: 8080).

### Local Development

The mock-monitored-component is automatically started when running the component-monitor locally using the development script:

```bash
./hack/local/component-monitor/local-dev.sh
```

This script:
- Starts the mock-monitored-component on port **8081**
- Waits for it to be ready (health check)
- Starts the component-monitor
- Handles cleanup of both processes on exit (Ctrl+C)

The mock component will be available at `http://localhost:8081` and can be used for local testing and development. To have component-monitor monitor it, add an entry to `hack/local/component-monitor/config.yaml`:

```yaml
components:
  - component_slug: "mock-component"
    sub_component_slug: "mock-component"
    http_monitor:
      url: "http://localhost:8081/health"
      code: 200
      retry_after: 2s
```

You can then test the component-monitor's behavior by:
- Bringing the mock component down: `curl http://localhost:8081/down`
- Bringing it back up: `curl http://localhost:8081/up`
- Updating metrics: `curl -X POST http://localhost:8081/update-metrics -H "Content-Type: application/json" -d '{"success_rate": 0.5}'`
- Viewing metrics: `curl http://localhost:8081/metrics`

## Prometheus Query Examples

Once the mock component is running and metrics are exposed, you can test different query types:

**Scalar** (single value):
```
scalar(success_rate)
```

**Vector** (time series with labels):
```
data_load_failure{component="api"}
data_load_failure
```

**Matrix** (time series over range):
```
request_count[5m]
success_rate[1m]
data_load_failure{component="api"}[10m]
```

