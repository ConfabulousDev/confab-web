# Confab Backend API Reference

This document describes the backend API surface for the Confab web application. It is intended for Claude Code working on the frontend or CLI.

## Authentication

The API uses two authentication methods:

### 1. API Key Authentication (CLI)
Used by CLI tools. All CLI requests include these headers:
```
Authorization: Bearer cfb_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
User-Agent: confab/1.2.3 (darwin; arm64)
```

The `User-Agent` header includes CLI version, OS, and architecture.

### 2. Session Cookie Authentication (Web)
Used by the web frontend. Session cookie (`confab_session`) is set after OAuth login. CSRF token required for mutating requests.

## Base URL

All API endpoints are prefixed with `/api/v1` unless otherwise noted.

---

## CLI Endpoints (API Key Auth)

### Validate API Key
```
GET /api/v1/auth/validate
Authorization: Bearer <api_key>
```

**Response:**
```json
{
  "valid": true,
  "user_id": 123,
  "email": "user@example.com",
  "name": "User Name"
}
```

---

### Sync Init
Initialize or resume a sync session.

```
POST /api/v1/sync/init
Authorization: Bearer <api_key>
Content-Type: application/json
```

**Request (recommended):**
```json
{
  "external_id": "session-uuid",
  "transcript_path": "/path/to/transcript.jsonl",
  "metadata": {
    "cwd": "/working/directory",
    "git_info": { ... },
    "hostname": "macbook.local",
    "username": "jackie"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `external_id` | string | Yes | Unique session identifier (UUID from Claude Code) |
| `transcript_path` | string | Yes | Path to transcript file on user's machine |
| `metadata` | object | No | Session metadata (see below) |
| `metadata.cwd` | string | No | Current working directory |
| `metadata.git_info` | object | No | Git repository metadata (branch, remote, etc.) |
| `metadata.hostname` | string | No | Client machine hostname |
| `metadata.username` | string | No | OS username of the client |

**Deprecated fields (backward compatibility):**

The following top-level fields are deprecated but still supported for backward compatibility with older CLI versions. When both top-level and `metadata` fields are provided, `metadata` takes precedence.

| Field | Type | Description |
|-------|------|-------------|
| `cwd` | string | *Deprecated:* Use `metadata.cwd` instead |
| `git_info` | object | *Deprecated:* Use `metadata.git_info` instead |

**Response:**
```json
{
  "session_id": "uuid",
  "files": {
    "transcript.jsonl": { "last_synced_line": 150 },
    "agent.jsonl": { "last_synced_line": 42 }
  }
}
```

---

### Sync Chunk
Upload a chunk of lines for a file.

```
POST /api/v1/sync/chunk
Authorization: Bearer <api_key>
Content-Type: application/json
Content-Encoding: zstd  (optional, for compressed payloads)
```

**Request:**
```json
{
  "session_id": "uuid",
  "file_name": "transcript.jsonl",
  "file_type": "transcript",
  "first_line": 151,
  "lines": ["line 151 content", "line 152 content", ...],
  "metadata": {
    "git_info": { ... },
    "summary": "Session summary text",
    "first_user_message": "First user message"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | Yes | UUID from sync/init response |
| `file_name` | string | Yes | Name of the file being synced |
| `file_type` | string | Yes | `"transcript"` or `"agent"` |
| `first_line` | int | Yes | Line number of first line (1-indexed, must be contiguous) |
| `lines` | string[] | Yes | Array of line contents |
| `metadata` | object | No | Optional metadata (only processed for transcript files) |
| `metadata.git_info` | object | No | Git repository metadata |
| `metadata.summary` | string | No | Session summary (nil=don't update, ""=clear) |
| `metadata.first_user_message` | string | No | First user message (nil=don't update, ""=clear) |

**Response:**
```json
{
  "last_synced_line": 175
}
```

**Notes:**
- Chunks must be contiguous (no gaps or overlaps with previous chunks)
- Max 30,000 chunks per file
- Request body supports zstd compression

---

### Sync Event
Record a session lifecycle event.

```
POST /api/v1/sync/event
Authorization: Bearer <api_key>
Content-Type: application/json
```

**Request:**
```json
{
  "session_id": "uuid",
  "event_type": "session_end",
  "timestamp": "2024-01-15T10:30:00Z",
  "payload": { ... }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | Yes | UUID from sync/init response |
| `event_type` | string | Yes | Currently only `"session_end"` |
| `timestamp` | string | Yes | ISO 8601 timestamp |
| `payload` | object | No | Event-specific payload |

**Response:**
```json
{
  "success": true
}
```

---

### Update Session Summary

```
PATCH /api/v1/sessions/{external_id}/summary
Authorization: Bearer <api_key>
Content-Type: application/json
```

**Request:**
```json
{
  "summary": "New summary text"
}
```

**Response:**
```json
{
  "status": "ok"
}
```

---

## Web Endpoints (Session Auth + CSRF)

All web endpoints require:
1. Valid session cookie (`confab_session`)
2. CSRF token in `X-CSRF-Token` header (for POST/PUT/DELETE)

### Get CSRF Token

```
GET /api/v1/csrf-token
```

**Response:**
```json
{
  "csrf_token": "token-value"
}
```
Also sets the CSRF cookie.

---

### Get Current User

```
GET /api/v1/me
```

**Response:**
```json
{
  "id": 123,
  "email": "user@example.com",
  "name": "User Name",
  "avatar_url": "https://...",
  "status": "active",
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

### API Key Management

#### Create API Key
```
POST /api/v1/keys
X-CSRF-Token: <token>
Content-Type: application/json
```

**Request:**
```json
{
  "name": "My CLI Key"
}
```

**Response:**
```json
{
  "id": 1,
  "key": "cfb_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "name": "My CLI Key",
  "created_at": "2024-01-15 10:30:00"
}
```
Note: The `key` is only returned once at creation time.

#### List API Keys
```
GET /api/v1/keys
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "My CLI Key",
    "created_at": "2024-01-15T10:30:00Z",
    "last_used_at": "2024-01-16T14:20:00Z"
  }
]
```

#### Delete API Key
```
DELETE /api/v1/keys/{id}
X-CSRF-Token: <token>
```

**Response:** `204 No Content`

---

### Session Management

#### List Sessions
```
GET /api/v1/sessions?view=owned
GET /api/v1/sessions?view=shared
```

| Query Param | Type | Default | Description |
|-------------|------|---------|-------------|
| `view` | string | `owned` | Which sessions to list: `owned` or `shared` |

Supports conditional requests for efficient polling:
- **Request header:** `If-None-Match: "<etag>"`
- **Response header:** `ETag: "<timestamp>"`
- Returns `304 Not Modified` if content unchanged

**Response:**
```json
[
  {
    "id": "uuid",
    "external_id": "session-uuid",
    "custom_title": "My Custom Title",
    "summary": "Session summary",
    "first_user_message": "First message",
    "first_seen": "2024-01-15T10:00:00Z",
    "last_sync_time": "2024-01-15T11:30:00Z",
    "session_type": "Claude Code",
    "file_count": 2,
    "total_lines": 1500,
    "git_repo": "org/repo",
    "git_repo_url": "https://github.com/org/repo",
    "git_branch": "main",
    "github_prs": ["123", "456"],
    "github_commits": ["abc1234", "def5678"],
    "is_owner": true,
    "access_type": "owner",
    "shared_by_email": null,
    "hostname": "macbook.local",
    "username": "developer"
  }
]
```

**Notes:**
- `custom_title` is null/omitted when not set. Frontend displays: `custom_title || summary || first_user_message || fallback`.
- `hostname` and `username` are **owner-only fields** for privacy. They are returned as `null` for shared sessions (`is_owner: false`).
- `github_prs` contains linked PR refs (ordered by creation time ascending).
- `github_commits` contains linked commit SHAs (ordered by creation time descending, so latest is first).

#### Get Session Detail (Canonical Access)
```
GET /api/v1/sessions/{id}
```

This endpoint provides unified access to session details. It supports:
- **Owner access**: Authenticated session owner (full details including hostname/username)
- **Public share**: Anyone (no auth required) if session has a public share
- **System share**: Any authenticated user if session has a system share
- **Recipient share**: Authenticated user who is a private share recipient

Authentication is optional - the endpoint extracts user from session cookie if present.

**Response:**
```json
{
  "id": "uuid",
  "external_id": "session-uuid",
  "custom_title": "My Custom Title",
  "summary": "Session summary",
  "first_user_message": "First message",
  "first_seen": "2024-01-15T10:00:00Z",
  "last_sync_at": "2024-01-15T11:30:00Z",
  "cwd": "/project/path",
  "transcript_path": "/home/user/.claude/projects/.../session.jsonl",
  "git_info": {
    "repo_url": "https://github.com/org/repo",
    "branch": "main",
    "commit_sha": "abc123",
    "commit_message": "Initial commit",
    "author": "developer",
    "is_dirty": false
  },
  "hostname": "macbook.local",
  "username": "developer",
  "is_owner": true,
  "files": [
    {
      "file_name": "transcript.jsonl",
      "file_type": "transcript",
      "last_synced_line": 100,
      "updated_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `is_owner` | bool | `true` if the viewer is the session owner |
| `hostname` | string\|null | Machine hostname (owner-only, null for shared access) |
| `username` | string\|null | OS username (owner-only, null for shared access) |

**Errors:**
- `403` - Session owner is deactivated
- `404` - Session not found or no access

#### Read Session Sync File
```
GET /api/v1/sessions/{id}/sync/file?file_name=<name>&line_offset=<n>
```

Read the contents of a synced file, or incrementally fetch new lines. Uses the same access logic as Get Session Detail.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| file_name | string | Yes | Name of the file (e.g., "transcript.jsonl") |
| line_offset | integer | No | Return only lines after this line number (default: 0 = all lines) |

**Response:** `text/plain` - concatenated file contents (lines after line_offset if specified)

**Notes:**
- When `line_offset=0` or omitted, returns all lines (backward compatible)
- When `line_offset >= last_synced_line`, returns empty response without S3 access (efficient polling)
- Useful for incremental fetching: poll with line_offset = number of lines already loaded
- Optimizations: DB short-circuit for no new lines, chunk filtering before download

#### Update Session Title
```
PATCH /api/v1/sessions/{id}/title
X-CSRF-Token: <token>
Content-Type: application/json
```

**Request:**
```json
{
  "custom_title": "My Custom Title"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `custom_title` | string\|null | Yes | Custom title (max 255 chars). Pass `null` to clear and revert to auto-derived title. |

**Response:** Returns the updated session detail (same as Get Session Detail).

**Errors:**
- `400` - Title exceeds 255 characters
- `403` - Not the session owner
- `404` - Session not found

#### Delete Session
```
DELETE /api/v1/sessions/{id}
X-CSRF-Token: <token>
```

**Response:** `204 No Content`

Deletes session, all files, and all shares.

---

### Session Sharing

#### Create Share
```
POST /api/v1/sessions/{id}/share
X-CSRF-Token: <token>
Content-Type: application/json
```

**Request:**
```json
{
  "is_public": true,
  "recipients": [],
  "expires_in_days": 30,
  "skip_notifications": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `is_public` | bool | Yes | `true` for public links, `false` for private (email-only) |
| `recipients` | string[] | For private | Email addresses to invite (max 50) |
| `expires_in_days` | int | No | Days until expiration (null = never) |
| `skip_notifications` | bool | No | Skip sending invitation emails (default: false) |

**Response:**
```json
{
  "share_id": 123,
  "share_url": "https://confab.dev/sessions/{id}",
  "is_public": true,
  "recipients": [],
  "expires_at": "2024-02-15T10:00:00Z",
  "emails_sent": true,
  "email_failures": []
}
```

**Note:** Share URLs use the canonical session URL format (`/sessions/{id}`). For private shares, invitation emails include the recipient's email as a query parameter: `https://confab.dev/sessions/{id}?email={recipient_email}`. This allows the login flow to guide the recipient to sign in with the correct email address.

#### List Shares for Session
```
GET /api/v1/sessions/{id}/shares
```

#### List All User's Shares
```
GET /api/v1/shares
```

#### Revoke Share
```
DELETE /api/v1/shares/{shareId}
X-CSRF-Token: <token>
```

**Response:** `204 No Content`

**Note:** The `shareId` is the numeric ID returned from Create Share.

---

### GitHub Links

Link sessions to GitHub artifacts (commits and PRs) for bidirectional navigation.

#### Create GitHub Link
```
POST /api/v1/sessions/{id}/github-links
Authorization: Bearer <api_key>  (CLI)
   or
X-CSRF-Token: <token>  (Web)
Content-Type: application/json
```

**Request:**
```json
{
  "url": "https://github.com/owner/repo/pull/123",
  "title": "Optional PR/commit title",
  "source": "cli_hook"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | GitHub PR or commit URL |
| `title` | string | No | Title/description of the PR or commit |
| `source` | string | Yes | `"cli_hook"` (from CLI hook) or `"manual"` (user-added) |

**Response:** `201 Created`
```json
{
  "id": 1,
  "session_id": "uuid",
  "link_type": "pull_request",
  "url": "https://github.com/owner/repo/pull/123",
  "owner": "owner",
  "repo": "repo",
  "ref": "123",
  "title": "Add new feature",
  "source": "cli_hook",
  "created_at": "2024-01-15T10:30:00Z"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `link_type` | string | `"commit"` or `"pull_request"` (auto-detected from URL) |
| `owner` | string | GitHub repository owner (parsed from URL) |
| `repo` | string | GitHub repository name (parsed from URL) |
| `ref` | string | PR number or commit SHA (parsed from URL) |

**Errors:**
- `400` - Invalid GitHub URL (must be PR or commit URL)
- `404` - Session not found or not owner
- `409` - Link already exists for this session

#### List GitHub Links
```
GET /api/v1/sessions/{id}/github-links
```

Works for any user with session access (owner, shared, public).

**Response:**
```json
{
  "links": [
    {
      "id": 1,
      "session_id": "uuid",
      "link_type": "pull_request",
      "url": "https://github.com/owner/repo/pull/123",
      "owner": "owner",
      "repo": "repo",
      "ref": "123",
      "title": "Add new feature",
      "source": "cli_hook",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ]
}
```

#### Delete GitHub Link
```
DELETE /api/v1/sessions/{id}/github-links/{linkId}
X-CSRF-Token: <token>
```

Requires session ownership (web session auth only, no API key).

**Response:** `204 No Content`

**Errors:**
- `404` - Link not found or not session owner

---

### Session Analytics

#### Get Session Analytics
```
GET /api/v1/sessions/{id}/analytics?as_of_line=<n>
```

Returns computed analytics for a session. Uses the same canonical access model as Get Session Detail.

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| as_of_line | integer | No | Client's current line count for conditional requests |

**Conditional Request Behavior:**
- If `as_of_line` >= current transcript line count: returns `304 Not Modified`
- Useful for polling: pass the `computed_lines` from a previous response to avoid redundant computation

**Response:**
```json
{
  "computed_at": "2024-01-15T10:30:00Z",
  "computed_lines": 150,
  "tokens": {
    "input": 125000,
    "output": 48000,
    "cache_creation": 5000,
    "cache_read": 12000
  },
  "cost": {
    "estimated_usd": "1.25"
  },
  "compaction": {
    "auto": 3,
    "manual": 1,
    "avg_time_ms": 5000
  },
  "cards": {
    "tokens": {
      "input": 125000,
      "output": 48000,
      "cache_creation": 5000,
      "cache_read": 12000
    },
    "cost": {
      "estimated_usd": "1.25"
    },
    "compaction": {
      "auto": 3,
      "manual": 1,
      "avg_time_ms": 5000
    },
    "session": {
      "duration_ms": 3600000,
      "models_used": ["claude-sonnet-4-20241022", "claude-opus-4"]
    },
    "tools": {
      "total_calls": 42,
      "tool_breakdown": {"Read": 15, "Write": 10, "Bash": 12, "Grep": 5},
      "error_count": 2
    },
    "code_activity": {
      "files_read": 42,
      "files_modified": 12,
      "lines_added": 156,
      "lines_removed": 23,
      "search_count": 18,
      "language_breakdown": {"go": 28, "ts": 18, "css": 5}
    },
    "conversation": {
      "user_turns": 15,
      "assistant_turns": 14,
      "avg_assistant_turn_ms": 45000,
      "avg_user_thinking_ms": 120000
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `computed_at` | string | ISO timestamp when analytics were computed |
| `computed_lines` | int | Line count through which analytics are computed |
| `tokens.*` | object | *Deprecated:* Use `cards.tokens` instead |
| `cost.*` | object | *Deprecated:* Use `cards.cost` instead |
| `compaction.*` | object | *Deprecated:* Use `cards.compaction` instead |
| `cards` | object | Card-based analytics data (keyed by card name) |
| `cards.tokens.input` | int | Total input tokens sent to model |
| `cards.tokens.output` | int | Total output tokens generated |
| `cards.tokens.cache_creation` | int | Tokens written to cache |
| `cards.tokens.cache_read` | int | Tokens served from cache |
| `cards.cost.estimated_usd` | string | Estimated API cost (assumes 5-min prompt caching) |
| `cards.compaction.auto` | int | Auto-triggered compaction count |
| `cards.compaction.manual` | int | Manual compaction count |
| `cards.compaction.avg_time_ms` | int\|null | Avg auto-compaction time in ms (null if none) |
| `cards.session.duration_ms` | int\|null | Session duration in ms (null if single message) |
| `cards.session.models_used` | string[] | Unique model IDs used in the session |
| `cards.tools.total_calls` | int | Total number of tool invocations |
| `cards.tools.tool_breakdown` | object | Map of tool name to call count |
| `cards.tools.error_count` | int | Number of tool calls that returned errors |
| `cards.code_activity.files_read` | int | Number of unique files read |
| `cards.code_activity.files_modified` | int | Number of unique files modified |
| `cards.code_activity.lines_added` | int | Total lines added across all edits |
| `cards.code_activity.lines_removed` | int | Total lines removed across all edits |
| `cards.code_activity.search_count` | int | Number of search operations (Grep/Glob) |
| `cards.code_activity.language_breakdown` | object | Map of file extension to count |
| `cards.conversation.user_turns` | int | Number of user prompts (human messages) |
| `cards.conversation.assistant_turns` | int | Number of assistant text responses |
| `cards.conversation.avg_assistant_turn_ms` | int\|null | Average time per assistant turn including tool calls (null if no data) |
| `cards.conversation.avg_user_thinking_ms` | int\|null | Average time between assistant response and next user prompt (null if no data) |

**Notes:**
- Analytics are cached in the database and recomputed when new data is synced
- Returns empty analytics if session has no transcript file
- `304 Not Modified` has no body

---

## OAuth Endpoints (No prefix)

These endpoints handle OAuth authentication flow:

| Endpoint | Description |
|----------|-------------|
| `GET /auth/login` | Login provider selector page |
| `GET /auth/github/login` | Initiate GitHub OAuth |
| `GET /auth/github/callback` | GitHub OAuth callback |
| `GET /auth/google/login` | Initiate Google OAuth |
| `GET /auth/google/callback` | Google OAuth callback |
| `GET /auth/logout` | Logout (clears session) |

### OAuth Login Parameters

The login endpoints accept optional query parameters to support share link flows:

| Parameter | Description |
|-----------|-------------|
| `redirect` | URL path to redirect to after successful login |
| `email` | Expected email address (for share link login hints) |

When `email` is provided:
- The login selector page shows "Sign in with **{email}** to view this shared session"
- GitHub OAuth URL includes `&login={email}` (pre-fills username field)
- Google OAuth URL includes `&login_hint={email}` (pre-fills email field)
- After OAuth callback, if the logged-in email doesn't match, redirect includes `?email_mismatch=1&expected={email}&actual={actual_email}`

### Device Code Flow (CLI on headless machines)

| Endpoint | Description |
|----------|-------------|
| `POST /auth/device/code` | Request device code |
| `POST /auth/device/token` | Poll for access token |
| `GET /auth/device` | User verification page |
| `POST /auth/device/verify` | Submit user code |

---

## Admin Endpoints (Super Admin Only)

Admin functionality is accessed via HTML pages at an obfuscated path. Requires web session authentication and super admin privileges (configured via `SUPER_ADMIN_EMAILS` environment variable).

Features:
- **User Management**: View, activate, deactivate, and delete users
- **System Shares**: Create shares accessible to all authenticated users

All admin actions are audit logged with admin identity and action details.

---

## Utility Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check (`{"status": "ok"}`) |
| `GET /help/delete-account` | Account deletion help page |

---

## Error Responses

All errors return JSON:
```json
{
  "error": "Error message here"
}
```

Common HTTP status codes:
- `400` - Bad request (validation error)
- `401` - Unauthorized (missing/invalid auth)
- `403` - Forbidden (CSRF failure, insufficient permissions)
- `404` - Not found
- `409` - Conflict (e.g., API key limit reached)
- `410` - Gone (e.g., share expired)
- `429` - Too many requests (rate limited)
- `500` - Internal server error

---

## Rate Limits

| Endpoint Group | Limit | Burst |
|----------------|-------|-------|
| Global | 100 req/sec | 200 |
| Auth endpoints | 1 req/sec | 30 |
| Upload endpoints | 2.78 req/sec (10k/hour) | 2000 |
| Validation | 0.5 req/sec | 10 |

Upload rate limiting is per-user (not per-IP) to support backfill scenarios.

---

## Request Body Size Limits

| Size | Limit | Used For |
|------|-------|----------|
| XS | 2 KB | GET/DELETE requests |
| S | 16 KB | Auth tokens, simple metadata |
| M | 128 KB | API keys, shares, session updates |
| L | 2 MB | Batch operations |
| XL | 16 MB | Sync chunk uploads |
