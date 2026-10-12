#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

echo "************ ship-status-dash e2e test command ************"

# Set up test environment
export KUBECTL_CMD="${KUBECTL_CMD:=oc}"
export E2E_NS="${E2E_NS:=ship-status-e2e}"
export TEST_DASHBOARD_CONFIG_PATH="${TEST_DASHBOARD_CONFIG_PATH:=dashboard-config}"
export TEST_COMPONENT_MONITOR_CONFIG_PATH="${TEST_COMPONENT_MONITOR_CONFIG_PATH:=component-monitor-config}"
export OPENSHIFT_CI="${OPENSHIFT_CI:=true}"
export ARTIFACT_DIR="${ARTIFACT_DIR:=/tmp/ship_status_artifacts}"
mkdir -p "${ARTIFACT_DIR}"
read -r -a KUBECTL_COMMAND <<<"${KUBECTL_CMD}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/e2e/scripts/port-forward-diagnostics.sh
source "${SCRIPT_DIR}/port-forward-diagnostics.sh"

PORT_FORWARD_NAMES=()
PORT_FORWARD_PIDS=()

start_port_forward() {
  local name="$1"
  local service="$2"
  local local_port="$3"
  local remote_port="$4"

  port_forward_start "${name}" "${service}" "${local_port}" "${remote_port}"
  PORT_FORWARD_NAMES+=("${name}")
  PORT_FORWARD_PIDS+=("${PORT_FORWARD_PID}")
}

# Get the dashboard, mock-oauth-proxy, and mock-monitored-component service ports for port forwarding
DASHBOARD_PORT=$("${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" get svc dashboard -o jsonpath='{.spec.ports[0].port}')
PROXY_PORT=$("${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" get svc mock-oauth-proxy -o jsonpath='{.spec.ports[0].port}')
MOCK_MONITORED_COMPONENT_PORT=$("${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" get svc mock-monitored-component -o jsonpath='{.spec.ports[0].port}')

# Set up port forwarding to the dashboard
echo "Setting up port forwarding to dashboard service..."
start_port_forward dashboard dashboard "${DASHBOARD_PORT}" "${DASHBOARD_PORT}"

# Set up port forwarding to the mock-oauth-proxy
echo "Setting up port forwarding to mock-oauth-proxy service..."
start_port_forward mock-oauth-proxy mock-oauth-proxy "${PROXY_PORT}" "${PROXY_PORT}"

# Set up port forwarding to the mock-monitored-component
echo "Setting up port forwarding to mock-monitored-component service..."
start_port_forward mock-monitored-component mock-monitored-component "${MOCK_MONITORED_COMPONENT_PORT}" "${MOCK_MONITORED_COMPONENT_PORT}"

# Wait for port forwarding to establish
sleep 5

# Test connection to dashboard
echo "Testing connection to public dashboard..."
DASHBOARD_HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${DASHBOARD_PORT}/health" --max-time 5 || echo "000")
echo "Dashboard HTTP status code: ${DASHBOARD_HTTP_STATUS}"

# Test connection to mock-oauth-proxy with basic auth
echo "Testing connection to mock-oauth-proxy with developer:developer credentials..."
PROXY_HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -u "developer:developer" "http://localhost:${PROXY_PORT}/health" --max-time 5 || echo "000")
echo "Mock oauth-proxy HTTP status code: ${PROXY_HTTP_STATUS}"

# Test connection to mock-monitored-component
echo "Testing connection to mock-monitored-component..."
MOCK_COMPONENT_HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${MOCK_MONITORED_COMPONENT_PORT}/health" --max-time 5 || echo "000")
echo "Mock-monitored-component HTTP status code: ${MOCK_COMPONENT_HTTP_STATUS}"

export TEST_SERVER_URL="http://localhost:${DASHBOARD_PORT}"
export TEST_MOCK_OAUTH_PROXY_URL="http://localhost:${PROXY_PORT}"
export TEST_MOCK_MONITORED_COMPONENT_URL="http://localhost:${MOCK_MONITORED_COMPONENT_PORT}"

# Get Prometheus service port and set up port forwarding
PROMETHEUS_PORT=$("${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" get svc prometheus -o jsonpath='{.spec.ports[0].port}')
echo "Setting up port forwarding to Prometheus service..."
start_port_forward prometheus prometheus "${PROMETHEUS_PORT}" "${PROMETHEUS_PORT}"

# Wait for port forwarding to establish
sleep 2

# Test connection to Prometheus
echo "Testing connection to Prometheus..."
PROMETHEUS_HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:${PROMETHEUS_PORT}/-/healthy" --max-time 5 || echo "000")
echo "Prometheus HTTP status code: ${PROMETHEUS_HTTP_STATUS}"

export TEST_PROMETHEUS_URL="http://localhost:${PROMETHEUS_PORT}"

set +e
go test ./test/e2e/... -count 1 -p 1 -timeout 20m
TEST_EXIT_CODE=$?
set -e

echo "E2E tests completed with exit code: ${TEST_EXIT_CODE}"

# Save logs from all services to artifacts for debugging
echo "Saving service logs to artifacts..."
export ARTIFACT_DIR="${ARTIFACT_DIR:=/tmp/ship_status_artifacts}"
mkdir -p "${ARTIFACT_DIR}"

echo "Saving mock-oauth-proxy logs..."
"${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" logs mock-oauth-proxy >"${ARTIFACT_DIR}/mock-oauth-proxy-test.log" || echo "Failed to get mock-oauth-proxy logs"

echo "Saving dashboard logs..."
"${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" logs dashboard >"${ARTIFACT_DIR}/dashboard-test.log" || echo "Failed to get dashboard logs"

echo "Saving mock-monitored-component logs..."
"${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" logs mock-monitored-component >"${ARTIFACT_DIR}/mock-monitored-component-test.log" || echo "Failed to get mock-monitored-component logs"

echo "Saving component-monitor logs..."
"${KUBECTL_COMMAND[@]}" -n "${E2E_NS}" logs component-monitor >"${ARTIFACT_DIR}/component-monitor-test.log" || echo "Failed to get component-monitor logs"

echo "Logs saved to ${ARTIFACT_DIR}/"

# Clean up port forwarding. The marker is written before signaling so the
# supervisor does not treat normal teardown as a forwarding failure.
for index in "${!PORT_FORWARD_PIDS[@]}"; do
  echo "Cleaning up ${PORT_FORWARD_NAMES[$index]} port forwarding..."
  port_forward_stop "${PORT_FORWARD_NAMES[$index]}" "${PORT_FORWARD_PIDS[$index]}"
done

# Cleanup: Delete the test namespace
echo "Cleaning up test namespace..."
"${KUBECTL_COMMAND[@]}" delete namespace "${E2E_NS}" --ignore-not-found=true --wait=false || echo "Failed to delete namespace, continuing..."

exit ${TEST_EXIT_CODE}
