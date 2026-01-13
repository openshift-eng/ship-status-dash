#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/config.yaml"
JOB_GUID=$(uuidgen | tr '[:upper:]' '[:lower:]' | cut -d'-' -f1)
JOB_NAME="component-monitor-dry-run-${JOB_GUID}"
NAMESPACE="${NAMESPACE:-ship-status}"
CLUSTER="${CLUSTER:-app.ci}"
IMAGE="${IMAGE:-quay.io/openshiftci/component-monitor:latest}"

if [ ! -f "${CONFIG_FILE}" ]; then
    echo "Error: config file not found: ${CONFIG_FILE}"
    exit 1
fi

CONFIG_MAP_NAME="${JOB_NAME}-config"

echo "Creating ConfigMap from ${CONFIG_FILE}..."
oc create configmap "${CONFIG_MAP_NAME}" \
    --from-file=config.yaml="${CONFIG_FILE}" \
    --namespace="${NAMESPACE}" \
    --context="${CLUSTER}" \
    --dry-run=client -o yaml | oc apply --context="${CLUSTER}" -f -

if ! oc get configmap "${CONFIG_MAP_NAME}" -n "${NAMESPACE}" --context="${CLUSTER}" &>/dev/null; then
    echo "Error: Failed to create ConfigMap ${CONFIG_MAP_NAME}"
    exit 1
fi

echo "Creating Job ${JOB_NAME} in namespace ${NAMESPACE}..."
oc apply --context="${CLUSTER}" -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB_NAME}
  namespace: ${NAMESPACE}
spec:
  ttlSecondsAfterFinished: 86400
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: component-monitor
        image: ${IMAGE}
        command:
        - /app/component-monitor
        args:
        - --config-path
        - /config/config.yaml
        - --name
        - ${JOB_NAME}
        - --kubeconfig-dir
        - /kubeconfigs
        - --dry-run
        volumeMounts:
        - name: config
          mountPath: /config
          readOnly: true
        - name: kubeconfigs
          mountPath: /kubeconfigs
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: ${CONFIG_MAP_NAME}
      # This secret contains the kubeconfigs for app.ci and the build_farm clusters
      - name: kubeconfigs
        secret:
          secretName: component-monitor-kubeconfigs
EOF

echo "Job ${JOB_NAME} created. To view the output:"
echo "  oc logs job/${JOB_NAME} -n ${NAMESPACE} --context=${CLUSTER}"
