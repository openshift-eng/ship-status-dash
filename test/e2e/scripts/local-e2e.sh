#!/bin/bash

set -e

POSTGRES_CONTAINER_NAME="ship-status-test-db"
PROMETHEUS_CONTAINER_NAME="prometheus-e2e"
DB_PORT="5433"
DB_USER="postgres"
DB_PASSWORD="testpass"
DB_NAME="ship_status_test"

cleanup() {
  echo "Cleaning up component-monitor processes..."
  # Kill by PID first if we have it
  if [ ! -z "$COMPONENT_MONITOR_PID" ]; then
    kill -TERM $COMPONENT_MONITOR_PID 2>/dev/null || true
    sleep 1
    kill -KILL $COMPONENT_MONITOR_PID 2>/dev/null || true
  fi
  # Also kill all component-monitor processes by pattern to catch any orphaned processes
  pkill -TERM -f "component-monitor.*e2e-component-monitor" 2>/dev/null || true
  sleep 1
  pkill -KILL -f "component-monitor.*e2e-component-monitor" 2>/dev/null || true

  # It seems important to stop the Prometheus container prior to stopping the mock-monitored-component, apparently podman can crash otherwise.
  echo "Stopping Prometheus container..."
  if podman ps -a --format "{{.Names}}" 2>/dev/null | grep -q "^${PROMETHEUS_CONTAINER_NAME}$" 2>/dev/null; then
    podman stop "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
    podman rm -f "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
  fi
  
  echo "Cleaning up mock-monitored-component processes..."
  if [ ! -z "$MOCK_MONITORED_COMPONENT_PORT" ]; then
    lsof -ti :$MOCK_MONITORED_COMPONENT_PORT | xargs kill -TERM 2>/dev/null || true
    sleep 1
    lsof -ti :$MOCK_MONITORED_COMPONENT_PORT | xargs kill -KILL 2>/dev/null || true
  fi
  
  echo "Cleaning up proxy processes..."
  if [ ! -z "$PROXY_PORT" ]; then
    lsof -ti :$PROXY_PORT | xargs kill -TERM 2>/dev/null || true
    sleep 1
    lsof -ti :$PROXY_PORT | xargs kill -KILL 2>/dev/null || true
  fi
  
  echo "Cleaning up dashboard processes..."
  if [ ! -z "$DASHBOARD_PORT" ]; then
    lsof -ti :$DASHBOARD_PORT | xargs kill -TERM 2>/dev/null || true
    sleep 1
    lsof -ti :$DASHBOARD_PORT | xargs kill -KILL 2>/dev/null || true
  fi
  
  echo "Cleaning up postgres container..."
  podman rm -f $POSTGRES_CONTAINER_NAME > /dev/null 2>&1 || true
  
  echo "Cleaning up temporary files..."
  if [ ! -z "$DASHBOARD_CONFIG" ]; then
    rm -f "$DASHBOARD_CONFIG" 2>/dev/null || true
  fi
  if [ ! -z "$HMAC_SECRET_FILE" ] && [ -f "$HMAC_SECRET_FILE" ]; then
    rm -f "$HMAC_SECRET_FILE"
  fi
  if [ ! -z "$COMPONENT_MONITOR_CONFIG" ]; then
    rm -f "$COMPONENT_MONITOR_CONFIG" 2>/dev/null || true
  fi
  if [ ! -z "$PROMETHEUS_CONFIG_TMP" ] && [ -f "$PROMETHEUS_CONFIG_TMP" ]; then
    rm -f "$PROMETHEUS_CONFIG_TMP" 2>/dev/null || true
  fi
}

trap cleanup EXIT

echo "Starting PostgreSQL container..."
# Remove any existing container first
if podman ps -a --format "{{.Names}}" 2>/dev/null | grep -q "^${POSTGRES_CONTAINER_NAME}$" 2>/dev/null; then
  podman rm -f $POSTGRES_CONTAINER_NAME > /dev/null 2>&1 || true
fi

podman run -d \
  --name $POSTGRES_CONTAINER_NAME \
  -e POSTGRES_PASSWORD=$DB_PASSWORD \
  -p $DB_PORT:5432 \
  quay.io/enterprisedb/postgresql:latest

echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
  if podman exec $POSTGRES_CONTAINER_NAME pg_isready -U $DB_USER > /dev/null 2>&1; then
    echo "PostgreSQL is ready"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "PostgreSQL failed to start"
    podman logs $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    podman stop $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    podman rm $POSTGRES_CONTAINER_NAME 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "Creating test database..."
if ! podman exec $POSTGRES_CONTAINER_NAME psql -U $DB_USER -c "CREATE DATABASE $DB_NAME;" > /dev/null 2>&1; then
  echo "Failed to create test database (may already exist, continuing...)"
fi

DSN="postgres://$DB_USER:$DB_PASSWORD@localhost:$DB_PORT/$DB_NAME?sslmode=disable&client_encoding=UTF8"
export TEST_DATABASE_DSN="$DSN"

