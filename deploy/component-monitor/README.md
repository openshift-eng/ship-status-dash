# Component Monitor Deployment Files

This directory contains example OpenShift deployment files for the component-monitor service.
It is designed to be deployed in the same cluster as the component's it is monitoring, but it is possible to monitor components on different cluster by popluating the `kubeconfig-dir` with a secret containing kubeconfigs for each cluster.
This is done in `app.ci` to monitor each of the build-farm clusters.

## Files

- **`deployment.yaml`**: Defines the Deployment resource that runs the component-monitor container. It specifies the container image, command-line arguments, resource limits, and volume mounts for configuration and kubeconfig files.

- **`configmap.yaml`**: Contains the ConfigMap with the component-monitor configuration file. This includes the monitoring frequency and the list of components to monitor with their respective HTTP or Prometheus monitor configurations.

## Usage

These files are examples and should be customized for your specific deployment environment. Key things to modify:

- Adjust the `namespace` in both files to match your target namespace
- Modify `configmap.yaml` with your actual component monitoring configuration
- Update volume references (ConfigMap and Secret names) to match your environment
- Adjust resource requests and limits based on your workload requirements

