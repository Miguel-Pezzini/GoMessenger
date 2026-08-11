#!/usr/bin/env bash
# Install or upgrade GoMessenger on the local kind cluster.
#
# Usage:
#   ./deploy/scripts/helm-install.sh [release name]
#
# Prerequisites:
#   - kind cluster 'gomessenger' (deploy/scripts/kind-up.sh)
#   - Docker images built and loaded into kind
#   - helm 3 installed

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CHART_PATH="${REPO_ROOT}/deploy/helm/gomessenger"
RELEASE_NAME="${1:-gomessenger}"

if ! command -v helm >/dev/null 2>&1; then
  echo "helm is not installed. See https://helm.sh/docs/intro/install/"
  exit 1
fi

if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "kubectl cannot reach a cluster. Run deploy/scripts/kind-up.sh first."
  exit 1
fi

echo "Installing metrics-server (required for HPA)..."
kubectl apply -f "${REPO_ROOT}/deploy/kind/metrics-server.yaml"

echo "Waiting for metrics-server..."
kubectl rollout status deployment/metrics-server -n kube-system --timeout=120s

echo "Deploying Helm release '${RELEASE_NAME}'..."
helm upgrade --install "${RELEASE_NAME}" "${CHART_PATH}" \
  --wait \
  --timeout 10m

echo ""
echo "Deployment complete."
echo "  curl http://localhost:8080/healthz"
echo "  open http://localhost:30517"
