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
echo "Verifying template rendering..."
DEFAULT_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME")

echo "$DEFAULT_RENDER" | grep -q "name: cache" && {
  echo "Error: default template should not include cache volume"
  exit 1
}
echo "$DEFAULT_RENDER" | grep -q "type: Recreate" && {
  echo "Error: default template should not use Recreate strategy"
  exit 1
}
echo "$DEFAULT_RENDER" | grep -q "cache-dir" && {
  echo "Error: default template should not pass cache-dir"
  exit 1
}

echo "Verifying hostPath cache template rendering..."
CACHE_RENDER=$(helm template "$RELEASE_NAME" "$CHART_DIR" \
  --set secrets.deployToken=test-token \
  --set fullnameOverride="$RELEASE_NAME" \
  --set cache.hostPath.enabled=true)

echo "$CACHE_RENDER" | grep -q "type: Recreate" || {
  echo "Error: hostPath cache should use Recreate strategy"
  exit 1
}
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