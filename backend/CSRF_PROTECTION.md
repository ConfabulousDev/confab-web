# CSRF Protection Implementation

## Overview

Cross-Site Request Forgery (CSRF) protection has been implemented to prevent attackers from forging requests to Confab's API using a victim's session credentials.

## What CSRF Protection Prevents

### Attack Scenarios Blocked

1. **API Key Theft**
   - Attacker cannot create API keys via forged requests
   - Prevents data exfiltration through stolen keys

2. **API Key Deletion (DoS)**
   - Attacker cannot delete user's API keys
   - Prevents denial of service attacks

3. **Unauthorized Share Creation**
   - Attacker cannot create public shares of private sessions
   - Prevents data exposure

4. **Session Data Poisoning**
   - Attacker cannot upload malicious session data
   - Prevents data corruption

## How It Works

### Double Submit Cookie Pattern

1. **Backend generates** unique CSRF token per session
2. **Token delivered** via both:
   - JSON response from `/api/v1/csrf-token`
   - `X-CSRF-Token` response header
3. **Frontend stores** token in memory (not cookies)
4. **Frontend includes** token in `X-CSRF-Token` header for state-changing requests
5. **Backend validates** token matches session
6. **Attacker blocked** because they cannot read the token via XHR (Same-Origin Policy)

### Token Lifecycle

```
User visits frontend
    ↓
Frontend calls GET /api/v1/csrf-token
    ↓
Backend generates token tied to session
    ↓
Frontend stores token in memory
    ↓
User clicks "Create API Key"
    ↓
Frontend sends POST /api/v1/keys with X-CSRF-Token header
    ↓
Backend validates token matches session
    ↓
Request processed if valid, rejected if invalid
```

## Configuration

### Environment Variables

```bash
# Required in production
CSRF_SECRET_KEY=your-32-byte-random-secret-key-here

# Optional - defaults to "production" check
ENVIRONMENT=production
```

### Generate CSRF Secret

```bash
# Linux/Mac
openssl rand -base64 32

# Or in Python
python3 -c "import secrets; print(secrets.token_urlsafe(32))"

# Example output (DO NOT USE THIS):
# Xp8vN2mK9qR4tL7wZ3sF6bH1jD5cG8aY
```

### Development vs Production

**Development (default):**
- Uses default secret key with warning
- `Secure` cookie flag disabled (allows HTTP)
- CSRF still enforced for testing

**Production (when `ENVIRONMENT=production`):**
- Requires `CSRF_SECRET_KEY` environment variable
- `Secure` cookie flag enabled (requires HTTPS)
- CSRF strictly enforced

## Protected Endpoints

CSRF tokens are **required** for these operations:

| Method | Endpoint | Operation |
|--------|----------|-----------|
| POST | `/api/v1/keys` | Create API key |
| DELETE | `/api/v1/keys/{id}` | Delete API key |
| POST | `/api/v1/sessions/{id}/share` | Create session share |
| DELETE | `/api/v1/shares/{token}` | Revoke share |

**Not protected (read operations):**
- GET `/api/v1/keys` - List API keys
- GET `/api/v1/sessions` - List sessions
- GET `/api/v1/sessions/{id}` - View session
- GET `/api/v1/sessions/{id}/shares` - List shares
- GET `/api/v1/sessions/{id}/shared/{token}` - View shared session

**CLI routes not protected:**
- POST `/api/v1/sessions/save` - Uses API key auth, not sessions

## Frontend Implementation

### Automatic Token Management

CSRF tokens are managed automatically via `src/lib/csrf.ts`:

```typescript
// Initialize on app load (in +layout.svelte)
import { initCSRF } from '$lib/csrf';
onMount(() => {
  initCSRF();
});

// Use for API calls
import { fetchWithCSRF } from '$lib/csrf';

// Automatically includes CSRF token for POST/PUT/DELETE
const response = await fetchWithCSRF('/api/v1/keys', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ name: 'My Key' })
});
```

### Manual Token Access

If needed, you can access the token directly:

```typescript
import { getCSRFToken } from '$lib/csrf';

const token = getCSRFToken();
```

## Testing CSRF Protection

### Test 1: Valid Request (Should Succeed)

```bash
# 1. Get session cookie by logging in via browser
# 2. Get CSRF token
curl -c cookies.txt http://localhost:8080/api/v1/csrf-token

# 3. Extract token from response
TOKEN="..." # From JSON response

# 4. Make authenticated request with token
curl -b cookies.txt \
  -H "X-CSRF-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"name":"test key"}' \
  http://localhost:8080/api/v1/keys

# Expected: 200 OK, API key created
```

### Test 2: Missing Token (Should Fail)

```bash
# Make request without CSRF token
curl -b cookies.txt \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"name":"evil key"}' \
  http://localhost:8080/api/v1/keys

# Expected: 403 Forbidden
# Response: {"error":"CSRF token validation failed"}
```

### Test 3: Invalid Token (Should Fail)

```bash
# Make request with wrong token
curl -b cookies.txt \
  -H "X-CSRF-Token: invalid-token-xyz" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"name":"evil key"}' \
  http://localhost:8080/api/v1/keys

# Expected: 403 Forbidden
# Response: {"error":"CSRF token validation failed"}
```

### Test 4: Cross-Origin Attack (Should Fail)

```html
<!-- Attacker's evil.com page -->
<script>
// This will fail because browser blocks reading CSRF token from different origin
fetch('https://confab.example.com/api/v1/csrf-token')
  .then(r => r.json())
  .then(data => {
    // Browser blocks this due to CORS
    // Even if it worked, this would also fail:
    return fetch('https://confab.example.com/api/v1/keys', {
      method: 'POST',
      credentials: 'include', // Sends session cookie
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': data.csrf_token // Cannot obtain this!
      },
      body: JSON.stringify({name: 'stolen'})
    });
  });
// Result: CSRF attack blocked!
</script>
```

