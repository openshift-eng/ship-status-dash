#!/bin/bash

set -e

export ARTIFACT_DIR="${ARTIFACT_DIR:=/tmp/ship_status_artifacts}"
mkdir -p $ARTIFACT_DIR

DB_NAME="ship_status_test"
DSN="postgres://postgres:testpass@postgres.ship-status-e2e.svc.cluster.local:5432/${DB_NAME}?sslmode=disable&client_encoding=UTF8"

echo "The dashboard CI image: ${DASHBOARD_IMAGE}"
echo "The mock-oauth-proxy CI image: ${MOCK_OAUTH_PROXY_IMAGE}"
echo "The migrate CI image: ${MIGRATE_IMAGE}"
echo "The component-monitor CI image: ${COMPONENT_MONITOR_IMAGE}"
echo "The mock-monitored-component CI image: ${MOCK_MONITORED_COMPONENT_IMAGE}"
KUBECTL_CMD="${KUBECTL_CMD:=oc}"
echo "The kubectl command is: ${KUBECTL_CMD}"

is_ready=0
echo "Waiting for cluster to be usable..."

e2e_pause() {
  if [ -z $OPENSHIFT_CI ]; then
    return
  fi
  echo "Sleeping 30 seconds ..."
  sleep 30
}

set +e
for i in `seq 1 20`; do
  echo -n "${i})"
  e2e_pause
  echo "Checking cluster nodes"
  ${KUBECTL_CMD} get node
  if [ $? -eq 0 ]; then
    echo "Cluster looks ready"
    is_ready=1
    break
  fi
  echo "Cluster not ready yet..."
done
set -e

echo "KUBECONFIG=${KUBECONFIG}"
echo "Showing kube context"
${KUBECTL_CMD} config current-context

if [ $is_ready -eq 0 ]; then
  echo "Cluster never became ready aborting"
  exit 1
fi

e2e_pause

echo "Creating namespace..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ship-status-e2e
  labels:
    openshift.io/run-level: "0"
    openshift.io/cluster-monitoring: "true"
    pod-security.kubernetes.io/enforce: privileged
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/warn: privileged
END

e2e_pause

echo "Starting postgres..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  namespace: ship-status-e2e
  labels:
    app: postgres
spec:
  volumes:
    - name: postgresdb
      emptyDir: {}
  containers:
  - name: postgres
    image: quay.io/enterprisedb/postgresql:latest
    ports:
    - containerPort: 5432
    env:
    - name: POSTGRES_PASSWORD
      value: testpass
    volumeMounts:
      - mountPath: /var/lib/postgresql/data
        name: postgresdb
    securityContext:
      privileged: false
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      runAsNonRoot: true
      runAsUser: 3
      seccompProfile:
        type: RuntimeDefault
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: postgres
  name: postgres
  namespace: ship-status-e2e
spec:
  ports:
  - name: postgres
    port: 5432
    protocol: TCP
  selector:
    app: postgres
END

e2e_pause

echo "Waiting for postgres pod to be Ready ..."
set +e
TIMEOUT=120s
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=Ready pod/postgres --timeout=${TIMEOUT}
postgres_retVal=$?
set -e

${KUBECTL_CMD} -n ship-status-e2e logs postgres > ${ARTIFACT_DIR}/postgres.log || true
if [ ${postgres_retVal} -ne 0 ]; then
  echo "Postgres pod never came up"
  exit 1
fi

${KUBECTL_CMD} -n ship-status-e2e get po -o wide
${KUBECTL_CMD} -n ship-status-e2e get svc,ep

echo "Creating ${DB_NAME} database..."
${KUBECTL_CMD} -n ship-status-e2e exec postgres -- psql -U postgres -c "CREATE DATABASE ${DB_NAME};" || echo "Database might already exist"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEST_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
echo "SCRIPT_DIR: ${SCRIPT_DIR}"
echo "TEST_DIR: ${TEST_DIR}"
cd "$TEST_DIR"

CONFIG_FILE="${SCRIPT_DIR}/dashboard-config.yaml"
MOCK_OAUTH_PROXY_CONFIG_FILE="${SCRIPT_DIR}/mock-oauth-proxy-config.yaml"
COMPONENT_MONITOR_CONFIG_FILE="${SCRIPT_DIR}/component-monitor-config.yaml"

echo "Creating configmaps and secrets..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboard-config
  namespace: ship-status-e2e
data:
  dashboard-config.yaml: |
$(sed 's/^/    /' "${CONFIG_FILE}")
END

cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: mock-oauth-proxy-config
  namespace: ship-status-e2e
data:
  config.yaml: |
$(sed 's/^/    /' "${MOCK_OAUTH_PROXY_CONFIG_FILE}")
END

