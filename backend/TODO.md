# TODO: Future Enhancements

## API Access for Shared Sessions

Currently, shared sessions are only accessible via the web frontend. Add support for programmatic access via API.

### Use Cases

1. **CLI viewing of shared sessions** — Users should be able to view shared sessions from the CLI
2. **Automated analysis** — Scripts/tools that analyze shared sessions
3. **Integrations** — Third-party tools that need to access shared sessions

### Implementation Plan

**Current State:**
- Canonical session endpoint: `GET /api/v1/sessions/{id}`
- Endpoint returns JSON session data
- Access determined by share type (public, system, recipient) and user auth
- Public shares: no auth required
- Private shares: requires web session cookie to verify email

**Required Changes:**

1. **Support Bearer token authentication for private shares**
   - Allow `Authorization: Bearer <api-key>` as alternative to cookie
   - Look up API key to get user email
   - Check if email is in invited list
   - This allows CLI/API clients to access private shares they're invited to

2. **Add share token to API key authentication**
   - Currently, API keys only authenticate the owner
   - Add optional `X-Share-Token` header for accessing shared sessions
   - If share token provided, check share permissions instead of ownership

### Backward Compatibility

- Web frontend continues to use cookie-based auth (no changes needed)
- Public shares work without any auth (no changes needed)
- Private shares gain additional auth method (API key) without breaking cookies
