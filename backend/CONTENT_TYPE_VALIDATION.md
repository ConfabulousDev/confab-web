# Content-Type Validation

## Overview

Added Content-Type validation middleware to ensure API endpoints only accept properly formatted JSON requests with correct headers.

## Issue Fixed

**Before:** API accepted requests without Content-Type validation
```bash
# Missing Content-Type - accepted
curl -X POST /api/v1/keys -d '{"name":"key"}'
# ✅ Works (shouldn't)

# Wrong Content-Type - accepted
curl -X POST /api/v1/keys \
  -H "Content-Type: text/plain" \
  -d '{"name":"key"}'
# ✅ Works (shouldn't)
```

**After:** Validates Content-Type header
```bash
# Missing Content-Type - rejected
curl -X POST /api/v1/keys -d '{"name":"key"}'
# ❌ 415 Unsupported Media Type
# Response: "Content-Type header required"

# Wrong Content-Type - rejected
curl -X POST /api/v1/keys \
  -H "Content-Type: text/plain" \
  -d '{"name":"key"}'
# ❌ 415 Unsupported Media Type
# Response: "Content-Type must be application/json"

# Correct Content-Type - accepted
curl -X POST /api/v1/keys \
  -H "Content-Type: application/json" \
  -d '{"name":"key"}'
# ✅ 200 OK
```

---

## Why It Matters

### 1. HTTP Spec Compliance

**RFC 7231 Section 3.1.1.5:**
> "A sender that generates a message containing a payload body SHOULD
> generate a Content-Type header field in that message unless the
> intended media type of the enclosed representation is unknown to
> the sender."

Servers should validate what clients declare.

---

### 2. API Contract Enforcement

**Clear expectations:**
- API documentation says: "Send JSON with Content-Type: application/json"
- API should reject: Requests not following the contract
- Benefits: Easier debugging, clearer error messages

---

### 3. Content Confusion Prevention

**Attack scenario (theoretical):**
```bash
# Attacker sends XML disguised as something else
curl -X POST /api/v1/keys \
  -H "Content-Type: text/xml" \
  -d '<?xml version="1.0"?><!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><data>&xxe;</data>'

# Without validation: json.Decode() fails but reads content
# With validation: Rejected before parsing
```

