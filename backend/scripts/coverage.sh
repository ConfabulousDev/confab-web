#!/usr/bin/env bash
# Run full backend test coverage end-to-end, reliably.
#
# Why this script exists:
#   `go test ./... -coverprofile=coverage.out -covermode=atomic` runs one
#   test binary per package in parallel up to GOMAXPROCS. Each integration
#   test in this repo starts fresh Postgres + MinIO containers via
#   internal/testutil.SetupTestEnvironment, so the docker daemon can get
#   back-pressured under a parallel stampede and the MinIO health probe
#   times out. This script sidesteps that by running one package at a
#   time and merging per-package profiles into a single coverage.out.
#
# Output:
#   - backend/coverage.out                            (merged profile)
#   - backend/build/coverage/<slug>.cover             (per-package profiles)
#   - stdout: per-package statement coverage table    (printed at the end)
#
# Usage:
#   ./scripts/coverage.sh
#
# Environment:
#   DOCKER_HOST                  passed through to testcontainers
#   COVERAGE_DRYRUN=1            print resolved workdir and exit (for tests)
#   COVERAGE_SUMMARY_ONLY=<file> print the per-package table for an existing
#                                coverage profile and exit (runs no tests)
#
# Note: `set -e` is intentionally omitted so a single failing package does
# not abort the run — we want coverage data from every package that did
# pass, then exit non-zero at the end if anything failed.

set -uo pipefail

invocation_dir=$(pwd)
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
backend_dir=$(cd "${script_dir}/.." && pwd)
cd "${backend_dir}"

# print_package_summary <profile>
#
# Prints statement coverage per package, sorted by package, plus a total.
# A profile produced with -coverpkg repeats each block once per test binary
# that loaded its package, so blocks are deduped on file:range and a block
# counts as covered if ANY copy has a non-zero count (the same merge rule
# `go tool cover -func` applies). Packages are the block's directory, shown
# without the module prefix.
print_package_summary() {
  local profile=$1 module
  module=$(awk '$1 == "module" { print $2; exit }' go.mod)
  awk -v prefix="${module}/" '
    function pct(covered, statements) {
      return sprintf("%.1f%%", statements > 0 ? 100 * covered / statements : 0)
    }
    NF != 3 || $1 ~ /^mode:/ { next }
    {
      if (!($1 in stmts)) stmts[$1] = $2
      if ($3 > 0) hit[$1] = 1
    }
    END {
      n = 0
      for (block in stmts) {
        p = block
        sub(/:[^:]*$/, "", p)    # drop :startLine.col,endLine.col
        sub(/\/[^\/]*$/, "", p)  # drop the file name
        if (index(p, prefix) == 1) p = substr(p, length(prefix) + 1)
        if (!(p in pkg_stmts)) { names[++n] = p; pkg_stmts[p] = 0; pkg_hit[p] = 0 }
        pkg_stmts[p] += stmts[block]
        if (block in hit) pkg_hit[p] += stmts[block]
      }
      # Insertion sort: macOS awk has no asort().
      for (i = 2; i <= n; i++) {
        v = names[i]
        for (j = i - 1; j >= 1 && names[j] > v; j--) names[j + 1] = names[j]
        names[j + 1] = v
      }
      width = length("package")
      for (i = 1; i <= n; i++) if (length(names[i]) > width) width = length(names[i])
      fmt = "%-" width "s  %8s  %10s\n"
      printf fmt, "package", "coverage", "statements"
      total = 0; total_hit = 0
      for (i = 1; i <= n; i++) {
        p = names[i]
        printf fmt, p, pct(pkg_hit[p], pkg_stmts[p]), pkg_stmts[p]
        total += pkg_stmts[p]; total_hit += pkg_hit[p]
      }
      printf fmt, "total", pct(total_hit, total), total
    }
  ' "${profile}"
}

if [[ "${COVERAGE_DRYRUN:-}" == "1" ]]; then
  echo "WORKDIR=${backend_dir}"
  exit 0
fi

if [[ -n "${COVERAGE_SUMMARY_ONLY:-}" ]]; then
  summary_profile=${COVERAGE_SUMMARY_ONLY}
  [[ "${summary_profile}" == /* ]] || summary_profile="${invocation_dir}/${summary_profile}"
  if [[ ! -f "${summary_profile}" ]]; then
    echo "coverage profile not found: ${summary_profile}" >&2
    exit 1
  fi
  print_package_summary "${summary_profile}"
  exit
fi

profile_dir="${backend_dir}/build/coverage"
mkdir -p "${profile_dir}"

mapfile -t packages < <(./scripts/list-test-packages.sh)
if (( ${#packages[@]} == 0 )); then
  echo "no testable packages discovered under ./internal/..." >&2
  exit 1
fi

failed_packages=()
for pkg in "${packages[@]}"; do
  slug=$(echo "${pkg}" | sed 's#^\./##; s#/#_#g')
  profile="${profile_dir}/${slug}.cover"

  echo "==> ${pkg}"
  if ! go test -timeout 15m \
        -coverprofile="${profile}" \
        -covermode=atomic \
        -coverpkg=./internal/... \
        "${pkg}"; then
    failed_packages+=("${pkg}")
  fi
done

# Merge per-package profiles into backend/coverage.out. Atomic-mode
# profiles can be concatenated: go tool cover -func sums counts across
# duplicate block lines correctly. We iterate the current package list
# (not a glob over the profile dir) so stale profiles from removed or
# renamed packages don't pollute the merged total.
merged="${backend_dir}/coverage.out"
echo "mode: atomic" > "${merged}"
for pkg in "${packages[@]}"; do
  slug=$(echo "${pkg}" | sed 's#^\./##; s#/#_#g')
  profile="${profile_dir}/${slug}.cover"
  if [[ -f "${profile}" ]]; then
    tail -n +2 "${profile}" >> "${merged}"
  fi
done

# Printed before the failure exit so a partial run still reports numbers.
echo ""
echo "==> coverage by package (${merged})"
print_package_summary "${merged}"

if (( ${#failed_packages[@]} > 0 )); then
  echo "" >&2
  echo "coverage run completed with failures in ${#failed_packages[@]} package(s):" >&2
  for pkg in "${failed_packages[@]}"; do
    echo "  - ${pkg}" >&2
  done
  exit 1
fi
