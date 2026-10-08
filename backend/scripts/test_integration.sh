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
MEDIA_PID=""
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export GOTMPDIR="${GOTMPDIR:-/tmp/go-tmp}"

# shellcheck source=./test_helpers.sh
source "$SCRIPT_DIR/test_helpers.sh"

cd "$ROOT"
mkdir -p "$GOCACHE" "$GOTMPDIR"

# --- Cleanup on exit ---
BIN_DIR=""
cleanup() {
  echo ""
  echo "==> Stopping services..."
  for pid in "$AUTH_PID" "$FRIENDS_PID" "$CHAT_PID" "$PRESENCE_PID" "$WS_PID" "$GATEWAY_PID" "$LOGGING_PID" "$NOTIFICATION_PID" "$MEDIA_PID"; do
    if [ -n "$pid" ]; then
      kill "$pid" 2>/dev/null || true
    fi
  done

  for pid in "$AUTH_PID" "$FRIENDS_PID" "$CHAT_PID" "$PRESENCE_PID" "$WS_PID" "$GATEWAY_PID" "$LOGGING_PID" "$NOTIFICATION_PID" "$MEDIA_PID"; do
    if [ -n "$pid" ]; then
      wait "$pid" 2>/dev/null || true
    fi
  done

  if [ -n "${BIN_DIR:-}" ] && [ -d "$BIN_DIR" ]; then
    rm -rf "$BIN_DIR"
  fi

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
wait_for_container "mongo_media_test" "running"
wait_for_container "minio_test" "running"

# --- Load test environment ---
# Export vars so child processes (services) inherit them.
# Since config.MustString skips .env keys already in env, these take precedence.
set -a
# shellcheck source=../.env.test
source .env.test
set +a

# --- Build services once (avoids N concurrent cold `go run` compiles) ---
BIN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gomessenger-integration-bins.XXXXXX")"
echo "==> Building services..."
go build -o "$BIN_DIR/auth" ./services/auth/cmd
go build -o "$BIN_DIR/friends" ./services/friends/cmd
go build -o "$BIN_DIR/media" ./services/media/cmd
go build -o "$BIN_DIR/chat" ./services/chat/cmd
go build -o "$BIN_DIR/presence" ./services/presence_service/cmd
go build -o "$BIN_DIR/websocket" ./services/websocket/cmd
go build -o "$BIN_DIR/logging" ./services/logging/cmd
go build -o "$BIN_DIR/notification" ./services/notification/cmd
go build -o "$BIN_DIR/gateway" ./services/gateway/cmd

# --- Start Go services ---
echo "==> Starting services with test environment..."

"$BIN_DIR/auth"         &> /tmp/auth.log         & AUTH_PID=$!
"$BIN_DIR/friends"      &> /tmp/friends.log      & FRIENDS_PID=$!
"$BIN_DIR/media"        &> /tmp/media.log        & MEDIA_PID=$!
"$BIN_DIR/chat"         &> /tmp/chat.log         & CHAT_PID=$!
"$BIN_DIR/presence"     &> /tmp/presence.log     & PRESENCE_PID=$!
"$BIN_DIR/websocket"    &> /tmp/ws.log           & WS_PID=$!
"$BIN_DIR/logging"      &> /tmp/logging.log      & LOGGING_PID=$!
"$BIN_DIR/notification" &> /tmp/notification.log & NOTIFICATION_PID=$!
"$BIN_DIR/gateway"      &> /tmp/gateway.log      & GATEWAY_PID=$!

# --- Wait for gateway to be ready ---
echo "==> Waiting for gateway to be ready..."
GATEWAY_BASE_URL="$(http_base_url "$GATEWAY_ADDR")"
GATEWAY_WAIT_SECONDS="${GATEWAY_WAIT_SECONDS:-90}"
for i in $(seq 1 "$GATEWAY_WAIT_SECONDS"); do
  if curl -o /dev/null -s -w "%{http_code}" "$GATEWAY_BASE_URL/auth/login" | grep -qv "^000$" || \
     curl -o /dev/null -s -w "%{http_code}" "$GATEWAY_BASE_URL/" | grep -qv "^000$"; then
    break
  fi
  if [ "$i" -eq "$GATEWAY_WAIT_SECONDS" ]; then
    echo "ERROR: gateway did not become ready in time."
    echo "--- gateway log ---"
    cat /tmp/gateway.log
    echo "--- auth log ---"
    cat /tmp/auth.log
    echo "--- media log ---"
    cat /tmp/media.log
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
