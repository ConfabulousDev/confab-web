#!/usr/bin/env bash
# Smoke test for the canonical backend coverage runner.
#
# Does NOT require docker. Asserts the script + Makefile target exist and
# behave correctly at the shell level, and that the per-package summary
# math is right on a fixture profile. Run from anywhere; the test cds into
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

# --- 4. COVERAGE_SUMMARY_ONLY prints a deduped per-package table ---
# A merged -coverpkg profile repeats each block once per test binary that
# loaded its package. The summary must count a block's statements once, as
# covered if ANY copy has a non-zero count, and group blocks by package
# directory (not by path prefix). The profile path is passed relative so the
# script must resolve it against the caller's cwd before it cds into backend/.
fixture_dir=$(mktemp -d)
trap 'rm -rf "${fixture_dir}"' EXIT
module=$(awk '$1 == "module" { print $2; exit }' "${backend_dir}/go.mod")
cat >"${fixture_dir}/fixture.cov" <<EOF
mode: atomic
${module}/internal/foo/a.go:1.1,2.2 3 0
${module}/internal/foo/a.go:3.1,4.2 1 0
${module}/internal/bar/b.go:1.1,2.2 2 0
${module}/internal/bar/baz/c.go:1.1,2.2 1 4
${module}/internal/empty/e.go:1.1,1.2 0 0
${module}/internal/foo/a.go:1.1,2.2 3 5
EOF

# expect_row <package> <pct regex> <statements> — asserts a row in ${summary}.
expect_row() {
  if ! grep -Eq "^$1[[:space:]]+$2%[[:space:]]+$3\$" <<<"${summary}"; then
    fail "summary missing row '$1 $2% $3' (got:"$'\n'"${summary})"
  fi
}

total=$((total + 1))
if summary=$(cd "${fixture_dir}" && COVERAGE_SUMMARY_ONLY=fixture.cov "${backend_dir}/scripts/coverage.sh" 2>&1); then
  expect_row "internal/bar" "0\.0" 2
  expect_row "internal/bar/baz" "100\.0" 1
  expect_row "internal/empty" "0\.0" 0
  expect_row "internal/foo" "75\.0" 4
  expect_row "total" "57\.1" 7
  pkg_order=$(grep -E '^internal/' <<<"${summary}" | awk '{ print $1 }' | paste -sd' ' -)
  if [[ "${pkg_order}" != "internal/bar internal/bar/baz internal/empty internal/foo" ]]; then
    fail "summary packages not sorted by name (got: ${pkg_order})"
  fi
else
  fail "COVERAGE_SUMMARY_ONLY exited non-zero (got: ${summary})"
fi

# --- 5. COVERAGE_SUMMARY_ONLY with a missing profile fails loudly ---
total=$((total + 1))
if missing=$(COVERAGE_SUMMARY_ONLY="${fixture_dir}/nope.cov" "${backend_dir}/scripts/coverage.sh" 2>&1); then
  fail "COVERAGE_SUMMARY_ONLY with a missing profile exited 0 (got: ${missing})"
elif ! grep -q "nope.cov" <<<"${missing}"; then
  fail "missing-profile error does not name the profile (got: ${missing})"
fi

# --- Summary ---
if (( failures > 0 )); then
  echo "" >&2
  echo "${failures}/${total} checks failed" >&2
  exit 1
fi

echo "all ${total} checks passed"
