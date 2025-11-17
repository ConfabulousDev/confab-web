# Security Audit Report

**Date:** 2025-01-16
**Scope:** Production readiness security review for Confab application
**Total Issues Identified:** 21

## Status Summary

- ✅ **Critical (4/4 fixed)** - All fixed and committed
- ⏳ **High (0/8 fixed)** - Outstanding
- ⏳ **Medium (0/6 fixed)** - Outstanding
- ⏳ **Low (0/3 fixed)** - Outstanding

---

## ✅ CRITICAL ISSUES (ALL FIXED)

### 1. Missing CORS Configuration ✅ FIXED
**Severity:** Critical
**Status:** Fixed in commit e38d125
**Impact:** Any website could make authenticated requests to API
**Fix:** Implemented go-chi/cors middleware with environment-configurable origins

### 2. No CSRF Protection ✅ FIXED
**Severity:** Critical
**Status:** Fixed in commit e38d125
**Impact:** Attackers could forge requests to create/delete API keys and shares
**Fix:** Implemented gorilla/csrf with double-submit cookie pattern

### 3. Insecure Cookie Settings (Secure=false) ✅ FIXED
**Severity:** Critical
**Status:** Fixed in commit eb71bd4
**Impact:** Session hijacking via HTTP network sniffing
**Fix:** Added Secure flag with INSECURE_DEV_MODE opt-out, changed SameSite to Lax

### 4. Open Redirect Vulnerability ✅ FIXED
**Severity:** Critical
**Status:** Fixed in commit 96acad1
**Impact:** API key theft via malicious redirects
**Fix:** Replaced naive URL check with proper URL parsing and validation

---

## ⏳ HIGH SEVERITY ISSUES (OUTSTANDING)

### 5. No Rate Limiting
**Severity:** High
**Status:** Not fixed
**Impact:** Brute force attacks, DoS, API abuse

**Vulnerable Endpoints:**
- `/auth/github/login` - OAuth initiation spam
- `/auth/github/callback` - OAuth callback flooding
- `/api/v1/sessions/save` - Session upload spam
- `/api/v1/keys` - API key creation spam
- `/api/v1/sessions/{id}/share` - Share creation spam

**Recommended Fix:**
```go
import "github.com/ulule/limiter/v3"

// Rate limit by IP
limiter := limiter.New(store, rate.Limit{
    Period: 1 * time.Minute,
    Limit:  60, // 60 requests per minute
})

// Apply to sensitive routes
r.Use(middleware.RateLimiter(limiter))
```

**Attack Scenario:**
- Attacker floods `/api/v1/sessions/save` with 1000s of sessions
- Database fills up, application slows down
- Legitimate users unable to save sessions

---

### 6. Secrets in Environment Variables
**Severity:** High
**Status:** Not fixed
**Impact:** Secrets exposed in process listings, logs, error traces

**Current Issues:**
- `GITHUB_CLIENT_SECRET` in plain environment variable
- `CSRF_SECRET_KEY` in plain environment variable
- `DATABASE_URL` contains password in connection string

**Recommended Fix:**
Use secret management service:
```go
// Option 1: HashiCorp Vault
client, _ := vault.NewClient(vault.DefaultConfig())
secret, _ := client.Logical().Read("secret/data/confab")
githubSecret := secret.Data["github_client_secret"].(string)

// Option 2: AWS Secrets Manager
// Option 3: Google Secret Manager
// Option 4: At minimum, use encrypted files
```

**Attack Scenario:**
- Attacker gains read access to `/proc/{pid}/environ`
- Extracts `GITHUB_CLIENT_SECRET`
- Impersonates Confab application in OAuth flow

---

### 7. No Input Validation on Session Upload
**Severity:** High
**Status:** Not fixed
**Impact:** Storage exhaustion, malicious content injection

**Vulnerable Code:**
`internal/api/sessions_view.go` - `/api/v1/sessions/save` endpoint

**Current Issues:**
- No size limit on session data
- No validation of JSON structure
- No sanitization of user input in messages

