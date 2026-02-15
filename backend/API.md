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
GET /api/v1/sessions?repo=<repos>&branch=<branches>&owner=<owners>&pr=<prs>&q=<search>&page=<n>
```

Returns paginated sessions visible to the user (owned + shared) with server-side filtering and faceted filter counts.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `repo` | string | No | all | Comma-separated repo names (e.g., `org/repo1,org/repo2`) |
| `branch` | string | No | all | Comma-separated branch names |
| `owner` | string | No | all | Comma-separated owner emails |
| `pr` | string | No | all | Comma-separated PR numbers |
| `q` | string | No | none | Search query (matches title, summary, first message, commit SHA) |
| `page` | int | No | 1 | Page number (1-indexed, must be >= 1) |

**Response:**
```json
{
  "sessions": [
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
      "shared_by_email": null
    }
  ],
  "total": 142,
  "page": 1,
  "page_size": 50,
  "filter_options": {
    "repos": [
      { "value": "org/repo1", "count": 80 },
      { "value": "org/repo2", "count": 62 }
    ],
    "branches": [
      { "value": "main", "count": 100 },
      { "value": "feature-x", "count": 42 }
    ],
    "owners": [
      { "value": "alice@example.com", "count": 90 },
      { "value": "bob@example.com", "count": 52 }
    ],
    "total": 142
  }
}
```

**Notes:**
- **Breaking change**: Response is now an object (was previously a bare array).
- `custom_title` is null/omitted when not set. Frontend displays: `custom_title || summary || first_user_message || fallback`.
- `github_prs` contains linked PR refs (ordered by creation time ascending).
- `github_commits` contains linked commit SHAs (ordered by creation time descending, so latest is first).
- **Page size** is fixed at 50 sessions per page.
- **Visibility filter**: Only sessions with `total_lines > 0` and at least one of `summary` or `first_user_message` are included.
- **Faceted counts** use an exclude-own-dimension pattern: when filtering by `repo=X`, `filter_options.repos` shows counts for ALL repos (not just X), allowing the UI to display available options. Other dimensions (branches, owners) reflect the repo=X filter.
- **Multiple values** within a filter dimension use OR logic (e.g., `repo=a,b` matches either). Across dimensions, filters use AND logic.

**Errors:**
- `400` - Invalid page parameter (must be a positive integer)

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
  "shared_by_email": null,
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
| `shared_by_email` | string\|null | Email of session owner (non-owner access only, null for owners) |

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

**Errors:**
- `403` - Share creation is disabled (`DISABLE_SHARE_CREATION=true`): `{"error": "Share creation is disabled by the administrator"}`

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

### Trends (Aggregated Analytics)

#### Get Trends
```
GET /api/v1/trends?start_date=<date>&end_date=<date>&repos=<repos>&include_no_repo=<bool>
```

Returns aggregated analytics across multiple sessions for the authenticated user.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| start_date | string | No | 7 days ago | Start of date range (YYYY-MM-DD) |
| end_date | string | No | today | End of date range (YYYY-MM-DD) |
| repos | string | No | all | Comma-separated repo names to filter |
| include_no_repo | boolean | No | true | Include sessions without a git repo |

**Constraints:**
- Maximum date range: 90 days
- Date format: YYYY-MM-DD

**Response:**
```json
{
  "computed_at": "2024-01-15T10:30:00Z",
  "date_range": {
    "start_date": "2024-01-08",
    "end_date": "2024-01-15"
  },
  "session_count": 42,
  "repos_included": ["org/repo1"],
  "include_no_repo": true,
  "cards": {
    "overview": {
      "session_count": 42,
      "total_duration_ms": 86400000,
      "avg_duration_ms": 2057142,
      "days_covered": 7
    },
    "tokens": {
      "total_input_tokens": 5000000,
      "total_output_tokens": 2000000,
      "total_cache_creation_tokens": 100000,
      "total_cache_read_tokens": 500000,
      "total_cost_usd": "125.50",
      "daily_costs": [
        {"date": "2024-01-08", "cost_usd": "15.20"},
        {"date": "2024-01-09", "cost_usd": "18.50"}
      ]
    },
    "activity": {
      "total_files_read": 500,
      "total_files_modified": 150,
      "total_lines_added": 5000,
      "total_lines_removed": 2000,
      "daily_session_counts": [
        {"date": "2024-01-08", "session_count": 5},
        {"date": "2024-01-09", "session_count": 8}
      ]
    },
    "tools": {
      "total_calls": 2500,
      "total_errors": 50,
      "tool_stats": {
        "Read": {"success": 800, "errors": 5},
        "Write": {"success": 400, "errors": 10},
        "Bash": {"success": 600, "errors": 30}
      }
    },
    "agents_and_skills": {
      "total_agent_invocations": 45,
      "total_skill_invocations": 20,
      "agent_stats": {
        "Explore": {"success": 20, "errors": 1},
        "Plan": {"success": 12, "errors": 0}
      },
      "skill_stats": {
        "commit": {"success": 10, "errors": 1},
        "review-pr": {"success": 5, "errors": 0}
      }
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `computed_at` | string | ISO timestamp when trends were computed |
| `date_range.start_date` | string | Start date (inclusive) |
| `date_range.end_date` | string | End date (inclusive) |
| `session_count` | int | Total sessions in the date range |
| `repos_included` | string[] | Repos that were included in the filter |
| `include_no_repo` | bool | Whether sessions without repos were included |
| `cards.overview.session_count` | int | Total session count |
| `cards.overview.total_duration_ms` | int | Sum of all session durations |
| `cards.overview.avg_duration_ms` | int\|null | Average session duration |
| `cards.overview.days_covered` | int | Number of unique days with sessions |
| `cards.tokens.total_input_tokens` | int | Sum of input tokens across all sessions |
| `cards.tokens.total_output_tokens` | int | Sum of output tokens across all sessions |
| `cards.tokens.total_cost_usd` | string | Total estimated cost (decimal as string) |
| `cards.tokens.daily_costs` | array | Cost per day for charting |
| `cards.activity.total_files_read` | int | Sum of files read across all sessions |
| `cards.activity.total_files_modified` | int | Sum of files modified |
| `cards.activity.total_lines_added` | int | Sum of lines added |
| `cards.activity.total_lines_removed` | int | Sum of lines removed |
| `cards.activity.daily_session_counts` | array | Sessions per day for charting |
| `cards.tools.total_calls` | int | Sum of tool calls across all sessions |
| `cards.tools.total_errors` | int | Sum of tool errors |
| `cards.tools.tool_stats` | object | Per-tool success/error breakdown |
| `cards.agents_and_skills.total_agent_invocations` | int | Sum of agent invocations across all sessions |
| `cards.agents_and_skills.total_skill_invocations` | int | Sum of skill invocations across all sessions |
| `cards.agents_and_skills.agent_stats` | object | Per-agent-type success/error breakdown |
| `cards.agents_and_skills.skill_stats` | object | Per-skill success/error breakdown |

**Errors:**
- `400` - Invalid date format or range exceeds 90 days
- `401` - Authentication required

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
      "avg_user_thinking_ms": 120000,
      "total_assistant_duration_ms": 630000,
      "total_user_duration_ms": 1680000,
      "assistant_utilization_pct": 27.3
    },
    "agents_and_skills": {
      "agent_invocations": 5,
      "skill_invocations": 3,
      "agent_stats": {
        "Explore": {"success": 3, "errors": 0},
        "Plan": {"success": 2, "errors": 0}
      },
      "skill_stats": {
        "commit": {"success": 2, "errors": 0},
        "codebase-maintenance": {"success": 1, "errors": 0}
      }
    },
    "redactions": {
      "total_redactions": 5,
      "redaction_counts": {
        "GITHUB_TOKEN": 3,
        "API_KEY": 2
      }
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
| `cards.conversation.total_assistant_duration_ms` | int\|null | Total time Claude spent working across all turns (null if no data) |
| `cards.conversation.total_user_duration_ms` | int\|null | Total time user spent thinking between turns (null if no data) |
| `cards.conversation.assistant_utilization_pct` | float\|null | Percentage (0-100) of session time Claude was actively working (null if no data) |
| `cards.agents_and_skills.agent_invocations` | int | Total number of subagent/Task invocations |
| `cards.agents_and_skills.skill_invocations` | int | Total number of Skill invocations |
| `cards.agents_and_skills.agent_stats` | object | Map of agent type to stats object |
| `cards.agents_and_skills.agent_stats[type].success` | int | Successful invocations of this agent type |
| `cards.agents_and_skills.agent_stats[type].errors` | int | Failed invocations of this agent type |
| `cards.agents_and_skills.skill_stats` | object | Map of skill name to stats object |
| `cards.agents_and_skills.skill_stats[name].success` | int | Successful invocations of this skill |
| `cards.agents_and_skills.skill_stats[name].errors` | int | Failed invocations of this skill |
| `cards.redactions` | object\|null | Redaction metrics (null/omitted if no redactions) |
| `cards.redactions.total_redactions` | int | Total count of [REDACTED:TYPE] markers found |
| `cards.redactions.redaction_counts` | object | Map of redaction type to occurrence count |
| `card_errors` | object\|null | Map of card key to error message for failed computations (graceful degradation) |

**Graceful Degradation:**

If individual card computations fail, the API returns partial results. Successfully computed cards are included in `cards`, while failed cards have their errors reported in `card_errors`. This allows the frontend to display available data while showing error states for failed cards.

Example with partial failure:
```json
{
  "computed_at": "2024-01-15T10:30:00Z",
  "computed_lines": 150,
  "cards": {
    "tokens": { "input": 125000, ... },
    "session": { "duration_ms": 3600000, ... }
  },
  "card_errors": {
    "tools": "unexpected end of JSON input",
    "code_activity": "context deadline exceeded"
  }
}
```

**Notes:**
- Analytics are cached in the database and recomputed when new data is synced
- Returns empty analytics if session has no transcript file
- `304 Not Modified` has no body

---

## OAuth Endpoints (No prefix)

These endpoints handle OAuth authentication flow:

| Endpoint | Description |
|----------|-------------|
| `GET /auth/github/login` | Initiate GitHub OAuth |
| `GET /auth/github/callback` | GitHub OAuth callback |
| `GET /auth/google/login` | Initiate Google OAuth |
| `GET /auth/google/callback` | Google OAuth callback |
| `GET /auth/oidc/login` | Initiate generic OIDC OAuth (Okta, Auth0, Azure AD, Keycloak, etc.) |
| `GET /auth/oidc/callback` | Generic OIDC OAuth callback |
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

Admin functionality is accessed via HTML pages at `/admin`. Requires web session authentication and super admin privileges (configured via `SUPER_ADMIN_EMAILS` environment variable).

| Endpoint | Description |
|----------|-------------|
| `GET /admin/users` | List all users |
| `POST /admin/users/{id}/deactivate` | Deactivate user |
| `POST /admin/users/{id}/activate` | Activate user |
| `POST /admin/users/{id}/delete` | Delete user permanently |
| `GET /admin/users/new` | Create user form (password auth only) |
| `POST /admin/users/create` | Create user (password auth only) |
| `GET /admin/system-shares` | System shares form |
| `POST /admin/system-shares` | Create system share |

All admin actions are audit logged with admin identity and action details.

---

## Public API Endpoints (No Auth)

### Auth Config
```
GET /api/v1/auth/config
```

Returns the list of enabled authentication providers. No authentication required.

**Response:**
```json
{
  "providers": [
    {
      "name": "password",
      "display_name": "Password",
      "login_url": "/auth/password/login"
    },
    {
      "name": "github",
      "display_name": "GitHub",
      "login_url": "/auth/github/login"
    },
    {
      "name": "google",
      "display_name": "Google",
      "login_url": "/auth/google/login"
    },
    {
      "name": "oidc",
      "display_name": "Okta",
      "login_url": "/auth/oidc/login"
    }
  ],
  "features": {
    "shares_enabled": true,
    "footer_enabled": true
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `providers[].name` | string | Provider identifier: `"password"`, `"github"`, `"google"`, or `"oidc"` |
| `providers[].display_name` | string | Human-readable name for the provider (e.g., `"GitHub"`, `"Okta"`) |
| `providers[].login_url` | string | Path to initiate login with this provider |
| `features.shares_enabled` | bool | Whether share creation is enabled (`false` when `DISABLE_SHARE_CREATION=true`) |
| `features.footer_enabled` | bool | Whether the frontend footer is shown (`false` when `DISABLE_FOOTER=true`) |

Providers are returned in order: password, GitHub, Google, OIDC. Only enabled providers are included.

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
- `403` - Forbidden (CSRF failure, insufficient permissions, email domain not permitted)
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

## Email Domain Restrictions

When `ALLOWED_EMAIL_DOMAINS` is set (comma-separated list of domains), only users with matching email domains can access the instance. This applies to all authentication methods.

| Auth Path | Rejection Response |
|-----------|--------------------|
| OAuth callbacks (GitHub, Google, OIDC) | Redirect to `/login?error=access_denied&error_description=Your email domain is not permitted...` |
| Password login | Redirect to `/login?error=Your email domain is not permitted...` |
| API key requests | `403 Forbidden` with body `"Email domain not permitted"` |
| Session-authenticated requests | `403 Forbidden` with body `"Email domain not permitted"` |
| Device code token exchange | `403 Forbidden` with JSON `{"error": "access_denied"}` |
| Admin user creation | Redirect with error `"Email domain not permitted"` |

**Behavior:**
- Empty/unset `ALLOWED_EMAIL_DOMAINS` = no restriction (all domains allowed, backwards compatible)
- Strict domain match: `company.com` matches `@company.com` but NOT `@eng.company.com`
- Case-insensitive comparison
- Invalid domain entries cause fatal startup error
- `/api/v1/auth/config` does NOT expose domain restrictions

---

## Request Body Size Limits

| Size | Limit | Used For |
|------|-------|----------|
| XS | 2 KB | GET/DELETE requests |
| S | 16 KB | Auth tokens, simple metadata |
| M | 128 KB | API keys, shares, session updates |
| L | 2 MB | Batch operations |
| XL | 16 MB | Sync chunk uploads |
