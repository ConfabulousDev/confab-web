#!/usr/bin/env bash
# Lists all Go packages under ./internal/... that contain test files.
# Used by CI (dynamic matrix) and locally (sharded test runs).
#
# Usage:
#   ./scripts/list-test-packages.sh          # one package per line
#   ./scripts/list-test-packages.sh --json   # JSON array (for CI matrix)

set -euo pipefail
cd "$(dirname "$0")/.."

packages=$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./internal/...)

if [[ "${1:-}" == "--json" ]]; then
  echo "$packages" | jq -Rnc '[inputs]'
else
  echo "$packages"
fi
