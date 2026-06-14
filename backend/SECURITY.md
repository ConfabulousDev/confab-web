# Security Guide

Complete security documentation for the Confab backend application. This document consolidates all security features, configurations, and best practices. For the canonical environment variable reference see [`../CONFIGURATION.md`](../CONFIGURATION.md).

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
│ 5. CSRF Protection (Fetch Metadata)      │
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
- ✅ CSRF Attacks (Fetch metadata validation)
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

### OAuth 2.0 and OIDC

**Supported providers:** GitHub, Google, and generic OIDC (Okta, Auth0, Azure AD, Keycloak, etc.). Password auth is also supported. At least one method must be configured or the server refuses to start.

**Flow:** Authorization Code Grant.

**Endpoints (per provider):**
- `GET /auth/github/login`, `GET /auth/github/callback`
- `GET /auth/google/login`, `GET /auth/google/callback`
- `GET /auth/oidc/login`, `GET /auth/oidc/callback`
- `GET /auth/logout` — terminates session
- `POST /auth/password/login`, `POST /auth/password/register` — password auth (when `AUTH_PASSWORD_ENABLED=true`)

**Configuration:** see [`../CONFIGURATION.md`](../CONFIGURATION.md) for the full env-var reference (per-provider `*_CLIENT_ID` / `*_CLIENT_SECRET` / `*_REDIRECT_URL`).

**Security features:**
- ✅ State parameter validation (CSRF protection)
- ✅ One-time code exchange
- ✅ HttpOnly session cookies
- ✅ Secure flag in production
- ✅ SameSite=Lax for OAuth compatibility
- ✅ Open redirect protection on callbacks

**Email Whitelist (Optional):**

Restrict access across all auth methods to specific email domains:

```bash
# Allow only @company.com emails
ALLOWED_EMAIL_DOMAINS=company.com

# Allow multiple domains
ALLOWED_EMAIL_DOMAINS=company.com,partner.com
```

Implementation lives in `internal/auth/` per provider; the domain allow-list is applied consistently across GitHub, Google, OIDC, and password auth.

**Account linking (`OAUTH_AUTO_LINK_EMAIL`, default `false`) — cm4f:**

When a first-time OAuth login presents an email that already belongs to an account (e.g. a local password account, or an account created via a different provider) but carries no matching identity, the email match alone is **not** enough to link the new identity by default.

- **Default (`false`):** the login is refused and redirected to `/login?error=account_exists` ("An account with this email already exists. Sign in with your original method."). No identity is linked. This prevents **account takeover**: if an attacker controls a mailbox matching an existing user (a weakly-verified IdP, a look-alike domain in `ALLOWED_EMAIL_DOMAINS`, subdomain takeover), they otherwise could click "Login with GitHub/Google" and inherit that user's account.
- **Opt-in (`true`):** restores the previous behavior — a matching email auto-links the new identity to the existing account. Only enable this when you trust every configured IdP to strictly verify email ownership.

The gate is enforced at the store layer (`FindOrCreateUserByOAuth`, returning `ErrAutoLinkDisabled`) so all three providers behave identically. The already-linked path (returning users) and brand-new-email path are unaffected.

> **Operator note (behavior change):** deployments that ran both password auth and OAuth, or multiple OAuth providers, and relied on automatic email-based linking must set `OAUTH_AUTO_LINK_EMAIL=true` to keep that behavior. A user-initiated "Linked accounts" settings flow is tracked as a follow-up.

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