**Recommended Fix:**
```go
const MaxSessionSize = 10 * 1024 * 1024 // 10MB

func handleSaveSession(w http.ResponseWriter, r *http.Request) {
    // Limit request body size
    r.Body = http.MaxBytesReader(w, r.Body, MaxSessionSize)

    // Validate JSON structure
    var session SessionData
    if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
        http.Error(w, "Invalid session format", 400)
        return
    }

    // Validate content
    if len(session.Messages) > 1000 {
        http.Error(w, "Too many messages", 400)
        return
    }

    // Sanitize user content (prevent XSS when displayed)
    for i := range session.Messages {
        session.Messages[i].Content = sanitize(session.Messages[i].Content)
    }
}
```

**Attack Scenario:**
- Attacker uploads 1GB session payload
- Fills S3 storage, runs up AWS bill
- Or: Injects malicious JavaScript in session data
- When viewed in web UI, XSS executes

---

### 8. No Session Cleanup/Expiration
**Severity:** High
**Status:** Not fixed
**Impact:** Database bloat, stale sessions never removed

**Current Issues:**
- Web sessions created with 7-day expiry but never deleted
- Expired sessions still in database
- No cleanup job for old data

**Recommended Fix:**
```go
// Periodic cleanup job
func CleanupExpiredSessions(db *DB) {
    ticker := time.NewTicker(1 * time.Hour)

    for range ticker.C {
        result, err := db.Exec(`
            DELETE FROM web_sessions
            WHERE expires_at < NOW()
        `)

        if err == nil {
            rowsAffected, _ := result.RowsAffected()
            log.Printf("Cleaned up %d expired sessions", rowsAffected)
        }
    }
}

// Run in background
go CleanupExpiredSessions(database)
```

**Attack Scenario:**
- Millions of expired sessions accumulate
- Database grows to gigabytes
- Query performance degrades
- Backup/restore times increase

---

### 9. Missing Security Headers
**Severity:** High
**Status:** Not fixed
**Impact:** XSS, clickjacking, MIME sniffing attacks

**Missing Headers:**
- `Content-Security-Policy` - Prevents XSS
- `X-Frame-Options` - Prevents clickjacking
- `X-Content-Type-Options` - Prevents MIME sniffing
- `Strict-Transport-Security` - Forces HTTPS
- `Referrer-Policy` - Controls referrer leakage

**Recommended Fix:**
```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("Strict-Transport-Security",
            "max-age=31536000; includeSubDomains")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

        next.ServeHTTP(w, r)
    })
}

r.Use(securityHeaders)
```

---

### 10. API Keys Stored in Browser LocalStorage
**Severity:** High
**Status:** Not fixed (if applicable to frontend)
**Impact:** XSS can steal API keys

**Issue:**
If frontend stores API keys in localStorage, they're accessible to JavaScript and vulnerable to XSS.

**Recommended Fix:**
- Never store API keys in localStorage
- Use httpOnly cookies for web sessions (already done ✅)
- API keys only for CLI, never exposed to browser

---

### 11. No Audit Logging
**Severity:** High
**Status:** Not fixed
**Impact:** Cannot detect or investigate security incidents

**Missing Logs:**
- API key creation/deletion
- Session share creation/revocation
- Failed authentication attempts
- Unusual access patterns

