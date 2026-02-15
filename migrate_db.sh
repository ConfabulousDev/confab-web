#!/bin/sh
set -e

DB_URL="${MIGRATE_DATABASE_URL:-$DATABASE_URL}"

if [ -z "$DB_URL" ]; then
    echo "Error: MIGRATE_DATABASE_URL or DATABASE_URL environment variable is required"
    echo ""
    echo "Usage: MIGRATE_DATABASE_URL=postgres://admin:pass@host/db ./migrate_db.sh"
    echo "   or: DATABASE_URL=postgres://user:pass@host/db ./migrate_db.sh"
    exit 1
fi

exec migrate -path /app/migrations -database "$DB_URL" up