MOCK_MONITORED_COMPONENT_URL="http://mock-monitored-component.ship-status-e2e.svc.cluster.local:9000"
export MOCK_MONITORED_COMPONENT_URL
PROMETHEUS_URL="https://prometheus-k8s.openshift-monitoring.svc.cluster.local:9091"
export TEST_PROMETHEUS_URL="${PROMETHEUS_URL}"
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: component-monitor-config
  namespace: ship-status-e2e
data:
  component-monitor-config.yaml: |
$(envsubst < "${COMPONENT_MONITOR_CONFIG_FILE}" | sed 's/^/    /')
END

HMAC_SECRET=$(openssl rand -hex 32)
${KUBECTL_CMD} -n ship-status-e2e create secret generic hmac-secret --from-literal=secret="${HMAC_SECRET}" --dry-run=client -o yaml | ${KUBECTL_CMD} apply -f -

${KUBECTL_CMD} -n ship-status-e2e create secret generic regcred --from-file=.dockerconfigjson=${DOCKERCONFIGJSON} --type=kubernetes.io/dockerconfigjson --dry-run=client -o yaml | ${KUBECTL_CMD} apply -f -

e2e_pause

echo "Running database migration..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-db
  namespace: ship-status-e2e
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: ${MIGRATE_IMAGE}
        imagePullPolicy: Always
        command: ["./migrate"]
        args:
          - "--dsn=${DSN}"
      imagePullSecrets:
      - name: regcred
      restartPolicy: Never
  backoffLimit: 3
END

set +e
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=complete job/migrate-db --timeout=120s
migrate_retVal=$?
set -e

job_pod=$(${KUBECTL_CMD} -n ship-status-e2e get pod --selector=job-name=migrate-db --output=jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ ! -z "$job_pod" ]; then
  ${KUBECTL_CMD} -n ship-status-e2e logs ${job_pod} > ${ARTIFACT_DIR}/migrate.log || true
fi

if [ ${migrate_retVal} -ne 0 ]; then
  echo "Migration failed"
  exit 1
fi

e2e_pause

echo "Starting dashboard..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: dashboard
  namespace: ship-status-e2e
  labels:
    app: dashboard
spec:
  containers:
  - name: dashboard
    image: ${DASHBOARD_IMAGE}
    imagePullPolicy: Always
    ports:
    - containerPort: 8080
    command: ["./dashboard"]
    args:
      - "--config=/etc/config/dashboard-config.yaml"
      - "--port=8080"
      - "--dsn=${DSN}"
      - "--hmac-secret-file=/etc/hmac/secret"
    volumeMounts:
    - mountPath: /etc/config
      name: dashboard-config
      readOnly: true
    - mountPath: /etc/hmac
      name: hmac-secret
      readOnly: true
  imagePullSecrets:
  - name: regcred
  volumes:
    - name: dashboard-config
      configMap:
        name: dashboard-config
    - name: hmac-secret
      secret:
        secretName: hmac-secret
  securityContext:
    privileged: false
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL
    runAsNonRoot: true
    runAsUser: 1001
    seccompProfile:
      type: RuntimeDefault
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: dashboard
  name: dashboard
  namespace: ship-status-e2e
spec:
  ports:
  - name: http
    port: 8080
    protocol: TCP
  selector:
    app: dashboard
END

e2e_pause

echo "Starting mock-oauth-proxy..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: mock-oauth-proxy
  namespace: ship-status-e2e
  labels:
    app: mock-oauth-proxy
spec:
  containers:
  - name: mock-oauth-proxy
    image: ${MOCK_OAUTH_PROXY_IMAGE}
    imagePullPolicy: Always
    ports:
    - containerPort: 8443
    command: ["./mock-oauth-proxy"]
    args:
      - "--config=/etc/config/config.yaml"
      - "--port=8443"
      - "--upstream=http://dashboard.ship-status-e2e.svc.cluster.local:8080"
      - "--hmac-secret-file=/etc/hmac/secret"
    volumeMounts:
    - mountPath: /etc/config
      name: mock-oauth-proxy-config
      readOnly: true
    - mountPath: /etc/hmac
      name: hmac-secret
      readOnly: true
  imagePullSecrets:
  - name: regcred
  volumes:
    - name: mock-oauth-proxy-config
      configMap:
        name: mock-oauth-proxy-config
    - name: hmac-secret
      secret:
        secretName: hmac-secret
  securityContext:
    privileged: false
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL
    runAsNonRoot: true
    runAsUser: 1001
    seccompProfile:
      type: RuntimeDefault
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: mock-oauth-proxy
  name: mock-oauth-proxy
  namespace: ship-status-e2e
spec:
  ports:
  - name: http
    port: 8443
    protocol: TCP
  selector:
    app: mock-oauth-proxy
END

e2e_pause

echo "Starting mock-monitored-component..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: mock-monitored-component
  namespace: ship-status-e2e
  labels:
    app: mock-monitored-component
