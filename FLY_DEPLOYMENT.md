# Fly.io Deployment Guide

## Prerequisites

1. **Fly.io account** - Sign up at https://fly.io
2. **Fly CLI installed** - `brew install flyctl` (macOS)
3. **Docker** - For building images
4. **Neon.tech account** - For Postgres database
5. **GitHub OAuth app** - For authentication
6. **Tigris bucket** - For session storage

---

## Step 1: Create Tigris Bucket

```bash
# Create a bucket for session storage
fly storage create

# Follow prompts:
# Name: confab-sessions
# Organization: (your org)
# Region: (same as app region)

# Save the credentials shown:
# AWS_ACCESS_KEY_ID=...
# AWS_SECRET_ACCESS_KEY=...
```

---

## Step 2: Set Up Neon.tech Database

1. Go to https://neon.tech
2. Create new project: "confab"
3. Copy connection string (format: `postgresql://user:pass@host/db?sslmode=require`)
4. Save for next step

---

## Step 3: Create GitHub OAuth App (Production)

1. Go to https://github.com/settings/developers
2. Click "New OAuth App"
3. Fill in:
   - **Application name**: Confab Production
   - **Homepage URL**: `https://confabulous.dev`
   - **Authorization callback URL**: `https://confabulous.dev/auth/github/callback`
4. Click "Register application"
5. Save **Client ID** and **Client Secret**

---

## Step 4: Set Fly Secrets

**All secrets must be set via CLI (NOT in fly.toml):**

```bash
# Database
fly secrets set DATABASE_URL="postgresql://user:pass@host.neon.tech/confab?sslmode=require"

# GitHub OAuth
fly secrets set \
  GITHUB_CLIENT_ID="your_production_client_id" \
  GITHUB_CLIENT_SECRET="your_production_client_secret"

# Security Keys (generate random 32+ byte strings)
fly secrets set \
  SESSION_SECRET_KEY="$(openssl rand -base64 32)" \
  CSRF_SECRET_KEY="$(openssl rand -base64 32)"

# Tigris (from Step 1)
fly secrets set \
  AWS_ACCESS_KEY_ID="tid_xxxxx" \
  AWS_SECRET_ACCESS_KEY="tsec_xxxxx"

# Email Whitelist (optional - comma-separated)
fly secrets set ALLOWED_EMAILS="your-email@example.com,teammate@example.com"

# If no ALLOWED_EMAILS set, anyone can sign up (open registration)
```

---

## Step 5: Run Database Migrations

**Before first deployment, run migrations:**

```bash
# Option A: SSH into Fly machine after deploy
fly ssh console
/app/confab  # Run migrations (if you add --migrate flag)

# Option B: Run locally against Neon
cd backend
export DATABASE_URL="postgresql://..."
go run ./cmd/server/main.go  # Add migration flag if implemented
```

**TODO:** Add `--migrate` flag to backend to auto-run migrations on startup.

---

## Step 6: Deploy

```bash
# Deploy to Fly.io
fly deploy

# Watch logs
fly logs

# Open in browser
fly open
```

---

## Step 7: Verify Deployment

### Check Health
```bash
curl https://confabulous.dev/health
# Expected: {"status":"ok"}
```

### Check Frontend
```bash
open https://confabulous.dev
# Should see React frontend
```

### Test Login
1. Go to https://confabulous.dev
2. Click "Login with GitHub"
3. Authorize app
4. Should redirect back and be logged in

---

## Environment Variables Reference

### Non-Secret (in fly.toml)

```toml
[env]
  STATIC_FILES_DIR = "/app/static"              # Enables frontend serving
  FRONTEND_URL = "https://confabulous.dev"       # OAuth redirect target
  ALLOWED_ORIGINS = "https://confabulous.dev"    # CORS whitelist
  GITHUB_REDIRECT_URL = "https://confabulous.dev/auth/github/callback"
  S3_ENDPOINT = "https://fly.storage.tigris.dev"
  S3_USE_SSL = "true"
```

### Secret (via fly secrets set)

**Note:** `fly launch` automatically sets Tigris secrets: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `BUCKET_NAME`

```bash
# Already set by fly launch:
# AWS_ACCESS_KEY_ID            # Tigris access key (tid_...)
# AWS_SECRET_ACCESS_KEY        # Tigris secret key (tsec_...)
# AWS_REGION                   # auto
# BUCKET_NAME                  # winter-resonance-5820 (or your bucket name)

# You need to set these:
DATABASE_URL                 # Neon Postgres connection string
GITHUB_CLIENT_ID             # GitHub OAuth client ID
GITHUB_CLIENT_SECRET         # GitHub OAuth client secret
SESSION_SECRET_KEY           # 32+ random bytes for session encryption
CSRF_SECRET_KEY              # 32+ random bytes for CSRF tokens
ALLOWED_EMAILS               # Optional: comma-separated email whitelist
```

### Not Set (secure by default)

```bash
# INSECURE_DEV_MODE - DO NOT SET in production
# Leaving unset enables:
# - Secure cookies (HTTPS-only)
# - HSTS headers
# - Strict security headers
```

---

## Scaling

### Increase Memory (if needed)

```bash
# Check current usage
fly status

# Increase to 1GB
fly scale memory 1024
```

### Add Regions (multi-region)

