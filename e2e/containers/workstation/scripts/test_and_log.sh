#!/bin/bash
set -x

# Run pytest
pytest /e2e/tests -v --tb=short 2>&1

# Show daemon logs
echo ""
echo "=== CONFAB DAEMON LOGS ==="
cat ~/.confab/logs/confab.log 2>/dev/null | tail -50 || echo "No logs"