spec:
  containers:
  - name: mock-monitored-component
    image: ${MOCK_MONITORED_COMPONENT_IMAGE}
    imagePullPolicy: Always
    ports:
    - containerPort: 9000
    command: ["./mock-monitored-component"]
    args:
      - "--port=9000"
  imagePullSecrets:
  - name: regcred
  securityContext:
    privileged: false
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL
    runAsNonRoot: true
    runAsUser: 1001
    seccompProfile:
      type: RuntimeDefault
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: mock-monitored-component
  name: mock-monitored-component
  namespace: ship-status-e2e
spec:
  ports:
  - name: http
    port: 9000
    protocol: TCP
  selector:
    app: mock-monitored-component
END

e2e_pause

echo "Creating ServiceMonitor for cluster Prometheus to scrape mock-monitored-component..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mock-monitored-component
  namespace: ship-status-e2e
  labels:
    app: mock-monitored-component
spec:
  selector:
    matchLabels:
      app: mock-monitored-component
  endpoints:
  - port: http
    interval: 5s
    path: /metrics
END

e2e_pause

echo "Starting component-monitor..."
cat << END | ${KUBECTL_CMD} apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: component-monitor
  namespace: ship-status-e2e
  labels:
    app: component-monitor
spec:
  containers:
  - name: component-monitor
    image: ${COMPONENT_MONITOR_IMAGE}
    imagePullPolicy: Always
    command: ["./component-monitor"]
    args:
      - "--config-path=/etc/config/component-monitor-config.yaml"
      - "--dashboard-url=http://dashboard.ship-status-e2e.svc.cluster.local:8080"
      - "--name=e2e-component-monitor"
    volumeMounts:
    - mountPath: /etc/config
      name: component-monitor-config
      readOnly: true
  imagePullSecrets:
  - name: regcred
  volumes:
    - name: component-monitor-config
      configMap:
        name: component-monitor-config
  securityContext:
    privileged: false
    allowPrivilegeEscalation: false
    capabilities:
      drop:
      - ALL
    runAsNonRoot: true
    runAsUser: 1001
    seccompProfile:
      type: RuntimeDefault
END

e2e_pause

echo "Waiting for dashboard, mock-oauth-proxy, mock-monitored-component, and component-monitor pods to be Ready ..."
set +e
TIMEOUT=60s
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=Ready pod/dashboard --timeout=${TIMEOUT}
dashboard_retVal=$?
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=Ready pod/mock-oauth-proxy --timeout=${TIMEOUT}
proxy_retVal=$?
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=Ready pod/mock-monitored-component --timeout=${TIMEOUT}
mock_component_retVal=$?
${KUBECTL_CMD} -n ship-status-e2e wait --for=condition=Ready pod/component-monitor --timeout=${TIMEOUT}
component_monitor_retVal=$?
set -e

if [ ${dashboard_retVal} -ne 0 ] || [ ${proxy_retVal} -ne 0 ] || [ ${mock_component_retVal} -ne 0 ] || [ ${component_monitor_retVal} -ne 0 ]; then
  echo "Pod startup failed, debugging..."
  ${KUBECTL_CMD} -n ship-status-e2e describe pod dashboard
  ${KUBECTL_CMD} -n ship-status-e2e describe pod mock-oauth-proxy
  ${KUBECTL_CMD} -n ship-status-e2e describe pod mock-monitored-component
  ${KUBECTL_CMD} -n ship-status-e2e describe pod component-monitor
fi

${KUBECTL_CMD} -n ship-status-e2e logs dashboard > ${ARTIFACT_DIR}/dashboard.log || true
${KUBECTL_CMD} -n ship-status-e2e logs mock-oauth-proxy > ${ARTIFACT_DIR}/mock-oauth-proxy.log || true
${KUBECTL_CMD} -n ship-status-e2e logs mock-monitored-component > ${ARTIFACT_DIR}/mock-monitored-component.log || true
${KUBECTL_CMD} -n ship-status-e2e logs component-monitor > ${ARTIFACT_DIR}/component-monitor.log || true

if [ ${dashboard_retVal} -ne 0 ]; then
  echo "Dashboard pod never came up"
  exit 1
fi
if [ ${proxy_retVal} -ne 0 ]; then
  echo "Mock oauth-proxy pod never came up"
  exit 1
fi
if [ ${mock_component_retVal} -ne 0 ]; then
  echo "Mock-monitored-component pod never came up"
  exit 1
fi
if [ ${component_monitor_retVal} -ne 0 ]; then
  echo "Component-monitor pod never came up"
  exit 1
fi

echo "Waiting for cluster Prometheus to discover and scrape mock-monitored-component metrics..."
echo "ServiceMonitor created - cluster Prometheus should start scraping within its scrape interval"
sleep 30

${KUBECTL_CMD} -n ship-status-e2e get po -o wide
${KUBECTL_CMD} -n ship-status-e2e get svc,ep

date
