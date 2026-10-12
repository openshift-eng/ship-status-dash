#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=test/e2e/scripts/port-forward-diagnostics.sh
source "${SCRIPT_DIR}/port-forward-diagnostics.sh"

TEST_ROOT=$(mktemp -d)
trap 'rm -rf "${TEST_ROOT}"' EXIT

export KUBECTL_CMD="${TEST_ROOT}/kubectl"
export E2E_NS="test-namespace"
export PORT_FORWARD_DIAGNOSTIC_COMMAND_TIMEOUT_SECONDS=1

cat >"${KUBECTL_CMD}" <<'EOF'
#!/bin/bash
set -o nounset
set -o errexit

case " $* " in
  *" get svc dashboard "*)
    echo 8080
    ;;
  *" get svc mock-oauth-proxy "*)
    echo 8443
    ;;
  *" get svc mock-monitored-component "*)
    echo 9000
    ;;
  *" get svc prometheus "*)
    echo 9090
    ;;
  *" get endpoints/"*)
    echo "Pod/dashboard-pod uid=pod-uid"
    ;;
  *" port-forward "*)
    echo "forward stdout"
    echo "forward stderr" >&2
    if [ "${MOCK_MODE}" = "cleanup" ] || [ "${MOCK_MODE}" = "consumer" ]; then
      : > "${MOCK_STARTED_FILE}"
      trap 'exit 37' TERM INT HUP
      while true; do /usr/bin/sleep 1; done
    elif [ "${MOCK_MODE}" = "diagnostics-timeout" ]; then
      exit 19
    elif [ "${MOCK_MODE}" = "diagnostics-fail" ]; then
      exit 17
    else
      exit 42
    fi
    ;;
  *" get pod/"*)
    if [ "${MOCK_MODE}" = "diagnostics-timeout" ]; then
      sleep 30
    fi
    if [ "${MOCK_MODE}" = "diagnostics-fail" ]; then
      exit 9
    fi
    echo "NAME UID PHASE REASON DELETION NODE START READY RESTARTS WAITING_REASON TERMINATED_REASON EXIT_CODE STARTED_AT FINISHED_AT"
    echo "dashboard-pod pod-uid Failed Error <none> worker-a now false 1 CrashLoopBackOff Error 1 before after"
    ;;
  *" get events "*)
    if [ "${MOCK_MODE}" = "diagnostics-timeout" ]; then
      sleep 30
    fi
    if [ "${MOCK_MODE}" = "diagnostics-fail" ]; then
      exit 8
    fi
    echo "TIME TYPE REASON COUNT SOURCE MESSAGE"
    echo "now Warning BackOff 2 kubelet container-restarted"
    ;;
  *" logs "*)
    echo "service log"
    ;;
  *" delete namespace "*)
    : > "${MOCK_NAMESPACE_DELETED_FILE}"
    ;;
  *)
    echo "unexpected kubectl arguments: $*" >&2
    exit 99
    ;;
esac
EOF
chmod +x "${KUBECTL_CMD}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_file_contains() {
  local file="$1"
  local expected="$2"
  grep -Fq "${expected}" "${file}" || fail "${file} does not contain: ${expected}"
}

test_unexpected_exit_collects_diagnostics() {
  export ARTIFACT_DIR="${TEST_ROOT}/unexpected"
  export MOCK_MODE="unexpected"

  port_forward_start dashboard dashboard 8080 8080
  local pid="${PORT_FORWARD_PID}"
  local status=0
  wait "${pid}" || status=$?

  [ "${status}" -eq 42 ] || fail "unexpected forward status was ${status}, want 42"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-exit-status.txt" "42"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-lifecycle.log" "classification=unexpected-exit"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-targets.txt" "pod/dashboard-pod uid=pod-uid"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "dashboard-pod pod-uid Failed"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "Warning BackOff"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard.stderr.log" "forward stderr"
}

test_intentional_cleanup_skips_failure_diagnostics() {
  export ARTIFACT_DIR="${TEST_ROOT}/cleanup"
  export MOCK_MODE="cleanup"
  export MOCK_STARTED_FILE="${TEST_ROOT}/cleanup-forward-started"
  mkdir -p "${ARTIFACT_DIR}"
  echo 99 >"${ARTIFACT_DIR}/port-forward-dashboard-exit-status.txt"
  echo stale >"${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log"

  port_forward_start dashboard dashboard 8080 8080
  local pid="${PORT_FORWARD_PID}"
  for _ in $(seq 1 50); do
    [ -f "${MOCK_STARTED_FILE}" ] && break
    sleep 0.02
  done
  [ -f "${MOCK_STARTED_FILE}" ] || fail "cleanup forward did not start"

  port_forward_stop dashboard "${pid}"

  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-lifecycle.log" "classification=intentional-cleanup"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-exit-status.txt" "37"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-lifecycle.log" "exit_status=37"
  [ ! -e "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" ] || fail "intentional cleanup produced failure diagnostics"
}

