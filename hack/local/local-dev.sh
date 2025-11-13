#!/bin/bash

set -e

if [ -z "$1" ]; then
  echo "Usage: $0 <database-dsn>"
  echo "Example: $0 'postgres://user:pass@localhost:5432/ship_status?sslmode=disable'"
  exit 1
fi

DSN="$1"

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

kill_processes_on_port() {
  local port=$1
  local message=${2:-"Stopping processes on port $port..."}
  
  PIDS=$(lsof -ti :$port 2>/dev/null)
  if [ ! -z "$PIDS" ]; then
    echo "$message"
    for pid in $PIDS; do
      kill -TERM "$pid" 2>/dev/null || true
    done
    sleep 1
    PIDS=$(lsof -ti :$port 2>/dev/null)
    if [ ! -z "$PIDS" ]; then
      for pid in $PIDS; do
        kill -KILL "$pid" 2>/dev/null || true
      done
    fi
  fi
}

cleanup() {
  set +e
  echo ""
  echo "Cleaning up..."
  
  PROXY_PORT=${PROXY_PORT:-8443}
  DASHBOARD_PORT=${DASHBOARD_PORT:-8080}
  
  kill_processes_on_port "$PROXY_PORT"
  kill_processes_on_port "$DASHBOARD_PORT"
  
  if [ ! -z "$HMAC_SECRET_FILE" ] && [ -f "$HMAC_SECRET_FILE" ]; then
    rm -f "$HMAC_SECRET_FILE"
  fi
  
  echo "Cleanup complete"
  exit 0
}

trap cleanup EXIT

echo "Checking if ports are available..."
if lsof -i :8080 > /dev/null 2>&1; then
  echo "Error: Port 8080 is already in use"
  exit 1
fi

if lsof -i :8443 > /dev/null 2>&1; then
  echo "Error: Port 8443 is already in use"
  exit 1
fi

DASHBOARD_PORT=8080
PROXY_PORT=8443

echo "Generating HMAC secret..."
HMAC_SECRET=$(openssl rand -hex 32)
HMAC_SECRET_FILE=$(mktemp)
echo -n "$HMAC_SECRET" > "$HMAC_SECRET_FILE"
echo "HMAC secret written to $HMAC_SECRET_FILE"

echo "Starting dashboard server..."
DASHBOARD_PID=""
DASHBOARD_LOG="/tmp/dashboard-local-dev.log"
echo "Dashboard server logs: $DASHBOARD_LOG"

go run ./cmd/dashboard --config hack/local/config.yaml --port $DASHBOARD_PORT --dsn "$DSN" --hmac-secret-file "$HMAC_SECRET_FILE" --cors-origin "http://localhost:3000" 2>&1 | tee "$DASHBOARD_LOG" &
DASHBOARD_PID=$!

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
    kill_processes_on_port "$DASHBOARD_PORT"
    exit 1
  fi
  sleep 1
done

echo "Starting mock oauth-proxy..."
PROXY_PID=""
PROXY_LOG="/tmp/mock-oauth-proxy-local-dev.log"
echo "Mock oauth-proxy logs: $PROXY_LOG"

go run ./cmd/mock-oauth-proxy --config hack/local/mock-oauth-proxy-config.yaml --port $PROXY_PORT --upstream "http://localhost:$DASHBOARD_PORT" --hmac-secret-file "$HMAC_SECRET_FILE" 2> "$PROXY_LOG" &
PROXY_PID=$!

echo "Waiting for mock oauth-proxy to be ready..."
for i in {1..30}; do
  if curl -s -u developer:password http://localhost:$PROXY_PORT/health > /dev/null 2>&1; then
    echo "Mock oauth-proxy is ready on port $PROXY_PORT"
    break
  fi
  if [ $i -eq 30 ]; then
    echo "Mock oauth-proxy failed to start"
    echo "=== Proxy Log ==="
    cat "$PROXY_LOG" 2>/dev/null || echo "No log found"
    kill_processes_on_port "$PROXY_PORT"
    kill_processes_on_port "$DASHBOARD_PORT"
    exit 1
  fi
  sleep 1
done

echo ""
echo "✓ Local development environment is ready!"
echo ""
echo "Routes (matching production setup):"
echo "  Public Route (no auth):     http://localhost:$DASHBOARD_PORT"
echo "  Protected Route (with auth): http://localhost:$PROXY_PORT"
echo ""
echo "Example API calls:"
echo "  Public route (no auth required):"
echo "    curl http://localhost:$DASHBOARD_PORT/health"
echo "    curl http://localhost:$DASHBOARD_PORT/api/components"
echo ""
echo "  Protected route (auth required):"
echo "    curl -u developer:password http://localhost:$PROXY_PORT/health"
echo "    curl -u developer:password http://localhost:$PROXY_PORT/api/components"
echo ""
echo "Frontend setup:"
echo "  Set these environment variables when running the frontend:"
echo "    REACT_APP_PUBLIC_DOMAIN=http://localhost:$DASHBOARD_PORT"
echo "    REACT_APP_PROTECTED_DOMAIN=http://localhost:$PROXY_PORT"
echo ""
echo "Log files:"
echo "  Dashboard server: $DASHBOARD_LOG"
echo "  Mock oauth-proxy: $PROXY_LOG"
echo ""
echo "Dashboard server logs will be displayed in the terminal"
echo "Press Ctrl+C to stop"

set +e
while kill -0 $DASHBOARD_PID 2>/dev/null || kill -0 $PROXY_PID 2>/dev/null; do
  sleep 1
done
exit 0