```bash
# Add region
fly regions add lax iad

# Fly will deploy machines in all regions
# Database should stay in single region (Neon)
```

### Autoscaling

Already configured in fly.toml:
- `min_machines_running = 0` - Scales to zero when idle
- `auto_stop_machines = 'stop'` - Stops after inactivity
- `auto_start_machines = true` - Starts on request

---

## Monitoring

### View Logs

```bash
# Live tail
fly logs

# Last 100 lines
fly logs --limit 100

# Filter by severity
fly logs --level error
```

### Check Status

```bash
# App status
fly status

# Resource usage
fly vm status

# See all machines
fly machines list
```

### Metrics Dashboard

```bash
fly dashboard
# Opens Grafana dashboard in browser
```

---

## Updating

### Deploy New Version

```bash
# Make code changes
git commit -am "Update feature"

# Build and deploy
fly deploy

# Fly will:
# 1. Build Docker image
# 2. Run health checks
# 3. Rolling deploy (zero downtime)
# 4. Keep old version if deploy fails
```

### Update Secrets

```bash
# Update a secret
fly secrets set ALLOWED_EMAILS="new-list@example.com"

# Fly will restart the app automatically
```

### Update Environment Variables

```bash
# Edit fly.toml
vim fly.toml

# Deploy changes
fly deploy
```

---

## Troubleshooting

### App Won't Start

```bash
# Check logs for errors
fly logs

# Common issues:
# - Missing DATABASE_URL secret
# - Database migrations not run
# - Wrong GitHub OAuth callback URL
# - Tigris credentials invalid
```

### Database Connection Fails

```bash
# Test connection from Fly machine
fly ssh console
ping your-db.neon.tech

# Check DATABASE_URL format:
# postgresql://user:pass@host.neon.tech/db?sslmode=require
#                                              ↑ sslmode required!
```

### OAuth Redirect Fails

```bash
# Verify callback URL matches GitHub OAuth app:
# GitHub: https://confabulous.dev/auth/github/callback
# fly.toml: GITHUB_REDIRECT_URL should match exactly
```

### Static Files Not Serving

```bash
# Check STATIC_FILES_DIR is set
fly ssh console
ls -la /app/static/
# Should see: index.html, _app/, etc.

# Check logs for:
# "Serving static files from: /app/static"
```

### Email Whitelist Not Working

```bash
# Check secret is set
fly secrets list
# Should see ALLOWED_EMAILS (value hidden)

# Check logs when someone tries to login:
fly logs --grep "Email not in whitelist"

# Remember: Email must be verified on GitHub
```

---

## Security Checklist

Before going live:

- [ ] `DATABASE_URL` uses SSL (`?sslmode=require`)
- [ ] `SESSION_SECRET_KEY` is 32+ random bytes
- [ ] `CSRF_SECRET_KEY` is 32+ random bytes
- [ ] `INSECURE_DEV_MODE` is NOT set
- [ ] `ALLOWED_EMAILS` is set (if private app)
- [ ] GitHub OAuth callback URL matches production
- [ ] Tigris credentials are production keys (not dev)
- [ ] HTTPS is enforced (`force_https = true`)
- [ ] Database has proper indexes (see migrations)
- [ ] Backups enabled on Neon.tech

---

## Costs (Estimated)

**Fly.io (Free tier):**
- 3 shared-cpu VMs @ 256MB RAM = Free
- 160GB bandwidth/month = Free
- Our config (512MB) = ~$2-5/month

**Neon.tech (Free tier):**
- 0.5GB storage = Free
- Compute scales to zero = Free
- Upgrade to $19/month for more storage/compute

**Tigris (Free tier):**
- 5GB storage = Free
- 5GB egress = Free
- Upgrade for more usage

**Total: $0-10/month for small usage**

---

## Custom Domain (Optional)

```bash
# Add custom domain
fly certs add confab.example.com

# Follow DNS instructions (add CNAME or A record)

# Update fly.toml
[env]
  FRONTEND_URL = "https://confab.example.com"
  ALLOWED_ORIGINS = "https://confab.example.com"
  GITHUB_REDIRECT_URL = "https://confab.example.com/auth/github/callback"

# Update GitHub OAuth app callback URL

# Deploy
fly deploy
```

---

## Backup Strategy

### Database Backups

Neon.tech provides automatic backups:
- Point-in-time recovery (PITR)
- 7-day retention (free tier)
- 30-day retention (paid tier)

### Session File Backups

Tigris (S3-compatible):
```bash
# Use rclone to backup Tigris to another S3
rclone sync tigris:confab-sessions s3:backup-bucket
```

### Code Backups

GitHub is your backup:
```bash
git push origin main
```

---

## Next Steps

After deployment:

1. **Test thoroughly** - Login, create keys, upload sessions
2. **Monitor logs** - Watch for errors in first 24h
3. **Set up alerts** - Fly can notify on crashes
4. **Add monitoring** - Consider Sentry/LogRocket for errors
5. **Plan for scale** - Monitor storage usage, add regions if needed
6. **Document for team** - Share credentials securely (1Password/Vault)

---

## Support

- **Fly.io Docs**: https://fly.io/docs
- **Fly.io Community**: https://community.fly.io
- **Neon.tech Docs**: https://neon.tech/docs
- **Confab Issues**: https://github.com/ConfabulousDev/confab/issues
