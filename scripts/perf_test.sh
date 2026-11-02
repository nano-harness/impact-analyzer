#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN="${ROOT_DIR}/bin/impact"
if [[ ! -x "$BIN" ]]; then
  echo "Building impact binary..."
  make build
fi

TIME_BIN="/usr/bin/time"
TIME_ARGS=()
if [[ ! -x "$TIME_BIN" ]]; then
  TIME_BIN="time"
else
  TIME_ARGS=(-v)
fi

SMALL_REF="HEAD~1"
if ! git rev-parse --quiet "$SMALL_REF" >/dev/null; then
  SMALL_REF="HEAD"
fi

MEDIUM_REF="HEAD~5"
if ! git rev-parse --quiet "$MEDIUM_REF" >/dev/null; then
  MEDIUM_REF="$SMALL_REF"
fi

LARGE_REF="origin/main...HEAD"
if ! git rev-parse --quiet origin/main >/dev/null; then
  LARGE_REF="$MEDIUM_REF"
fi

measure() {
  local description="$1"
  shift
  echo "==> ${description}"
  "${TIME_BIN}" "${TIME_ARGS[@]}" "$@"
}

measure "Analyze small diff (${SMALL_REF})" "$BIN" analyze --diff "$SMALL_REF" --depth 2
measure "Analyze medium diff (${MEDIUM_REF})" "$BIN" analyze --diff "$MEDIUM_REF" --depth 2
measure "Analyze large diff (${LARGE_REF})" "$BIN" analyze --diff "$LARGE_REF" --depth 2

measure "Affected tests (small diff ${SMALL_REF})" "$BIN" test --diff "$SMALL_REF"
measure "CR report (medium diff ${MEDIUM_REF})" "$BIN" cr --diff "$MEDIUM_REF"
