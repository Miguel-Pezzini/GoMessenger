#!/usr/bin/env bash
# Create a local kind cluster and load GoMessenger images.
#
# Usage:
#   ./deploy/scripts/kind-up.sh [tag suffix]
#
# Prerequisites:
#   - kind installed (https://kind.sigs.k8s.io/)
#   - Docker images built via backend/scripts/docker-build-all.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLUSTER_CONFIG="${REPO_ROOT}/deploy/kind/cluster-config.yaml"
TAG_SUFFIX="${1:-local}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed. See https://kind.sigs.k8s.io/"
  exit 1
fi

if kind get clusters 2>/dev/null | grep -qx "gomessenger"; then
  echo "kind cluster 'gomessenger' already exists."
else
  echo "Creating kind cluster 'gomessenger'..."
  kind create cluster --name gomessenger --config "${CLUSTER_CONFIG}"
fi

backend_images=(
  auth
  friends
  gateway
  websocket
  chat
  presence
  logging
  notification
  media
)

echo "Loading backend images into kind..."
for name in "${backend_images[@]}"; do
  image="gomessenger/${name}:${TAG_SUFFIX}"
  if docker image inspect "${image}" >/dev/null 2>&1; then
    kind load docker-image "${image}" --name gomessenger
  else
    echo "Warning: image ${image} not found; run backend/scripts/docker-build-all.sh first."
  fi
done

frontend_image="gomessenger/frontend:${TAG_SUFFIX}"
if docker image inspect "${frontend_image}" >/dev/null 2>&1; then
  kind load docker-image "${frontend_image}" --name gomessenger
else
  echo "Warning: image ${frontend_image} not found; build frontend Dockerfile first."
fi

echo "kind cluster 'gomessenger' is ready."
echo "Helm install (Fase 2): helm install gomessenger ./deploy/helm/gomessenger"
