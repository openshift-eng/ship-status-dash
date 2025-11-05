#!/bin/bash

set -e

CONTAINER_NAME="ship-status-test-db"
DB_PORT="5433"
DB_USER="postgres"
DB_PASSWORD="testpass"
DB_NAME="ship_status_test"

cleanup() {
  echo "Cleaning up dashboard processes..."
  if [ ! -z "$DASHBOARD_PID" ]; then
    kill -TERM $DASHBOARD_PID 2>/dev/null || true
    sleep 2
    kill -KILL $DASHBOARD_PID 2>/dev/null || true
    wait $DASHBOARD_PID 2>/dev/null || true
  fi
  pkill -f "go run.*cmd/dashboard" 2>/dev/null || true
  pkill -f "dashboard.*--config.*test/e2e/config.yaml" 2>/dev/null || true
  sleep 2
  
  echo "Cleaning up test container..."
  podman stop $CONTAINER_NAME 2>/dev/null || true
  podman rm $CONTAINER_NAME 2>/dev/null || true
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
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"
go run ./cmd/migrate --dsn "$DSN"

echo "Finding available port..."
DASHBOARD_PORT=""
for port in {8080..8099}; do
  if ! lsof -i :$port > /dev/null 2>&1; then
    DASHBOARD_PORT=$port
    break
  fi
done

if [ -z "$DASHBOARD_PORT" ]; then
  echo "No available port found in range 8080-8099"
  exit 1
fi

echo "Using port $DASHBOARD_PORT for dashboard server"

echo "Starting dashboard server..."
DASHBOARD_PID=""
DASHBOARD_LOG="/tmp/dashboard-server.log"

# Start dashboard server in background
DEV_MODE=1 go run ./cmd/dashboard --config test/e2e/config.yaml --port $DASHBOARD_PORT --dsn "$DSN" 2> "$DASHBOARD_LOG" &
DASHBOARD_PID=$!

# Wait for server to be ready
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
    kill $DASHBOARD_PID 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "Running e2e tests..."
export TEST_SERVER_PORT="$DASHBOARD_PORT"
set +e
gotestsum ./test/e2e/... -count 1 -p 1
TEST_EXIT_CODE=$?
set -e

echo "Stopping dashboard server..."
kill -TERM $DASHBOARD_PID 2>/dev/null || true
sleep 2
kill -KILL $DASHBOARD_PID 2>/dev/null || true
wait $DASHBOARD_PID 2>/dev/null || true

echo "=== Server Log ==="
cat "$DASHBOARD_LOG" 2>/dev/null || echo "No log found"

echo ""
if [ $TEST_EXIT_CODE -eq 0 ]; then
  echo "✓ Tests passed"
else
  echo "✗ Tests failed with exit code $TEST_EXIT_CODE"
fi

exit $TEST_EXIT_CODE

