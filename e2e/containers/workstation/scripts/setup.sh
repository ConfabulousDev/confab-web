#!/usr/bin/env bash
# Setup script for E2E test workstation
# Configures confab and installs Claude Code hooks

set -e

echo "=== E2E Test Setup ==="

# Verify required environment variables
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "ERROR: ANTHROPIC_API_KEY is not set"
    exit 1
fi

if [ -z "$CONFAB_BACKEND_URL" ]; then
    echo "ERROR: CONFAB_BACKEND_URL is not set"
    exit 1
fi

if [ -z "$CONFAB_API_KEY" ]; then
    echo "ERROR: CONFAB_API_KEY is not set"
    exit 1
fi

# Run confab setup with API key (non-interactive)
echo "Running confab setup..."
confab setup --backend-url "${CONFAB_BACKEND_URL}" --api-key "${CONFAB_API_KEY}"

# Verify config was written
echo ""
echo "Verifying configuration..."
if [ -f ~/.confab/config.json ]; then
    echo "✓ Config file created"
else
    echo "✗ Config file not found"
    exit 1
fi

if [ -f ~/.claude/settings.json ]; then
    echo "✓ Claude hooks installed"
else
    echo "✗ Claude settings not found"
    exit 1
fi

echo ""
echo "=== Setup Complete ==="
