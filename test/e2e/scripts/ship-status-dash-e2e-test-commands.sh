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

# Get service ports for port forwarding
DASHBOARD_PORT=$(${KUBECTL_CMD} -n ${E2E_NS} get svc dashboard -o jsonpath='{.spec.ports[0].port}')
PROXY_PORT=$(${KUBECTL_CMD} -n ${E2E_NS} get svc mock-oauth-proxy -o jsonpath='{.spec.ports[0].port}')
MOCK_MONITORED_COMPONENT_PORT=$(${KUBECTL_CMD} -n ${E2E_NS} get svc mock-monitored-component -o jsonpath='{.spec.ports[0].port}')
POSTGRES_PORT=$(${KUBECTL_CMD} -n ${E2E_NS} get svc postgres -o jsonpath='{.spec.ports[0].port}')
POSTGRES_LOCAL_PORT=15432

# Set up port forwarding to the dashboard
echo "Setting up port forwarding to dashboard service..."
${KUBECTL_CMD} -n ${E2E_NS} port-forward svc/dashboard ${DASHBOARD_PORT}:${DASHBOARD_PORT} >"${ARTIFACT_DIR}/dashboard-port-forward.stdout.log" 2>"${ARTIFACT_DIR}/dashboard-port-forward.stderr.log" &
DASHBOARD_PORT_FORWARD_PID=$!

# Set up port forwarding to the mock-oauth-proxy
echo "Setting up port forwarding to mock-oauth-proxy service..."
${KUBECTL_CMD} -n ${E2E_NS} port-forward svc/mock-oauth-proxy ${PROXY_PORT}:${PROXY_PORT} >"${ARTIFACT_DIR}/mock-oauth-proxy-port-forward.stdout.log" 2>"${ARTIFACT_DIR}/mock-oauth-proxy-port-forward.stderr.log" &
PROXY_PORT_FORWARD_PID=$!

# Set up port forwarding to the mock-monitored-component
echo "Setting up port forwarding to mock-monitored-component service..."
${KUBECTL_CMD} -n ${E2E_NS} port-forward svc/mock-monitored-component ${MOCK_MONITORED_COMPONENT_PORT}:${MOCK_MONITORED_COMPONENT_PORT} >"${ARTIFACT_DIR}/mock-monitored-component-port-forward.stdout.log" 2>"${ARTIFACT_DIR}/mock-monitored-component-port-forward.stderr.log" &
MOCK_MONITORED_COMPONENT_PORT_FORWARD_PID=$!

# Set up port forwarding to the ephemeral test database for fixture setup
echo "Setting up port forwarding to postgres service..."
${KUBECTL_CMD} -n ${E2E_NS} port-forward svc/postgres ${POSTGRES_LOCAL_PORT}:${POSTGRES_PORT} >"${ARTIFACT_DIR}/postgres-port-forward.stdout.log" 2>"${ARTIFACT_DIR}/postgres-port-forward.stderr.log" &
POSTGRES_PORT_FORWARD_PID=$!

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
export TEST_DATABASE_DSN="postgres://postgres:testpass@localhost:${POSTGRES_LOCAL_PORT}/ship_status_test?sslmode=disable&client_encoding=UTF8"

# Get Prometheus service port and set up port forwarding
PROMETHEUS_PORT=$(${KUBECTL_CMD} -n ${E2E_NS} get svc prometheus -o jsonpath='{.spec.ports[0].port}')
echo "Setting up port forwarding to Prometheus service..."
${KUBECTL_CMD} -n ${E2E_NS} port-forward svc/prometheus ${PROMETHEUS_PORT}:${PROMETHEUS_PORT} >"${ARTIFACT_DIR}/prometheus-port-forward.stdout.log" 2>"${ARTIFACT_DIR}/prometheus-port-forward.stderr.log" &
PROMETHEUS_PORT_FORWARD_PID=$!

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

# Snapshot forwarding state without waiting on forwards that are still live.
PORT_FORWARD_STATUS_FILE="${ARTIFACT_DIR}/port-forward-status.log"
: >"${PORT_FORWARD_STATUS_FILE}"
for port_forward in \
  "dashboard:${DASHBOARD_PORT_FORWARD_PID}" \
  "mock-oauth-proxy:${PROXY_PORT_FORWARD_PID}" \
  "mock-monitored-component:${MOCK_MONITORED_COMPONENT_PORT_FORWARD_PID}" \
  "postgres:${POSTGRES_PORT_FORWARD_PID}" \
  "prometheus:${PROMETHEUS_PORT_FORWARD_PID}"; do
  port_forward_name="${port_forward%%:*}"
  port_forward_pid="${port_forward##*:}"
  if kill -0 "${port_forward_pid}" 2>/dev/null; then
    echo "${port_forward_name}: running at end-of-test snapshot (pid ${port_forward_pid})" >>"${PORT_FORWARD_STATUS_FILE}"
  else
    if wait "${port_forward_pid}" 2>/dev/null; then
      echo "${port_forward_name}: exited before end-of-test snapshot (exit code 0)" >>"${PORT_FORWARD_STATUS_FILE}"
    else
      port_forward_exit_code=$?
      if [ "${port_forward_exit_code}" -eq 127 ]; then
        echo "${port_forward_name}: exited before end-of-test snapshot (exit code unavailable)" >>"${PORT_FORWARD_STATUS_FILE}"
      else
        echo "${port_forward_name}: exited before end-of-test snapshot (exit code ${port_forward_exit_code})" >>"${PORT_FORWARD_STATUS_FILE}"
      fi
    fi
    case "${port_forward_name}" in
      dashboard) DASHBOARD_PORT_FORWARD_PID="" ;;
      mock-oauth-proxy) PROXY_PORT_FORWARD_PID="" ;;
      mock-monitored-component) MOCK_MONITORED_COMPONENT_PORT_FORWARD_PID="" ;;
      postgres) POSTGRES_PORT_FORWARD_PID="" ;;
      prometheus) PROMETHEUS_PORT_FORWARD_PID="" ;;
    esac
  fi