echo "Running migration..."
PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$PROJECT_ROOT"
go run ./cmd/migrate --dsn "$DSN"

echo "Generating HMAC secret..."
HMAC_SECRET=$(openssl rand -hex 32)
HMAC_SECRET_FILE=$(mktemp)
echo -n "$HMAC_SECRET" > "$HMAC_SECRET_FILE"
echo "HMAC secret written to $HMAC_SECRET_FILE"

echo "Finding available ports..."
DASHBOARD_PORT=""
for port in {8080..8099}; do
  if ! lsof -i :$port > /dev/null 2>&1; then
    DASHBOARD_PORT=$port
    break
  fi
done

if [ -z "$DASHBOARD_PORT" ]; then
  echo "No available port found in range 8080-8099 for dashboard"
  exit 1
fi

PROXY_PORT=""
for port in {8443..8499}; do
  if ! lsof -i :$port > /dev/null 2>&1; then
    PROXY_PORT=$port
    break
  fi
done

if [ -z "$PROXY_PORT" ]; then
  echo "No available port found in range 8443-8499 for proxy"
  exit 1
fi

MOCK_MONITORED_COMPONENT_PORT="9000"
if lsof -i :$MOCK_MONITORED_COMPONENT_PORT > /dev/null 2>&1; then
  echo "Port $MOCK_MONITORED_COMPONENT_PORT is already in use for mock-monitored-component"
  exit 1
fi

PROMETHEUS_PORT=""
for port in {9090..9119}; do
  if ! lsof -i :$port > /dev/null 2>&1; then
    PROMETHEUS_PORT=$port
    break
  fi
done

if [ -z "$PROMETHEUS_PORT" ]; then
  echo "No available port found in range 9090-9119 for Prometheus"
  exit 1
fi

echo "Using port $DASHBOARD_PORT for dashboard server"
echo "Using port $PROXY_PORT for mock oauth-proxy"
echo "Using port $MOCK_MONITORED_COMPONENT_PORT for mock-monitored-component"
echo "Using port $PROMETHEUS_PORT for Prometheus"

echo "Starting dashboard server..."
DASHBOARD_PID=""
DASHBOARD_LOG="/tmp/dashboard-server.log"

# Create temporary dashboard config file so tests don't modify the original
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DASHBOARD_CONFIG=$(mktemp)
cp "$SCRIPT_DIR/dashboard-config.yaml" "$DASHBOARD_CONFIG"
export TEST_DASHBOARD_CONFIG_PATH="$DASHBOARD_CONFIG"

# Start dashboard server in background
unset SKIP_AUTH # make sure we are using authentication
go run ./cmd/dashboard --config "$DASHBOARD_CONFIG" --port $DASHBOARD_PORT --dsn "$DSN" --hmac-secret-file "$HMAC_SECRET_FILE" --absent-report-check-interval 15s --config-update-poll-interval 10s --slack-base-url "http://localhost:3000" --slack-workspace-url "https://rhsandbox.slack.com/" 2> "$DASHBOARD_LOG" &
DASHBOARD_PID=$!

# Wait for dashboard server to be ready
echo "Waiting for dashboard server to be ready..."
for i in {1..30}; do
  if curl -s http://localhost:$DASHBOARD_PORT/health > /dev/null 2>&1; then
    echo "Dashboard server is ready on port $DASHBOARD_PORT"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "Dashboard server failed to start"
    echo "=== Server Log ==="
    cat "$DASHBOARD_LOG" 2>/dev/null || echo "No log found"
    if [ ! -z "$DASHBOARD_PORT" ]; then
      lsof -ti :$DASHBOARD_PORT | xargs kill -KILL 2>/dev/null || true
    fi
    exit 1
  fi
  sleep 1
done

echo "Starting mock oauth-proxy..."
PROXY_PID=""
PROXY_LOG="/tmp/mock-oauth-proxy.log"

# Start mock oauth-proxy in background
go run ./cmd/mock-oauth-proxy --config test/e2e/scripts/mock-oauth-proxy-config.yaml --port $PROXY_PORT --upstream "http://localhost:$DASHBOARD_PORT" --hmac-secret-file "$HMAC_SECRET_FILE" 2> "$PROXY_LOG" &
PROXY_PID=$!

# Wait for proxy to be ready
echo "Waiting for mock oauth-proxy to be ready..."
for i in {1..30}; do
  if curl -s -u developer:developer http://localhost:$PROXY_PORT/health > /dev/null 2>&1; then
    echo "Mock oauth-proxy is ready on port $PROXY_PORT"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "Mock oauth-proxy failed to start"
    echo "=== Proxy Log ==="
    cat "$PROXY_LOG" 2>/dev/null || echo "No log found"
    if [ ! -z "$PROXY_PORT" ]; then
      lsof -ti :$PROXY_PORT | xargs kill -KILL 2>/dev/null || true
    fi
    exit 1
  fi
  sleep 1
done

echo "Starting mock-monitored-component..."
# Start mock-monitored-component in background
go run ./cmd/mock-monitored-component --port $MOCK_MONITORED_COMPONENT_PORT > /dev/null 2>&1 &

