#!/bin/bash

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

PROMETHEUS_CONTAINER_NAME="prometheus-local-dev"

cleanup() {
  set +e
  echo ""
  echo "Cleaning up..."
  if [ ! -z "$MOCK_COMPONENT_PID" ]; then
    kill -TERM $MOCK_COMPONENT_PID 2>/dev/null || true
    sleep 1
    kill -KILL $MOCK_COMPONENT_PID 2>/dev/null || true
  fi
  if [ ! -z "$COMPONENT_MONITOR_PID" ]; then
    kill -TERM $COMPONENT_MONITOR_PID 2>/dev/null || true
    sleep 1
    kill -KILL $COMPONENT_MONITOR_PID 2>/dev/null || true
  fi
  if podman ps -a --format "{{.Names}}" | grep -q "^${PROMETHEUS_CONTAINER_NAME}$"; then
    podman stop "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
    podman rm "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
  fi
  echo "Cleanup complete"
  exit 0
}

trap cleanup EXIT

echo "Starting mock-monitored-component on port 8081..."
go run ./cmd/mock-monitored-component --port 8081 > /dev/null 2>&1 &
MOCK_COMPONENT_PID=$!

echo "Waiting for mock-monitored-component to be ready..."
for i in {1..10}; do
  if curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo "✓ Mock-monitored-component is ready on port 8081"
    break
  fi
  if [ $i -eq 10 ]; then
    echo "✗ Mock-monitored-component failed to start"
    exit 1
  fi
  sleep 1
done

echo "Starting Prometheus in podman container..."
PROMETHEUS_CONFIG_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/prometheus.yml"
if podman ps -a --format "{{.Names}}" | grep -q "^${PROMETHEUS_CONTAINER_NAME}$"; then
  podman rm -f "$PROMETHEUS_CONTAINER_NAME" > /dev/null 2>&1 || true
fi

podman run -d \
  --name "$PROMETHEUS_CONTAINER_NAME" \
  -p 9090:9090 \
  -v "$PROMETHEUS_CONFIG_PATH:/etc/prometheus/prometheus.yml:ro" \
  quay.io/prometheus/prometheus:latest \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/prometheus \
  --web.console.libraries=/usr/share/prometheus/console_libraries \
  --web.console.templates=/usr/share/prometheus/consoles \
  --web.enable-lifecycle \
  > /dev/null 2>&1

echo "Waiting for Prometheus to complete initial scrape..."
for i in {1..60}; do
  if curl -s "http://localhost:9090/api/v1/query?query=success_rate" | grep -q "success_rate"; then
    echo "✓ Prometheus has completed initial scrape"
    break
  fi
  if [ $i -eq 60 ]; then
    echo "✗ Prometheus failed to complete initial scrape within 60 seconds"
    podman logs "$PROMETHEUS_CONTAINER_NAME" 2>/dev/null || true
    exit 1
  fi
  sleep 1
done

echo "Starting component-monitor..."
COMPONENT_MONITOR_LOG="/tmp/component-monitor-local-dev.log"
echo "Component-monitor logs: $COMPONENT_MONITOR_LOG"

go run ./cmd/component-monitor --config-path hack/local/component-monitor/config.yaml --dashboard-url http://localhost:8080 --name local-component-monitor 2>&1 | tee "$COMPONENT_MONITOR_LOG" &
COMPONENT_MONITOR_PID=$!

echo ""
echo "✓ Component-monitor is running!"
echo "✓ Mock-monitored-component is running on http://localhost:8081"
echo "✓ Prometheus is running on http://localhost:9090"
echo ""
echo "Log file: $COMPONENT_MONITOR_LOG"
echo "Press Ctrl+C to stop"

set +e
while kill -0 $COMPONENT_MONITOR_PID 2>/dev/null; do
  sleep 1
done
exit 0

