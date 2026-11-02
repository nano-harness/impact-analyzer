#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN="${ROOT_DIR}/bin/impact"

if [[ ! -x "$BIN" ]]; then
  echo "Building impact binary..."
  make build
fi

DIFF_REF="HEAD~1"
if ! git rev-parse --quiet "$DIFF_REF" >/dev/null; then
  DIFF_REF="HEAD"
fi

DEPTH_REF="HEAD~5"
if ! git rev-parse --quiet "$DEPTH_REF" >/dev/null; then
  DEPTH_REF="$DIFF_REF"
fi

BRANCH_REF="origin/main...HEAD"
if ! git rev-parse --quiet origin/main >/dev/null; then
  BRANCH_REF="$DIFF_REF"
fi

run_cmd() {
  local description="$1"
  shift
  echo "==> ${description}"
  "$@"
}

expect_failure() {
  local description="$1"
  shift
  echo "==> ${description}"
  if "$@"; then
    echo "Expected failure but command succeeded"
    exit 1
  fi
  echo "Got expected failure."
}

run_cmd "Analyze diff (${DIFF_REF})" "$BIN" analyze --diff "$DIFF_REF"
run_cmd "Analyze diff (${DIFF_REF}) JSON" "$BIN" analyze --diff "$DIFF_REF" --format json
run_cmd "Analyze diff (${DIFF_REF}) Markdown" "$BIN" analyze --diff "$DIFF_REF" --format markdown
run_cmd "Analyze file main.go" "$BIN" analyze --file main.go
run_cmd "Analyze depth 3 (${DIFF_REF})" "$BIN" analyze --diff "$DIFF_REF" --depth 3
run_cmd "Affected tests (${DIFF_REF})" "$BIN" test --diff "$DIFF_REF"
run_cmd "Affected tests JSON (${DIFF_REF})" "$BIN" test --diff "$DIFF_REF" --format json
run_cmd "Graph dependencies main.go" "$BIN" graph --file main.go
run_cmd "Graph mermaid main.go" "$BIN" graph --file main.go --format mermaid
run_cmd "CR description (${DIFF_REF})" "$BIN" cr --diff "$DIFF_REF"
run_cmd "CR description (${BRANCH_REF})" "$BIN" cr --diff "$BRANCH_REF"

expect_failure "Missing arguments error" "$BIN" analyze
expect_failure "Invalid diff reference" "$BIN" analyze --diff "invalid-ref-123"
expect_failure "Missing file graph" "$BIN" graph --file "non-existent-file.go"
