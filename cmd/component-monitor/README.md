# Component Monitor

The component-monitor is a service that periodically probes sub-components to detect outages and report their status to the dashboard API.

## Overview

The component-monitor supports these types of monitoring:

1. **HTTP Monitoring**: Probes HTTP endpoints and checks for expected status codes
2. **Prometheus Monitoring**: Executes Prometheus queries (both instant and range queries) to check component health
3. **JUnit Monitoring**: Fetches Prow JUnit XML from GCS (or the GCSweb URL style) for named jobs and derives health from that file
4. **Systemd Monitoring** (if enabled in config): Probes a systemd unit on a host
5. **Jira Monitoring**: Searches Jira Cloud for matching issues and reports each as a probe reason

## Architecture

The component-monitor runs as a standalone service that:
- Loads configuration from a YAML file
- Creates probers for each configured component/sub-component
- Wakes on the instance `frequency`, starts only probes that are due, and reports those results
- Sends probe results to the dashboard API via HTTP POST requests
- Does not expose any HTTP endpoints itself (only makes outbound requests)

## Probe scheduling

Instance `frequency` (required at the top of the YAML) is the orchestrator tick and the default cadence for every component entry. Each cycle the orchestrator starts only probes whose cadence has elapsed, waits up to the instance `frequency` for those results, then sleeps until the next tick.

An entry may set its own `frequency` as a sibling of the `*_monitor` blocks to run less often than the instance default:

```yaml
frequency: 5m
components:
  - component_slug: "prow"
    sub_component_slug: "deck"
    http_monitor:
      url: "https://prow.ci.openshift.org/"
      code: 200
      retry_after: 4m
    # inherits 5m; HTTP and Prometheus on this entry share that cadence
  - component_slug: "trt-incidents"
    sub_component_slug: "incidents"
    frequency: 30m
    jira_monitor:
      url: "https://redhat.atlassian.net"
      jql: 'labels = trt-incident'
```

Rules:

- An entry override must be a positive duration at least the instance `frequency`. Faster polling means lowering the instance value. Faster than the tick cannot work: collection timeout and HTTP `retry_after` are bounded by the tick (production HTTP uses `retry_after: 4m` inside a 5m tick).
- HTTP `retry_after` must not exceed that entry's resolved frequency (instance default or override).
- Every `*_monitor` on the same YAML entry shares the resolved frequency.
- Multiple YAML entries for the same `component_slug` / `sub_component_slug` (for example two build-farm `build01` items) must resolve to the same frequency. A report is the dashboard's full picture for that sub-component. If one entry ran and the other did not, the dashboard could auto-resolve outages from the missing probe.

After a successful probe, that prober is not due again until its frequency elapses from the start of that cycle, not when the probe finished. A probe that occupies part of the tick (HTTP `retry_after`, for example) still runs on the next tick at that frequency. A failed probe does not count as a run, so it retries on the next instance tick.

Reports include only probes that ran. If no probes are due, or every due result is omitted (a probe error, for example), the orchestrator does not POST. An empty `statuses` list is rejected by the dashboard. Dry-run (`--dry-run`) still runs every prober once and ignores the schedule.

Set the dashboard sub-component `monitoring.frequency` to the same resolved cadence so the absent-report checker (threshold 5x frequency) does not treat the gap between runs as a missing monitor.

## JUnit monitor (`junit_monitor`)

Use this for Prow jobs that write a JUnit file into `test-platform-results-public` (or another GCS bucket), under `logs/<job>/`. Every monitor names its jobs and the artifact path. There is no default job list or artifact path.

If omitted, **junit_monitor.severity** defaults to **Degraded**.

**Fields:**

