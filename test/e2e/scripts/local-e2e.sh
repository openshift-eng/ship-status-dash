#!/bin/bash

set -e

CONTAINER_NAME="ship-status-test-db"
DB_PORT="5433"
DB_USER="postgres"
DB_PASSWORD="testpass"
DB_NAME="ship_status_test"

cleanup() {
  echo "Cleaning up component-monitor processes..."
  if [ ! -z "$COMPONENT_MONITOR_PID" ]; then
    kill -TERM $COMPONENT_MONITOR_PID 2>/dev/null || true
    sleep 1
    kill -KILL $COMPONENT_MONITOR_PID 2>/dev/null || true
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
  
  echo "Cleaning up test container..."
  podman stop $CONTAINER_NAME 2>/dev/null || true
  podman rm $CONTAINER_NAME 2>/dev/null || true
  
  echo "Cleaning up temporary files..."
  if [ ! -z "$HMAC_SECRET_FILE" ] && [ -f "$HMAC_SECRET_FILE" ]; then
    rm -f "$HMAC_SECRET_FILE"
  fi
}

trap cleanup EXIT

echo "Cleaning up any existing test postgres container..."
podman stop $CONTAINER_NAME 2>/dev/null || true
podman rm $CONTAINER_NAME 2>/dev/null || true

echo "Starting PostgreSQL container..."
podman run -d \
  --name $CONTAINER_NAME \
  -e POSTGRES_PASSWORD=$DB_PASSWORD \
  -p $DB_PORT:5432 \
  quay.io/enterprisedb/postgresql:latest

echo "Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
  if podman exec $CONTAINER_NAME pg_isready -U $DB_USER > /dev/null 2>&1; then
    echo "PostgreSQL is ready"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "PostgreSQL failed to start"
    podman logs $CONTAINER_NAME
    podman stop $CONTAINER_NAME
    podman rm $CONTAINER_NAME
    exit 1
  fi
  sleep 1
done

echo "Creating test database..."
podman exec $CONTAINER_NAME psql -U $DB_USER -c "CREATE DATABASE $DB_NAME;"

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

echo "Using port $DASHBOARD_PORT for dashboard server"
echo "Using port $PROXY_PORT for mock oauth-proxy"
echo "Using port $MOCK_MONITORED_COMPONENT_PORT for mock-monitored-component"

echo "Starting dashboard server..."
DASHBOARD_PID=""
DASHBOARD_LOG="/tmp/dashboard-server.log"

# Start dashboard server in background
unset SKIP_AUTH # make sure we are using authentication
go run ./cmd/dashboard --config test/e2e/scripts/dashboard-config.yaml --port $DASHBOARD_PORT --dsn "$DSN" --hmac-secret-file "$HMAC_SECRET_FILE" 2> "$DASHBOARD_LOG" &
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

echo "Starting component-monitor..."
COMPONENT_MONITOR_LOG="/tmp/component-monitor.log"

# Start component-monitor in background
go run ./cmd/component-monitor --config-path test/e2e/scripts/component-monitor-config.yaml --dashboard-url "http://localhost:$DASHBOARD_PORT" --name "e2e-component-monitor" 2> "$COMPONENT_MONITOR_LOG" &
COMPONENT_MONITOR_PID=$!

echo "Running e2e tests..."
export TEST_SERVER_URL="http://localhost:$DASHBOARD_PORT"
export TEST_MOCK_OAUTH_PROXY_URL="http://localhost:$PROXY_PORT"
export TEST_MOCK_MONITORED_COMPONENT_URL="http://localhost:$MOCK_MONITORED_COMPONENT_PORT"
set +e
gotestsum ./test/e2e/... -count 1 -p 1
TEST_EXIT_CODE=$?
set -e

echo "Stopping component-monitor..."
if [ ! -z "$COMPONENT_MONITOR_PID" ]; then
  kill -TERM $COMPONENT_MONITOR_PID 2>/dev/null || true
  sleep 1
  kill -KILL $COMPONENT_MONITOR_PID 2>/dev/null || true
fi

echo "Stopping mock-monitored-component..."
if [ ! -z "$MOCK_MONITORED_COMPONENT_PORT" ]; then
  lsof -ti :$MOCK_MONITORED_COMPONENT_PORT | xargs kill -TERM 2>/dev/null || true
  sleep 1
  lsof -ti :$MOCK_MONITORED_COMPONENT_PORT | xargs kill -KILL 2>/dev/null || true
fi

echo "Stopping mock oauth-proxy..."
if [ ! -z "$PROXY_PORT" ]; then
  lsof -ti :$PROXY_PORT | xargs kill -TERM 2>/dev/null || true
  sleep 1
  lsof -ti :$PROXY_PORT | xargs kill -KILL 2>/dev/null || true
fi

echo "Stopping dashboard server..."
if [ ! -z "$DASHBOARD_PORT" ]; then
  lsof -ti :$DASHBOARD_PORT | xargs kill -TERM 2>/dev/null || true
  sleep 1
  lsof -ti :$DASHBOARD_PORT | xargs kill -KILL 2>/dev/null || true
fi

echo "=== Component Monitor Log ==="
cat "$COMPONENT_MONITOR_LOG" 2>/dev/null || echo "No log found"

echo ""
echo "=== Dashboard Server Log ==="
cat "$DASHBOARD_LOG" 2>/dev/null || echo "No log found"

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo "✓ Tests passed"
else
  echo "✗ Tests failed with exit code $TEST_EXIT_CODE"
fi

exit $TEST_EXIT_CODE