**Recommended Fix:**
```go
type AuditLog struct {
    Timestamp time.Time
    UserID    int64
    Action    string // "api_key_created", "share_created", etc.
    Resource  string // Resource affected
    IPAddress string
    UserAgent string
    Success   bool
}

func logAudit(db *DB, log AuditLog) {
    db.Exec(`
        INSERT INTO audit_logs
        (timestamp, user_id, action, resource, ip_address, user_agent, success)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, log.Timestamp, log.UserID, log.Action, log.Resource,
       log.IPAddress, log.UserAgent, log.Success)
}
```

---

### 12. OAuth State Token Not Cryptographically Random
**Severity:** High
**Status:** Needs verification
**Impact:** OAuth CSRF bypass if state is predictable

**Current Code:**
Uses `crypto/rand` (good) but should verify entropy is sufficient.

**Verification Needed:**
```go
// Check generateRandomString() uses crypto/rand correctly
bytes := make([]byte, length)
if _, err := rand.Read(bytes); err != nil {
    return "", err
}
```

If using math/rand instead of crypto/rand, this is critical.

---

## ⏳ MEDIUM SEVERITY ISSUES (OUTSTANDING)

### 13. No HTTPS Enforcement
**Severity:** Medium
**Status:** Not fixed
**Impact:** Relies on reverse proxy configuration

**Issue:**
Application doesn't enforce HTTPS itself, depends on deployment.

**Recommended Fix:**
```go
func httpsRedirect(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Forwarded-Proto") != "https" &&
           os.Getenv("INSECURE_DEV_MODE") != "true" {
            https := "https://" + r.Host + r.RequestURI
            http.Redirect(w, r, https, http.StatusMovedPermanently)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

### 14. Database Passwords in Plain Connection Strings
**Severity:** Medium
**Status:** Not fixed
**Impact:** Password exposed in logs, error messages

**Current:**
```go
DATABASE_URL=postgres://user:password@host/db
```

**Recommended Fix:**
Use IAM authentication or connection string parsing that hides passwords in logs.

---

### 15. No SQL Injection Testing
**Severity:** Medium
**Status:** Unknown
**Impact:** Potential SQL injection if parameterization missed

**Action Needed:**
- Audit all database queries
- Verify all use parameterized queries ($1, $2, etc.)
- Never use string concatenation with user input

---

### 16. Error Messages Too Verbose
**Severity:** Medium
**Status:** Not fixed
**Impact:** Information leakage

**Current Code:**
```go
fmt.Printf("Error creating user: %v\n", err)
```

**Issue:** Detailed errors logged/returned could reveal:
- Database schema
- Internal paths
- Implementation details

**Recommended Fix:**
```go
// Log detailed error internally
log.Printf("Error creating user %d: %v", userID, err)

// Return generic error to client
http.Error(w, "Failed to create user", 500)
```

---

### 17. No Account Lockout
**Severity:** Medium
**Status:** Not fixed
**Impact:** Unlimited login attempts possible

**Issue:**
No protection against brute force on OAuth or session endpoints.

**Fix:** Combine with rate limiting (#5)

---

### 18. Session Fixation Possible
**Severity:** Medium
**Status:** Needs verification
**Impact:** Attacker could set victim's session ID

**Check:**
- Is session ID regenerated after login?
- Can attacker set session cookie before auth?

**Recommended Fix:**
```go
// After successful authentication
newSessionID, _ := generateRandomString(32)
http.SetCookie(w, &http.Cookie{
    Name:  SessionCookieName,
    Value: newSessionID, // New ID, not reusing old one
    // ...
})
```

---

## ⏳ LOW SEVERITY ISSUES (OUTSTANDING)

### 19. No Dependency Vulnerability Scanning
**Severity:** Low
**Status:** Not implemented
**Impact:** Using vulnerable dependencies unknowingly

**Recommended Fix:**
```bash
# Add to CI/CD
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

### 20. No Security.txt File
**Severity:** Low
**Status:** Not implemented
**Impact:** Security researchers don't know how to report issues

**Recommended Fix:**
Create `/.well-known/security.txt`:
```
Contact: security@confab.example.com
Expires: 2026-01-01T00:00:00.000Z
Preferred-Languages: en
```

---

### 21. No Content-Type Validation
**Severity:** Low
**Status:** Not fixed
**Impact:** Could accept wrong content types

**Issue:**
API doesn't validate `Content-Type: application/json` header.

**Recommended Fix:**
```go
func validateContentType(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == "POST" || r.Method == "PUT" {
            ct := r.Header.Get("Content-Type")
            if ct != "application/json" {
                http.Error(w, "Content-Type must be application/json", 415)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## Priority Recommendations

### Immediate (Before Production)
1. ✅ CORS protection (DONE)
2. ✅ CSRF protection (DONE)
3. ✅ Secure cookies (DONE)
4. ✅ Open redirect fix (DONE)
5. ⏳ Rate limiting (#5)
6. ⏳ Input validation (#7)
7. ⏳ Security headers (#9)

### Short Term (First Month)
8. ⏳ Secrets management (#6)
9. ⏳ Session cleanup (#8)
10. ⏳ Audit logging (#11)
11. ⏳ HTTPS enforcement (#13)

### Medium Term (First Quarter)
12. ⏳ SQL injection audit (#15)
13. ⏳ Error message sanitization (#16)
14. ⏳ Session fixation check (#18)
15. ⏳ Dependency scanning (#19)

### Nice to Have
16. ⏳ API key localStorage check (#10)
17. ⏳ Account lockout (#17)
18. ⏳ Security.txt (#20)
19. ⏳ Content-Type validation (#21)

---

## References

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [Go Security Checklist](https://github.com/securego/gosec)
