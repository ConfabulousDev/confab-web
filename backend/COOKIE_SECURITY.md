# Cookie Security Configuration

## Overview

All session cookies now have the `Secure` flag enabled by default, preventing transmission over unencrypted HTTP connections. This protects against session hijacking via network sniffing attacks.

## Security Impact

### Vulnerability Fixed: Session Hijacking Over HTTP

**Before (Secure=false):**
```
User connects to public WiFi
    ↓
Browser makes HTTP request (typo, downgrade attack, or mixed content)
    GET /api/v1/sessions HTTP/1.1
    Cookie: confab_session=abc123xyz789  ← Sent in plaintext!
    ↓
Attacker on same network captures packet
    [Packet sniffer reads cookie]
    ↓
Attacker steals session cookie
    ↓
Attacker impersonates victim
    ✓ Access all sessions
    ✓ Create API keys
    ✓ Exfiltrate data
```

**After (Secure=true by default):**
```
User connects to public WiFi
    ↓
Browser makes HTTP request
    GET /api/v1/sessions HTTP/1.1
    Cookie: [NOT SENT - Secure flag prevents HTTP transmission]
    ↓
Attacker captures packet
    [No cookie in packet - nothing to steal]
    ↓
Attack blocked ✓
```

## Configuration

### Secure by Default

Cookies are HTTPS-only by default. To disable for local development:

```bash
# Local development with HTTP
export INSECURE_DEV_MODE=true

# Production (default - no env var needed)
# Cookies only sent over HTTPS
```

### Protected Cookies

All authentication cookies now have `Secure` flag:

| Cookie | Purpose | Secure | SameSite | HttpOnly |
|--------|---------|--------|----------|----------|
| `confab_session` | User session (7 days) | ✅ | Lax | ✅ |
| `oauth_state` | OAuth CSRF protection (5 min) | ✅ | Lax | ✅ |
| `cli_redirect` | CLI auth redirect (5 min) | ✅ | Lax | ✅ |
| `_gorilla_csrf` | CSRF token (session) | ✅ | Lax | ✅ |

## Development Setup

### Local Development (HTTP)

```bash
# In your shell or .env file
export INSECURE_DEV_MODE=true

# Start backend
./confab

# Cookies will work over http://localhost
```

**What this does:**
- Disables `Secure` flag on all cookies
- Allows cookies to be sent over HTTP
- Only for local development on localhost
- **Name is intentionally scary** - never use in production!

### Production (HTTPS)

```bash
# No configuration needed - secure by default
# Just ensure HTTPS is configured

# Do NOT set INSECURE_DEV_MODE in production!
```

**What this does:**
- Enables `Secure` flag on all cookies (default)
- Cookies only sent over HTTPS
- Prevents session hijacking attacks

## Cookie Flags Explained

### Secure Flag

```go
Secure: true  // Cookie only sent over HTTPS
```

**Protects against:**
- Network sniffing (WiFi, ISP, router)
- Man-in-the-middle attacks
- Downgrade attacks (HTTPS → HTTP)
- Mixed content vulnerabilities

**Does NOT protect against:**
- XSS attacks (use HttpOnly for that)
- Stolen laptop/device (implement logout)
- Compromised TLS certificate

### HttpOnly Flag

```go
HttpOnly: true  // Cookie not accessible via JavaScript
```

**Protects against:**
- XSS cookie theft
- JavaScript-based session hijacking

**Already enabled on all cookies ✅**

### SameSite Flag

```go
SameSite: http.SameSiteLaxMode  // Cookie sent on top-level navigation
```

**Protects against:**
- CSRF attacks (POST from other sites blocked)
- Cross-site cookie leakage

**Lax mode chosen because:**
- ✅ Compatible with OAuth redirects from GitHub
- ✅ Works with email links to app
- ✅ Still blocks POST/XHR from other sites
- ✅ Combined with CSRF tokens for defense in depth

**Strict mode would:**
- ❌ Break OAuth login flow
- ❌ Log users out from email links
- ❌ Require same-domain architecture
- ✅ Provide marginally more security

## Testing

### Test 1: Verify Secure Flag in Production Mode

```bash
# Start backend without INSECURE_DEV_MODE
./confab

# Make request and check cookie headers
curl -v http://localhost:8080/auth/github/login 2>&1 | grep -i "set-cookie"

# Expected: Secure flag present
Set-Cookie: oauth_state=...; Secure; HttpOnly; SameSite=Lax
```

### Test 2: Verify HTTP Works in Dev Mode

```bash
# Start backend with INSECURE_DEV_MODE
export INSECURE_DEV_MODE=true
./confab

# Make request
curl -v http://localhost:8080/auth/github/login 2>&1 | grep -i "set-cookie"

# Expected: Secure flag ABSENT
Set-Cookie: oauth_state=...; HttpOnly; SameSite=Lax
#                              ↑ No "Secure" flag
```

### Test 3: Verify Browser Behavior

