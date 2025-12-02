#!/bin/bash
# Fly.io Secrets Setup Template
# Fill in values and run to configure production secrets

# DO NOT commit this file with real secrets!
# Add to .gitignore after filling in

echo "Setting Fly.io secrets for confab..."
echo ""
echo "Note: Tigris (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION)"
echo "      were already set by 'fly launch' - no need to set again!"
echo ""

# 1. Database (from Neon.tech)
echo "Setting DATABASE_URL..."
fly secrets set \
  DATABASE_URL="postgresql://user:password@host.neon.tech/confab?sslmode=require"

# 2. GitHub OAuth (from https://github.com/settings/developers)
# Create at: https://github.com/settings/developers
# Callback URL: https://confabulous.dev/auth/github/callback
echo "Setting GitHub OAuth credentials..."
fly secrets set \
  GITHUB_CLIENT_ID="your_production_github_client_id" \
  GITHUB_CLIENT_SECRET="your_production_github_client_secret"

# 3. Security Keys (auto-generated with openssl)
echo "Generating and setting security keys..."
fly secrets set \
  SESSION_SECRET_KEY="$(openssl rand -base64 32)" \
  CSRF_SECRET_KEY="$(openssl rand -base64 32)"

# 4. User Limit (optional)
# Maximum number of users allowed in the system (default: 50)
# Set to 0 to block all new registrations
echo ""
echo "User limit: Using default (50 users)"
echo "To change, run:"
echo "  fly secrets set MAX_USERS=\"100\""
# fly secrets set MAX_USERS="100"

echo ""
echo "✅ Done! Secrets set. App will restart automatically."
echo ""
echo "Next steps:"
echo "  1. Verify: fly secrets list"
echo "  2. Deploy: fly deploy"
echo "  3. Check:  fly logs"
echo "  4. Open:   fly open"