- `job_names` (required): Prow job names. The prober does not discover jobs. Each job contributes up to `history_runs` recent finished builds.
- `artifact_path` (required): literal object path under the build id, shared by every `job_names` entry. The prober does not substitute a per-job test directory. For example `artifacts/junit_canary.xml`, or install JUnit at `artifacts/<test-dir>/gather-must-gather/artifacts/junit_install.xml`. Jobs whose `<test-dir>` differs need separate monitors.
- `testcase_prefix`: when set, only testcase names with this prefix are scored. A file with no matching case is skipped. If every looked-at build in the job is skipped that way, the probe returns an error. When omitted, any failed case counts, and a file with zero tests is a failure.
- `alert_on_no_recent_run`: when true, the newest finished build whose `started.json` is older than `max_age` opens an outage. An unfinished id in `latest-build.txt` is skipped first. `max_age` is required then. Build-farm monitors set this. It cannot be combined with `group_by: lease-resource-prefix`.
- `missing_artifact_unhealthy`: when true (the default), a finished build with no `artifact_path` is a failure pattern and can meet `failed_runs_threshold`. When false, that build is left out of the count. If that leaves a job with no scored run, the probe returns an error instead of Healthy. A `testcase_prefix` that matches nothing in the window does the same.
- `group_by`: group key. `Reason.Check` is that key. Omitted means `job`.
  - `job`: one group per Prow job. `failed_runs_threshold` is a shared failure signature.
  - `lease-resource-prefix`: read `build-log.txt` until the first `Acquired <n> lease(s) for <type>: [<names>]` line, or 8 MiB. The key is the first `--` field when the leased name has at least three fields (`aws-2--us-east-2--openshift-org-aws-quota-slice-210` is `aws-2`). A UUID or any other name that does not yield a key is skipped. Runs that share a key are one group, ordered by `started.json` (newest first). Without `latest_run_group_window`, `failed_runs_threshold` is the length of the consecutive failure streak and a pass ends the streak. With that window, failures inside the cohort are counted and a pass does not end the count.
- `max_age`: optional. Finished builds whose `started.json` is older than this are dropped after `history_runs` is applied, and that does not open an outage. When `alert_on_no_recent_run` is true, the same duration opens an outage when the newest finished build is older than this. An unfinished latest build is not that check. This duration is measured from now.
- `latest_run_group_window`: optional duration, for example `30m`. Empty keeps today's rules. When set, each group keeps runs whose `started.json` is within this duration of the newest run in that group. It is applied after `max_age`. A pass in the cohort does not cancel a failure. For `lease-resource-prefix`, the account is unhealthy when the failure count in the cohort is at least `failed_runs_threshold`. For `job`, the shared failure signature is computed only on the cohort. A passing run is ignored while that signature is counted, so an older failure in the cohort still counts.
- `history_runs` / `failed_runs_threshold`: for `group_by: job`, Y runs in a group's last N must share one failure signature. For `lease-resource-prefix` without `latest_run_group_window`, the newest Y runs must be failures until a pass. With the window, Y is the number of failures in the cohort. A missing file counts only when `missing_artifact_unhealthy` is true.

Finished builds are cached in memory by job and build id. If a job cannot be read and no readable job is unhealthy, the probe returns an error. The orchestrator omits that sub-component from the report, and the absent-report checker can open an outage if the errors continue. An unhealthy job is still reported when a sibling job cannot be read.

`LOG_LEVEL=debug make component-monitor-dry-run` prints one debug line per JUnit fetch: the artifact URL, whether it was found, and, when `testcase_prefix` matches, the matched test names.

**What each job reads (no build-cluster login):**

1. **`latest-build.txt`** at `logs/<job>/`: a single line with the current Prow build id.
2. **`started.json`** for the build being evaluated. An unfinished latest build is skipped, and so is a later build with no `finished.json` while it is still inside `max_age` (logged as a warning). The previous build is checked as long as it remains in that window. When `alert_on_no_recent_run` is true and that finished build's start time is older than `max_age`, the probe reports unhealthy, regardless of JUnit. When `max_age` is set without that flag, an older build is dropped.
3. The configured `artifact_path` for that id. With `group_by: lease-resource-prefix`, also `build-log.txt`.

**Single run (`history_runs: 1`, default):** the latest finished build is the only JUnit read. Failing means zero total JUnit tests, any failed testcase, or a missing file on a finished build when `missing_artifact_unhealthy` is true. `failed_runs_threshold` is ignored (internally 1).

