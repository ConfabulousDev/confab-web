#!/bin/bash
cd /tmp && rm -rf test_project && mkdir test_project && cd test_project
git init -q
git config user.email test@test.com
git config user.name Test
echo 'test' > README.md
git add . && git commit -q -m 'init'

echo '=== Running Claude with debug ==='
claude -p 'Say hello' --output-format json --max-turns 1 --debug 2>&1 | head -100
