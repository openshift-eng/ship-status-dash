#!/bin/bash

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

cleanup() {
  set +e
  echo ""
  echo "Cleaning up..."
  echo "Cleanup complete"
  exit 0
}

trap cleanup EXIT

echo "Starting component-monitor..."
COMPONENT_MONITOR_LOG="/tmp/component-monitor-local-dev.log"
echo "Component-monitor logs: $COMPONENT_MONITOR_LOG"

go run ./cmd/component-monitor --config-path hack/local/component-monitor/config.yaml --dashboard-url http://localhost:8080 --name local-component-monitor 2>&1 | tee "$COMPONENT_MONITOR_LOG" &
COMPONENT_MONITOR_PID=$!

echo ""
echo "✓ Component-monitor is running!"
echo ""
echo "Log file: $COMPONENT_MONITOR_LOG"
echo "Press Ctrl+C to stop"

set +e
while kill -0 $COMPONENT_MONITOR_PID 2>/dev/null; do
  sleep 1
done
exit 0