done

# These status-only snapshots avoid pod specs and environment values.
if ! timeout 10s ${KUBECTL_CMD} -n ${E2E_NS} get \
  pod/dashboard pod/mock-oauth-proxy pod/mock-monitored-component pod/postgres pod/prometheus \
  -o 'custom-columns=NAME:.metadata.name,UID:.metadata.uid,PHASE:.status.phase,DELETION:.metadata.deletionTimestamp,READY:.status.containerStatuses[*].ready,RESTARTS:.status.containerStatuses[*].restartCount,WAITING_REASON:.status.containerStatuses[*].state.waiting.reason,TERMINATED_REASON:.status.containerStatuses[*].state.terminated.reason,EXIT_CODE:.status.containerStatuses[*].state.terminated.exitCode' \
  >"${ARTIFACT_DIR}/port-forward-target-pods.log" 2>/dev/null; then
  echo "Target pod status unavailable at end-of-test snapshot." >"${ARTIFACT_DIR}/port-forward-target-pods.log"
fi

if ! timeout 10s ${KUBECTL_CMD} -n ${E2E_NS} get events --field-selector involvedObject.kind=Pod --sort-by=.metadata.creationTimestamp \
  -o 'custom-columns=TIME:.metadata.creationTimestamp,OBJECT:.involvedObject.name,TYPE:.type,REASON:.reason,COUNT:.count,MESSAGE:.message' 2>/dev/null \
  | awk 'NR == 1 || $2 == "dashboard" || $2 == "mock-oauth-proxy" || $2 == "mock-monitored-component" || $2 == "prometheus"' \
  >"${ARTIFACT_DIR}/port-forward-target-events.log"; then
  echo "Target pod events unavailable at end-of-test snapshot." >"${ARTIFACT_DIR}/port-forward-target-events.log"
fi

# Save logs from all services to artifacts for debugging
echo "Saving service logs to artifacts..."

echo "Saving mock-oauth-proxy logs..."
${KUBECTL_CMD} -n ${E2E_NS} logs mock-oauth-proxy > ${ARTIFACT_DIR}/mock-oauth-proxy-test.log || echo "Failed to get mock-oauth-proxy logs"

echo "Saving dashboard logs..."
${KUBECTL_CMD} -n ${E2E_NS} logs dashboard > ${ARTIFACT_DIR}/dashboard-test.log || echo "Failed to get dashboard logs"

echo "Saving mock-monitored-component logs..."
${KUBECTL_CMD} -n ${E2E_NS} logs mock-monitored-component > ${ARTIFACT_DIR}/mock-monitored-component-test.log || echo "Failed to get mock-monitored-component logs"

echo "Saving component-monitor logs..."
${KUBECTL_CMD} -n ${E2E_NS} logs component-monitor > ${ARTIFACT_DIR}/component-monitor-test.log || echo "Failed to get component-monitor logs"

echo "Logs saved to ${ARTIFACT_DIR}/"

# Clean up port forwarding
if [ ! -z "${DASHBOARD_PORT_FORWARD_PID:-}" ]; then
  echo "Cleaning up dashboard port forwarding..."
  kill $DASHBOARD_PORT_FORWARD_PID 2>/dev/null || true
fi
if [ ! -z "${PROXY_PORT_FORWARD_PID:-}" ]; then
  echo "Cleaning up mock-oauth-proxy port forwarding..."
  kill $PROXY_PORT_FORWARD_PID 2>/dev/null || true
fi
if [ ! -z "${MOCK_MONITORED_COMPONENT_PORT_FORWARD_PID:-}" ]; then
  echo "Cleaning up mock-monitored-component port forwarding..."
  kill $MOCK_MONITORED_COMPONENT_PORT_FORWARD_PID 2>/dev/null || true
fi
if [ ! -z "${POSTGRES_PORT_FORWARD_PID:-}" ]; then
  echo "Cleaning up postgres port forwarding..."
  kill $POSTGRES_PORT_FORWARD_PID 2>/dev/null || true
fi
if [ ! -z "${PROMETHEUS_PORT_FORWARD_PID:-}" ]; then
  echo "Cleaning up Prometheus port forwarding..."
  kill $PROMETHEUS_PORT_FORWARD_PID 2>/dev/null || true
fi

# Cleanup: Delete the test namespace
echo "Cleaning up test namespace..."
${KUBECTL_CMD} delete namespace ${E2E_NS} --ignore-not-found=true --wait=false || echo "Failed to delete namespace, continuing..."

exit ${TEST_EXIT_CODE}
