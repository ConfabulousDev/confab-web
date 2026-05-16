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
#
# Usage:
#   ./scripts/coverage.sh
#
# Environment:
#   DOCKER_HOST                  passed through to testcontainers
#   COVERAGE_DRYRUN=1            print resolved workdir and exit (for tests)
#
# Note: `set -e` is intentionally omitted so a single failing package does
# not abort the run — we want coverage data from every package that did
# pass, then exit non-zero at the end if anything failed.

set -uo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
backend_dir=$(cd "${script_dir}/.." && pwd)
cd "${backend_dir}"

if [[ "${COVERAGE_DRYRUN:-}" == "1" ]]; then
  echo "WORKDIR=${backend_dir}"
  exit 0
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

if (( ${#failed_packages[@]} > 0 )); then
  echo "" >&2
  echo "coverage run completed with failures in ${#failed_packages[@]} package(s):" >&2
  for pkg in "${failed_packages[@]}"; do
    echo "  - ${pkg}" >&2
  done
  exit 1
fi
