# Email Whitelist

## Overview

Controls which email addresses can sign up and login to Confab via GitHub OAuth.

## Configuration

### Environment Variable

```bash
ALLOWED_EMAILS=alice@example.com,bob@example.com,charlie@company.org
```

- **Format**: Comma-separated list of email addresses
- **Case-insensitive**: `Alice@Example.COM` matches `alice@example.com`
- **Whitespace-tolerant**: Spaces around commas are ignored

### Behavior

**If `ALLOWED_EMAILS` is NOT set:**
- ✅ Open registration - anyone with a GitHub account can sign up

**If `ALLOWED_EMAILS` IS set:**
- ✅ Only whitelisted emails can sign up/login
- ❌ All other emails get: "Access denied: Your email is not authorized to use this application"

## Use Cases

### 1. Private App (Team Only)
```bash
ALLOWED_EMAILS=alice@company.com,bob@company.com,charlie@company.com
```
Only your team members can access.

### 2. Beta Testing
```bash
ALLOWED_EMAILS=tester1@gmail.com,tester2@yahoo.com,friend@example.org
```
Limit access to specific beta testers.

### 3. Single User (Personal)
```bash
ALLOWED_EMAILS=me@example.com
```
Only you can access.

### 4. Public App
```bash
# Don't set ALLOWED_EMAILS (or set it to empty)
```
Anyone can sign up.

## Implementation

### When Checked

Email is validated **during GitHub OAuth callback**, after GitHub returns user info but before creating/updating user in database.

**Location:** `backend/internal/auth/oauth.go:132-137`

```go
// Check email whitelist (if configured)
if !isEmailAllowed(user.Email) {
    log.Printf("Email not in whitelist: %s", user.Email)
    http.Error(w, "Access denied: Your email is not authorized...", 403)
    return
}
```

### Both Signup AND Login

The check applies to:
- ✅ **New signups** (first time logging in)
- ✅ **Existing logins** (returning users)

This means if you **remove** someone from the whitelist, they can't login anymore (immediate revocation).

## Examples

### Example 1: Restrict to Company Domain

```bash
# This won't work - no wildcard support
ALLOWED_EMAILS=*@company.com  # ❌ Won't match anyone

# You need to list each email explicitly
ALLOWED_EMAILS=alice@company.com,bob@company.com,charlie@company.com  # ✅ Works
```

**Note:** Wildcard domains are not supported (yet). Each email must be listed explicitly.

### Example 2: Testing Whitelist

```bash
# Run with whitelist
ALLOWED_EMAILS=allowed@example.com docker run ...

# Try to login with:
# - allowed@example.com  → ✅ Works
# - notallowed@test.com  → ❌ 403 Forbidden
```

### Example 3: Revoking Access

```bash
# Before: Alice has access
ALLOWED_EMAILS=alice@example.com,bob@example.com

# After: Alice removed
ALLOWED_EMAILS=bob@example.com

# Result:
# - Alice can't login anymore (immediately)
# - Bob can still login
```

## Security Notes

### ✅ What It Protects

- **Unauthorized signups**: Random people can't create accounts
- **Controlled access**: You decide who gets in
- **Immediate revocation**: Remove email = instant logout

### ❌ What It Doesn't Protect

- **GitHub account takeover**: If attacker steals GitHub account with whitelisted email, they get access
- **Email spoofing**: Relies on GitHub's email verification (GitHub is trusted)
- **Existing sessions**: Removing from whitelist doesn't kill active sessions (they expire in 7 days)

### Best Practice

**For maximum security, combine with:**

1. **GitHub 2FA**: Require team to enable 2FA on GitHub
2. **Short session duration**: Reduce from 7 days to 1 day if needed
3. **Audit logging**: Monitor who's accessing the system
4. **IP restrictions**: Add IP allowlist for extra paranoia

## Testing

### Test 1: Open Registration (Default)

```bash
# No ALLOWED_EMAILS set
docker run ... confab:test

# Login with any GitHub account → ✅ Should work
```

### Test 2: Whitelist One Email

```bash
# Set whitelist to your GitHub email
ALLOWED_EMAILS=your-github-email@example.com docker run ... confab:test

# Login with:
# - your-github-email@example.com → ✅ Should work
# - different-email@test.com       → ❌ Should get 403
```

### Test 3: Case Insensitive

```bash
ALLOWED_EMAILS=Alice@Example.COM docker run ... confab:test

# Login with GitHub account that has email: alice@example.com
# → ✅ Should work (case doesn't matter)
```

### Test 4: Multiple Emails

```bash
ALLOWED_EMAILS=alice@example.com,bob@test.org,charlie@company.io \
  docker run ... confab:test

# All three should be able to login
```

## Error Messages

### User Sees (403 Forbidden)

```
Access denied: Your email is not authorized to use this application. Please contact the administrator.
```

### Server Logs

```
Email not in whitelist: unauthorized@example.com
```

## Future Enhancements

### Wildcard Domains

```bash
# Not yet supported, but could add:
ALLOWED_DOMAINS=@company.com,@partner.org
```

Would allow any email from those domains.

### Database-Based Whitelist

Instead of environment variable, store in database:
- Admin UI to manage allowed emails
- Per-team allowlists
- Temporary access (expires after X days)

### GitHub Org Restriction

```bash
ALLOWED_GITHUB_ORGS=my-company,my-team
```

Only members of specific GitHub organizations can sign up.

## Deployment Examples

### Fly.io

```bash
fly secrets set ALLOWED_EMAILS="alice@company.com,bob@company.com"
```

### Docker Compose

```yaml
services:
  confab:
    image: confab:latest
    environment:
      - ALLOWED_EMAILS=alice@example.com,bob@example.com
```

### Kubernetes

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: confab-secrets
stringData:
  allowed-emails: "alice@example.com,bob@example.com"
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: confab
        env:
        - name: ALLOWED_EMAILS
          valueFrom:
            secretKeyRef:
              name: confab-secrets
              key: allowed-emails
```

## FAQ

**Q: Can I use wildcards like `*@company.com`?**
A: Not yet. Each email must be listed explicitly.

**Q: What happens to existing sessions if I remove someone?**
A: They can't login again, but existing sessions remain valid until they expire (7 days).

**Q: Is it case-sensitive?**
A: No. `Alice@Example.COM` matches `alice@example.com`.

**Q: Can I use multiple GitHub accounts?**
A: Yes, as long as each account's email is in the whitelist.

**Q: What if GitHub account has multiple emails?**
A: GitHub OAuth returns the **primary, verified** email. That's what gets checked.

**Q: Does this work with the CLI?**
A: Yes! CLI users authenticate via web OAuth, so the whitelist applies to them too.

**Q: Can I change the whitelist without restarting?**
A: No, it reads from environment variable at startup. Need to restart container.

## Related

- `backend/.env.example` - Example configuration
- `backend/internal/auth/oauth.go` - Implementation
- `SECURITY_AUDIT.md` - Overall security documentation
