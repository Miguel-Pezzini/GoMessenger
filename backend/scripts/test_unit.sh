#!/usr/bin/env bash
# Run all unit tests from the centralized tests/unit tree.
#
# Usage:
#   ./scripts/test_unit.sh [go test flags]
#
# The tests are staged into a temporary copy of the repository so package-private
# unit tests continue to run against their original package paths.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gomessenger-unit.XXXXXX")"
WORKTREE="$TMP_DIR/repo"
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"
export GOTMPDIR="${GOTMPDIR:-/tmp/go-tmp}"

# shellcheck source=./test_helpers.sh
source "$SCRIPT_DIR/test_helpers.sh"

cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

mkdir -p "$GOCACHE" "$GOTMPDIR"

mapfile -t UNIT_SOURCE_DIRS < <(
	find "$ROOT/tests/unit" -type f -name '*_test.go' -printf '%h\n' | sort -u
)

if [ "${#UNIT_SOURCE_DIRS[@]}" -eq 0 ]; then
	echo "ERROR: no unit tests were found under $ROOT/tests/unit."
	exit 1
fi

echo "==> Preparing temporary unit test workspace..."
mkdir -p "$WORKTREE"
cp -a "$ROOT/." "$WORKTREE"

UNIT_PACKAGES=()
for source_dir in "${UNIT_SOURCE_DIRS[@]}"; do
	rel_dir="${source_dir#"$ROOT/tests/unit/"}"
	mkdir -p "$WORKTREE/$rel_dir"
	cp "$source_dir"/*_test.go "$WORKTREE/$rel_dir/"
	UNIT_PACKAGES+=("./$rel_dir")
done

echo "==> Running unit tests..."
(
	cd "$WORKTREE"
	run_go_test_with_ui "$WORKTREE" "$@" "${UNIT_PACKAGES[@]}"
)
