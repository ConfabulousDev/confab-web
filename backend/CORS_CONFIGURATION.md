# CORS Configuration Guide

## Overview

CORS (Cross-Origin Resource Sharing) is now properly configured to prevent unauthorized websites from making requests to the Confab API.

## Security Impact

**Without CORS protection:**
- ❌ Any website could make authenticated requests to your API
- ❌ Stolen session cookies could be exploited from malicious sites
- ❌ CSRF attacks were possible

**With CORS protection:**
- ✅ Only whitelisted frontend domains can access the API
- ✅ Unauthorized websites are blocked by the browser
- ✅ Credentials (cookies, auth headers) only sent to trusted origins

## Configuration

### Environment Variables

#### Development (Local)

```bash
# .env or export
FRONTEND_URL=http://localhost:5173

# Or use ALLOWED_ORIGINS for multiple origins
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
```

#### Production

```bash
# Single production domain
ALLOWED_ORIGINS=https://confab.yourdomain.com

# Multiple domains (staging + production)
ALLOWED_ORIGINS=https://confab.yourdomain.com,https://staging.confab.yourdomain.com
```

### Configuration Details

The CORS middleware is configured with:

- **AllowedOrigins**: Comma-separated list from `ALLOWED_ORIGINS` env var
  - Falls back to `FRONTEND_URL` if not set
  - Defaults to `http://localhost:5173` in development

- **AllowedMethods**: `GET, POST, PUT, DELETE, OPTIONS`

- **AllowedHeaders**: `Accept, Authorization, Content-Type, X-CSRF-Token`

- **AllowCredentials**: `true` (allows cookies and Authorization headers)

- **MaxAge**: `300` seconds (5 minutes browser cache)

## Examples

### Single Frontend

```bash
# Production
ALLOWED_ORIGINS=https://confab.example.com

# Development
ALLOWED_ORIGINS=http://localhost:5173
```

### Multiple Frontends

```bash
# Production with multiple domains
ALLOWED_ORIGINS=https://confab.example.com,https://app.example.com,https://www.example.com
```

### Staging + Production

```bash
ALLOWED_ORIGINS=https://confab.example.com,https://staging-confab.example.com
```

## Testing CORS

### Browser DevTools Test

1. Open browser DevTools → Network tab
2. Make a request from your frontend to the API
3. Check the response headers:
   - `Access-Control-Allow-Origin` should match your frontend URL
   - `Access-Control-Allow-Credentials` should be `true`

### curl Test

```bash
# Test from allowed origin (should succeed)
curl -H "Origin: http://localhost:5173" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS \
     http://localhost:8080/api/v1/sessions

# Test from unauthorized origin (should be blocked by browser)
curl -H "Origin: https://evil.com" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -X OPTIONS \
     http://localhost:8080/api/v1/sessions
```

Expected response headers:
```
Access-Control-Allow-Origin: http://localhost:5173
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Accept, Authorization, Content-Type, X-CSRF-Token
Access-Control-Allow-Credentials: true
Access-Control-Max-Age: 300
```

## Security Best Practices

### ✅ DO:
- Use HTTPS in production: `https://confab.example.com`
- Keep the allowed origins list minimal
- Use specific domains, not wildcards
- Review and update allowed origins regularly

### ❌ DON'T:
- Use `*` wildcard with credentials (not supported)
- Allow `http://` origins in production
- Add development URLs to production config
- Include trailing slashes in origins

## Troubleshooting

### Issue: "CORS error" in browser console

**Solution:**
1. Check `ALLOWED_ORIGINS` includes your frontend URL
2. Verify no trailing slash: `http://localhost:5173` not `http://localhost:5173/`
3. Check protocol matches (http vs https)
4. Restart backend after changing env vars

### Issue: Credentials not being sent

**Solution:**
1. Verify `AllowCredentials: true` in CORS config ✓ (already set)
2. Frontend must set `credentials: 'include'` in fetch calls ✓ (already implemented)
3. `AllowedOrigins` must be specific, not `*`

### Issue: Preflight OPTIONS requests failing

**Solution:**
1. CORS middleware is now before routes ✓ (fixed)
2. Check `AllowedHeaders` includes headers you're sending
3. Verify `AllowedMethods` includes your HTTP method

## Migration Notes

**Changes Made:**
- Added `github.com/go-chi/cors` dependency
- CORS middleware added in `server.go:66-79`
- New env var: `ALLOWED_ORIGINS` (optional, comma-separated)
- Falls back to `FRONTEND_URL` for backward compatibility

**Breaking Changes:**
- None. Defaults to `http://localhost:5173` for local development

**Deployment Checklist:**
- [ ] Set `ALLOWED_ORIGINS` in production environment
- [ ] Use HTTPS URLs in production
- [ ] Remove any development URLs from production config
- [ ] Test CORS with browser DevTools
- [ ] Verify credentials are being sent correctly
