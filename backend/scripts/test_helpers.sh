#!/usr/bin/env bash

has_flag() {
  local needle="$1"
  shift || true

  for arg in "$@"; do
    if [ "$arg" = "$needle" ]; then
      return 0
    fi
  done

  return 1
}

run_go_test_with_ui() {
  local root="$1"
  shift

  local -a go_test_args=("$@")

  if [ "${GO_TEST_UI:-rich}" = "plain" ]; then
    go test "${go_test_args[@]}"
    return
  fi

  if ! has_flag "-json" "${go_test_args[@]}"; then
    go_test_args=("-json" "${go_test_args[@]}")
  fi

  go test "${go_test_args[@]}" | go run "$root/cmd/testui"
}
