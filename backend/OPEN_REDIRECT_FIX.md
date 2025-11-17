# Open Redirect Vulnerability Fix

## Overview

Fixed an **open redirect vulnerability** in the CLI authorization flow that could allow attackers to steal API keys by redirecting users to malicious websites.

## Vulnerability Details

### What is an Open Redirect?

An open redirect occurs when an application redirects users to URLs controlled by attackers. It's dangerous because users trust *your domain* but get sent to *malicious sites*.

### The Vulnerability

**Location:** `backend/internal/auth/oauth.go` - `HandleCLIAuthorize` function

**Vulnerable Code (Before Fix):**
```go
func isLocalhostURL(urlStr string) bool {
    if urlStr == "" {
        return false
    }
    // Simple check for localhost/127.0.0.1
    return len(urlStr) >= 16 && (urlStr[:16] == "http://localhost" || urlStr[:16] == "http://127.0.0.1")
}
```

**Problem:** Only checks the first 16 characters. Doesn't parse URL structure properly.

### Attack Scenarios

#### Attack 1: Username Bypass
```bash
# Attacker crafts malicious URL:
callback="http://localhost@evil.com/steal"

# Naive validation:
urlStr[:16] == "http://localhost"  # ✅ PASSES (checks first 16 chars only)

# What actually happens:
# Browser interprets "localhost@" as username in URL
# Redirects to: http://evil.com/steal?key=confab_live_abc123xyz
# API key stolen by attacker!
```

#### Attack 2: Subdomain Bypass
```bash
# Attacker crafts malicious URL:
callback="http://localhost.evil.com/phish"

# Naive validation:
urlStr[:16] == "http://localhost"  # ✅ PASSES

# What actually happens:
# Redirects to attacker's subdomain: localhost.evil.com
# API key sent to attacker's server
```

### Attack Flow

```
1. Attacker sends phishing email with link:
   https://confab.example.com/auth/cli/authorize?callback=http://localhost@evil.com

2. Victim clicks link (trusts confab.example.com domain)

3. Victim logs in with GitHub if needed

4. Confab generates API key

5. Confab redirects to: http://localhost@evil.com?key=confab_live_abc123
   (Browser interprets this as http://evil.com)

6. Attacker's server logs API key from URL

7. Attacker uses stolen key to access victim's data
```

### Impact

- **API Key Theft** - Full account access
- **Data Exfiltration** - Access to all user sessions
- **Trusted Domain Abuse** - Users trust confab.example.com
- **No User Warning** - Browser doesn't warn about server-side redirects
- **Bypasses CORS/CSRF** - Redirect happens server-side, not from JavaScript

## The Fix

### Secure Implementation

```go
import (
    "net/url"
    "strconv"
)

func isLocalhostURL(urlStr string) bool {
    if urlStr == "" {
        return false
    }

    // Parse URL properly using net/url package
    u, err := url.Parse(urlStr)
    if err != nil {
        return false  // Reject malformed URLs
    }

    // Only allow http scheme (localhost doesn't need https)
    if u.Scheme != "http" {
        return false
    }

    // Get hostname without port
    hostname := u.Hostname()

    // Only allow exact match for localhost or 127.0.0.1
    if hostname != "localhost" && hostname != "127.0.0.1" {
        return false
    }

    // Reject URLs with username/password (e.g., http://localhost@evil.com)
    if u.User != nil {
        return false
    }

    // Validate port if present
    if port := u.Port(); port != "" {
        portNum, err := strconv.Atoi(port)
        if err != nil {
            return false
        }
        // Port must be in valid range
        if portNum < 1 || portNum > 65535 {
            return false
        }
    }

    return true
}
```

### What Changed

**Before:**
- ❌ String prefix check only
- ❌ No URL parsing
- ❌ Vulnerable to `@` bypass
- ❌ Vulnerable to subdomain bypass
- ❌ No port validation

**After:**
- ✅ Proper URL parsing with `net/url`
- ✅ Validates scheme (http only)
- ✅ Validates hostname exactly
- ✅ Rejects username/password in URL
- ✅ Validates port range
- ✅ Returns false on any malformed input

## Testing

### Test 1: Valid Localhost URLs (Should Pass)

```go
// All of these should return true
isLocalhostURL("http://localhost:8080")          // ✅
isLocalhostURL("http://localhost:3000/callback") // ✅
isLocalhostURL("http://127.0.0.1:8080")          // ✅
isLocalhostURL("http://localhost")               // ✅
```

