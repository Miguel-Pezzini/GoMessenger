#!/usr/bin/env bash
# Build all Go microservice images in parallel.
#
# Usage:
#   ./scripts/docker-build-all.sh [tag suffix]
#
# Default tag: gomessenger/<service>:local

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TAG_SUFFIX="${1:-local}"

services=(
  "auth:auth"
  "friends:friends"
  "gateway:gateway"
  "websocket:websocket"
  "chat:chat"
  "presence_service:presence"
  "logging:logging"
  "notification:notification"
  "media:media"
)

build_one() {
  local service_path="$1"
  local image_name="$2"
  local tag="gomessenger/${image_name}:${TAG_SUFFIX}"
  echo "==> building ${tag}"
  docker build \
    --build-arg "SERVICE=${service_path}" \
    -t "${tag}" \
    "${BACKEND_DIR}"
}

pids=()
for entry in "${services[@]}"; do
  IFS=':' read -r service_path image_name <<< "${entry}"
  build_one "${service_path}" "${image_name}" &
  pids+=("$!")
done

failed=0
for pid in "${pids[@]}"; do
  if ! wait "${pid}"; then
    failed=1
  fi
done

if [[ "${failed}" -ne 0 ]]; then
  echo "One or more image builds failed."
  exit 1
fi

echo "All backend images built with tag suffix: ${TAG_SUFFIX}"