# Wait for mock-monitored-component to be ready
echo "Waiting for mock-monitored-component to be ready..."
for i in {1..30}; do
  if curl -s http://localhost:$MOCK_MONITORED_COMPONENT_PORT/health > /dev/null 2>&1; then
    echo "Mock-monitored-component is ready on port $MOCK_MONITORED_COMPONENT_PORT"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "Mock-monitored-component failed to start"
    exit 1
  fi
  sleep 1
done

echo "Starting Prometheus in podman container..."
PROMETHEUS_CONFIG_PATH="$(cd "$(dirname "$0")" && pwd)/prometheus.yml"
# Create temporary config file with substituted values
PROMETHEUS_CONFIG_TMP=$(mktemp)
MOCK_MONITORED_COMPONENT_TARGET="host.containers.internal:${MOCK_MONITORED_COMPONENT_PORT}"
export MOCK_MONITORED_COMPONENT_TARGET
envsubst < "$PROMETHEUS_CONFIG_PATH" > "$PROMETHEUS_CONFIG_TMP"

# Remove any existing container first
if podman ps -a --format "{{.Names}}" 2>/dev/null | grep -q "^${PROMETHEUS_CONTAINER_NAME}$" 2>/dev/null; then
  podman rm -f "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
fi

podman run -d \
  --name "$PROMETHEUS_CONTAINER_NAME" \
  -p $PROMETHEUS_PORT:9090 \
  -v "$PROMETHEUS_CONFIG_TMP:/etc/prometheus/prometheus.yml:ro" \
  quay.io/prometheus/prometheus:latest \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/prometheus \
  --web.console.libraries=/usr/share/prometheus/console_libraries \
  --web.console.templates=/usr/share/prometheus/consoles \
  --web.enable-lifecycle \
  > /dev/null 2>&1

echo "Waiting for Prometheus to complete initial scrape..."
for i in {1..60}; do
  if curl -s "http://localhost:$PROMETHEUS_PORT/api/v1/query?query=success_rate" | grep -q "success_rate"; then
    echo "Prometheus has completed initial scrape"
    break
  fi
  if [ $i -eq 60 ]; then
    echo "Prometheus failed to complete initial scrape within 60 seconds"
    podman logs "$PROMETHEUS_CONTAINER_NAME" 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "Starting component-monitor..."
COMPONENT_MONITOR_LOG="/tmp/component-monitor.log"

# Export environment variables for config substitution
export TEST_SERVER_URL="http://localhost:$DASHBOARD_PORT"
export TEST_MOCK_OAUTH_PROXY_URL="http://localhost:$PROXY_PORT"
export TEST_MOCK_MONITORED_COMPONENT_URL="http://localhost:$MOCK_MONITORED_COMPONENT_PORT"
export TEST_PROMETHEUS_URL="http://localhost:$PROMETHEUS_PORT"

# Create temporary config file with substituted values
COMPONENT_MONITOR_CONFIG=$(mktemp)
envsubst < test/e2e/scripts/component-monitor-config.yaml > "$COMPONENT_MONITOR_CONFIG"
export TEST_COMPONENT_MONITOR_CONFIG_PATH="$COMPONENT_MONITOR_CONFIG"

# Create temporary token file for component-monitor authentication
COMPONENT_MONITOR_TOKEN=$(mktemp)
echo "component-monitor-sa-token" > "$COMPONENT_MONITOR_TOKEN"

# Start component-monitor in background
go run ./cmd/component-monitor --config-path "$COMPONENT_MONITOR_CONFIG" --dashboard-url "$TEST_MOCK_OAUTH_PROXY_URL" --name "e2e-component-monitor" --report-auth-token-file "$COMPONENT_MONITOR_TOKEN" --config-update-poll-interval 10s 2> "$COMPONENT_MONITOR_LOG" &
COMPONENT_MONITOR_PID=$!

echo "Running e2e tests..."
set +e
gotestsum --format testname ./test/e2e/... -count 1 -p 1
TEST_EXIT_CODE=$?
set -e

# Only show logs if tests failed
if [ $TEST_EXIT_CODE -ne 0 ]; then
  echo ""
  echo "=== Component Monitor Log (last 50 lines) ==="
  tail -n 50 "$COMPONENT_MONITOR_LOG" 2>/dev/null || echo "No log found"
  echo "Full log: $COMPONENT_MONITOR_LOG"
  
  echo ""
  echo "=== Dashboard Server Log (last 50 lines) ==="
  tail -n 50 "$DASHBOARD_LOG" 2>/dev/null || echo "No log found"
  echo "Full log: $DASHBOARD_LOG"

  echo ""
  echo "=== Mock OAuth Proxy Log (last 50 lines) ==="
  tail -n 50 "$PROXY_LOG" 2>/dev/null || echo "No log found"
  echo "Full log: $PROXY_LOG"
  echo ""
fi
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo "✓ Tests passed"
else
  echo "✗ Tests failed with exit code $TEST_EXIT_CODE"
fi

exit $TEST_EXIT_CODE

