#!/bin/sh
set -e

if [ -z "$DATABASE_URL" ]; then
    echo "Error: DATABASE_URL environment variable is required"
    echo ""
    echo "Usage: DATABASE_URL=postgres://user:pass@host/db ./migrate_db.sh"
    exit 1
fi

exec migrate -path /app/migrations -database "$DATABASE_URL" up
