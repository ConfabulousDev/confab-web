#!/bin/bash

# Confab - Deploy to Fly.io
# Runs database migrations against production DB, then deploys to Fly.io
# Used both locally and in CI (GitHub Actions)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Local-only: deploy breadcrumb logs
if [ -z "$CI" ]; then
    DEPLOY_LOG_DIR="${SCRIPT_DIR}/deploy-logs"
    mkdir -p "$DEPLOY_LOG_DIR"
    TIMESTAMP=$(date -u +"%Y%m%d-%H%M%S")
    DIRTY_SUFFIX=""
    if ! git diff --quiet HEAD 2>/dev/null || git status --porcelain 2>/dev/null | grep -q .; then
        DIRTY_SUFFIX="-dirty"
    fi
    DEPLOY_LOG_FILE="${DEPLOY_LOG_DIR}/${TIMESTAMP}-${COMMIT_HASH}${DIRTY_SUFFIX}.log"
    exec > >(tee -a "$DEPLOY_LOG_FILE") 2>&1
    echo "Log file: $DEPLOY_LOG_FILE"
fi

echo "=== Confab Fly.io Deployment ==="
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Commit: $COMMIT_HASH ($(git log -1 --format='%s' 2>/dev/null || echo 'unknown'))"
echo ""

# Check for required tools
if ! command -v flyctl &> /dev/null; then
    echo "Error: flyctl not found. Install with: brew install flyctl"
    exit 1
fi

if ! command -v migrate &> /dev/null; then
    echo "Error: migrate CLI not found. Install with: brew install golang-migrate"
    exit 1
fi

# Check for PRODUCTION_DATABASE_URL
if [ -z "$PRODUCTION_DATABASE_URL" ]; then
    echo "Error: PRODUCTION_DATABASE_URL environment variable is required"
    echo ""
    echo "Set it with:"
    echo "  export PRODUCTION_DATABASE_URL='postgresql://user:pass@host/db?sslmode=require'"
    exit 1
fi

# Run migrations
echo "Running database migrations..."
cd "$SCRIPT_DIR/backend"
migrate -database "$PRODUCTION_DATABASE_URL" -path internal/db/migrations up

echo ""
echo "Migrations complete. Current version:"
migrate -database "$PRODUCTION_DATABASE_URL" -path internal/db/migrations version

# Deploy to Fly
echo ""
echo "Deploying to Fly.io..."
cd "$SCRIPT_DIR"
flyctl deploy

echo ""
echo "=== Deployment complete ==="

if [ -z "$CI" ]; then
    echo ""
    echo "Useful commands:"
    echo "  flyctl logs          # View logs"
    echo "  flyctl status        # Check status"
    echo "  flyctl open          # Open in browser"
fi