## Security Guarantees

### What CSRF Protection Provides

✅ **Verifies request origin** - Only your frontend can obtain tokens
✅ **Prevents forged requests** - Attackers cannot include valid tokens
✅ **Session binding** - Tokens tied to specific user sessions
✅ **Automatic expiration** - Tokens expire with session
✅ **No user action needed** - Transparent protection

### What CSRF Protection Does NOT Provide

❌ **Authentication** - Still requires valid session/API key
❌ **Authorization** - Still requires permission checks
❌ **XSS protection** - Use Content Security Policy for that
❌ **SQL injection protection** - Use parameterized queries (already implemented)
❌ **Rate limiting** - Implement separately (still needed)

## Troubleshooting

### Issue: "CSRF token validation failed"

**Symptoms:**
- Frontend POST/DELETE requests fail with 403
- Error message: "CSRF token validation failed"

**Solutions:**

1. **Check token initialization**
   ```typescript
   // Verify CSRF initialized in browser console
   import { getCSRFToken } from '$lib/csrf';
   console.log(getCSRFToken()); // Should show token
   ```

2. **Check backend logs**
   ```
   # Look for warnings
   WARNING: CSRF_SECRET_KEY not set, using default (INSECURE for production)
   CSRF validation failed for POST /api/v1/keys from 127.0.0.1
   ```

3. **Verify request includes header**
   ```javascript
   // In browser DevTools Network tab
   // Check request headers for:
   X-CSRF-Token: <token-value>
   ```

4. **Check cookies**
   ```
   # Verify session cookie exists
   # In browser DevTools Application/Storage → Cookies
   confab_session=<session-id>
   ```

### Issue: Token not fetched on page load

**Cause:** `initCSRF()` not called or failed

**Solution:**
```typescript
// In +layout.svelte, verify:
import { onMount } from 'svelte';
import { initCSRF } from '$lib/csrf';

onMount(() => {
  initCSRF(); // Must be called!
});
```

### Issue: Different token in header vs JSON

**Cause:** This is normal - both are valid

**Solution:** Use either, frontend prefers header if available

### Issue: CSRF protection in development

**Symptoms:** CSRF blocks requests in local development

**Solutions:**

Option 1: Set secret key
```bash
export CSRF_SECRET_KEY=dev-secret-minimum-32-characters-long
```

Option 2: Temporarily disable (NOT RECOMMENDED)
```go
// In server.go (FOR TESTING ONLY, REMOVE BEFORE COMMIT)
if os.Getenv("DISABLE_CSRF") == "true" {
  // Skip CSRF middleware
} else {
  r.Use(csrfMiddleware)
}
```

## Deployment Checklist

- [ ] Set `CSRF_SECRET_KEY` to 32+ character random string
- [ ] Set `ENVIRONMENT=production`
- [ ] Verify HTTPS enabled (`Secure` flag on cookies)
- [ ] Test CSRF protection works in production
- [ ] Monitor logs for CSRF validation failures
- [ ] Document secret key in secure password manager
- [ ] Rotate secret key periodically (invalidates all tokens)

## Architecture

### Backend (Go)

```
middleware.RealIP
    ↓
middleware.Logger
    ↓
cors.Handler (CORS check)
    ↓
    ├─ /api/v1/csrf-token (public)
    │
    ├─ /api/v1/sessions/save (API key auth, no CSRF)
    │
    └─ Web routes (session auth + CSRF)
        ├─ csrf.Protect (validate token)
        ├─ auth.SessionMiddleware (verify session)
        └─ handlers (create key, create share, etc.)
```

### Frontend (Svelte)

```
+layout.svelte
    ├─ initCSRF() on mount
    │   └─ fetch('/api/v1/csrf-token')
    │       └─ store token in memory
    │
    └─ All pages use fetchWithCSRF()
        └─ Automatically adds X-CSRF-Token header
```

## Security Best Practices

1. **Never disable in production** - CSRF protection is critical
2. **Rotate secrets regularly** - Change `CSRF_SECRET_KEY` quarterly
3. **Monitor failed validations** - May indicate attack attempts
4. **Use HTTPS in production** - Required for `Secure` cookie flag
5. **Keep tokens secret** - Never log or expose tokens
6. **Don't cache tokens long-term** - They expire with session

## Interaction with Other Security Measures

### CORS + CSRF = Defense in Depth

```
Layer 1: CORS
    ↓ (Blocks unauthorized origins)
Layer 2: CSRF
    ↓ (Verifies request authenticity)
Layer 3: Authentication
    ↓ (Verifies user identity)
Layer 4: Authorization
    ↓ (Verifies permissions)
Handler executes
```

### Session Security

CSRF protection relies on secure sessions:

- Session cookies use `HttpOnly` flag
- Session cookies use `Secure` flag in production
- Session cookies use `SameSite=Strict` mode
- Sessions expire after 7 days
- Invalid sessions rejected before CSRF check

## References

- [OWASP CSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- [gorilla/csrf Documentation](https://pkg.go.dev/github.com/gorilla/csrf)
- [MDN: CSRF Attacks](https://developer.mozilla.org/en-US/docs/Glossary/CSRF)

## Changelog

### 2025-01-16: Initial Implementation

- Added gorilla/csrf middleware
- Created CSRF token endpoint
- Protected POST/DELETE web session routes
- Updated frontend with automatic token management
- CLI routes excluded (use API key auth)
