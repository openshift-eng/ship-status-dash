#!/bin/bash

# Helpers for supervising e2e port-forwards. The caller must set KUBECTL_CMD,
# E2E_NS, and ARTIFACT_DIR before starting a forward.

port_forward_kubectl() {
  local timeout_seconds="${PORT_FORWARD_DIAGNOSTIC_COMMAND_TIMEOUT_SECONDS:-5}"
  local -a kubectl_command

  read -r -a kubectl_command <<<"${KUBECTL_CMD}"
  timeout --foreground "${timeout_seconds}s" "${kubectl_command[@]}" "$@"
}

port_forward_capture_targets() {
  local name="$1"
  local service="$2"
  local target_file="${ARTIFACT_DIR}/port-forward-${name}-targets.txt"
  local output
  local status
  local target_count
  local jsonpath='{range .subsets[*].addresses[*]}{.targetRef.kind}{"/"}{.targetRef.name}{" uid="}{.targetRef.uid}{"\n"}{end}'

  set +e
  output=$(port_forward_kubectl -n "${E2E_NS}" get "endpoints/${service}" -o "jsonpath=${jsonpath}" 2>/dev/null)
  status=$?
  set -e

  if [ "${status}" -ne 0 ]; then
    echo "target resolution failed with exit code ${status}" >"${target_file}"
    return 0
  fi

  output=$(printf '%s\n' "${output}" | sed 's|^Pod/|pod/|' | sed '/^$/d')
  if [ -z "${output}" ]; then
    echo "target unresolved: service has no ready pod endpoints" >"${target_file}"
    return 0
  fi

  target_count=$(printf '%s\n' "${output}" | grep -c '^pod/' || true)
  if [ "${target_count}" -eq 1 ]; then
    echo "selection: single ready endpoint, this is the service port-forward target" >"${target_file}"
  else
    echo "selection: ${target_count} ready endpoint candidates, the client-selected target is not observable" >"${target_file}"
  fi
  printf '%s\n' "${output}" >>"${target_file}"
}