test_diagnostic_failure_preserves_forward_status() {
  export ARTIFACT_DIR="${TEST_ROOT}/diagnostics-fail"
  export MOCK_MODE="diagnostics-fail"

  port_forward_start dashboard dashboard 8080 8080
  local pid="${PORT_FORWARD_PID}"
  local status=0
  wait "${pid}" || status=$?

  [ "${status}" -eq 17 ] || fail "diagnostic failure changed forward status to ${status}, want 17"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-exit-status.txt" "17"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "Pod status query failed with exit code 9."
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "Pod events query failed with exit code 8."
}

test_unavailable_api_is_bounded() {
  export ARTIFACT_DIR="${TEST_ROOT}/diagnostics-timeout"
  export MOCK_MODE="diagnostics-timeout"
  local started_at="${SECONDS}"

  port_forward_start dashboard dashboard 8080 8080
  local pid="${PORT_FORWARD_PID}"
  local status=0
  wait "${pid}" || status=$?
  local elapsed=$((SECONDS - started_at))

  [ "${status}" -eq 19 ] || fail "timed-out diagnostics changed forward status to ${status}, want 19"
  [ "${elapsed}" -lt 5 ] || fail "timed-out diagnostics took ${elapsed}s, want less than 5s"
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "Pod status query failed with exit code 124."
  assert_file_contains "${ARTIFACT_DIR}/port-forward-dashboard-failure-diagnostics.log" "Pod events query failed with exit code 124."
}

test_consumer_preserves_test_status_and_artifacts() {
  local consumer_bin="${TEST_ROOT}/consumer-bin"
  local consumer_output="${TEST_ROOT}/consumer-output.log"
  local status=0
  local name
  mkdir -p "${consumer_bin}"

  cat >"${consumer_bin}/go" <<'EOF'
#!/bin/bash
exit 29
EOF
  cat >"${consumer_bin}/curl" <<'EOF'
#!/bin/bash
printf 200
EOF
  cat >"${consumer_bin}/sleep" <<'EOF'
#!/bin/bash
exit 0
EOF
  chmod +x "${consumer_bin}/go" "${consumer_bin}/curl" "${consumer_bin}/sleep"

  export ARTIFACT_DIR="${TEST_ROOT}/consumer"
  export MOCK_MODE="consumer"
  export MOCK_STARTED_FILE="${TEST_ROOT}/consumer-forward-started"
  export MOCK_NAMESPACE_DELETED_FILE="${TEST_ROOT}/consumer-namespace-deleted"

  set +e
  PATH="${consumer_bin}:${PATH}" bash "${SCRIPT_DIR}/ship-status-dash-e2e-test-commands.sh" >"${consumer_output}" 2>&1
  status=$?
  set -e

  [ "${status}" -eq 29 ] || fail "consumer returned ${status}, want original test status 29"
  [ -f "${MOCK_NAMESPACE_DELETED_FILE}" ] || fail "consumer did not request namespace cleanup"
  assert_file_contains "${ARTIFACT_DIR}/dashboard-test.log" "service log"
  assert_file_contains "${ARTIFACT_DIR}/component-monitor-test.log" "service log"
  for name in dashboard mock-oauth-proxy mock-monitored-component prometheus; do
    assert_file_contains "${ARTIFACT_DIR}/port-forward-${name}-exit-status.txt" "37"
    assert_file_contains "${ARTIFACT_DIR}/port-forward-${name}-lifecycle.log" "classification=intentional-cleanup"
    [ ! -e "${ARTIFACT_DIR}/port-forward-${name}-failure-diagnostics.log" ] || fail "consumer cleanup produced ${name} failure diagnostics"
  done
}

test_unexpected_exit_collects_diagnostics
test_intentional_cleanup_skips_failure_diagnostics
test_diagnostic_failure_preserves_forward_status
test_unavailable_api_is_bounded
test_consumer_preserves_test_status_and_artifacts

echo "port-forward diagnostics tests passed"
