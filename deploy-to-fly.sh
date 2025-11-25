#!/bin/bash

# Confab - Deploy to Fly.io
# Runs database migrations against production DB, then deploys to Fly.io

set -e

echo "=== Confab Fly.io Deployment ==="
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
