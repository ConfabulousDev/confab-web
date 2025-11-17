#!/bin/bash
# Test Confab Docker image locally
# Connects to local Postgres and MinIO

docker run -it --rm \
  --name confab-test \
  -p 8080:8080 \
  --add-host=host.docker.internal:host-gateway \
  -e PORT=8080 \
  -e DATABASE_URL="postgres://confab:confab@host.docker.internal:5432/confab?sslmode=disable" \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=minioadmin \
  -e AWS_SECRET_ACCESS_KEY=minioadmin \
  -e S3_ENDPOINT=host.docker.internal:9000 \
  -e BUCKET_NAME=confab \
  -e S3_USE_SSL=false \
  -e GITHUB_CLIENT_ID="${GITHUB_CLIENT_ID:-Ov23liYet9NvnMG52g7k}" \
  -e GITHUB_CLIENT_SECRET="${GITHUB_CLIENT_SECRET:-35f979cfbecb1f4f77604f2f465715748fe84ed1}" \
  -e GITHUB_REDIRECT_URL="http://localhost:8080/auth/github/callback" \
  -e FRONTEND_URL="http://localhost:8080" \
  -e ALLOWED_ORIGINS="http://localhost:8080" \
  -e SESSION_SECRET_KEY="dev-session-secret-key-min-32-chars-long-change-me-in-prod" \
  -e CSRF_SECRET_KEY="dev-csrf-secret-key-minimum-32-characters-long-change-me" \
  -e INSECURE_DEV_MODE=true \
  -e ALLOWED_EMAILS="${ALLOWED_EMAILS:-jackie.tung@gmail.com,santa.claude.202@gmail.com}" \
  confab:test
