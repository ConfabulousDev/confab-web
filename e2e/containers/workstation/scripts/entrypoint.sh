#!/usr/bin/env bash
# Entrypoint for workstation container
# Runs setup then executes the provided command

set -e

# Run setup
/opt/e2e/setup.sh

# Execute command (defaults to bash)
exec "$@"