**History (`history_runs` N > 1):** the prober takes up to N recent build ids per job (GCS list API, merged with `latest-build.txt`, then sorted), drops any build older than `max_age` when that field is set, fetches JUnit for each remaining run, and classifies it. `group_by: job` then applies the signature rule. `failed_runs_threshold` (Y) requires the same failure pattern, not Y arbitrary red runs in N:

- A failure pattern is the sorted set of failed testcase `name` values, a shared bucket for runs with no JUnit tests, or a missing artifact on a finished build when that counts.
- Let K be the size of the largest set of runs in the group that share the identical pattern.
- If K is at least Y, that group is unhealthy. Otherwise it is healthy.

When `group_by` is `job`, the sub-component is unhealthy if any job is unhealthy. When `group_by` is `lease-resource-prefix`, runs in a key are ordered by `started.json`. Without `latest_run_group_window`, the streak is the newest failures until a pass, and the key is unhealthy when that streak is at least Y. With the window, the key is unhealthy when the cohort contains at least Y failures. Each such key is its own reason. Set `monitoring.outage_per_reason: true` on the dashboard sub-component so each key opens its own outage.

`max_age` and `latest_run_group_window` stack. `max_age` is measured from now and runs first. `latest_run_group_window` is measured from the newest surviving run in the group and runs second.

For one job the prober takes up to `history_runs` finished builds, drops any whose `started.json` is older than `max_age`, groups what remains, then applies the window inside each group.

Example: `max_age: 48h`, `history_runs: 20`, `latest_run_group_window: 30m`, `failed_runs_threshold: 1`. The probe runs at 10:00.

- A finished run that started at 08:00 yesterday is inside 48 hours, so `max_age` keeps it. It is 26 hours before the newest run below, so the window ignores it.
- Account `aws-2` passed a run that started at 09:50 today. That is the newest run in the group, so it is the anchor.
- The same account failed a run that started at 09:35. That is 15 minutes before the anchor, inside the window, so the pass does not clear it. Threshold 1 reports the account unhealthy.
- The same account failed a run that started at 08:00 today. `max_age` keeps it, and it is inside the 20 finished builds, but it is 1 hour 50 minutes before the anchor, so the window ignores it.
- A run that started 3 days ago never reaches the window. `max_age` already dropped it. That drop does not open an outage.
- If every finished run is older than 48 hours, the window has nothing to anchor on. Those builds are dropped and the probe stays healthy. `alert_on_no_recent_run` still uses `max_age` on the newest finished build, separate from this window.

**Example (build-farm job):**

```yaml
junit_monitor:
  job_names:
    - "periodic-build-farm-canary-build11"
  artifact_path: "artifacts/junit_canary.xml"
  alert_on_no_recent_run: true
  missing_artifact_unhealthy: true
  gcs_bucket: "test-platform-results-public"  # default if omitted
  max_age: "2h"
  severity: "Degraded"
  artifact_url_style: "gcs"            # or "gcsweb" for the app.ci GCSweb host
  history_runs: 5
  failed_runs_threshold: 3            # 3+ runs in last 5 must share one failure pattern
```

**Example (jobs that share one install artifact path, grouped by leased account):**

`periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn` and `periodic-ci-openshift-release-main-ci-5.0-e2e-aws-ovn` both publish install JUnit at `artifacts/e2e-aws-ovn/gather-must-gather/artifacts/junit_install.xml`, so they can share one monitor. A job whose test directory is not `e2e-aws-ovn` needs its own `junit_monitor` and its own `artifact_path`. `artifacts/gather-must-gather/junit_install.xml` is not a real object.

```yaml
junit_monitor:
  job_names:
    - "periodic-ci-openshift-release-main-ci-5.1-e2e-aws-ovn"
    - "periodic-ci-openshift-release-main-ci-5.0-e2e-aws-ovn"
  artifact_path: "artifacts/e2e-aws-ovn/gather-must-gather/artifacts/junit_install.xml"
  testcase_prefix: "install should succeed:"
  missing_artifact_unhealthy: false
  group_by: "lease-resource-prefix"
  max_age: "48h"
  severity: "Degraded"
  history_runs: 15
  failed_runs_threshold: 3
```

