#!/usr/bin/env bash
# Lists all Go packages under ./internal/... that contain test files.
# Used by CI (dynamic matrix) and locally (sharded test runs).
#
# Usage:
#   ./scripts/list-test-packages.sh          # one package per line (relative paths)
#   ./scripts/list-test-packages.sh --json   # JSON array (for CI matrix)
#
# Output uses relative paths (e.g., ./internal/db/session) that work
# with `go test` when run from the backend/ directory.

set -euo pipefail
cd "$(dirname "$0")/.."

module=$(go list -m)
packages=$(go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./internal/... \
  | sed "s|^${module}/|./|")

if [[ "${1:-}" == "--json" ]]; then
  echo "$packages" | jq -Rnc '[inputs]'
else
  echo "$packages"
fi
