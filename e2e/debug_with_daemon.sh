#!/bin/bash
set -x

cd /tmp && rm -rf test_project && mkdir test_project && cd test_project
git init -q
git config user.email test@test.com
git config user.name Test
echo 'test' > README.md
git add .
git commit -q -m 'init'

echo '=== Running Claude ==='
claude -p 'Say hello' --output-format json --max-turns 1 2>&1

echo ''
echo '=== Checking for daemon state files ==='
ls -la ~/.confab/daemons/ 2>/dev/null || echo 'No daemons dir'
find ~/.confab -name "*.json" 2>/dev/null

echo ''
echo '=== Checking confab logs ==='
cat ~/.confab/logs/confab.log 2>/dev/null | tail -30

echo ''
echo '=== Checking for daemon processes ==='
ps aux | grep confab | grep -v grep || echo 'No confab processes'
