#!/usr/bin/env bash
# Run the full integration test suite against isolated test infrastructure.
#
# Usage:
#   ./scripts/test_integration.sh [go test flags]
#
# What it does:
#   1. Starts test docker containers (separate ports from dev)
#   2. Starts all Go services pointing at the test containers
#   3. Runs go test ./tests/integration/... (TestMain wipes DBs/Redis before tests)
#   4. Tears everything down on exit (even on failure)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."
AUTH_PID=""
FRIENDS_PID=""
CHAT_PID=""
PRESENCE_PID=""
WS_PID=""
GATEWAY_PID=""
LOGGING_PID=""
NOTIFICATION_PID=""
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export GOTMPDIR="${GOTMPDIR:-/tmp/go-tmp}"

# shellcheck source=./test_helpers.sh
source "$SCRIPT_DIR/test_helpers.sh"

cd "$ROOT"
mkdir -p "$GOCACHE" "$GOTMPDIR"

# --- Cleanup on exit ---
cleanup() {
  echo ""
  echo "==> Stopping services..."
  for pid in "$AUTH_PID" "$FRIENDS_PID" "$CHAT_PID" "$PRESENCE_PID" "$WS_PID" "$GATEWAY_PID" "$LOGGING_PID" "$NOTIFICATION_PID"; do
    if [ -n "$pid" ]; then
      kill "$pid" 2>/dev/null || true
    fi
  done

  for pid in "$AUTH_PID" "$FRIENDS_PID" "$CHAT_PID" "$PRESENCE_PID" "$WS_PID" "$GATEWAY_PID" "$LOGGING_PID" "$NOTIFICATION_PID"; do
    if [ -n "$pid" ]; then
      wait "$pid" 2>/dev/null || true
    fi
  done

  echo "==> Tearing down test infra..."
  docker compose -f docker-compose.test.yml down --remove-orphans
}
trap cleanup EXIT

http_base_url() {
  local addr="$1"

  if [[ "$addr" == http://* || "$addr" == https://* ]]; then
    printf '%s\n' "${addr%/}"
    return 0
  fi

  if [[ "$addr" == :* ]]; then
    printf 'http://localhost%s\n' "$addr"
    return 0
  fi

  printf 'http://%s\n' "${addr%/}"
}

wait_for_container() {
  local container="$1"
  local expected="$2"

  for _ in $(seq 1 30); do
    local status
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)"
    if [ "$status" = "$expected" ]; then
      return 0
    fi
    sleep 1
  done

  echo "ERROR: container $container did not reach status '$expected' in time."
  docker inspect "$container" 2>/dev/null || true
  return 1
}

# --- Start test infrastructure ---
echo "==> Starting test infrastructure..."
docker compose -f docker-compose.test.yml up -d --remove-orphans

wait_for_container "mongo_user_test" "running"
wait_for_container "mongo_chat_test" "running"
wait_for_container "redis_test" "running"
wait_for_container "mongo_friends_test" "running"

# --- Load test environment ---
# Export vars so child processes (services) inherit them.
# Since config.MustString skips .env keys already in env, these take precedence.
set -a
# shellcheck source=../.env.test
source .env.test
set +a

# --- Start Go services ---
echo "==> Starting services with test environment..."

go run ./services/auth/cmd     &> /tmp/auth.log     & AUTH_PID=$!
go run ./services/friends/cmd  &> /tmp/friends.log  & FRIENDS_PID=$!
go run ./services/chat/cmd     &> /tmp/chat.log     & CHAT_PID=$!
go run ./services/presence_service/cmd &> /tmp/presence.log & PRESENCE_PID=$!
go run ./services/websocket/cmd &> /tmp/ws.log      & WS_PID=$!
go run ./services/logging/cmd  &> /tmp/logging.log  & LOGGING_PID=$!
go run ./services/notification/cmd &> /tmp/notification.log & NOTIFICATION_PID=$!
go run ./services/gateway/cmd  &> /tmp/gateway.log  & GATEWAY_PID=$!

# --- Wait for gateway to be ready ---
echo "==> Waiting for gateway to be ready..."
GATEWAY_BASE_URL="$(http_base_url "$GATEWAY_ADDR")"
for i in $(seq 1 30); do
  if curl -o /dev/null -s -w "%{http_code}" "$GATEWAY_BASE_URL/auth/login" | grep -qv "^000$" || \
     curl -o /dev/null -s -w "%{http_code}" "$GATEWAY_BASE_URL/" | grep -qv "^000$"; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: gateway did not become ready in time."
    echo "--- gateway log ---"
    cat /tmp/gateway.log
    echo "--- logging log ---"
    cat /tmp/logging.log
    exit 1
  fi
  sleep 1
done

echo "==> Gateway ready. Running integration tests..."

# --- Run tests ---
(
  cd "$ROOT/tests/integration"
  run_go_test_with_ui "$ROOT" "$@" ./...
)
