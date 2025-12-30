#!/bin/bash

# Confab - Deploy to Fly.io
# Runs database migrations against production DB, then deploys to Fly.io

set -e

# Create deploy breadcrumb log
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_LOG_DIR="${SCRIPT_DIR}/deploy-logs"
mkdir -p "$DEPLOY_LOG_DIR"
TIMESTAMP=$(date -u +"%Y%m%d-%H%M%S")
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DIRTY_SUFFIX=""
if ! git diff --quiet HEAD 2>/dev/null || git status --porcelain 2>/dev/null | grep -q .; then
    DIRTY_SUFFIX="-dirty"
fi
DEPLOY_LOG_FILE="${DEPLOY_LOG_DIR}/${TIMESTAMP}-${COMMIT_HASH}${DIRTY_SUFFIX}.log"

# Start logging
exec > >(tee -a "$DEPLOY_LOG_FILE") 2>&1

echo "=== Confab Fly.io Deployment ==="
echo "Log file: $DEPLOY_LOG_FILE"
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Commit: $COMMIT_HASH ($(git log -1 --format='%s' 2>/dev/null || echo 'unknown'))"
echo ""

# Check for required tools
if ! command -v fly &> /dev/null; then
    echo "Error: fly CLI not found. Install with: brew install flyctl"
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
cd backend
migrate -database "$PRODUCTION_DATABASE_URL" -path internal/db/migrations up

echo ""
echo "Migrations complete. Current version:"
migrate -database "$PRODUCTION_DATABASE_URL" -path internal/db/migrations version
cd ..

# Deploy to Fly
echo ""
echo "Deploying to Fly.io..."
fly deploy

echo ""
echo "=== Deployment complete ==="
echo ""
echo "Useful commands:"
echo "  fly logs          # View logs"
echo "  fly status        # Check status"
echo "  fly open          # Open in browser"
