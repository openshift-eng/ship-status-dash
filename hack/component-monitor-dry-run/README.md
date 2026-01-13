# Component Monitor Dry-Run Job

This directory contains scripts and configuration for running the component-monitor in dry-run mode as an on-demand Kubernetes/OpenShift Job.

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

3. **View the output** (JSON report):
   ```bash
   oc logs job/component-monitor-dry-run -n <namespace>
   ```

4. **Follow logs in real-time**:
   ```bash
   oc logs -f job/component-monitor-dry-run -n <namespace>
   ```

## Configuration

Edit `config.yaml` to customize which components to monitor. The config follows the same format as the component-monitor configuration file.

## Notes

- The job will automatically clean up after 1 hour (TTL)
- The job will not retry on failure (backoffLimit: 0)
- The output will be the JSON report that would be sent to the dashboard
- The job runs once and exits (dry-run mode)