## Jira monitor (`jira_monitor`)

Use this to surface matching Jira issues as dashboard outages. Only the issue summary is sent as `Reason.Results` and stored on `Outage.Description`. The Jira description body is never copied.

Requests are unauthenticated. The JQL must match issues that anonymous callers can Browse (for example TRT and OCPBUGS on `redhat.atlassian.net`). Restricted issues are omitted by Jira itself. Do not add Jira credentials to the component-monitor.

If omitted, **jira_monitor.severity** defaults to **Degraded**. `jql` is required.

Jira JQL is often slower than HTTP or Prometheus. Set `frequency` on the component entry (sibling of `jira_monitor`) as described in [Probe scheduling](#probe-scheduling). Set the dashboard sub-component `monitoring.frequency` to the same value.

Dashboard sub-components that should create one outage per Jira issue must set `monitoring.outage_per_reason: true` (with `auto_resolve: true`). Without that flag, the dashboard still creates at most one active outage per sub-component.

The example JQL matches the Refinement and In Progress columns on the [TRT Incidents board](https://redhat.atlassian.net/jira/software/c/projects/TRT/boards/10048). `statusCategory != Done` is too broad: statuses such as `ON_QA` are still "In Progress" in Jira but sit in that board's Done column.

**Example:**

```yaml
- component_slug: "trt-incidents"
  sub_component_slug: "incidents"
  frequency: 5m
  jira_monitor:
    url: "https://redhat.atlassian.net"
    jql: 'labels = trt-incident AND status in ("Planning", "New", "Refinement", "To Do", "Approved", "Dev Complete", "Review", "In Progress", "Testing", "ASSIGNED", "POST")'
    severity: "Degraded"
```

## Configuration

The component-monitor is configured via command-line flags and a YAML configuration file:

**Command-Line Flags:**
- `--config-path` (required): Path to the component monitor configuration file (YAML)
- `--dashboard-url`: Base URL of the dashboard API
- `--name` (required): Name identifier for this component monitor instance
- `--kubeconfig-dir` (optional): Path to a directory containing kubeconfig files for different clusters
- `--report-auth-token-file` (required): Path to file containing bearer token for authenticating report requests to the dashboard API

**Configuration File Structure:**
```yaml
frequency: 5m
components:
  - component_slug: "prow"
    sub_component_slug: "deck"
    http_monitor:
      url: "https://prow.ci.openshift.org/"
      code: 200
      retry_after: 4m
      severity: "Down"  # Optional: severity when probe fails (defaults to "Down")
    prometheus_monitor:
      prometheus_location:
        cluster: "app.ci"
        namespace: "openshift-monitoring"
        route: "thanos-querier"
      queries:
        - query: "up{job=\"deck\"} == 1"
          failure_query: "up{job=\"deck\"}"
          duration: "5m"
          step: "30s"
          severity: "Down"  # Optional: severity when query fails (defaults to "Down")
  - component_slug: "trt-incidents"
    sub_component_slug: "incidents"
    frequency: 30m
    jira_monitor:
      url: "https://redhat.atlassian.net"
      jql: 'labels = trt-incident AND status in ("Planning", "New", "Refinement", "To Do", "Approved", "Dev Complete", "Review", "In Progress", "Testing", "ASSIGNED", "POST")'
```

Instance `frequency` and optional per-entry `frequency` are described in [Probe scheduling](#probe-scheduling).

**Prometheus Query Configuration:**
- `query`: The Prometheus query to run (must return results for healthy state)
- `failure_query`: Optional query to run when the main query fails, providing additional context
- `duration`: Optional duration string (e.g., `"5m"`, `"30s"`). If provided, the query will be executed as a range query
- `step`: Optional resolution for range queries (e.g., `"30s"`, `"15s"`). If not provided, a default step is calculated based on the duration
- `severity`: Optional severity level when the query fails. Valid values: `"Down"`, `"Degraded"`, `"CapacityExhausted"`, `"Suspected"`. Defaults to `"Down"` if not specified

**HTTP Monitor Configuration:**
- `url`: The URL to probe
- `code`: The expected HTTP status code
- `retry_after`: Duration to wait before retrying the probe when the status code is not as expected
- `severity`: Optional severity level when the probe fails. Valid values: `"Down"`, `"Degraded"`, `"CapacityExhausted"`, `"Suspected"`. Defaults to `"Down"` if not specified

## Prometheus Location Configuration

The `prometheus_location` field is a struct that specifies how to connect to a Prometheus instance. It can be configured in two ways:

### 1. URL-based (for local development and e2e testing)

Use the `url` field to connect directly to Prometheus without authentication:

```yaml
prometheus_monitor:
  prometheus_location:
    url: "http://localhost:9090"  # Direct URL to Prometheus
  queries:
    - query: "up{job=\"test\"} == 1"
```

**Requirements:**
- Only `url` field should be set (mutually exclusive with `cluster`, `namespace`, `route`)
- Do not provide `--kubeconfig-dir` flag
- The component-monitor connects directly to Prometheus without authentication

### 2. Cluster-based (for production deployments)

Use `cluster`, `namespace`, and `route` fields to connect via OpenShift Routes:

```yaml
prometheus_monitor:
  prometheus_location:
    cluster: "app.ci"                    # Cluster name (must match kubeconfig filename)
    namespace: "openshift-monitoring"   # Namespace where the Prometheus route exists
    route: "thanos-querier"             # Name of the OpenShift Route to Prometheus
  queries:
    - query: "up{job=\"deck\"} == 1"
      duration: "5m"
      step: "30s"
```

**Requirements:**
- All three fields (`cluster`, `namespace`, `route`) must be set together
- `url` field must not be set (mutually exclusive)
- Provide `--kubeconfig-dir` flag pointing to a directory with kubeconfig files
- Each kubeconfig file should be named after the cluster with a `.config` suffix (e.g., `app.ci.config`)

**How it works:**
1. Loads the kubeconfig file for the specified cluster
2. Uses the kubeconfig's authentication (bearer token, TLS certificates)
3. Discovers the Prometheus route via OpenShift Routes API using the provided namespace and route name
4. Creates an authenticated Prometheus client

### 3. In-cluster configuration

Use `"in-cluster"` as the cluster name to use the in-cluster Kubernetes configuration:

```yaml
prometheus_monitor:
  prometheus_location:
    cluster: "in-cluster"              # Special cluster name for in-cluster config
    namespace: "openshift-monitoring"   # Namespace where the Prometheus route exists
    route: "thanos-querier"             # Name of the OpenShift Route to Prometheus
  queries:
    - query: "up{job=\"deck\"} == 1"
      duration: "5m"
      step: "30s"
```

**Requirements:**
- Set `cluster` to `"in-cluster"`
- All three fields (`cluster`, `namespace`, `route`) must be set together
- `url` field must not be set (mutually exclusive)
- Do not provide `--kubeconfig-dir` flag (uses in-cluster service account credentials)

**How it works:**
1. Uses the in-cluster Kubernetes configuration (service account token and CA certificate)
2. Discovers the Prometheus route via OpenShift Routes API using the provided namespace and route name
3. Creates an authenticated Prometheus client

**Note:** Options 2 (cluster-based) and 3 (in-cluster) can be used together within the same deployment. You can configure some components to use cluster-based configuration (with kubeconfig files) and others to use in-cluster configuration, all in the same component-monitor instance.

## Service Account Authentication

The component-monitor authenticates to the dashboard API using OpenShift ServiceAccount bearer tokens:

1. **Token Configuration**: The component-monitor reads a bearer token from a file specified via the `--report-auth-token-file` command-line flag
2. **Request Authentication**: When sending reports to the dashboard API, the component-monitor includes the token in the `Authorization` header as `Bearer <token>`
3. **OAuth Proxy Processing**: In production, requests go through the OAuth proxy which:
   - Validates the bearer token
   - Extracts the service account name (e.g., `system:serviceaccount:ship-status:component-monitor`)
   - Sets the `X-Forwarded-User` header to the service account name
   - Signs the request with HMAC and adds the `GAP-Signature` header
4. **Dashboard Authorization**: The dashboard validates that:
   - The HMAC signature is valid
   - The service account (from `X-Forwarded-User`) is listed as an owner of the component in the dashboard configuration
   - Only service accounts that are owners of a component can report status for that component's sub-components

**Component Configuration**: Components must have the service account listed in their `owners` section with a `service_account` field. For example, in the Dashboard configuration:
```yaml
components:
  - slug: "prow"
    owners:
      - service_account: "system:serviceaccount:ship-status:component-monitor"
```

## How It Works

1. The component-monitor loads the configuration file and validates all settings
2. For each configured component, it creates appropriate probers (HTTP, Prometheus, JUnit, systemd, Jira, etc.)
3. At the instance frequency, it starts probes that are due (see [Probe scheduling](#probe-scheduling)), waits for those results, and reports only the probes that ran
4. Probe results are aggregated and sent to the dashboard API via POST to `/api/component-monitor/report` with bearer token authentication
5. The dashboard API processes the reports and creates/resolves outages accordingly

## Status Reporting

The component-monitor reports status for each sub-component based on probe results. The status levels are configurable per query or monitor via the `severity` field.

### Available Severity Levels

When a probe fails, it reports a status based on the configured severity level:

- **Down**: Most critical severity level. Indicates the component is completely unavailable
- **Degraded**: Indicates the component is functioning but with reduced performance or capabilities
- **CapacityExhausted**: Indicates the component is unavailable due to resource exhaustion (e.g., no available cloud accounts)

If `severity` is not specified for a query or monitor, it defaults to `"Down"`.

### Status Determination

When multiple queries fail for the same sub-component, the most critical severity (highest level) is used:
- `Down` (level 4) > `Degraded` (level 3) > `CapacityExhausted` (level 2) > `Suspected` (level 1) is only used when the sub-component is configured to require `confirmation`

### Examples

**Example 1: HTTP monitor with Degraded severity**
```yaml
http_monitor:
  url: "https://example.com/api"
  code: 200
  retry_after: 4m
  severity: "Degraded"  # Reports Degraded status if probe fails
```

**Example 2: Prometheus queries with different severities**
```yaml
prometheus_monitor:
  queries:
    - query: "up{job=\"critical\"} == 1"
      severity: "Down"  # Critical failure
    - query: "response_time_seconds > 1"
      severity: "Degraded"  # Performance issue
    - query: "available_resources == 0"
      severity: "CapacityExhausted"  # Resource exhaustion
```

If the first query fails, the status will be `Down` (most critical). If only the second query fails, the status will be `Degraded`.

## Range Queries

When a `duration` is specified for a Prometheus query, the component-monitor executes it as a range query:
- The query looks back over the specified duration from the current time
- The `step` parameter controls the resolution (time between data points)
- If `step` is not provided, a default is calculated:
  - For durations ≤ 1 hour: 15 seconds
  - For longer durations: duration / 250
- Range queries return a `Matrix` type, which is evaluated by checking if any time series have data points

## Error Handling

- If a probe fails to execute (network error, etc.), an error is logged but the probe continues
- If the dashboard API is unavailable, errors are logged and the component-monitor continues running
- Configuration validation errors (invalid durations, steps, or prometheus locations) cause the component-monitor to exit immediately

## Configuration Testing

### Config Validation

The `--validate-only` flag validates a component-monitor configuration file without starting the monitor or connecting to any external services:

```bash
component-monitor --validate-only --config-path path/to/config.yaml
```

It loads and validates the YAML structure, frequency values, monitor configurations, and Prometheus location settings. It exits with a non-zero code and prints errors when validation fails. When `--kubeconfig-dir` is not provided, kubeconfig file existence checks are skipped so the command works in CI environments without cluster credentials.

### Dry-Run Mode

To test component-monitor configuration in dry-run mode, see [`hack/component-monitor-dry-run/`](../../hack/component-monitor-dry-run/README.md) and the `component-monitor-dry-run` make target. The job runs on core-ci in the `ship-status` namespace, mounts the production `component-monitor-kubeconfigs` secret, and prints a JSON report without sending it to the dashboard.
