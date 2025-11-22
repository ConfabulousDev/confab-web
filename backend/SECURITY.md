# Security Guide

**Last Updated:** 2025-01-21

Complete security documentation for the Confab backend application. This document consolidates all security features, configurations, and best practices.

## Table of Contents

1. [Security Overview](#security-overview)
2. [Authentication & Authorization](#authentication--authorization)
3. [Cross-Origin & CSRF Protection](#cross-origin--csrf-protection)
4. [Input Validation](#input-validation)
5. [Response Security](#response-security)
6. [Session Security](#session-security)
7. [Rate Limiting](#rate-limiting)
8. [Security Headers](#security-headers)
9. [Testing Security](#testing-security)
10. [Deployment Checklist](#deployment-checklist)

---

## Security Overview

### Security Layers

Confab implements defense-in-depth with multiple security layers:

```
┌─────────────────────────────────────────┐
│ 1. Network Layer (TLS/HTTPS)            │
├─────────────────────────────────────────┤
│ 2. Security Headers (CSP, HSTS, etc.)   │
├─────────────────────────────────────────┤
│ 3. CORS (Cross-Origin Restrictions)     │
├─────────────────────────────────────────┤
│ 4. Rate Limiting (DoS Protection)       │
├─────────────────────────────────────────┤
│ 5. CSRF Protection (Token Validation)   │
├─────────────────────────────────────────┤
│ 6. Authentication (OAuth + API Keys)    │
├─────────────────────────────────────────┤
│ 7. Authorization (User Ownership)       │
├─────────────────────────────────────────┤
│ 8. Input Validation (Injection Defense) │
├─────────────────────────────────────────┤
│ 9. SQL Parameterization (SQL Injection) │
└─────────────────────────────────────────┘
```

### Threat Model

**Protected Against:**
- ✅ SQL Injection (parameterized queries)
- ✅ XSS Attacks (Content-Security-Policy)
- ✅ CSRF Attacks (double-submit cookie)
- ✅ Clickjacking (X-Frame-Options: DENY)
- ✅ MIME Sniffing (X-Content-Type-Options)
- ✅ Man-in-the-Middle (HSTS, Secure cookies)
- ✅ Open Redirects (URL validation)
- ✅ Path Traversal (filepath.Clean validation)
- ✅ DoS Attacks (rate limiting)
- ✅ Brute Force (rate limiting)

**Current Limitations:**
- ⚠️ Secrets stored in environment variables (not using secret manager)
- ⚠️ No request signing for API keys
- ⚠️ In-memory rate limiting (doesn't scale across multiple servers)

---

## Authentication & Authorization

### OAuth 2.0 (GitHub)

**Flow:** Authorization Code Grant with PKCE-like protection

**Endpoints:**
- `GET /auth/github/login` - Initiates OAuth flow
- `GET /auth/github/callback` - Handles GitHub redirect
- `GET /auth/logout` - Terminates session

**Configuration:**
```bash
# Required environment variables
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret
FRONTEND_URL=https://confab.yourdomain.com
```

**Security Features:**
- ✅ State parameter validation (CSRF protection)
- ✅ One-time code exchange
- ✅ HttpOnly session cookies
- ✅ Secure flag in production
- ✅ SameSite=Lax for OAuth compatibility
- ✅ Open redirect protection on callbacks

**Email Whitelist (Optional):**

Restrict access to specific email domains:

```bash
# Allow only @company.com emails
ALLOWED_EMAIL_DOMAINS=company.com

# Allow multiple domains
ALLOWED_EMAIL_DOMAINS=company.com,partner.com
```

Implementation: `internal/auth/oauth.go:isEmailAllowed()`

### API Keys (CLI Authentication)

**Format:** `confab_<32 hex chars>` (e.g., `confab_a1b2c3d4...`)

**Storage:** SHA-256 hashed in database (raw key never stored)

**Endpoints:**
- `POST /api/v1/keys` - Create new API key (web session required)
- `GET /api/v1/keys` - List user's API keys
- `DELETE /api/v1/keys/{id}` - Revoke API key
- `GET /api/v1/auth/validate` - Validate API key

**Usage:**
```bash
# CLI authorization flow
curl https://confab.dev/auth/cli/authorize

# API request with key
curl -H "Authorization: Bearer confab_abc123..." \
     https://confab.dev/api/v1/sessions
```

**Security Features:**
- ✅ Cryptographically secure random generation
- ✅ SHA-256 hashing before storage
- ✅ User-scoped (cannot access other users' data)
- ✅ Revocable (can be deleted at any time)
- ✅ Rate limited validation endpoint

**Authorization Flow:**

1. User visits `/auth/cli/authorize` in browser (requires GitHub login)
2. Server generates API key and displays once
3. User copies key to CLI: `confab cloud configure --api-key <key>`
4. CLI stores key locally (encrypted on disk)
5. CLI sends `Authorization: Bearer <key>` on every request
6. Server validates key and retrieves user ID

---

## Cross-Origin & CSRF Protection

### CORS (Cross-Origin Resource Sharing)

**Purpose:** Prevent unauthorized websites from accessing the API

**Configuration:**

```bash
# Development (multiple local ports)
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000

# Production (single domain)
ALLOWED_ORIGINS=https://confab.yourdomain.com

# Production (multiple domains)
ALLOWED_ORIGINS=https://confab.yourdomain.com,https://staging.confab.yourdomain.com
```

**Settings:**
```go
AllowedOrigins:   // From ALLOWED_ORIGINS env var
AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"}
ExposedHeaders:   []string{"Link", "X-CSRF-Token"}
AllowCredentials: true  // Allow cookies
MaxAge:           300   // Cache preflight for 5 minutes
```

**How It Works:**

1. Browser sends preflight `OPTIONS` request with `Origin: https://evil.com`
2. Server checks if origin is in `ALLOWED_ORIGINS`
3. If not allowed, browser blocks request (no response sent)
4. If allowed, browser sends actual request

**Attack Prevented:**

```javascript
// evil.com tries to steal user's sessions
fetch('https://confab.dev/api/v1/sessions', {
  credentials: 'include'  // Include cookies
})
// ❌ Blocked by CORS - evil.com not in ALLOWED_ORIGINS
```

### CSRF (Cross-Site Request Forgery)

**Purpose:** Prevent attackers from forging requests using victim's session

**Implementation:** Double-submit cookie pattern using gorilla/csrf

**Configuration:**
```bash
# Required: 32-byte secret key (use openssl rand -base64 32)
CSRF_SECRET_KEY=<random-32-byte-key>

# Development only (disable HTTPS requirement)
INSECURE_DEV_MODE=true
```

**How It Works:**

1. Client requests `/api/v1/csrf-token` (web session required)
2. Server sets `_gorilla_csrf` cookie and returns token in response
3. Client includes token in `X-CSRF-Token` header on state-changing requests
4. Server validates token matches cookie

**Protected Endpoints:**
- `POST /api/v1/keys` - Create API key
- `DELETE /api/v1/keys/{id}` - Delete API key
- `POST /api/v1/sessions/{id}/share` - Create share
- `DELETE /api/v1/shares/{token}` - Revoke share

**Exempt Endpoints:**
- All `GET` requests (read-only)
- `/api/v1/sessions/save` - Uses API key auth (not cookies)
- Public shared session endpoints

**Attack Prevented:**

```html
<!-- Attacker's website: evil.com -->
<form action="https://confab.dev/api/v1/keys" method="POST">
  <input name="name" value="stolen-key">
</form>
<script>document.forms[0].submit()</script>
<!-- ❌ Blocked: Missing valid CSRF token -->
```

**Settings:**
- `Secure: true` - HTTPS only (except `INSECURE_DEV_MODE=true`)
- `SameSite: Lax` - Compatible with OAuth redirects
- `Path: /` - All routes
- `TrustedOrigins: <from ALLOWED_ORIGINS>` - Match CORS

---

## Input Validation

### Content-Type Validation

**Purpose:** Prevent content-type confusion attacks

**Enforced on:** `POST`, `PUT`, `PATCH` requests

**Required:** `Content-Type: application/json`

**Implementation:** `internal/api/content_type.go:validateContentType()`

**Validation:**
```go
if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
    contentType := r.Header.Get("Content-Type")
    if !strings.Contains(contentType, "application/json") {
        return 415 Unsupported Media Type
    }
}
```

**Attack Prevented:**
```bash
# Attacker tries to send XML/form data to JSON endpoint
curl -X POST https://confab.dev/api/v1/sessions/save \
     -H "Content-Type: application/xml" \
     -d "<malicious>...</malicious>"
# ❌ Rejected with 415 Unsupported Media Type
```

### URL Parameter Validation

**Purpose:** Prevent injection and malformed input

**Validated Parameters:**

**Session ID:**
```go
// internal/validation/input.go:ValidateSessionID()
- Length: 1-256 characters
- Must be valid UTF-8
- Used in: /api/v1/sessions/{sessionId}
```

**Share Token:**
```go
// internal/validation/input.go:ValidateShareToken()
- Length: Exactly 32 characters
- Must be hexadecimal [0-9a-fA-F]
- Used in: /api/v1/shares/{shareToken}
```

**Email:**
```go
// internal/validation/email.go:ValidateEmail()
- Max length: 254 characters
- Regex: ^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$
- Used in: Share invites
```

**File ID:**
```go
// Parsed as int64 via strconv.ParseInt()
- Must be valid integer
- Used in: /api/v1/files/{fileId}
```

### Request Body Validation

**Session Upload:**
```go
// internal/api/sessions.go
MaxRequestBodySize: 200 MB  // Total request
MaxFileSize:        50 MB   // Per file
MaxFiles:           100     // Files per session
MaxSessionIDLength: 256
MaxPathLength:      1024
MaxReasonLength:    10000
MaxCWDLength:       4096
```

**Share Creation:**
```go
// internal/api/shares.go
Visibility: Must be "public" or "private"
ExpiresInDays: 1-365 (optional)
InvitedEmails: Max 50 emails, each validated
```

**Attack Prevention:**
```bash
# Attacker tries to upload 500MB file
curl -X POST https://confab.dev/api/v1/sessions/save \
     -d '{"files":[{"content":"<500MB>"}]}'
# ❌ Rejected: 413 Request Entity Too Large

# Attacker tries SQL injection in session_id
curl https://confab.dev/api/v1/sessions/'; DROP TABLE sessions; --
# ❌ Rejected: 400 Bad Request (invalid session_id)
```

### Path Traversal Protection

**Purpose:** Prevent access to files outside static directory

**Implementation:** `internal/api/server.go:serveSPA()`

```go
func (s *Server) serveSPA(staticDir string) http.HandlerFunc {
    cleanStaticDir := filepath.Clean(staticDir)
    return func(w http.ResponseWriter, r *http.Request) {
        requestPath := filepath.Clean(r.URL.Path)
        fullPath := filepath.Join(cleanStaticDir, requestPath)

        // CRITICAL: Ensure resolved path is under staticDir
        if !strings.HasPrefix(fullPath, cleanStaticDir) {
            // Path traversal attempt - serve index.html instead
            http.ServeFile(w, r, filepath.Join(cleanStaticDir, "index.html"))
            return
        }
        // ...
    }
}
```

**Attack Prevented:**
```bash
# Attacker tries to read /etc/passwd
curl https://confab.dev/../../../../etc/passwd
# ✅ Blocked: Serves index.html instead (SPA fallback)
```

### Open Redirect Protection

**Purpose:** Prevent redirecting users to malicious sites

**Context:** OAuth callback redirect_url validation

**Implementation:** `internal/auth/oauth.go:isLocalhostURL()`

**Validation Rules:**
1. Must be valid URL (parsed by `url.Parse`)
2. Scheme must be `http://` or `https://`
3. Localhost redirect: Host must be exactly `localhost` or `127.0.0.1` (no port tricks)
4. Production redirect: Must match `FRONTEND_URL` exactly

**Attack Prevented:**
```bash
# Attacker tries to steal OAuth code
https://confab.dev/auth/github/login?redirect_url=https://evil.com/steal

# Server validates redirect_url
# ❌ Rejected: https://evil.com/steal doesn't match FRONTEND_URL
```

**Safe Redirects:**
```bash
# Development
http://localhost:3000  ✅
http://localhost:5173  ✅
http://127.0.0.1:8080  ✅

# Production (if FRONTEND_URL=https://confab.com)
https://confab.com  ✅
https://confab.com/dashboard  ✅

# Blocked
https://evil.com  ❌
http://localhost.evil.com  ❌
http://localhost@evil.com  ❌
```

---

## Response Security

### Security Headers

**Implementation:** `internal/api/server.go:securityHeadersMiddleware()`

All security headers are applied to every response:

#### Content-Security-Policy (CSP)

**Purpose:** Prevent XSS attacks by controlling resource loading

**API-Only Mode:**
```
Content-Security-Policy: default-src 'self';
                         script-src 'self';
                         style-src 'self' 'unsafe-inline';
                         img-src 'self' data: https:;
                         font-src 'self';
                         connect-src 'self';
                         frame-ancestors 'none'
```

**SPA Mode (with STATIC_FILES_DIR):**
```
Content-Security-Policy: default-src 'self';
                         script-src 'self' 'unsafe-inline';  // SvelteKit needs inline
                         style-src 'self' 'unsafe-inline';
                         img-src 'self' data: https:;
                         font-src 'self';
                         connect-src 'self';
                         frame-ancestors 'none'
```

**What it prevents:**
- ❌ Inline `<script>` tags (except in SPA mode)
- ❌ External JavaScript from CDNs
- ❌ iframe embedding
- ❌ Form submissions to external domains

#### X-Frame-Options

**Value:** `DENY`

**Purpose:** Prevent clickjacking attacks

**Effect:** Page cannot be embedded in `<iframe>`, `<frame>`, `<object>`, or `<embed>`

**Attack Prevented:**
```html
<!-- evil.com tries to frame confab.dev -->
<iframe src="https://confab.dev/api/v1/keys"></iframe>
<!-- ❌ Browser blocks: X-Frame-Options: DENY -->
```

#### Strict-Transport-Security (HSTS)

**Value:** `max-age=31536000; includeSubDomains`

**Purpose:** Force HTTPS for all future requests

**Effect:**
- Browser remembers to use HTTPS for 1 year
- Applies to all subdomains
- Prevents HTTPS downgrade attacks

**Only set when:** `INSECURE_DEV_MODE != "true"` (production only)

#### X-Content-Type-Options

**Value:** `nosniff`

**Purpose:** Prevent MIME type sniffing

**Effect:** Browser must respect `Content-Type` header exactly

**Attack Prevented:**
```html
<!-- Attacker uploads image.jpg with JavaScript content -->
<script src="/uploads/image.jpg"></script>
<!-- ❌ Browser won't execute: Content-Type: image/jpeg (not text/javascript) -->
```

#### Referrer-Policy

**Value:** `strict-origin-when-cross-origin`

**Purpose:** Control referrer information leakage

**Effect:**
- Same-origin: Send full URL
- Cross-origin: Send only origin (no path/query)

**Example:**
```
User on https://confab.dev/sessions/secret-123 clicks link to https://example.com
Referrer sent to example.com: https://confab.dev (not /sessions/secret-123)
```

#### X-Permitted-Cross-Domain-Policies

**Value:** `none`

**Purpose:** Prevent Flash/PDF from loading cross-domain content

**Effect:** Flash Player and Adobe Reader cannot load resources from this domain

---

## Session Security

### Session Cookies

**Purpose:** Maintain web dashboard authentication state

**Cookie Name:** `confab_session`

**Settings:**
```go
http.SetCookie(w, &http.Cookie{
    Name:     "confab_session",
    Value:    sessionID,
    Path:     "/",
    HttpOnly: true,   // JavaScript cannot access
    Secure:   true,   // HTTPS only (except INSECURE_DEV_MODE)
    SameSite: http.SameSiteLaxMode,  // OAuth compatible
    MaxAge:   7 * 24 * 60 * 60,  // 7 days
})
```

**Security Features:**

**HttpOnly:**
- ✅ Prevents JavaScript from reading cookie
- ✅ Mitigates XSS-based session theft

**Secure:**
- ✅ Cookie only sent over HTTPS
- ✅ Prevents MITM session hijacking
- ⚠️ Disabled in development (`INSECURE_DEV_MODE=true`)

**SameSite=Lax:**
- ✅ Cookie sent on top-level navigation (OAuth redirects work)
- ✅ Cookie NOT sent on cross-site POST requests
- ✅ Provides CSRF protection

**Why Not SameSite=Strict?**
- SameSite=Strict would block OAuth callback flows
- GitHub redirects user to `/auth/github/callback`
- Strict mode would drop session cookie on redirect
- Lax mode allows cookies on GET redirects

### Session Lifecycle

**Creation:**
1. User completes GitHub OAuth
2. Server generates random session ID (32 bytes, hex)
3. Session stored in database with expiry (7 days)
4. Session ID returned in HttpOnly cookie

**Validation:**
```go
// internal/auth/auth.go:SessionMiddleware()
sessionID := cookie.Value
session := db.GetWebSession(ctx, sessionID)
if session.ExpiresAt.Before(time.Now()) {
    return 401 Unauthorized
}
```

**Cleanup:**
- Automatic: `db.CleanupExpiredSessions()` removes sessions older than 7 days
- Manual: `DELETE /auth/logout` deletes session immediately

### CSRF Cookie

**Cookie Name:** `_gorilla_csrf`

**Purpose:** Store CSRF token for validation

**Settings:**
- HttpOnly: `false` (JavaScript needs to read for token extraction)
- Secure: `true` (HTTPS only in production)
- SameSite: `Lax`
- Path: `/`

**Why Not HttpOnly?**
- Frontend needs to read cookie to extract token
- Token is not sensitive (requires matching cookie)
- Double-submit pattern provides security

---

## Rate Limiting

### Implementation

**Package:** `internal/ratelimit/`

**Algorithm:** Token Bucket (golang.org/x/time/rate)

**Storage:** In-memory (per-server instance)

### Rate Limit Tiers

**1. Global Rate Limiter (All Endpoints)**
```go
Requests: 100 per second
Burst: 200
Key: Client IP address
Scope: All requests
```

**2. Auth Rate Limiter (OAuth Endpoints)**
```go
Requests: 10 per minute (0.167/sec)
Burst: 5
Key: Client IP address
Endpoints:
  - GET /auth/github/login
  - GET /auth/github/callback
  - GET /auth/logout
  - GET /auth/cli/authorize
```

**3. Upload Rate Limiter (Session Uploads)**
```go
Requests: 1000 per hour (0.278/sec)
Burst: 200
Key: User ID (not IP!)
Endpoints:
  - POST /api/v1/sessions/save
```

**4. Validation Rate Limiter (API Key Checks)**
```go
Requests: 30 per minute (0.5/sec)
Burst: 10
Key: Client IP address
Endpoints:
  - GET /api/v1/auth/validate
```

### IP Address Detection

**Priority Order:**
```go
1. Fly-Client-IP (Fly.io proxy)
2. CF-Connecting-IP (Cloudflare)
3. X-Real-IP (Nginx)
4. True-Client-IP (Akamai/Cloudflare)
5. X-Forwarded-For (first IP)
6. RemoteAddr (direct connection)
```

**Anti-Spoofing:**
- Uses composite key from ALL headers
- Example: `fly:1.2.3.4|cf:1.2.3.4|xff:1.2.3.4`
- Prevents IP spoofing via single header

### Response Headers

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json

{"error": "Rate limit exceeded. Please try again later."}
```

### Cleanup

**Auto-cleanup:** Removes inactive rate limiters every 5 minutes

**Criteria:** No requests in last 10 minutes

**Memory:** ~32 bytes per active IP/user

### Future: Redis-Based Limiter

**Current limitation:** In-memory doesn't scale across multiple servers

**Solution:**
```go
type RateLimiter interface {
    Allow(ctx context.Context, key string) bool
}

// Swap implementation
limiter := NewRedisRateLimiter(redisClient, rate, burst)
```

---

## Security Headers

See [Response Security](#response-security) section above for comprehensive header documentation.

**Quick Reference:**
- ✅ Content-Security-Policy (XSS prevention)
- ✅ X-Frame-Options: DENY (clickjacking prevention)
- ✅ Strict-Transport-Security (HTTPS enforcement)
- ✅ X-Content-Type-Options: nosniff (MIME sniffing prevention)
- ✅ Referrer-Policy (privacy)
- ✅ X-Permitted-Cross-Domain-Policies: none (Flash/PDF)

---

## Testing Security

### Manual Testing

**CORS:**
```bash
# Should be blocked (wrong origin)
curl -H "Origin: https://evil.com" https://confab.dev/api/v1/sessions

# Should be allowed
curl -H "Origin: https://confab.dev" https://confab.dev/api/v1/sessions
```

**CSRF:**
```bash
# Should fail (missing token)
curl -X POST https://confab.dev/api/v1/keys \
     -H "Cookie: confab_session=abc" \
     -d '{"name":"test"}'

# Should succeed (with token)
curl -X POST https://confab.dev/api/v1/keys \
     -H "Cookie: confab_session=abc; _gorilla_csrf=xyz" \
     -H "X-CSRF-Token: valid_token" \
     -d '{"name":"test"}'
```

**Rate Limiting:**
```bash
# Flood endpoint
for i in {1..150}; do
  curl https://confab.dev/api/v1/sessions
done
# Expected: First 100 succeed, rest get 429
```

**Input Validation:**
```bash
# Invalid session ID (too long)
curl https://confab.dev/api/v1/sessions/$(python -c "print('a'*300)")
# Expected: 400 Bad Request

# Path traversal
curl https://confab.dev/../../../../etc/passwd
# Expected: Serves index.html (SPA fallback)
```

### Automated Testing

**Run all tests:**
```bash
go test ./...
```

**Security-specific tests:**
```bash
# CORS tests
go test -v ./internal/api -run TestCORS

# CSRF tests
go test -v ./internal/auth -run TestCSRF

# Input validation tests
go test -v ./internal/validation -run TestValidate

# Rate limiting tests
go test -v ./internal/ratelimit -run TestRateLimit
```

---

## Deployment Checklist

### Required Environment Variables

```bash
# Database
DATABASE_URL=postgresql://user:pass@host:5432/dbname

# GitHub OAuth
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret

# Security
CSRF_SECRET_KEY=$(openssl rand -base64 32)
SESSION_SECRET=$(openssl rand -base64 32)

# CORS/Frontend
ALLOWED_ORIGINS=https://confab.yourdomain.com
FRONTEND_URL=https://confab.yourdomain.com

# Production flags
INSECURE_DEV_MODE=  # Must be unset or false in production
```

### Optional Environment Variables

```bash
# Email whitelist (restrict to specific domains)
ALLOWED_EMAIL_DOMAINS=company.com

# S3 storage (if using)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret
S3_BUCKET_NAME=confab-sessions

# Static file serving (if bundling frontend)
STATIC_FILES_DIR=/app/frontend/build
```

### Pre-Deployment Checklist

- [ ] `INSECURE_DEV_MODE` is unset or `false`
- [ ] `CSRF_SECRET_KEY` is set (32+ random bytes)
- [ ] `SESSION_SECRET` is set (32+ random bytes)
- [ ] `ALLOWED_ORIGINS` contains only trusted domains
- [ ] `FRONTEND_URL` points to production frontend
- [ ] `DATABASE_URL` uses SSL (`sslmode=require`)
- [ ] GitHub OAuth callback URL is registered
- [ ] All secrets are rotated from development values
- [ ] HTTPS is enforced at load balancer/proxy level
- [ ] Database backups are configured
- [ ] Log aggregation is configured
- [ ] Monitoring/alerts are configured

### Post-Deployment Verification

```bash
# 1. Verify HTTPS redirect
curl -I http://confab.yourdomain.com
# Should redirect to https://

# 2. Verify HSTS header
curl -I https://confab.yourdomain.com
# Should include: Strict-Transport-Security: max-age=31536000

# 3. Verify CORS
curl -H "Origin: https://evil.com" https://confab.yourdomain.com/api/v1/sessions
# Should NOT include Access-Control-Allow-Origin header

# 4. Verify CSP
curl -I https://confab.yourdomain.com
# Should include: Content-Security-Policy: ...

# 5. Test OAuth flow
# Visit https://confab.yourdomain.com in browser
# Click "Login with GitHub"
# Should redirect to GitHub, then back to confab

# 6. Test rate limiting
for i in {1..150}; do curl https://confab.yourdomain.com/health; done
# Should eventually return 429 Too Many Requests
```

---

## Security Contacts

**Report vulnerabilities to:** security@yourcompany.com

**Response SLA:**
- Critical: 24 hours
- High: 72 hours
- Medium: 1 week
- Low: 2 weeks

---

## Changelog

### 2025-01-21: Documentation Consolidation
- Consolidated 8 security documents into single SECURITY.md
- Added deployment checklist
- Added testing procedures
- Updated rate limiting documentation (now implemented)

### 2025-01-16: Initial Security Implementation
- Added CORS configuration
- Added CSRF protection
- Fixed cookie security
- Fixed open redirect vulnerability
- Added input validation
- Added content-type validation
- Added security headers
- Added rate limiting