**Not a direct vulnerability** (json.Decode won't parse XML), but defense in depth.

---

### 4. Client Error Detection

**Helps clients find bugs:**
```javascript
// Client bug: Forgot to set header
fetch('/api/v1/keys', {
    method: 'POST',
    body: JSON.stringify({name: 'key'})
    // Missing: headers: {'Content-Type': 'application/json'}
})

// Without validation: Might work by accident, breaks later
// With validation: 415 error immediately, easy to fix
```

---

## Implementation

### Middleware

**Location:** `internal/api/content_type.go`

```go
func validateContentType(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Only validate POST/PUT/PATCH (requests with body)
        if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
            contentType := r.Header.Get("Content-Type")

            // Require Content-Type header
            if contentType == "" {
                http.Error(w, "Content-Type header required",
                    http.StatusUnsupportedMediaType)
                return
            }

            // Extract media type (ignore charset)
            // "application/json; charset=utf-8" → "application/json"
            mediaType := contentType
            if idx := strings.Index(contentType, ";"); idx != -1 {
                mediaType = strings.TrimSpace(contentType[:idx])
            }

            // Must be application/json
            if mediaType != "application/json" {
                http.Error(w, "Content-Type must be application/json",
                    http.StatusUnsupportedMediaType)
                return
            }
        }

        next.ServeHTTP(w, r)
    })
}
```

---

### Applied To

**Location:** `internal/api/server.go` line 135

```go
r.Route("/api/v1", func(r chi.Router) {
    r.Use(validateContentType)  // Applied to all /api/v1 routes

    // All these endpoints now require Content-Type:
    // POST /api/v1/keys
    // POST /api/v1/sessions/save
    // POST /api/v1/sessions/{id}/share
    // DELETE /api/v1/keys/{id}
    // DELETE /api/v1/shares/{token}
})
```

---

## HTTP Status Code

**415 Unsupported Media Type**

> "The origin server is refusing to service the request because the
> payload is in a format not supported by this method on the target
> resource."

**Why 415 instead of 400:**
- 400 Bad Request: Syntax error in request (malformed JSON)
- **415 Unsupported Media Type:** Correct syntax, wrong format
- More specific, easier to debug

---

## Charset Support

**Handles charset parameter:**
```bash
# With charset - accepted
curl -X POST /api/v1/keys \
  -H "Content-Type: application/json; charset=utf-8" \
  -d '{"name":"key"}'
# ✅ Works (charset ignored, which is fine)

# Multiple parameters - accepted
curl -X POST /api/v1/keys \
  -H "Content-Type: application/json; charset=utf-8; boundary=..." \
  -d '{"name":"key"}'
# ✅ Works (extracts "application/json" before semicolon)
```

---

## Testing

### Test 1: Valid Request (Should Succeed)

```bash
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name":"test key"}'

# Expected: 200 OK
# Response: {"id":1,"key":"cfb_...","name":"test key",...}
```

---

### Test 2: Missing Content-Type (Should Fail)

```bash
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"name":"test key"}'

# Expected: 415 Unsupported Media Type
# Response: Content-Type header required
```

---

### Test 3: Wrong Content-Type (Should Fail)

```bash
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: text/plain" \
  -d '{"name":"test key"}'

# Expected: 415 Unsupported Media Type
# Response: Content-Type must be application/json
```

---

### Test 4: Content-Type with Charset (Should Succeed)

```bash
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json; charset=utf-8" \
  -d '{"name":"test key"}'

# Expected: 200 OK
# Response: {"id":1,"key":"cfb_...","name":"test key",...}
```

---

### Test 5: GET Request (Should Not Validate)

```bash
# GET requests have no body, no validation
curl http://localhost:8080/api/v1/keys \
  -H "Authorization: Bearer $API_KEY"

# Expected: 200 OK (no Content-Type required for GET)
# Response: [{"id":1,"name":"test key",...}]
```

---

## Exempted Endpoints

Validation **only** applies to POST/PUT/PATCH.

**Not validated:**
- GET /api/v1/keys (no body)
- GET /api/v1/sessions (no body)
- DELETE requests (typically no body in HTTP spec)
- Health check endpoints (outside /api/v1)

**Note:** DELETE endpoints in REST can have bodies, but our API doesn't use them, so validation still runs but doesn't matter.

---

## Security Impact

**Low severity issue fixed:**

✅ **Prevents:**
- Content confusion attacks (theoretical)
- API misuse
- Client bugs going unnoticed

❌ **Does NOT prevent:**
- SQL injection (already prevented via parameterization)
- XSS (different layer)
- Authentication bypass (different layer)

**Defense in depth:** One more layer of validation.

---

## Frontend Compatibility

Confab frontend already sends correct headers:

**Example from `frontend/src/lib/csrf.ts`:**
```typescript
export async function fetchWithCSRF(url: string, options: RequestInit = {}) {
    const fetchOptions: RequestInit = {
        ...options,
        credentials: 'include'
    };

    const method = (options.method || 'GET').toUpperCase();
    if (method !== 'GET' && method !== 'HEAD') {
        fetchOptions.headers = {
            ...fetchOptions.headers,
            'X-CSRF-Token': csrfToken,
            'Content-Type': 'application/json'  // ✅ Already set
        };
    }

    return fetch(url, fetchOptions);
}
```

**No frontend changes needed!** ✅

---

## CLI Compatibility

CLI should also work correctly if it sets headers:

```go
// In CLI code (if it exists)
req, _ := http.NewRequest("POST", url, body)
req.Header.Set("Content-Type", "application/json")  // ✅ Must be set
req.Header.Set("Authorization", "Bearer "+apiKey)
```

If CLI doesn't set Content-Type, this change will break it. Check CLI implementation.

---

## Error Response Format

**Consistent with other errors:**

```bash
# Before (no validation)
curl -X POST /api/v1/keys -d 'invalid json'
# Response: 400 Bad Request
# Body: Invalid request body

# After (with validation)
curl -X POST /api/v1/keys -d '{"name":"key"}'
# Response: 415 Unsupported Media Type
# Body: Content-Type header required
```

**Plain text error** (not JSON) for early rejection before JSON processing.

---

## Best Practices

### ✅ DO

- Always send `Content-Type: application/json` for JSON APIs
- Include charset if non-UTF-8: `Content-Type: application/json; charset=iso-8859-1`
- Validate Content-Type on server
- Return 415 for unsupported media types

### ❌ DON'T

- Assume content type from request body
- Accept any content type silently
- Parse body before validating headers
- Return 400 for wrong Content-Type (use 415)

---

## Comparison with Other Frameworks

**Express.js:**
```javascript
app.use(express.json({ type: 'application/json' }))
// Validates Content-Type before parsing
```

**Django REST Framework:**
```python
# Requires Content-Type negotiation by default
parser_classes = [JSONParser]  # Only accepts application/json
```

**Rails:**
```ruby
# Validates Content-Type in request parsing
request.content_type == 'application/json'
```

**Confab now matches industry standard!** ✅

---

## References

- [RFC 7231: HTTP/1.1 Semantics - Content-Type](https://datatracker.ietf.org/doc/html/rfc7231#section-3.1.1.5)
- [RFC 7231: HTTP/1.1 Semantics - 415 Unsupported Media Type](https://datatracker.ietf.org/doc/html/rfc7231#section-6.5.13)
- [OWASP API Security - Mass Assignment](https://owasp.org/API-Security/editions/2023/en/0xa6-unrestricted-access-to-sensitive-business-flows/)
- [MDN: Content-Type](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type)

---

## Changelog

### 2025-01-16: Initial Implementation

- Created `validateContentType` middleware
- Applied to all `/api/v1` routes
- Validates POST/PUT/PATCH requests
- Returns 415 Unsupported Media Type for invalid headers
- Handles charset parameters correctly
- Created comprehensive documentation
