#!/bin/bash
set -e

# This script tests the deployment-operator Helm chart installation
# It verifies that the chart can be installed without errors
# Usage: ./test/helm/test-chart-install.sh

CHART_DIR="charts/deployment-operator"
RELEASE_NAME="deployment-operator-test"
NAMESPACE="default"
KIND_CLUSTER_NAME="chart-test-$(date +%s)"

echo "Testing Helm chart installation for deployment-operator..."

# Check if Helm is installed
if ! command -v helm &> /dev/null; then
    echo "Error: Helm is not installed. Please install Helm first."
    exit 1
fi

# Check if Kind is installed
if ! command -v kind &> /dev/null; then
    echo "Error: Kind is not installed. Please install Kind first."
    echo "On macOS, you can use: brew install kind"
    echo "Other platforms: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

# Check if the chart directory exists
if [ ! -d "$CHART_DIR" ]; then
    echo "Error: Chart directory $CHART_DIR not found."
    exit 1
fi

# Setup cleanup function for graceful exit
cleanup() {
    echo "Cleaning up..."
    if kind get clusters | grep -q "$KIND_CLUSTER_NAME"; then
        echo "Deleting Kind cluster $KIND_CLUSTER_NAME..."
        kind delete cluster --name "$KIND_CLUSTER_NAME"
    fi
    echo "Cleanup complete."
}

# Register cleanup function to run on script exit
trap cleanup EXIT

# Create a temporary Kind cluster for testing
echo "Creating a temporary Kind cluster for testing..."
kind create cluster --name "$KIND_CLUSTER_NAME" --wait 60s

# Check cluster access
if ! kubectl cluster-info &> /dev/null; then
    echo "Error: Cannot access Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi

echo "Kubernetes cluster is ready for testing."

# Validate the chart
echo "Validating Helm chart..."
helm lint "$CHART_DIR"

# Verify template rendering
has_resource() {
  local manifest="$1"
  local resource_kind="$2"
  local resource_name="$3"

  echo "$manifest" | awk -v kind="$resource_kind" -v name="$resource_name" '
    $1 == "kind:" { current_kind = $2; in_metadata = 0; next }
    current_kind == kind && $1 == "metadata:" { in_metadata = 1; next }
    in_metadata && $1 == "name:" && $2 == name { found = 1 }
    END { exit !found }
  '
}

echo "Verifying template rendering..."
DEFAULT_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME")

echo "$DEFAULT_RENDER" | grep -q "name: cache" && {
  echo "Error: default template should not include cache volume"
  exit 1
}
echo "$DEFAULT_RENDER" | grep -q "cache-dir" && {
  echo "Error: default template should not pass cache-dir"
  exit 1
}
has_resource "$DEFAULT_RENDER" "ClusterRoleBinding" "console-read-binding" || {
  echo "Error: default template should include the console reader binding"
  exit 1
}
has_resource "$DEFAULT_RENDER" "ClusterRole" "plrl-console-reader" || {
  echo "Error: default template should include the console reader role"
  exit 1
}

echo "Verifying disabled console reader template rendering..."
DISABLED_CONSOLE_READER_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME" \
  --set rbac.consoleReader.enabled=false)
if has_resource "$DISABLED_CONSOLE_READER_RENDER" "ClusterRoleBinding" "console-read-binding"; then
  echo "Error: disabled console reader should not include the console reader binding"
  exit 1
fi
if has_resource "$DISABLED_CONSOLE_READER_RENDER" "ClusterRole" "plrl-console-reader"; then
  echo "Error: disabled console reader should not include the console reader role"
  exit 1
fi
echo "Verifying hostPath cache template rendering..."
CACHE_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME" \
  --set cache.hostPath.enabled=true)

echo "$CACHE_RENDER" | grep -q "path: /var/lib/plural/deployment-operator" || {
  echo "Error: hostPath cache should mount the default host path"
  exit 1
}
echo "$CACHE_RENDER" | grep -q -- "-cache-dir=/plural/cache" || {
  echo "Error: hostPath cache should pass cache-dir"
  exit 1
}
echo "$CACHE_RENDER" | grep -q -- "-cache-persist-interval=10s" || {
  echo "Error: hostPath cache should pass cache-persist-interval"
  exit 1
}
echo "$CACHE_RENDER" | grep -q "name: cache-dir" || {
  echo "Error: hostPath cache should include a cache-dir init container"
  exit 1
}
echo "$CACHE_RENDER" | grep -q 'chmod' || {
  echo "Error: hostPath cache init container should chmod the mount"
  exit 1
}

if helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set cache.hostPath.enabled=true \
  --set replicaCount=2 >/dev/null 2>&1; then
  echo "Error: hostPath cache should fail when replicaCount > 1"
  exit 1
fi

echo "Verifying certs and spawned CA bundle template rendering..."
CA_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME" \
  --set-json 'certs=[{"name":"ca-bundle","file":"ca-certificates.crt","data":"-----BEGIN CERTIFICATE-----\nTEST\n-----END CERTIFICATE-----\n"}]' \
  --set spawnedWorkloads.caBundle.enabled=true \
  --set spawnedWorkloads.caBundle.configMapName=plural-ca-bundle \
  --set spawnedWorkloads.caBundle.sourceFile=/etc/ssl/certs/ca-certificates.crt \
  --set spawnedWorkloads.caBundle.key=ca-certificates.crt)

echo "$CA_RENDER" | grep -q 'PLURAL_SPAWNED_CA_ENABLED' || {
  echo "Error: enabled spawned CA should set PLURAL_SPAWNED_CA_ENABLED"
  exit 1
}
echo "$CA_RENDER" | grep -q 'PLURAL_SPAWNED_CA_CONFIGMAP' || {
  echo "Error: enabled spawned CA should set PLURAL_SPAWNED_CA_CONFIGMAP"
  exit 1
}
echo "$CA_RENDER" | grep -q 'PLURAL_SPAWNED_CA_SOURCE_FILE' || {
  echo "Error: enabled spawned CA should set PLURAL_SPAWNED_CA_SOURCE_FILE"
  exit 1
}
echo "$CA_RENDER" | grep -q 'PLURAL_SPAWNED_CA_KEY' || {
  echo "Error: enabled spawned CA should set PLURAL_SPAWNED_CA_KEY"
  exit 1
}
echo "$CA_RENDER" | grep -q 'mountPath: "/etc/ssl/certs/ca-certificates.crt"' || {
  echo "Error: certs should mount ca-certificates.crt"
  exit 1
}
# agentk and operator containers both mount certs — expect at least two mounts
MOUNT_COUNT=$(echo "$CA_RENDER" | grep -c 'mountPath: "/etc/ssl/certs/ca-certificates.crt"' || true)
if [ "$MOUNT_COUNT" -lt 2 ]; then
  echo "Error: expected cert mounts on operator and agentk, found $MOUNT_COUNT"
  exit 1
fi
has_resource "$CA_RENDER" "ConfigMap" "${RELEASE_NAME}-ca-bundle" || {
  echo "Error: certs should render a ConfigMap"
  exit 1
}

echo "$DEFAULT_RENDER" | grep -q 'PLURAL_SPAWNED_CA_ENABLED' && {
  echo "Error: default template should not set spawned CA env"
  exit 1
}

# Install the chart with dry-run first
echo "Performing dry-run installation..."
helm install "$RELEASE_NAME" "$CHART_DIR" \
  --dry-run \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME" \
  --namespace "$NAMESPACE" \
  --create-namespace

echo "All tests passed! The deployment-operator Helm chart is installable."
# Cleanup happens automatically via the trap 