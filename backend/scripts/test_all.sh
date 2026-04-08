#!/usr/bin/env bash
# Run the complete test suite for the project:
#   1. Centralized unit tests
#   2. Integration tests
#
# Usage:
#   ./scripts/test_all.sh [go test flags]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/test_unit.sh" "$@"

echo
"$SCRIPT_DIR/test_integration.sh" "$@"