1. User visits `/auth/cli/authorize` in browser (requires an authenticated web session via any configured provider).
2. Server generates API key and displays it once.
3. User copies the key into the [Confab CLI](https://github.com/ConfabulousDev/confab).
4. CLI sends `Authorization: Bearer <key>` on every sync request.
5. Server validates key and retrieves user ID.

Headless flow: the same key can also be obtained via the OAuth 2.0 device-code flow (`POST /auth/device/code` + `POST /auth/device/token`) — useful for CI runners and remote dev environments where no browser is available on the same host.

**Device-code verification hardening (8epk):**

- ✅ **Unbiased user codes.** The human-friendly `user_code` is generated with rejection sampling over its 31-symbol alphabet, so every symbol is equiprobable (a plain `byte % 31` would favor the first 8 symbols, shrinking the effective keyspace).
- ✅ **Per-verifier brute-force lockout.** `POST /auth/device/verify` binds a logged-in viewer's session to a pending device code. Beyond the shared per-IP rate limit, each verifier (keyed by user ID) is locked out for 15 minutes after 5 failed `user_code` submissions, mirroring the password-auth lockout — defense-in-depth against a logged-in attacker brute-forcing outstanding codes within the short expiry window. The lockout is in-memory (no DB write per attempt) and resets on a successful authorization. Keying on user ID (not session ID) means it cannot be bypassed by re-logging-in.

### Admin authorization (5k4v)

Access to the `/api/v1/admin/` panel is gated by the **union** of two signals: the `SUPER_ADMIN_EMAILS` env allowlist OR the per-user `users.is_admin` column. Either grants admin; neither is a 403. The column is toggleable at runtime by any admin via `POST /admin/users/{id}/grant-admin` / `revoke-admin` (CSRF-protected, audit-logged), so admins are managed without an env edit + restart.

- ✅ **Env super-admins are the recovery path.** If the column is cleared for everyone, a `SUPER_ADMIN_EMAILS` entry + restart restores access.
- ✅ **Last-effective-admin protection (g0bq).** Revoke-admin, deactivate, and delete refuse with **409 Conflict** when the action would leave zero effective admins (active users that can still reach the panel: `is_admin=true` OR an email in `SUPER_ADMIN_EMAILS`). Self-demote / self-deactivate / self-delete as the last admin is hard-blocked. The guard runs **before** any irreversible work (e.g. the delete S3 wipe). It still honors the env-recovery design: revoking the column flag from a user who is also an env super-admin is allowed (they keep panel access). The guard is best-effort against a concurrent double-removal race; env recovery remains the backstop.
- ✅ **`SUPER_ADMIN_EMAILS` is validated at startup (g0bq).** The value is parsed, normalized, and de-duplicated once at boot; malformed / empty / duplicate entries are logged as warnings (a typo no longer silently fails to match) and the effective normalized list is logged. Request-time `IsSuperAdmin` reads the cached set rather than re-parsing the env per request.
- ✅ **Demo-site safety.** The grant endpoint rejects promoting a `read_only` user, the demo identity is re-asserted `is_admin=false` every boot, and operators are instructed to keep the demo email out of `SUPER_ADMIN_EMAILS` — so the demo user can never become an admin.
- The raw `is_admin` column is never serialized as a user field (`json:"-"`); clients see admin status only via the `/me` union flag and the admin list's explicit `is_admin` / `is_super_admin` fields.

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
AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"}
ExposedHeaders:   []string{"Link"}
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

**Implementation:** Fetch metadata validation using `filippo.io/csrf` (successor to gorilla/csrf)

**Configuration:**
```bash
# Required: 32-byte secret key (use openssl rand -base64 32)
CSRF_SECRET_KEY=<random-32-byte-key>

# Development only (disable HTTPS requirement)
INSECURE_DEV_MODE=true
```

**Startup guard against default secrets:** the shipped `docker-compose.yml` and
`.env.example` use a public default `CSRF_SECRET_KEY` so `docker compose up` works
with zero config for local evaluation. To prevent that default (or the default
`ADMIN_BOOTSTRAP_PASSWORD`) from reaching an exposed instance, the server refuses
to start when it detects **production intent** — `INSECURE_DEV_MODE` not `true`,
or an `https://` `FRONTEND_URL` — while a known template default is still in use.
See `cmd/server/security_guard.go`.

**How It Works:**

The library validates CSRF using browser-set Fetch metadata headers, which cannot be forged by cross-origin requests:

1. Browser automatically sets `Sec-Fetch-Site` and `Origin` headers on requests
2. Server validates that state-changing requests come from a same-origin or trusted origin
3. Cross-origin requests from untrusted origins are rejected with 403

No client-side token management is needed. The browser's built-in Fetch metadata headers provide the protection automatically.

**Protected Endpoints:**
All state-changing endpoints behind `csrfMiddleware`:
- `POST /api/v1/keys` - Create API key
- `DELETE /api/v1/keys/{id}` - Delete API key
- `POST /api/v1/sessions/{id}/share` - Create share
- `DELETE /api/v1/shares/{shareId}` - Revoke share
- All admin form submissions

**Exempt Endpoints:**
- All `GET` requests (read-only, safe methods)
- API key authenticated routes (CLI uses Bearer tokens, not cookies)
- Public shared session endpoints

**Cross-origin guard on browser auth routes (56mw):**

`GET /auth/cli/authorize` (mints an API key) and `POST /auth/device/verify` (binds a device code from a session cookie) are session-cookie-authenticated and state-changing, but they are registered *outside* the `csrfMiddleware` group (they are top-level browser navigations / device-page form posts, not `/api/v1` SPA calls). Because `filippo.io/csrf` — like the Fetch-Metadata standard — treats `GET`/`HEAD`/`OPTIONS` as always-safe, it would not protect the state-changing GET even if those routes were moved under it. Both routes are therefore wrapped in `crossOriginGuard` (`internal/api/fetch_metadata.go`), which applies the same `Sec-Fetch-Site`/`Origin` check to **all** methods, reusing the same `trustedOrigins` allowlist, and fails closed when neither header is present. Same-origin requests and top-level navigations (`Sec-Fetch-Site: same-origin` / `none`) pass; `cross-site` / `same-site` are rejected with 403.

**Attack Prevented:**

```html
<!-- Attacker's website: evil.com -->
<form action="https://confab.dev/api/v1/keys" method="POST">
  <input name="name" value="stolen-key">
</form>
<script>document.forms[0].submit()</script>
<!-- ❌ Blocked: Sec-Fetch-Site: cross-site (not same-origin/trusted) -->
```

**Settings:**
- `Secure: true` - HTTPS only (except `INSECURE_DEV_MODE=true`)
- `SameSite: Lax` - Compatible with OAuth redirects
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
curl -X POST https://confab.dev/api/v1/sync/chunk \
     -H "Content-Type: application/xml" \
     -d "<malicious>...</malicious>"
# ❌ Rejected with 415 Unsupported Media Type
```

### URL Parameter Validation

UUIDs are parsed and validated by chi + Postgres; external IDs and emails go through helpers in [`internal/validation/`](internal/validation/). Email normalization and the canonical domain allow-list both live there.

### Request Body Validation

Every route in [`internal/api/server.go`](internal/api/server.go) is wrapped with `withMaxBody(limit, handler)`. T-shirt-sized constants are defined in `server.go`:

| Constant | Size | Typical use |
|---|---|---|
| `MaxBodyXS` | 2 KB | GETs, simple lookups |
| `MaxBodyS` | 16 KB | Small POSTs (login, share create) |
| `MaxBodyM` | 128 KB | Mid-sized writes (sync init/event, summary patch) |
| `MaxBodyL` | 2 MB | Admin smart-recap prompt body |
| `MaxBodyXL` | 16 MB | Sync chunk upload |

Per-endpoint validation (share visibility, expiration window, invited-email list size, etc.) lives in the corresponding handler file.

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
                         script-src 'self' 'unsafe-inline';  // React apps may need inline
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

**Hashed at rest (40hj):**
- ✅ `web_sessions.id` stores **SHA-256(cookie value)**, never the raw token — mirroring the API-key pattern (`db.HashToken`).
- ✅ A read of the database (backup leak, SQLi, replica, support tooling) yields only digests, not replayable session tokens; the raw high-entropy value lives only in the HttpOnly cookie.
- ✅ Lookups stay a single indexed exact-match on the hash.
- The same applies to `device_codes.device_code`. The human-typed `device_codes.user_code` stays plaintext (low-entropy, 5-minute expiry — its defense is the per-verifier verify throttle, 8epk — not at-rest hashing).

### Session Lifecycle

**Creation:**
1. User completes GitHub OAuth
2. Server generates random session ID (32 bytes, hex)
3. Session stored in database (as `sha256(id)`) with expiry (7 days)
4. Session ID returned in HttpOnly cookie (raw value)

**Validation:** sessions are gated by **two independent timeouts** (60j6):
- **Absolute cap (7 days):** `expires_at > NOW()`. The browser cookie also has a
  7-day `MaxAge`, so the cookie itself disappears at the cap.
- **Sliding idle timeout (default 48h, `SESSION_IDLE_TIMEOUT`):** a session
  inactive longer than the window is rejected even within the 7-day cap, so a
  stolen cookie is no longer usable for the full week if the victim stops using
  it. Enforced inside `GetWebSession` (`COALESCE(last_activity_at, created_at) >
  NOW() - idle`), so every entry point — middleware and the CLI/device flows —
  inherits it. On a valid request `last_activity_at` is refreshed, throttled to
  at most one write per 60s. Both comparisons use server-side `NOW()` (no client
  clock). An idle session is treated identically to an expired one (401 →
  re-login); there is no grace UI or refresh token. **Note:** the cookie's 7-day
  browser `MaxAge` is unchanged — the server simply rejects idle sessions earlier.
  The CF-483 demo shared session is exempt from the idle gate (it sits idle
  between anonymous visitors by design).

**Cleanup:**
- Automatic: `db.CleanupExpiredSessions()` removes sessions older than 7 days
- Manual: `DELETE /auth/logout` deletes session immediately

### CSRF Protection

CSRF is enforced by [`filippo.io/csrf`](https://pkg.go.dev/filippo.io/csrf) via browser-supplied Fetch metadata headers (`Sec-Fetch-Site`, `Origin`). The frontend does not read or manage a CSRF token cookie — protection is fully server-side. State-changing requests from cross-origin contexts are rejected with 403.

---

## Rate Limiting

### Implementation

**Package:** `internal/ratelimit/`

**Algorithm:** Token Bucket (golang.org/x/time/rate)

**Storage:** In-memory (per-server instance)

### Rate Limit Tiers

| Limiter | Rate | Burst | Key | Scope |
|---|---|---|---|---|
| Global | 100 req/s | 200 | Client IP | All requests |
| Auth | 1 req/s | 30 | Client IP | OAuth login/callback (GitHub, Google, OIDC), password login/register, device-code flow, CLI authorize |
| Upload (sync) | 2.78 req/s | 2000 | User ID | `POST /api/v1/sync/{init,chunk,event}` |
| Validation | 0.5 req/s | 10 | Client IP | `GET /api/v1/auth/validate` |
| Client error | 0.5 req/s | 5 | Client IP | `POST /api/v1/client-errors` |
| External read | 30 req/s | 60 | API key user | External read endpoints (condensed transcript, file list/download) |

Numbers come from `NewServer()` in `internal/api/server.go` — see [`PERFORMANCE.md`](PERFORMANCE.md) for the rationale and tuning notes.

### IP Address Detection

**Priority Order:**
```go
1. Fly-Client-IP (Fly.io proxy)
2. CF-Connecting-IP (Cloudflare)
3. True-Client-IP (Akamai/Cloudflare Enterprise)
4. X-Real-IP (nginx)
5. X-Forwarded-For (first IP)
6. RemoteAddr (direct connection)
```

**Anti-Spoofing:**
- Uses composite key from ALL trusted headers plus `RemoteAddr`
- Example: `1.2.3.4|5.6.7.8` (sorted, deduplicated)
- `RemoteAddr` (the real TCP peer) always anchors the key, so spoofing a single header cannot fully forge a client's rate-limit identity

**Trusted proxy headers (`TRUSTED_PROXY_HEADERS`):**

By default every proxy header above is honored. If the server is *not* behind a
proxy that strips these headers, an attacker can spoof them to evade IP-based
rate limits. To close this, set `TRUSTED_PROXY_HEADERS` to the comma-separated
list of headers your edge actually sets; any header not in the list is ignored
(including `X-Forwarded-For`), even when present.

- Fly.io: `TRUSTED_PROXY_HEADERS=Fly-Client-IP` (the Fly edge always sets and strips this header)
- Cloudflare: `TRUSTED_PROXY_HEADERS=CF-Connecting-IP`
- Behind nginx setting `X-Real-IP`: `TRUSTED_PROXY_HEADERS=X-Real-IP`

Header names are matched case-insensitively. When unset, behavior is unchanged
(trust all headers). See [`CONFIGURATION.md`](../CONFIGURATION.md).

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

**Bucket cap:** Each limiter has a `maxBuckets` cap on the number of live per-key
buckets (set per-tier in `NewServer()`; e.g. 100K for the global IP limiter).
Cleanup only runs every 5 minutes, so without a cap a fast-rotating-IP attacker
could grow the map enough to exhaust memory within that window. When the cap is
reached, the oldest (least-recently-used) buckets are evicted to admit new keys,
so legitimate new clients are never starved. The cap is a soft bound — it may
transiently overshoot under concurrent inserts.

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
# Should fail (cross-site request, no valid Fetch metadata)
curl -X POST https://confab.dev/api/v1/keys \
     -H "Cookie: confab_session=abc" \
     -H "Content-Type: application/json" \
     -d '{"name":"test"}'

# Should succeed (same-origin request with proper Fetch metadata)
curl -X POST https://confab.dev/api/v1/keys \
     -H "Cookie: confab_session=abc" \
     -H "Content-Type: application/json" \
     -H "Origin: https://confab.dev" \
     -H "Sec-Fetch-Site: same-origin" \
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

### Environment Variables

See [`../CONFIGURATION.md`](../CONFIGURATION.md) for the canonical reference. The security-critical settings are:

- `CSRF_SECRET_KEY` — must be ≥ 32 chars; required.
- `DATABASE_URL` — should use `sslmode=require` (or stricter) in production.
- `FRONTEND_URL` / `ALLOWED_ORIGINS` — must list only trusted production domains; wildcard `*` is rejected at startup because cookie-based auth requires `AllowCredentials=true`.
- `INSECURE_DEV_MODE` — leave unset or `false` in production. When `true`, session/CSRF cookies skip the Secure flag, HSTS is disabled, and the server logs a WARN at startup.
- `S3_USE_SSL` — must be `true` (default) for any non-local-MinIO deployment.
- At least one auth provider (`AUTH_PASSWORD_ENABLED`, `GITHUB_*`, `GOOGLE_*`, or `OIDC_*`) must be configured.

Note: web sessions use a cryptographically random 32-byte session ID per session; there is no app-wide `SESSION_SECRET` to configure.

For optional settings (email allow-list, S3 credentials, `STATIC_FILES_DIR`, `ENABLE_PPROF` — bound to `localhost:6060` only, never publicly exposed), see [`../CONFIGURATION.md`](../CONFIGURATION.md).

> **MinIO defaults:** local Docker Compose examples use `minioadmin/minioadmin`.
> These are the upstream MinIO demo credentials — **change them before any
> production deployment** and re-run with `MINIO_ROOT_USER` /
> `MINIO_ROOT_PASSWORD` (and matching `AWS_ACCESS_KEY_ID` /
> `AWS_SECRET_ACCESS_KEY`) set to strong random values.

### Pre-Deployment Checklist

- [ ] `INSECURE_DEV_MODE` is unset or `false`
- [ ] `CSRF_SECRET_KEY` is set (32+ random bytes)
- [ ] `ALLOWED_ORIGINS` contains only trusted domains (no wildcard `*`)
- [ ] `FRONTEND_URL` points to production frontend
- [ ] `DATABASE_URL` uses SSL (`sslmode=require`)
- [ ] OAuth callback URLs are registered for every configured provider
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

## Reporting Vulnerabilities

Report suspected vulnerabilities by opening a GitHub issue or contacting the project maintainers via the channels listed in the root [`README.md`](../README.md).