port_forward_collect_diagnostics() {
  local name="$1"
  local service="$2"
  local target_file="${ARTIFACT_DIR}/port-forward-${name}-targets.txt"
  local diagnostics_file="${ARTIFACT_DIR}/port-forward-${name}-failure-diagnostics.log"
  local max_targets="${PORT_FORWARD_DIAGNOSTIC_MAX_TARGETS:-4}"
  local target
  local target_uid
  local pod
  local event_selector
  local status
  local collected=0

  {
    echo "Port-forward ${name} to service/${service} exited unexpectedly."
    echo "Failure observed at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Recorded target identity:"
    sed 's/^/  /' "${target_file}" 2>/dev/null || echo "  target identity unavailable"

    while IFS=' ' read -r target target_uid; do
      case "${target}" in
        pod/*)
          if [ "${collected}" -ge "${max_targets}" ]; then
            echo "Diagnostic target limit (${max_targets}) reached."
            break
          fi
          pod="${target#pod/}"
          target_uid="${target_uid#uid=}"
          collected=$((collected + 1))

          echo "Pod status for ${pod}:"
          set +e
          port_forward_kubectl -n "${E2E_NS}" get "pod/${pod}" -o 'custom-columns=NAME:.metadata.name,UID:.metadata.uid,PHASE:.status.phase,REASON:.status.reason,DELETION:.metadata.deletionTimestamp,NODE:.spec.nodeName,START:.status.startTime,CONTAINERS:.status.containerStatuses[*].name,READY:.status.containerStatuses[*].ready,RESTARTS:.status.containerStatuses[*].restartCount,WAITING_REASON:.status.containerStatuses[*].state.waiting.reason,TERMINATED_REASON:.status.containerStatuses[*].state.terminated.reason,EXIT_CODE:.status.containerStatuses[*].state.terminated.exitCode,STARTED_AT:.status.containerStatuses[*].state.terminated.startedAt,FINISHED_AT:.status.containerStatuses[*].state.terminated.finishedAt' 2>/dev/null
          status=$?
          set -e
          if [ "${status}" -ne 0 ]; then
            echo "Pod status query failed with exit code ${status}."
          fi

          echo "Events for ${pod}:"
          event_selector="involvedObject.kind=Pod,involvedObject.name=${pod}"
          if [ -n "${target_uid}" ]; then
            event_selector="${event_selector},involvedObject.uid=${target_uid}"
          fi
          set +e
          port_forward_kubectl -n "${E2E_NS}" get events --field-selector "${event_selector}" --sort-by=.metadata.creationTimestamp -o 'custom-columns=TIME:.metadata.creationTimestamp,TYPE:.type,REASON:.reason,COUNT:.count,SOURCE:.source.component,MESSAGE:.message' 2>/dev/null
          status=$?
          set -e
          if [ "${status}" -ne 0 ]; then
            echo "Pod events query failed with exit code ${status}."
          fi
          ;;
      esac
    done <"${target_file}"

    if [ "${collected}" -eq 0 ]; then
      echo "No target pod identity was available for status or event collection."
    fi
  } >"${diagnostics_file}"
}

port_forward_run() {
  local name="$1"
  local service="$2"
  local local_port="$3"
  local remote_port="$4"
  local intentional_marker="${ARTIFACT_DIR}/port-forward-${name}-intentional-cleanup"
  local status_file="${ARTIFACT_DIR}/port-forward-${name}-exit-status.txt"
  local lifecycle_file="${ARTIFACT_DIR}/port-forward-${name}-lifecycle.log"
  local stdout_file="${ARTIFACT_DIR}/port-forward-${name}.stdout.log"
  local stderr_file="${ARTIFACT_DIR}/port-forward-${name}.stderr.log"
  local child_pid
  local status
  local second_status
  local classification
  local cleanup_requested=0
  local -a kubectl_command

  printf 'started_at=%s\nservice=%s\nnamespace=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${service}" "${E2E_NS}" >"${lifecycle_file}"

  read -r -a kubectl_command <<<"${KUBECTL_CMD}"
  "${kubectl_command[@]}" -n "${E2E_NS}" port-forward "service/${service}" "${local_port}:${remote_port}" >"${stdout_file}" 2>"${stderr_file}" &
  child_pid=$!
  trap 'cleanup_requested=1; kill "${child_pid}" 2>/dev/null || true' TERM INT HUP
  port_forward_capture_targets "${name}" "${service}"

  if wait "${child_pid}"; then
    status=0
  else
    status=$?
  fi
  if [ "${cleanup_requested}" -eq 1 ]; then
    if wait "${child_pid}" 2>/dev/null; then
      status=0
    else
      second_status=$?
      # If the first wait already reaped the child, retain that status.
      if [ "${second_status}" -ne 127 ]; then
        status="${second_status}"
      fi
    fi
  fi
  trap - TERM INT HUP

  printf '%s\n' "${status}" >"${status_file}.tmp"
  mv "${status_file}.tmp" "${status_file}"

  if [ -f "${intentional_marker}" ]; then
    classification="intentional-cleanup"
  else
    classification="unexpected-exit"
  fi
  printf 'finished_at=%s\nexit_status=%s\nclassification=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${status}" "${classification}" >>"${lifecycle_file}"

  if [ "${classification}" = "unexpected-exit" ]; then
    port_forward_collect_diagnostics "${name}" "${service}" || true
  fi

  return "${status}"
}

port_forward_start() {
  local name="$1"
  local service="$2"
  local local_port="$3"
  local remote_port="$4"

  mkdir -p "${ARTIFACT_DIR}"
  rm -f \
    "${ARTIFACT_DIR}/port-forward-${name}-intentional-cleanup" \
    "${ARTIFACT_DIR}/port-forward-${name}-exit-status.txt" \
    "${ARTIFACT_DIR}/port-forward-${name}-exit-status.txt.tmp" \
    "${ARTIFACT_DIR}/port-forward-${name}-lifecycle.log" \
    "${ARTIFACT_DIR}/port-forward-${name}-targets.txt" \
    "${ARTIFACT_DIR}/port-forward-${name}-failure-diagnostics.log" \
    "${ARTIFACT_DIR}/port-forward-${name}.stdout.log" \
    "${ARTIFACT_DIR}/port-forward-${name}.stderr.log"
  port_forward_run "${name}" "${service}" "${local_port}" "${remote_port}" &
  # shellcheck disable=SC2034 # This is the return value consumed by the caller.
  PORT_FORWARD_PID=$!
}

port_forward_stop() {
  local name="$1"
  local pid="$2"
  local intentional_marker="${ARTIFACT_DIR}/port-forward-${name}-intentional-cleanup"
  local status_file="${ARTIFACT_DIR}/port-forward-${name}-exit-status.txt"

  if [ ! -f "${status_file}" ] && kill -0 "${pid}" 2>/dev/null; then
    : >"${intentional_marker}"
    kill "${pid}" 2>/dev/null || true
  fi
  wait "${pid}" 2>/dev/null || true
}
