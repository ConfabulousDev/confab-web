# TODO: Future Enhancements

## Device Code Flow for CLI Authentication

Currently, `confab login` uses a localhost callback server which requires the browser to be on the same machine as the CLI. This works great for local development but not for remote/headless servers.

### Implementation Plan

Add device code flow (similar to GitHub CLI `gh auth login`):

1. **CLI initiates device flow**
   ```
   POST /api/v1/auth/device/code
   Response: { "device_code": "xxx", "user_code": "ABCD-1234", "verification_uri": "https://backend.com/device" }
   ```

2. **CLI shows user code**
   ```
   Visit: https://backend.com/device
   Enter code: ABCD-1234
   ```

3. **CLI polls for authorization**
   ```
   GET /api/v1/auth/device/poll?device_code=xxx
   Response (pending): { "status": "pending" }
   Response (approved): { "status": "approved", "api_key": "cfb_xxx" }
   ```

4. **User visits web page**
   - User already logged in via GitHub OAuth
   - Enter device code: ABCD-1234
   - Approve device
   - Backend marks device_code as approved

5. **CLI receives API key**
   - Polling detects approval
   - CLI saves API key to config
   - Cloud sync enabled

### Database Schema

```sql
CREATE TABLE device_codes (
    device_code TEXT PRIMARY KEY,
    user_code TEXT NOT NULL UNIQUE,
    user_id BIGINT REFERENCES users(id),
    api_key_hash TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### Security Considerations

- User codes should be short (8 chars) and human-readable (uppercase, no ambiguous chars)
- Device codes should be long and random (32+ chars)
- Both should expire after 15 minutes
- Polling should have rate limiting (max 1 req/5 seconds)
- User must be authenticated (web session) to approve device code

### References

- [GitHub Device Flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow)
- [OAuth 2.0 Device Authorization Grant (RFC 8628)](https://datatracker.ietf.org/doc/html/rfc8628)

## API Access for Shared Sessions

Currently, shared sessions are only accessible via the web frontend. Add support for programmatic access via API.

### Use Cases

1. **CLI viewing of shared sessions** - Users should be able to view shared sessions from the CLI
2. **Automated analysis** - Scripts/tools that analyze shared sessions
3. **Integrations** - Third-party tools that need to access shared sessions

### Implementation Plan

**Current State:**
- Canonical session endpoint: `GET /api/v1/sessions/{id}`
- Endpoint returns JSON session data
- Access determined by share type (public, system, recipient) and user auth
- Public shares: no auth required
- Private shares: requires web session cookie to verify email

**Required Changes:**

1. **Support Bearer token authentication for private shares**
   - Allow `Authorization: Bearer cfb_xxx` header as alternative to cookie
   - Look up API key to get user email
   - Check if email is in invited list
   - This allows CLI/API clients to access private shares they're invited to

2. **Add share token to API key authentication**
   - Currently, API keys only authenticate the owner
   - Add optional `X-Share-Token` header for accessing shared sessions
   - Example:
     ```
     GET /api/v1/sessions/:sessionId
     Authorization: Bearer cfb_xxx
     X-Share-Token: abc123def456
     ```
   - If share token provided, check share permissions instead of ownership

3. **CLI commands for shared sessions**
   ```bash
   # View public shared session (no login needed)
   confab view --share-url https://confab.dev/sessions/xyz/shared/abc123

   # View private shared session (requires login + invitation)
   confab view --share-url https://confab.dev/sessions/xyz/shared/abc123

   # Extract share token from URL and use API key for auth
   ```

### Backward Compatibility

- Web frontend continues to use cookie-based auth (no changes needed)
- Public shares work without any auth (no changes needed)
- Private shares gain additional auth method (API key) without breaking cookies
