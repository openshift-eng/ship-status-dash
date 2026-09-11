# Component Monitor Dry-Run Job

This directory contains scripts and configuration for running the component-monitor in dry-run mode as an on-demand Kubernetes/OpenShift Job on app.ci.

The job uses the same Prometheus access path as production: it mounts the `component-monitor-kubeconfigs` secret and queries the `thanos-querier` route on app.ci. Edit `config.yaml` to the probes you want to test; keep `prometheus_location.cluster: app.ci` (and `route: thanos-querier`) unless you are intentionally targeting a different cluster whose kubeconfig is in that secret.

## Usage

1. **Modify the config** (`config.yaml`) to test the components you want to monitor.

2. **Run the make target** to create and start the job:
   ```bash
   make component-monitor-dry-run
   ```

   Or set custom namespace/image:
   ```bash
   NAMESPACE=my-namespace IMAGE=my-registry/component-monitor:tag make component-monitor-dry-run
   ```

   The script prints the generated job name (`component-monitor-dry-run-<guid>`). Use that name for logs.

3. **View the output** (JSON report):
   ```bash
   oc logs job/<job-name> -n ship-status --context=app.ci
   ```

4. **Follow logs in real-time**:
   ```bash
   oc logs -f job/<job-name> -n ship-status --context=app.ci
   ```

## Configuration

Edit `config.yaml` to customize which components to monitor. The config follows the same format as the production component-monitor configuration in `openshift/release` (`core-services/ship-status/component-monitor-config.yaml`).

For Prometheus probes against app.ci metrics, use:

```yaml
prometheus_location:
  cluster: "app.ci"
  namespace: "openshift-monitoring"
  route: "thanos-querier"
```

`cluster` must match a kubeconfig filename in the mounted secret (`app.ci.config`, `build01.config`, and so on).

## Notes

- The job mounts `component-monitor-kubeconfigs` at `/kubeconfigs` and passes `--kubeconfig-dir /kubeconfigs`
- The job will automatically clean up after 1 day (`ttlSecondsAfterFinished: 86400`)
- The job will not retry on failure (`backoffLimit: 0`)
- The output will be the JSON report that would be sent to the dashboard
- The job runs once and exits (dry-run mode)