**Production (HTTPS):**
```javascript
// In browser console on https://confab.example.com
document.cookie
// Should see: confab_session=... (cookie present)

// Try to access over HTTP (if allowed by server)
// Cookie NOT sent - Secure flag prevents it
```

**Development (HTTP):**
```javascript
// In browser console on http://localhost:5173
document.cookie
// Should see: confab_session=... (cookie present with INSECURE_DEV_MODE=true)
```

## Security Best Practices

### ✅ DO

- **Always use HTTPS in production**
  - Required for Secure cookies
  - Protects all traffic, not just cookies

- **Set INSECURE_DEV_MODE=true only in local development**
  - Never in staging or production
  - Document in team onboarding
  - Name is intentionally scary to prevent misuse

- **Use environment variables**
  - Never hardcode INSECURE_DEV_MODE=true
  - Check via `os.Getenv()` at runtime

- **Monitor failed auth attempts**
  - May indicate stolen cookie attempts
  - Log and alert on suspicious patterns

### ❌ DON'T

- **Never disable Secure in production**
  - Exposes sessions to network attackers
  - Violates security best practices

- **Don't use self-signed certificates in production**
  - Browsers may not trust them
  - Use Let's Encrypt for free HTTPS

- **Don't mix HTTP and HTTPS content**
  - Mixed content can leak cookies
  - Ensure all resources load over HTTPS

## HTTPS Setup

### Let's Encrypt (Recommended)

```bash
# Using certbot
sudo certbot --nginx -d confab.example.com

# Auto-renewal
sudo crontab -e
0 0 * * * certbot renew --quiet
```

### Reverse Proxy (Nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name confab.example.com;

    ssl_certificate /etc/letsencrypt/live/confab.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/confab.example.com/privkey.pem;

    # Modern SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # HSTS (force HTTPS)
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name confab.example.com;
    return 301 https://$host$request_uri;
}
```

## Troubleshooting

### Issue: Cookies not being sent in production

**Symptoms:**
- User can't log in
- Authentication fails
- "Unauthorized" errors

**Diagnosis:**
```bash
# Check if HTTPS is configured
curl -I https://confab.example.com
# Should return: HTTP/2 200 (not 404 or connection refused)

# Check cookie flags in response
curl -v https://confab.example.com/auth/github/login 2>&1 | grep -i set-cookie
# Should show: Secure flag present
```

**Solutions:**

1. **HTTPS not configured**
   ```bash
   # Set up HTTPS (see HTTPS Setup above)
   # Or temporarily enable dev mode (NOT RECOMMENDED for production):
   export INSECURE_DEV_MODE=true
   ```

2. **Mixed content (HTTPS page loading HTTP resources)**
   ```bash
   # Ensure all resources use HTTPS
   # Check browser console for mixed content warnings
   # Update all http:// URLs to https://
   ```

3. **Reverse proxy not forwarding HTTPS headers**
   ```nginx
   # Add to nginx config:
   proxy_set_header X-Forwarded-Proto $scheme;
   ```

### Issue: Cookies work in dev but not production

**Cause:** INSECURE_DEV_MODE not set in development

**Solution:**
```bash
# Development
export INSECURE_DEV_MODE=true
./confab

# Production (default)
./confab  # No INSECURE_DEV_MODE env var
```

### Issue: OAuth login broken in production

**Symptoms:**
- OAuth redirects fail
- "Invalid state parameter" error

**Diagnosis:**
- Check if `oauth_state` cookie has `Secure` flag
- Verify HTTPS is working
- Check SameSite setting

**Solution:**
```bash
# Ensure HTTPS is configured
# oauth_state cookie requires Secure flag in production

# Verify redirect URI in GitHub OAuth app settings:
# Must be https://confab.example.com/auth/github/callback
# NOT http://confab.example.com/auth/github/callback
```

## Deployment Checklist

- [ ] HTTPS configured with valid certificate
- [ ] INSECURE_DEV_MODE environment variable NOT set (or set to false)
- [ ] OAuth redirect URIs use https:// in GitHub app settings
- [ ] Test login flow works over HTTPS
- [ ] Verify Secure flag present in cookie headers
- [ ] Test that cookies are NOT sent over HTTP
- [ ] Monitor logs for authentication failures
- [ ] Document HTTPS setup in deployment guide

## References

- [OWASP Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [MDN: Set-Cookie Secure](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#secure)
- [RFC 6265: HTTP State Management (Cookies)](https://datatracker.ietf.org/doc/html/rfc6265)
- [OWASP Testing for Cookie Attributes](https://owasp.org/www-project-web-security-testing-guide/latest/4-Web_Application_Security_Testing/06-Session_Management_Testing/02-Testing_for_Cookies_Attributes)

## Changelog

### 2025-01-16: Initial Implementation

- Added `Secure` flag to all authentication cookies
- Implemented secure-by-default pattern
- Added `INSECURE_DEV_MODE` environment variable for local development
- Updated CSRF cookie settings to match
- Changed SameSite from Strict to Lax for OAuth compatibility
- Added cookieSecure() helper function in auth package