### Test 2: Attack Attempts (Should Fail)

```go
// All of these should return false (blocked)
isLocalhostURL("http://localhost@evil.com")           // ❌ Username bypass
isLocalhostURL("http://localhost.evil.com")           // ❌ Subdomain
isLocalhostURL("http://user:pass@localhost")          // ❌ Credentials
isLocalhostURL("https://localhost")                   // ❌ Wrong scheme
isLocalhostURL("http://localhost.localdomain")        // ❌ Subdomain
isLocalhostURL("http://localhost:99999")              // ❌ Invalid port
isLocalhostURL("http://localhost:-1")                 // ❌ Invalid port
isLocalhostURL("http://evil.com?redirect=localhost")  // ❌ Wrong host
isLocalhostURL("javascript:alert('xss')")             // ❌ Wrong scheme
isLocalhostURL("//localhost")                         // ❌ No scheme
```

### Manual Testing

```bash
# Test CLI authorization with valid localhost
curl "http://localhost:8080/auth/cli/authorize?callback=http://localhost:8081/callback&name=test" \
  -H "Cookie: confab_session=<valid-session>"

# Expected: Redirects to localhost:8081 with API key

# Test with attack payload
curl "http://localhost:8080/auth/cli/authorize?callback=http://localhost@evil.com&name=test" \
  -H "Cookie: confab_session=<valid-session>"

# Expected: 400 Bad Request - "Callback must be localhost"
```

## Security Guarantees

### Attack Vectors Blocked

✅ **Username/password in URL** - `http://localhost@evil.com` rejected
✅ **Subdomain attacks** - `http://localhost.evil.com` rejected
✅ **Scheme switching** - Only `http` allowed
✅ **Port overflow** - Validates port 1-65535
✅ **Malformed URLs** - Parser rejects invalid syntax
✅ **Hostname variations** - Exact match required

### Legitimate Use Cases Preserved

✅ **Local CLI tools** - `http://localhost:8080` works
✅ **Different ports** - `http://localhost:3000` works
✅ **IP address** - `http://127.0.0.1:8080` works
✅ **With paths** - `http://localhost/callback` works

## Why This Matters

### Real-World Impact

Similar vulnerabilities have affected major platforms:

- **GitHub (2012)** - Open redirect in OAuth flow
- **Google (2015)** - Open redirect in authentication
- **Slack (2017)** - Redirect after login vulnerability

### Confab-Specific Risks

Without this fix:
1. Attacker sends phishing email: "Verify your Confab CLI"
2. Link goes to real Confab domain (looks legitimate)
3. User authorizes (trusts the domain)
4. User gets redirected to attacker site
5. **API key leaked in URL to attacker**
6. Attacker accesses victim's sessions, creates malicious shares

## Best Practices

### URL Validation Rules

1. **Always use URL parsers** - Never use string manipulation
2. **Validate each component** - Scheme, hostname, port, user, path
3. **Use allowlists** - Whitelist exact values, don't blocklist patterns
4. **Reject by default** - Return false for any unexpected input
5. **Test attack vectors** - Include security tests in test suite

### Redirect Safety

For any redirect endpoint:
- ✅ Parse destination URL with `net/url`
- ✅ Validate against strict allowlist
- ✅ Never trust user input directly
- ✅ Log rejected redirect attempts
- ✅ Show warning if redirecting to different domain

## References

- [OWASP: Unvalidated Redirects and Forwards](https://cheatsheetseries.owasp.org/cheatsheets/Unvalidated_Redirects_and_Forwards_Cheat_Sheet.html)
- [CWE-601: URL Redirection to Untrusted Site](https://cwe.mitre.org/data/definitions/601.html)
- [Go net/url Documentation](https://pkg.go.dev/net/url)
- [RFC 3986: URI Generic Syntax](https://datatracker.ietf.org/doc/html/rfc3986)

## Changelog

### 2025-01-16: Initial Fix

- Replaced naive string prefix check with proper URL parsing
- Added `net/url` and `strconv` imports
- Implemented comprehensive validation:
  - Scheme validation (http only)
  - Hostname exact match (localhost or 127.0.0.1)
  - Username/password rejection
  - Port range validation (1-65535)
- Added detailed comments explaining each check
- Created comprehensive documentation with attack scenarios
