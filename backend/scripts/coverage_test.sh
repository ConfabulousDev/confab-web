#!/usr/bin/env bash
# Smoke test for the canonical backend coverage runner.
#
# Does NOT require docker. Asserts the script + Makefile target exist and
# behave correctly at the shell level. Run from anywhere; the test cds into
# backend/ itself.

set -uo pipefail

# Locate the backend dir relative to this script.
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
backend_dir=$(cd "${script_dir}/.." && pwd)

failures=0
total=0

fail() {
  failures=$((failures + 1))
  echo "FAIL: $1" >&2
}

check() {
  total=$((total + 1))
  local desc=$1
  shift
  if ! "$@"; then
    fail "${desc}"
  fi
}

# --- 1. coverage.sh exists, is executable, and parses cleanly ---
# `test -x` on a regular file implies existence, so we don't check -f separately.
check "coverage.sh is executable" test -x "${backend_dir}/scripts/coverage.sh"
check "coverage.sh parses (bash -n)" bash -n "${backend_dir}/scripts/coverage.sh"

# --- 2. Makefile coverage target exists and invokes scripts/coverage.sh ---
# A single `make -n` invocation tells us both whether the target exists
# (exit status) and whether it wires up the script (output).
total=$((total + 1))
if dry_run=$(make -C "${backend_dir}" -n coverage 2>/dev/null); then
  if ! grep -q "scripts/coverage.sh" <<<"${dry_run}"; then
    fail "make coverage does not invoke scripts/coverage.sh (got: ${dry_run})"
  fi
else
  fail "make -n coverage failed (target missing or Makefile error)"
fi

# --- 3. coverage.sh cds into backend/ regardless of invocation cwd ---
# We can't run a full coverage cycle without docker. Smoke-test the cd
# behavior by invoking the script with a sentinel env var that makes it
# exit early after printing its resolved working dir.
#
# The script must honor COVERAGE_DRYRUN=1 to print "WORKDIR=<abs path>" and
# exit 0 without running any go test.
total=$((total + 1))
output=$(cd /tmp && COVERAGE_DRYRUN=1 "${backend_dir}/scripts/coverage.sh" 2>&1 || true)
if ! grep -q "WORKDIR=${backend_dir}" <<<"${output}"; then
  fail "coverage.sh did not cd to ${backend_dir} (got: ${output})"
fi

# --- Summary ---
if (( failures > 0 )); then
  echo "" >&2
  echo "${failures}/${total} checks failed" >&2
  exit 1
fi

echo "all ${total} checks passed"
