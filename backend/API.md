# Confab Backend API Reference

This document describes the backend API surface for the Confab web application. It is intended for engineers (and AI coding agents) working on the frontend or CLI.

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
Used by the web frontend. Session cookie (`confab_session`) is set after OAuth login. CSRF protection is provided automatically via Fetch metadata validation (no token required).

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
  "provider": "claude-code",
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
| `provider` | string | No | Agent that produced the session: `"claude-code"` or `"codex"`. **Omit the field to default to `"claude-code"`** (preserves backward compatibility with older CLIs). An explicit empty string returns 400. Any other value returns 400. |
| `external_id` | string | Yes | Unique session identifier (UUID from the originating agent) |
| `transcript_path` | string | Yes | Path to transcript file on user's machine |
| `metadata` | object | No | Session metadata (see below) |
| `metadata.cwd` | string | No | Current working directory |
| `metadata.git_info` | object | No | Git repository metadata. See [`git_info` fields](#git_info-fields). |
| `metadata.hostname` | string | No | Client machine hostname |
| `metadata.username` | string | No | OS username of the client |

Session uniqueness is `(user_id, provider, external_id)`. The same `external_id` may exist under different providers without colliding.

##### `git_info` fields

The `git_info` object is stored as JSONB and passed through verbatim, but the
backend validates and consumes a known subset. Unknown fields are preserved
silently. Fields:

| Field | Type | Description |
|-------|------|-------------|
| `repo_url` | string | Canonical repo URL (typically `remote.origin.url`). Owner/repo is extracted from this for the repo filter. |
| `branch` | string | Current branch (e.g. `main`). |
| `commit_sha` | string | Optional commit SHA at session start. |
| `commit_message` | string | Optional commit subject line. |
| `author` | string | Optional commit author. |
| `is_dirty` | bool | Optional working-tree-dirty flag. |
| `remotes` | array | Optional list of git remotes (CF-494). Max 50 entries. Each entry: `name` (string, ≤256 chars, required, non-empty), `fetch_url` (string, ≤2048 chars), `push_url` (string, ≤2048 chars). At least one of `fetch_url`/`push_url` must be non-empty per entry. Malformed entries return 400. |
| `tracking_remote` | string | Optional name of the entry in `remotes` that points to the upstream (CF-494). Max 256 chars. When set and resolvable, the backend resolves the upstream live at read time (`db.RepoRootExpr`) so it's shown in the repo filter instead of the fork (CF-510). Stored verbatim in `git_info`; nothing is precomputed. |

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
  "provider": "claude-code",
  "files": {
    "transcript.jsonl": { "last_synced_line": 150 },
    "agent.jsonl": { "last_synced_line": 42 }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | Backend UUID for this session |
| `provider` | string | The resolved provider — echoes the request value, or `"claude-code"` if the request omitted it |
| `files` | object | Map of file_name to current sync state |

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
    "first_user_message": "First user message",
    "codex_rollout": {
      "thread_uuid": "uuid",
      "parent_thread_uuid": "uuid",
      "rollout_path": "/home/user/.codex/sessions/rollout-...jsonl",
      "cwd": "/home/user/project",
      "model": "gpt-5",
      "source": "codex-cli",
      "thread_source": "...",
      "agent_path": "...",
      "agent_role": "...",
      "agent_nickname": "..."
    }
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | Yes | UUID from sync/init response |
| `file_name` | string | Yes | Name of the file being synced |
| `file_type` | string | Yes | `"transcript"`, `"agent"`, or `"workflow_journal"` (see [Workflow files](#workflow-files) below) |
| `first_line` | int | Yes | Line number of first line (1-indexed, must be contiguous) |
| `lines` | string[] | Yes | Array of line contents |
| `metadata` | object | No | Optional metadata (only processed for transcript files) |
| `metadata.git_info` | object | No | Git repository metadata. See [`git_info` fields](#git_info-fields). |
| `metadata.summary` | string | No | Session summary (nil=don't update, ""=clear) |
| `metadata.first_user_message` | string | No | First user message (nil=don't update, ""=clear) |
| `metadata.codex_rollout` | object | No | Codex rollout sidecar metadata (codex sessions only). See [Codex Rollout Metadata](#codex-rollout-metadata) below. |

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

#### Workflow files

Claude **workflow** subagents (the `Workflow` tool) write their transcripts and a
run journal under a path-encoded `file_name`. The CLI uploads them with these
exact names so the run grouping (`<runId>`) is recoverable from the path alone:

| `file_name` | `file_type` | Parsed? | Analytics |
|-------------|-------------|---------|-----------|
| `subagents/workflows/<runId>/agent-<id>.jsonl` | `agent` | yes (transcript) | tokens attributed to the session, no double count |
| `subagents/workflows/<runId>/journal.jsonl` | `workflow_journal` | status only | excluded from token/transcript compute; parsed for per-agent success in the Workflows card (`cards.workflows`) |

- Slashes in `file_name` are permitted (they become extra S3 key segments). Read
  them back via the [file read endpoint](#download-session-file) with the
  `file_name` query parameter URL-encoded.
- `workflow_journal` is stored and surfaced in the session `files[]` API but is
  never Claude-parsed and never contributes to analytics.
- A backend's support for these files is discoverable via
  [`GET /api/v1/capabilities`](#capabilities); older backends omit that endpoint,
  and the CLI then skips workflow uploads.

#### Codex Rollout Metadata

When the session's provider is `codex`, each chunk may carry a `codex_rollout`
sub-block that registers the chunk's thread (root or child subagent) in a
metadata sidecar table (`codex_rollouts`). Child rollouts still upload their
content under the **root's hosted session and file**; this block records the
parent-child tree shape and per-thread metadata.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `thread_uuid` | string | Yes | UUID identifying the thread. Required, must be a valid UUID. |
| `parent_thread_uuid` | string | No | UUID of the parent thread. Omit for root rollouts. If provided: must be a valid UUID and must not equal `thread_uuid`. An explicit empty string is rejected. Orphan parents (referencing UUIDs not yet uploaded) are allowed. |
| `rollout_path` | string | Yes | Filesystem path to the Codex rollout JSONL on the CLI host. ≤ 8192 chars. |
| `cwd` | string | No | Working directory. ≤ 8192 chars. |
| `model` | string | No | Model name. ≤ 255 chars. |
| `source` | string | No | Producer label. ≤ 64 chars. |
| `thread_source` | string | No | Origin label for the thread. ≤ 255 chars. |
| `agent_path` | string | No | Agent definition path (subagents only). ≤ 8192 chars. |
| `agent_role` | string | No | Agent role (subagents only). ≤ 255 chars. |
| `agent_nickname` | string | No | Agent display name. ≤ 255 chars. |

**Validation errors (400):**
- `codex_rollout` on a non-codex session
- Missing or invalid `thread_uuid`
- Invalid `parent_thread_uuid` (empty string when set, malformed UUID, equal to `thread_uuid`)
- Missing `rollout_path`
- Any field exceeding its length limit

**Idempotency:**
- Repeated upserts (same chunk re-sent, or block included on chunks 1, 2, 3, …) produce a single row.
- **First-write-wins on `parent_thread_uuid`**: once set, never overwritten. A `nil → set` transition is allowed; a `set → different value` is silently preserved as the original.
- Other free-form fields: a non-empty incoming value overwrites the stored value; an empty incoming value preserves the stored non-empty value.

**Cascade behavior:** Deleting the hosted session or the owning user removes the row via `ON DELETE CASCADE`.

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

## External API Endpoints (API Key Auth)

Machine-consumable endpoints for external tooling (local AI, scripts, integrations). These use API key authentication and have a dedicated rate limiter (30 req/s, burst 60).

### Condensed Transcript (by UUID)

Returns a condensed, AI-readable transcript for a session. Uses the canonical access model — owner, recipient share, system share, and public share all grant access.

```
GET /api/v1/sessions/{id}/condensed-transcript
Authorization: Bearer <api_key>
```

**Query Parameters:**
| Parameter  | Type   | Required | Description |
|-----------|--------|----------|-------------|
| `max_chars` | int  | No       | Maximum character limit for the transcript. Truncates from the beginning, keeping the end (resolution) of the session. Preserves complete XML element boundaries. |

**Response:**
```json
{
  "metadata": {
    "session_id": "uuid",
    "external_id": "cli-session-id",
    "title": "Session title",
    "repo": "org/repo",
    "branch": "main",
    "first_seen": "2025-01-01T00:00:00Z",
    "last_sync_at": "2025-01-01T01:00:00Z",
    "total_lines": 500,
    "estimated_cost_usd": 0.42,
    "smart_recap": {
      "recap": "Summary of the session...",
      "went_well": ["Efficient debugging"],
      "went_bad": ["Missed edge case"],
      "human_suggestions": ["Add more tests"],
      "environment_suggestions": [],
      "default_context_suggestions": [],
      "computed_at": "2025-01-01T01:05:00Z"
    },
    "analytics": { }
  },
  "transcript": "<transcript>\n<user id=\"1\">\nHello\n</user>\n<assistant id=\"2\">\nHi there!\n</assistant>\n</transcript>"
}
```

**Access rules:** Follows the canonical access model (CF-132). Owner always has access. Recipient shares, system shares, and public shares also grant access. The `SHARE_ALL_SESSIONS` env var (on-prem) is respected.

**Error responses:**
- `400` — Invalid `max_chars` value
- `401` — Missing or invalid API key
- `404` — Session not found or no access
- `404` — No transcript available (session has no sync files)

---

### List Session Files

Returns the list of transcript files (main transcript + agent files) for a session.

```
GET /api/v1/sessions/{id}/files
Authorization: Bearer <api_key>
```

**Response:**
```json
{
  "files": [
    {
      "file_name": "transcript.jsonl",
      "file_type": "transcript",
      "last_synced_line": 150,
      "updated_at": "2026-03-28T12:00:00Z"
    },
    {
      "file_name": "agent-abc123.jsonl",
      "file_type": "agent",
      "last_synced_line": 42,
      "updated_at": "2026-03-28T12:01:00Z"
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `files[].file_name` | string | File name (e.g., `transcript.jsonl`, `agent-{id}.jsonl`, `subagents/workflows/{runId}/agent-{id}.jsonl`) |
| `files[].file_type` | string | `"transcript"`, `"agent"`, or `"workflow_journal"` (see [Workflow files](#workflow-files)) |
| `files[].last_synced_line` | integer | Number of lines synced for this file |
| `files[].updated_at` | string | ISO 8601 timestamp of last sync |

Uses canonical access model (CF-132) — owner, recipient, system, and public shares. Returns an empty `files` array if the session has no sync files.

**Error responses:**
- `401` — Missing or invalid API key
- `404` — Session not found or no access

### Download Session File

Downloads the full raw JSONL content of a single transcript file.

```
GET /api/v1/sessions/{id}/files/download?file_name=transcript.jsonl
Authorization: Bearer <api_key>
```

**Query Parameters:**
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file_name` | string | Yes | Name of the file to download (e.g., `transcript.jsonl`) |

**Response:** `text/plain; charset=utf-8` — raw JSONL content (one JSON object per line).

Uses canonical access model (CF-132). Validates the file exists in the session's sync_files before downloading from S3.

**Error responses:**
- `400` — Missing `file_name` query parameter
- `401` — Missing or invalid API key
- `404` — Session not found, no access, or file not found

---

### TIL Export

Returns a paginated list of TILs visible to the authenticated user, enriched with session URLs for external consumption. Designed for machine consumers that sync TILs to downstream systems (Confluence, Notion, wikis, etc.).

```
GET /api/v1/tils/export
Authorization: Bearer <api_key>
```

**Query Parameters:**
| Parameter   | Type   | Required | Description |
|------------|--------|----------|-------------|
| `owner`     | string | No       | Filter by TIL owner email. If omitted, returns all visible TILs. |
| `from`      | string | No       | Start of date range, inclusive (RFC 3339, e.g. `2026-03-01T00:00:00Z`) |
| `to`        | string | No       | End of date range, exclusive (RFC 3339, e.g. `2026-03-16T00:00:00Z`) |
| `page_size` | int    | No       | Results per page. Default 100, max 500. |
| `cursor`    | string | No       | Pagination cursor from previous response's `next_cursor`. |

**Date range semantics:** Semi-open interval `[from, to)` — `from` is inclusive, `to` is exclusive. Both must be valid RFC 3339 timestamps if provided.

**Response:**
```json
{
  "tils": [
    {
      "id": 42,
      "title": "Go's context.AfterFunc simplifies cleanup",
      "summary": "Learned that context.AfterFunc (Go 1.21+) ...",
      "created_at": "2026-03-14T18:30:00Z",
      "session_id": "abc-123",
      "session_title": "Refactor auth middleware",
      "session_url": "https://confab.example.com/sessions/abc-123",
      "transcript_deep_link": "https://confab.example.com/sessions/abc-123?msg=uuid-456",
      "git_repo": "org/repo",
      "git_branch": "main",
      "owner_email": "dev@example.com"
    }
  ],
  "has_more": false,
  "next_cursor": "",
  "page_size": 100,
  "count": 1
}
```

**URL fields:**
- `session_url` — Full URL to the session in the Confab web UI
- `transcript_deep_link` — URL to the session.
  - Claude TIL with `message_uuid`: appends `?msg=<message_uuid>`.
  - Codex TIL (CF-475): appends `?msg=<created_at>` (RFC 3339, URL-encoded). The Codex transcript resolves the timestamp to the latest row with `timestamp <= target`, landing on the row that was active when `/til` was invoked.
  - Claude TIL with null `message_uuid`: equals `session_url` (no usable anchor).

**Access rules:** Uses the same visibility model as the frontend TIL list. The caller sees TILs on sessions they own, sessions shared with them (private shares), and sessions with system shares. TILs on unshared sessions owned by other users are excluded.

**Error responses:**
- `400` — Invalid `from` or `to` (not valid RFC 3339)
- `400` — Invalid `page_size` (not a positive integer, or exceeds 500)
- `401` — Missing or invalid API key

---

## Web Endpoints (Session Auth)

All web endpoints require a valid session cookie (`confab_session`). CSRF protection is handled automatically by the server via Fetch metadata headers (`Sec-Fetch-Site`, `Origin`).

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
  "created_at": "2024-01-01T00:00:00Z",
  "has_own_sessions": false,
  "has_api_keys": false,
  "is_admin": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `has_own_sessions` | bool | Whether the user owns any synced sessions |
| `has_api_keys` | bool | Whether the user has any API keys configured |
| `is_admin` | bool | Whether the user is a super admin (checked against `SUPER_ADMIN_EMAILS`) |

---

### API Key Management

#### Create API Key
```
POST /api/v1/keys
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
```

**Response:** `204 No Content`

---

### Session Management

#### List Sessions
```
GET /api/v1/sessions?repo=<repos>&branch=<branches>&owner=<owners>&pr=<prs>&provider=<providers>&q=<search>&cursor=<cursor>
```

Returns cursor-paginated sessions visible to the user (owned + shared) with server-side filtering and pre-materialized filter options.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `repo` | string | No | all | Comma-separated repo names (e.g., `org/repo1,org/repo2`) |
| `branch` | string | No | all | Comma-separated branch names |
| `owner` | string | No | all | Comma-separated owner emails |
| `pr` | string | No | all | Comma-separated PR numbers |
| `provider` | string | No | all | Comma-separated canonical agent identifiers: `claude-code`, `codex`. Case-insensitive (`CODEX`, `Claude-Code` accepted; lowercased before matching). Any other value (including the legacy display form `Claude Code` with a space) returns 400. When `claude-code` is requested, rows with the legacy `session_type='Claude Code'` value also match. |
| `q` | string | No | none | Full-text search with prefix matching. Searches session metadata (titles, summary, first message), smart recap, and user transcript messages via PostgreSQL FTS. Multiple words use AND semantics. Prefix matching is automatic (e.g., `auth` matches `authentication`). Also matches commit SHA prefixes as fallback. |
| `cursor` | string | No | none | Opaque cursor for pagination (from `next_cursor` of previous response) |

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
      "provider": "claude-code",
      "file_count": 2,
      "total_lines": 1500,
      "git_repo": "org/repo",
      "git_repo_url": "https://github.com/org/repo",
      "git_branch": "main",
      "github_prs": ["https://github.com/org/repo/pull/123", "https://github.com/org/repo/pull/456"],
      "github_commits": ["abc1234", "def5678"],
      "estimated_cost_usd": "4.2300",
      "is_owner": true,
      "access_type": "owner",
      "shared_by_email": null,
      "owner_email": "alice@example.com"
    }
  ],
  "has_more": true,
  "next_cursor": "MjAyNS0wMS0xNVQxMTozMDowMFp8dXVpZA",
  "page_size": 50,
  "filter_options": {
    "repos": ["org/repo1", "org/repo2"],
    "branches": ["main", "feature-x"],
    "owners": ["alice@example.com", "bob@example.com"],
    "providers": ["claude-code", "codex"]
  }
}
```

**Notes:**
- `custom_title` is null/omitted when not set. Frontend displays: `custom_title || summary || first_user_message || fallback`.
- `github_prs` contains linked PR URLs (ordered by creation time ascending).
- `github_commits` contains linked commit SHAs (ordered by creation time descending, so latest is first).
- `estimated_cost_usd` is the estimated API cost (decimal as string, e.g. `"4.2300"`). Null/omitted when analytics have not been computed for the session.
- **Page size** is fixed at 50 sessions per page.
- **Cursor pagination**: Pass `next_cursor` from the response as `cursor` param to get the next page. When `has_more` is false, there are no more results. `next_cursor` is only present when `has_more` is true.
- **Visibility filter**: Only sessions with `total_lines > 0` and at least one of `summary` or `first_user_message` are included.
- **Filter options** are pre-materialized from lookup tables and the users table. They show all possible values (not affected by active filters). Repos and branches come from append-only lookup tables populated during sync; owners come from all users. `providers` is a static enum (`["claude-code", "codex"]`) returned regardless of which providers the requesting user has rows for, so the filter chip stays selectable in all cases.
- **Multiple values** within a filter dimension use OR logic (e.g., `repo=a,b` matches either). Across dimensions, filters use AND logic.

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

**Note:** When `ALLOWED_EMAIL_DOMAINS` is configured, authentication is **required** for all access types (including public shares). Unauthenticated requests return `401`.

**Response:**
```json
{
  "id": "uuid",
  "external_id": "session-uuid",
  "provider": "claude-code",
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
    "is_dirty": false,
    "remotes": [
      {"name": "origin",   "fetch_url": "git@github.com:me/repo.git",      "push_url": "git@github.com:me/repo.git"},
      {"name": "upstream", "fetch_url": "https://github.com/org/repo.git", "push_url": "https://github.com/org/repo.git"}
    ],
    "tracking_remote": "upstream"
  },
  "hostname": "macbook.local",
  "username": "developer",
  "is_owner": true,
  "shared_by_email": null,
  "owner_email": "alice@example.com",
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
| `owner_email` | string | Email of session owner (always populated) |

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
```

**Response:** `204 No Content`

Deletes session, all files, and all shares.

---

### Session Sharing

#### Create Share
```
POST /api/v1/sessions/{id}/share
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
- `403` - Share creation is not enabled (`ENABLE_SHARE_CREATION` is not set to `true`): `{"error": "Share creation is disabled by the administrator"}`

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
```

**Response:** `204 No Content`

**Note:** The `shareId` is the numeric ID returned from Create Share.

---

### Client Error Reporting

Report client-side errors to the backend for observability. Currently used by the frontend to report transcript validation errors (schema drift detection). Errors are logged server-side with structured fields for querying.

#### Report Client Errors
```
POST /api/v1/client-errors
Content-Type: application/json
```

**Request:**
```json
{
  "category": "transcript_validation",
  "session_id": "uuid",
  "errors": [
    {
      "line": 42,
      "message_type": "assistant",
      "details": [
        {
          "path": "content.0.type",
          "message": "Invalid discriminator value",
          "expected": "text|thinking|tool_use|tool_result",
          "received": "new_block_type"
        }
      ],
      "raw_json_preview": "{\"type\":\"assistant\",...}"
    }
  ],
  "context": {
    "url": "/sessions/abc-123",
    "user_agent": "Mozilla/5.0..."
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `category` | string | Yes | Error category (e.g., `"transcript_validation"`) |
| `session_id` | string | No | Session UUID for context |
| `errors` | array | Yes | 1-50 error items |
| `errors[].line` | int | Yes | Line number where the error occurred |
| `errors[].message_type` | string | No | Transcript message type (e.g., `"assistant"`, `"user"`) |
| `errors[].details` | array | Yes | 1+ validation issue details |
| `errors[].details[].path` | string | Yes | JSON path to the failing field |
| `errors[].details[].message` | string | Yes | Validation error message |
| `errors[].details[].expected` | string | No | Expected value/type |
| `errors[].details[].received` | string | No | Actual value/type received |
| `errors[].raw_json_preview` | string | No | Truncated raw JSON (max 500 chars) |
| `context` | object | No | Additional context |
| `context.url` | string | No | Page URL where the error occurred |
| `context.user_agent` | string | No | Browser user agent string |

**Response:**
```json
{
  "status": "ok"
}
```

**Errors:**
- `400` - Missing category, empty errors array, or more than 50 errors
- `401` - Authentication required

**Rate Limiting:** 0.5 req/sec, burst 5 (dedicated limiter).

**Notes:**
- Errors are logged server-side at Warn level with structured fields for log aggregation
- The frontend fires this as fire-and-forget (does not block transcript display)
- Deduplication is handled client-side (one report per session per page load)

---

### GitHub Links

Link sessions to GitHub artifacts (commits and PRs) for bidirectional navigation.

#### Create GitHub Link
```
POST /api/v1/sessions/{id}/github-links
Authorization: Bearer <api_key>  (CLI, or session cookie for Web)
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
```

Requires session ownership (web session auth only, no API key).

**Response:** `204 No Content`

**Errors:**
- `404` - Link not found or not session owner

---

### TILs (Today I Learned)

#### List TILs

```
GET /api/v1/tils
```

Lists TILs visible to the authenticated user. Supports the same filter dimensions as the session list: owner, repo, branch, and full-text search. Uses the three-CTE visibility pattern (owned sessions, private shares, system shares).

**Auth:** Session cookie

**Query Parameters:**
| Parameter | Type   | Description |
|-----------|--------|-------------|
| `q`       | string | Full-text search on title and summary |
| `owner`   | string | Comma-separated owner emails |
| `repo`    | string | Comma-separated git repo names (org/repo format) |
| `branch`  | string | Comma-separated branch names |
| `cursor`  | string | Pagination cursor |
| `page_size` | number | Results per page (default 50, max 100) |

**Response:** `200 OK`
```json
{
  "tils": [
    {
      "id": 1,
      "title": "Go channels for concurrency",
      "summary": "Learned that Go channels are...",
      "session_id": "uuid",
      "message_uuid": "msg-uuid-123",
      "created_at": "2026-03-14T10:00:00Z",
      "session_title": "Working on API endpoints",
      "git_repo": "org/repo",
      "git_branch": "main",
      "owner_email": "user@example.com",
      "is_owner": true,
      "access_type": "owner",
      "provider": "claude-code"
    }
  ],
  "has_more": false,
  "next_cursor": "",
  "page_size": 50,
  "filter_options": {
    "repos": ["org/repo"],
    "branches": ["main"],
    "owners": ["user@example.com"]
  }
}
```

The `provider` field (CF-475) carries the session's canonical `session_type`, normalized via `models.NormalizeProvider` (so legacy `"Claude Code"` values surface as `"claude-code"`). The frontend uses it to pick between UUID and timestamp deep-link targets.

#### Get TIL

```
GET /api/v1/tils/{id}
```

**Auth:** Session cookie

**Response:** `200 OK` — TIL object

**Errors:**
- `404` - TIL not found

#### Delete TIL

```
DELETE /api/v1/tils/{id}
```

Deletes a TIL. Only the TIL owner can delete.

**Auth:** Session cookie

**Response:** `204 No Content`

**Errors:**
- `404` - TIL not found or not owner (uniform 404 to avoid existence leak)

#### Create TIL (CLI)

```
POST /api/v1/tils
```

Creates a new TIL. Called by the CLI (`confab til` / `/til`).

**Auth:** API key

**Request Body:**
```json
{
  "title": "Go channels for concurrency",
  "summary": "Learned that Go channels are a first-class concurrency primitive...",
  "session_id": "<backend UUID>",
  "message_uuid": "<transcript message UUID>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | TIL title (max 500 chars) |
| `summary` | string | Yes | TIL summary (max 10000 chars) |
| `session_id` | string | Yes | Backend session UUID (must be owned by caller) |
| `message_uuid` | string | No | UUID of the transcript message this TIL is anchored to |

**Response:** `201 Created` — Created TIL object

**Errors:**
- `400` - Missing/invalid fields
- `404` - Session not found or not owned

#### List Session TILs

```
GET /api/v1/sessions/{id}/tils
```

Returns all TILs for a session. Uses canonical access model (CF-132) — anyone who can view the session transcript can see its TIL markers.

**Auth:** Optional (canonical access)

**Response:** `200 OK`
```json
{
  "tils": [
    {
      "id": 1,
      "title": "...",
      "summary": "...",
      "session_id": "uuid",
      "message_uuid": "msg-uuid",
      "created_at": "2026-03-14T10:00:00Z"
    }
  ]
}
```

**Errors:**
- `401` - Sign in to view (if auth may help)
- `404` - Session not found or no access

---

### Trends (Aggregated Analytics)

#### Get Trends
```
GET /api/v1/trends?start_ts=<epoch>&end_ts=<epoch>&tz_offset=<minutes>&repos=<repos>&include_no_repo=<bool>&provider=<providers>&owner=<emails>
```

Returns aggregated analytics across sessions **visible to the authenticated user**. Visibility follows the same model as `GET /api/v1/sessions` — owned ∪ private-share ∪ system-share, or all sessions when `SHARE_ALL_SESSIONS_TO_AUTHENTICATED` is on. The visibility predicate is shared with sessions/TILs via `db.VisibleSessionsCTE` (CF-495), so a session that's invisible to the caller on `/api/v1/sessions` is also invisible here.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| start_ts | integer | No | 7 days ago (UTC) | Start of date range as epoch seconds (inclusive, typically local midnight) |
| end_ts | integer | No | tomorrow (UTC) | End of date range as epoch seconds (exclusive, typically local midnight of day after last day) |
| tz_offset | integer | No | 0 | Client timezone offset in minutes (from JS `getTimezoneOffset()`; positive = behind UTC, e.g. 480 for PST) |
| repos | string | No | all | Comma-separated repo names to filter. Omitted/empty = include all repos. |
| include_no_repo | boolean | No | true | Include sessions without a git repo |
| provider | string | No | all | Comma-separated canonical AI providers (`claude-code`, `codex`). Case-insensitive; the legacy DB form `Claude Code` is rejected on the wire. Returns `400` for unknown values. Omitted/empty aggregates across all providers. |
| owner | string | No | all | Comma-separated owner emails to narrow the visible set (CF-495). Case-insensitive. **Privacy invariant**: narrows within the visible set; cannot broaden access. `?owner=ghost@x.com` for an owner the caller can't see returns zero rows (no 403, no existence leak). Omitted/empty aggregates across all visible owners. Max 50 values. |

**Constraints:**
- Maximum date range: 90 days
- Maximum owner values: 50 (per `validation.MaxFilterCount`)

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
  "providers_present": ["claude-code", "codex"],
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
        {"date": "2024-01-08", "cost_usd": "15.20", "per_provider": {"claude-code": "14.50", "codex": "0.70"}},
        {"date": "2024-01-09", "cost_usd": "18.50", "per_provider": {"claude-code": "17.80", "codex": "0.70"}}
      ],
      "per_provider": {
        "claude-code": {
          "total_input_tokens": 4500000,
          "total_output_tokens": 1900000,
          "total_cache_creation_tokens": 100000,
          "total_cache_read_tokens": 480000,
          "total_cost_usd": "121.25"
        },
        "codex": {
          "total_input_tokens": 500000,
          "total_output_tokens": 100000,
          "total_cache_creation_tokens": 0,
          "total_cache_read_tokens": 20000,
          "total_cost_usd": "4.25"
        }
      }
    },
    "activity": {
      "total_files_read": 500,
      "total_files_modified": 150,
      "total_lines_added": 5000,
      "total_lines_removed": 2000,
      "daily_session_counts": [
        {"date": "2024-01-08", "session_count": 5, "per_provider": {"claude-code": 4, "codex": 1}},
        {"date": "2024-01-09", "session_count": 8, "per_provider": {"claude-code": 5, "codex": 3}}
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
    },
    "top_sessions": {
      "sessions": [
        {
          "id": "550e8400-e29b-41d4-a716-446655440000",
          "title": "Implement dark mode with theme system",
          "provider": "claude-code",
          "estimated_cost_usd": "45.20",
          "duration_ms": 7200000,
          "git_repo": "org/frontend-app"
        },
        {
          "id": "660e8400-e29b-41d4-a716-446655440001",
          "title": "Debug OAuth redirect loop",
          "provider": "codex",
          "estimated_cost_usd": "32.15",
          "duration_ms": 5400000,
          "git_repo": "org/auth-service"
        }
      ]
    }
  },
  "filter_options": {
    "owners": ["alice@example.com", "bob@example.com"],
    "repos": ["org/frontend-app", "org/auth-service"]
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
| `providers_present` | string[] | Distinct canonical AI providers in the filtered result set, sorted alphabetically. Always present; `[]` when no sessions match. Originally introduced for the Tokens card's multi-provider caveat (CF-424); now duplicated by `cards.tokens.per_provider` keys, which the Tokens UI switches on. Kept on the top-level response for other consumers (CLI, scripts). |
| `cards.overview.session_count` | int | Total session count |
| `cards.overview.total_duration_ms` | int | Sum of all session durations |
| `cards.overview.avg_duration_ms` | int\|null | Average session duration |
| `cards.overview.days_covered` | int | Number of unique days with sessions |
| `cards.tokens.total_input_tokens` | int | Sum of input tokens across all sessions |
| `cards.tokens.total_output_tokens` | int | Sum of output tokens across all sessions |
| `cards.tokens.total_cost_usd` | string | Total estimated cost across all providers (decimal as string) |
| `cards.tokens.daily_costs` | array | Cost per day for charting. Each entry includes `date` (YYYY-MM-DD), `cost_usd` (cross-provider total), and `per_provider` (canonical provider id → decimal cost string) for the stacked-bar chart. `per_provider` is always present; empty `{}` for days with no sessions. |
| `cards.tokens.per_provider` | object | Per-canonical-provider tokens & cost breakdown (CF-435). Map keyed by canonical provider id (`claude-code`, `codex`); each entry has `total_input_tokens`, `total_output_tokens`, `total_cache_creation_tokens`, `total_cache_read_tokens`, `total_cost_usd`. Always present; `{}` when no sessions match. Legacy `Claude Code` session_type rows fold into the `claude-code` key server-side via `models.NormalizeProvider`. The Tokens UI switches to a per-provider table when this map has 2+ keys. |
| `cards.activity.total_files_read` | int | Sum of files read across all sessions |
| `cards.activity.total_files_modified` | int | Sum of files modified |
| `cards.activity.total_lines_added` | int | Sum of lines added |
| `cards.activity.total_lines_removed` | int | Sum of lines removed |
| `cards.activity.daily_session_counts` | array | Sessions per day for charting. Each entry includes `date` (YYYY-MM-DD), `session_count` (cross-provider total), and `per_provider` (canonical provider id → session count) for the stacked-bar chart (CF-444). `per_provider` is always present; empty `{}` for days with no sessions. Legacy `Claude Code` session_type rows fold into the `claude-code` key server-side via `models.NormalizeProvider`. |
| `cards.tools.total_calls` | int | Sum of tool calls across all sessions |
| `cards.tools.total_errors` | int | Sum of tool errors |
| `cards.tools.tool_stats` | object | Per-tool success/error breakdown |
| `cards.agents_and_skills.total_agent_invocations` | int | Sum of agent invocations across all sessions |
| `cards.agents_and_skills.total_skill_invocations` | int | Sum of skill invocations across all sessions |
| `cards.agents_and_skills.agent_stats` | object | Per-agent-type success/error breakdown |
| `cards.agents_and_skills.skill_stats` | object | Per-skill success/error breakdown |
| `cards.top_sessions.sessions` | array | Top 10 most expensive sessions, ordered by cost descending |
| `cards.top_sessions.sessions[].id` | string | Session UUID (for linking to session detail) |
| `cards.top_sessions.sessions[].title` | string | Best available session title (custom > suggested > summary > first message > fallback) |
| `cards.top_sessions.sessions[].provider` | string | Canonical provider value (`claude-code` or `codex`). Legacy `Claude Code` is normalized server-side. |
| `cards.top_sessions.sessions[].estimated_cost_usd` | string | Session cost (decimal as string) |
| `filter_options.owners` | string[] | Lowercased owner emails from the caller's visible-session set, alphabetical. **Static across active filters** — mirrors `SessionFilterOptions` shape on `/api/v1/sessions`. Drives the owner dropdown on TrendsPage without a side-call. CF-495. |
| `filter_options.repos` | string[] | Repo names (`owner/name`) from the caller's visible-session set, alphabetical. Fork→upstream collapsed via `db.RepoRootExpr`, resolved live per session from its own `git_info` (CF-510). Static across active filters. CF-495. |
| `cards.top_sessions.sessions[].duration_ms` | int\|null | Session duration in milliseconds |
| `cards.top_sessions.sessions[].git_repo` | string\|null | Extracted repo name (e.g., "org/repo") |

**Errors:**
- `400` - Invalid date format or range exceeds 90 days
- `401` - Authentication required

---

### Organization Analytics

#### Get Organization Analytics
```
GET /api/v1/org/analytics?start_ts=<epoch>&end_ts=<epoch>&tz_offset=<minutes>&provider=<list>&repos=<list>&include_no_repo=<bool>
```

Returns per-user aggregated analytics across all sessions in the organization. Requires `ENABLE_ORG_ANALYTICS=true` environment variable.

**Privacy implications:** When enabled, **any authenticated user** can view every other user's name, email, session count, cost, and time breakdowns. This is designed for trusted-team deployments where full visibility is acceptable. There is no role-based restriction — all users see the same data. If you need to limit visibility to admins only, do not enable this feature. If `ALLOWED_EMAIL_DOMAINS` is configured, access is implicitly scoped to users within the allowed domains.

**Query Parameters:**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| start_ts | integer | No | 7 days ago (UTC) | Start of date range as epoch seconds (inclusive) |
| end_ts | integer | No | tomorrow (UTC) | End of date range as epoch seconds (exclusive) |
| tz_offset | integer | No | 0 | Client timezone offset in minutes |
| provider | string | No | (all) | Comma-separated canonical providers (`claude-code`, `codex`). Case-insensitive. Omitted/empty = aggregate across all providers. Legacy `Claude Code` rows fold into `claude-code` automatically. |
| repos | string | No | all | Comma-separated repo names (`owner/name` form) to include. Omitted/empty = include all repos. |
| include_no_repo | boolean | No | `true` | Whether to include sessions that have no `repo_url` in `git_info`, independent of the `repos` list. |

**Constraints:**
- Maximum date range: 90 days
- Route returns 404 when `ENABLE_ORG_ANALYTICS` is not enabled

**Response:**
```json
{
  "computed_at": "2024-01-15T10:30:00Z",
  "date_range": {
    "start_date": "2024-01-08",
    "end_date": "2024-01-15"
  },
  "providers_present": ["claude-code", "codex"],
  "users": [
    {
      "user": {
        "id": 1,
        "email": "alice@example.com",
        "name": "Alice Chen"
      },
      "session_count": 45,
      "total_cost_usd": "128.50",
      "total_duration_ms": 432000000,
      "total_assistant_time_ms": 216000000,
      "total_user_time_ms": 216000000,
      "avg_cost_usd": "2.86",
      "avg_duration_ms": 9600000,
      "avg_assistant_time_ms": 4800000,
      "avg_user_time_ms": 4800000
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `computed_at` | string | ISO timestamp when analytics were computed |
| `date_range.start_date` | string | Start date (inclusive) |
| `date_range.end_date` | string | End date (inclusive) |
| `providers_present` | array | Canonical providers (e.g. `claude-code`, `codex`) with at least one qualifying session in the date range × repo filter. Independent of the `?provider=` selection so clients can use this to populate a provider-filter dropdown that stays widenable when one provider is pinned. Always non-null; `[]` when nothing matches. |
| `users` | array | One entry per active user |
| `users[].user.id` | int | User ID |
| `users[].user.email` | string | User email |
| `users[].user.name` | string\|null | User display name |
| `users[].session_count` | int | Sessions with both tokens and conversation cards |
| `users[].total_cost_usd` | string | Total estimated cost (decimal as string) |
| `users[].total_duration_ms` | int | Total session duration in milliseconds |
| `users[].total_assistant_time_ms` | int | Total assistant time in milliseconds |
| `users[].total_user_time_ms` | int | Total user time in milliseconds |
| `users[].avg_cost_usd` | string | Average cost per session (decimal as string) |
| `users[].avg_duration_ms` | int\|null | Average session duration (null if 0 sessions) |
| `users[].avg_assistant_time_ms` | int\|null | Average assistant time per session (null if 0 sessions) |
| `users[].avg_user_time_ms` | int\|null | Average user time per session (null if 0 sessions) |

**Errors:**
- `400` - Invalid parameters or range exceeds 90 days
- `401` - Authentication required
- `404` - Feature not enabled (`ENABLE_ORG_ANALYTICS` env var not set)

#### List Organization Repos
```
GET /api/v1/org/repos?start_ts=<epoch>&end_ts=<epoch>&tz_offset=<minutes>
```

Returns the alphabetically sorted, deduplicated list of repos (owner/name) across all org sessions in the date range. Drives the repo filter dropdown on the Organization page.

Requires `ENABLE_ORG_ANALYTICS=true`. Same privacy model and middleware chain as the Organization Analytics endpoint: any authenticated user sees every repo name across the org.

**Query Parameters:** identical to `GET /api/v1/org/analytics` (`start_ts`, `end_ts`, `tz_offset`).

**Response:**
```json
{
  "computed_at": "2024-01-15T10:30:00Z",
  "date_range": {
    "start_date": "2024-01-08",
    "end_date": "2024-01-15"
  },
  "repos": ["ConfabulousDev/confab-cli", "ConfabulousDev/confab-web"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `repos` | array | Alphabetically sorted `owner/name` strings extracted from sessions.git_info->>'repo_url'. Always non-null; `[]` when nothing matches. |

**Errors:**
- `400` - Invalid parameters or range exceeds 90 days
- `401` - Authentication required
- `404` - Feature not enabled (`ENABLE_ORG_ANALYTICS` env var not set)

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
      "cache_read": 12000,
      "estimated_usd": "1.25",
      "fast_turns": 10,
      "fast_cost_usd": "0.90"
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
    },
    "workflows": {
      "runs": [
        {
          "run_id": "wf-2026-06-05_a1b2c3",
          "agent_count": 6,
          "input_tokens": 42000,
          "output_tokens": 18000,
          "cache_creation": 12000,
          "cache_read": 96000,
          "estimated_usd": "1.84",
          "succeeded_agents": 6,
          "has_journal": true,
          "duration_ms": 132000
        }
      ]
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `computed_at` | string | ISO timestamp when analytics were computed |
| `computed_lines` | int | Line count through which analytics are computed |
| `tokens.*` | object | *Deprecated:* Use `cards.tokens` instead |
| `cost.*` | object | *Deprecated:* Use `cards.tokens.estimated_usd` instead |
| `compaction.*` | object | *Deprecated:* Use `cards.session` instead |
| `cards` | object | Card-based analytics data (keyed by card name) |
| `cards.tokens.input` | int | Total input tokens sent to model |
| `cards.tokens.output` | int | Total output tokens generated |
| `cards.tokens.cache_creation` | int | Tokens written to cache |
| `cards.tokens.cache_read` | int | Tokens served from cache |
| `cards.tokens.estimated_usd` | string | Estimated API cost (assumes 5-min prompt caching) |
| `cards.tokens.fast_turns` | int\|omitted | Turns using fast mode (omitted if no fast mode usage) |
| `cards.tokens.fast_cost_usd` | string\|omitted | Cost from fast mode turns (omitted if no fast mode usage) |
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
| `cards.workflows` | object\|null | Workflow subagent runs (null/omitted if the session has no workflow runs) |
| `cards.workflows.runs` | array | Per-run aggregates, ordered by run start time |
| `cards.workflows.runs[].run_id` | string | Opaque workflow run ID (the `<runId>` path segment) |
| `cards.workflows.runs[].agent_count` | int | Number of subagent files in the run |
| `cards.workflows.runs[].input_tokens` | int | Input tokens summed across the run's agents |
| `cards.workflows.runs[].output_tokens` | int | Output tokens summed across the run's agents |
| `cards.workflows.runs[].cache_creation` | int | Cache-write tokens summed across the run's agents |
| `cards.workflows.runs[].cache_read` | int | Cache-read tokens summed across the run's agents |
| `cards.workflows.runs[].estimated_usd` | string | Estimated cost for the run |
| `cards.workflows.runs[].succeeded_agents` | int | Agents with a journal `result` line (0 when `has_journal` is false) |
| `cards.workflows.runs[].has_journal` | bool | Whether a run journal was uploaded; when false, `succeeded_agents` is not meaningful |
| `cards.workflows.runs[].duration_ms` | int | Activity span from the run's first to last agent timestamp (0 if unknown) |
| `card_errors` | object\|null | Map of card key to error message for failed computations (graceful degradation) |
| `smart_recap_quota` | object\|null | Per-user quota info (present when quota is capped and viewer is owner; omitted when unlimited or non-owner) |
| `smart_recap_quota.used` | int | Recaps generated this month |
| `smart_recap_quota.limit` | int | Monthly cap |
| `smart_recap_quota.exceeded` | bool | Whether the cap has been reached |
| `smart_recap_missing_reason` | string\|null | Why smart recap data is absent: `"quota_exceeded"` (owner) or `"unavailable"` (non-owner). Omitted when smart recap data is present. |

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

Admin API endpoints under `/api/v1/admin/`. Requires web session authentication + CSRF + super admin privileges (configured via `SUPER_ADMIN_EMAILS` environment variable). All admin actions are audit logged.

### List Users
```
GET /api/v1/admin/users
```

**Response:**
```json
{
  "users": [
    {
      "id": 1,
      "email": "user@example.com",
      "name": "User Name",
      "status": "active",
      "session_count": 42,
      "recap_cache_count": 10,
      "recaps_this_month": 3,
      "last_api_key_used": "2024-01-15T10:30:00Z",
      "last_logged_in": "2024-01-20T14:00:00Z",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "totals": {
    "total_sessions": 100,
    "non_empty_sessions": 80,
    "sessions_with_cache": 50,
    "computations_this_month": 25
  }
}
```

### Create User (Password Auth Only)
```
POST /api/v1/admin/users
```
**Request:** `{ "email": "new@example.com", "password": "securepass123" }`
**Response:** `{ "id": 2, "email": "new@example.com" }`
**Errors:** 400 (validation), 409 (duplicate email)

### Deactivate User
```
POST /api/v1/admin/users/{id}/deactivate
```
**Response:** `{ "id": 1, "status": "inactive" }`

### Activate User
```
POST /api/v1/admin/users/{id}/activate
```
**Response:** `{ "id": 1, "status": "active" }`

### Delete User
```
DELETE /api/v1/admin/users/{id}
```
Deletes S3 data first, then DB record (CASCADE). Uses 60s timeout.
**Response:** 204 No Content
**Errors:** 404 (not found)

### List System Shares
```
GET /api/v1/admin/system-shares
```
**Response:**
```json
{
  "shares": [
    {
      "id": 1,
      "session_id": "uuid",
      "external_id": "ext-id",
      "provider": "claude-code",
      "share_url": "https://app.example.com/sessions/uuid",
      "expires_at": null,
      "created_at": "2024-01-01T00:00:00Z",
      "last_accessed_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

`provider` is the canonical session provider (`"claude-code"` or `"codex"`). Legacy `"Claude Code"` rows are normalized at the DB boundary so the wire value is always canonical.

### Create System Share
```
POST /api/v1/admin/system-shares
```
**Request:** `{ "session_id": "uuid" }`
**Response:** `{ "share_id": 1, "external_id": "ext-id", "share_url": "https://..." }`
**Errors:** 400 (shares disabled, missing session_id), 404 (session not found)

### Get Smart Recap Prompt
```
GET /api/v1/admin/settings/smart-recap-prompt
```

Returns the current smart recap system prompt (custom or default) along with the fixed (non-customizable) prompt sections for reference.

**Response:**
```json
{
  "instructions": "You are an expert software engineer...",
  "is_custom": false,
  "updated_at": "2024-01-15T10:30:00Z",
  "input_format": "## Input Format\n...",
  "output_schema": "## Output Schema\n...",
  "example": "## Example\n..."
}
```

| Field | Type | Description |
|-------|------|-------------|
| `instructions` | string | The customizable instructions section (default or admin-set) |
| `is_custom` | bool | `true` if an admin has set custom instructions |
| `updated_at` | string? | RFC 3339 timestamp of last update (only present when `is_custom` is true) |
| `input_format` | string | Fixed input format section (read-only) |
| `output_schema` | string | Fixed output JSON schema section (read-only) |
| `example` | string | Fixed example section (read-only) |

### Get Smart Recap Prompt Default
```
GET /api/v1/admin/settings/smart-recap-prompt/default
```

Returns the hardcoded default instructions, useful for showing a "reset preview" in the UI.

**Response:**
```json
{
  "instructions": "You are an expert software engineer..."
}
```

### Set Smart Recap Prompt
```
PUT /api/v1/admin/settings/smart-recap-prompt
```

Sets custom instructions for the smart recap system prompt. The fixed sections (input format, output schema, example) are not affected.

**Request:**
```json
{
  "instructions": "Custom instructions for the LLM..."
}
```

**Response:**
```json
{
  "instructions": "Custom instructions for the LLM...",
  "is_custom": true,
  "updated_at": "2024-01-15T10:30:00Z"
}
```

**Errors:** 400 (invalid UTF-8, null bytes, exceeds 50,000 character limit)

### Reset Smart Recap Prompt
```
DELETE /api/v1/admin/settings/smart-recap-prompt
```

Resets the prompt to the hardcoded default by deleting the custom setting.

**Response:**
```json
{
  "instructions": "You are an expert software engineer...",
  "is_custom": false
}
```

### Get Smart Recap Regenerate Count
```
GET /api/v1/admin/settings/smart-recap-prompt/regenerate-count
```

Returns the number of sessions that have existing smart recap cards (i.e., would be affected by a bulk regeneration).

**Response:**
```json
{
  "count": 42
}
```

### Regenerate All Smart Recaps
```
POST /api/v1/admin/settings/smart-recap-prompt/regenerate-all
```

Triggers bulk regeneration of all smart recaps. Writes a timestamp to `admin_settings`; the background worker picks up cards with `computed_at` before this timestamp as stale (category 4). Admin-triggered regeneration bypasses per-user quota checks.

**Response:**
```json
{
  "sessions_queued": 42
}
```

### Invalidate Cards by Date Range
```
POST /api/v1/admin/cards/invalidate
```

Deletes `session_card_*` rows for sessions in a date window so the precompute worker recomputes them with current logic/pricing on the next tick (CF-343). Writes one audit row per affected session to `admin_card_invalidations`; the row also acts as a per-session smart-recap quota bypass signal.

**Request:**
```json
{
  "start_date": "2026-04-01T00:00:00Z",
  "end_date": "2026-04-20T23:59:59Z",
  "card_types": ["session_card_tokens"],
  "reason": "Opus 4.7 pricing backfill",
  "dry_run": false
}
```

| Field | Type | Description |
|-------|------|-------------|
| `start_date` | string | Required. ISO-8601 with explicit timezone (`Z` or `±hh:mm`). Filter: `sessions.last_message_at >= start_date`. |
| `end_date` | string | Optional. Same format as `start_date`. Filter: `last_message_at < end_date`. Must be after `start_date`. |
| `card_types` | string[] | Required, non-empty. Each entry must be one of: `session_card_tokens`, `session_card_session`, `session_card_tools`, `session_card_code_activity`, `session_card_conversation`, `session_card_agents_and_skills`, `session_card_redactions`, `session_card_workflows`, `session_card_smart_recap`. |
| `reason` | string | Required, 1–500 chars. Stored in the audit row. |
| `dry_run` | bool | Defaults to `true`. `false` to actually delete. |

**Response (dry-run or success):**
```json
{
  "correlation_id": "0191a3e0-1234-7000-8000-aabbccddeeff",
  "affected_sessions": 1234,
  "affected_cards": {
    "session_card_tokens": 1234
  },
  "executed": false
}
```

| Field | Type | Description |
|-------|------|-------------|
| `correlation_id` | string | UUID grouping all writes from this run (shared across batches). Generated for dry-run too so the UI can stitch preview → execute. |
| `affected_sessions` | int | `COUNT(DISTINCT s.id)` where the session has at least one row in any selected card table (intersection semantic). |
| `affected_cards[<table>]` | int | Per-table count of rows that would be / were deleted. |
| `executed` | bool | `false` for dry-run; `true` for actual execute. |

**Partial-failure response (500):** the body includes `completed_batches` and `affected_sessions_executed` reporting progress before the failure, plus an `error` string. Already-committed batches remain invalidated; re-running the same window is safe.

**Auth:** super-admin only (`admin.Middleware`).

### List Card Invalidations
```
GET /api/v1/admin/cards/invalidations
GET /api/v1/admin/cards/invalidations?correlation_id=<uuid>
```

Returns up to 500 most recent audit rows (ordered by `invalidated_at DESC`). Pass `correlation_id` to drill down into a single run.

**Response:**
```json
{
  "rows": [
    {
      "id": 42,
      "session_id": "...",
      "admin_user_id": 7,
      "admin_email": "admin@example.com",
      "invalidated_at": "2026-04-20T15:30:00Z",
      "card_types": ["session_card_tokens"],
      "correlation_id": "0191a3e0-1234-7000-8000-aabbccddeeff",
      "reason": "Opus 4.7 pricing backfill"
    }
  ]
}
```

`admin_email` is empty (omitted) when the admin user has been deleted.

**Auth:** super-admin only.

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
    "saas_footer_enabled": false,
    "saas_termly_enabled": false,
    "support_email": "support@example.com"
  },
  "version": {
    "current": "v0.4.1",
    "latest": "v0.5.0",
    "latest_url": "https://github.com/ConfabulousDev/confab-web/releases/tag/v0.5.0",
    "update_available": true,
    "update_severity": "recommended",
    "update_check_disabled": false,
    "update_check_failed": false
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `providers[].name` | string | Provider identifier: `"password"`, `"github"`, `"google"`, or `"oidc"` |
| `providers[].display_name` | string | Human-readable name for the provider (e.g., `"GitHub"`, `"Okta"`) |
| `providers[].login_url` | string | Path to initiate login with this provider |
| `features.shares_enabled` | bool | Whether share creation is enabled (`true` when `ENABLE_SHARE_CREATION=true`) |
| `features.saas_footer_enabled` | bool | Whether the SaaS footer is shown (`true` when `ENABLE_SAAS_FOOTER=true`) |
| `features.saas_termly_enabled` | bool | Whether Termly cookie consent is enabled (`true` when `ENABLE_SAAS_TERMLY=true`) |
| `features.org_analytics_enabled` | bool | Whether org-wide analytics is enabled (`true` when `ENABLE_ORG_ANALYTICS=true`). See [Organization Analytics](#organization-analytics) for privacy implications |
| `features.password_auth_enabled` | bool | Whether password-based authentication is enabled |
| `features.support_email` | string | Support contact email address (from `SUPPORT_EMAIL` env var, defaults to `"support@example.com"`) |
| `version.current` | string | Running backend build tag (e.g., `"v0.4.1"`). Empty in local dev (`go run` without ldflags). |
| `version.latest` | string | Latest stable release tag on GitHub. Omitted when the check is disabled or the GitHub fetch failed. |
| `version.latest_url` | string | URL of the latest release notes (GitHub `html_url`). Omitted with `latest`. |
| `version.update_available` | bool | `true` when the frontend should show the update badge. Forced `true` in local dev (empty `current`) so the badge is visible during development. Equivalent to `update_severity != ""`. |
| `version.update_severity` | string | Grades the badge's loudness by how far behind the build is. `"available"` (patch-only behind, or dev build) → regular badge "Update available"; `"recommended"` (minor or major behind) → red badge "Update recommended". Omitted when there is no update to badge. |
| `version.update_check_disabled` | bool | `true` when the operator set `DISABLE_UPDATE_CHECK=true` or `ENABLE_SAAS_FOOTER=true` (SaaS users can't self-upgrade). |
| `version.update_check_failed` | bool | `true` when the most recent GitHub fetch failed; cached for 15 min before retrying. |

Providers are returned in order: password, GitHub, Google, OIDC. Only enabled providers are included.

The `version` object surfaces the running backend build alongside the latest GitHub release so the frontend can render the "Update available" badge. The backend caches the GitHub response for 6 hours (15 minutes on failure) and never blocks the caller for longer than 3 seconds. See [`internal/updatecheck`](internal/updatecheck/) for details.

### Backend Version
```
GET /api/v1/version
```

Reports what build this backend *is*. No authentication required and **no external dependency** — it never touches the database, the update checker, or the network, so it's always fast and safe for liveness probes, post-deploy verification, and the CLI capability probe. "Is there a newer release available?" is a separate question that stays in [`/api/v1/auth/config`](#auth-config) (which does fetch from GitHub).

**Response:**
```json
{
  "version": "v0.4.3",
  "commit": "224ede8a1b2c3d4e5f60718293a4b5c6d7e8f901",
  "build_time": "2026-05-29T21:00:00Z",
  "go_version": "go1.26.3"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `version` | string | Running backend build tag (e.g., `"v0.4.3"`). Reports `"dev"` for non-release builds (`go run`, or a `flyctl` deploy without a release tag). |
| `go_version` | string | Go runtime version the binary was built with (`runtime.Version()`). |
| `commit` | string | Full git SHA the binary was built from. Set via `-ldflags` at build time; **omitted** when unset (e.g. `go run` dev build). |
| `build_time` | string (RFC 3339) | UTC timestamp of the build. Set via `-ldflags` at build time; **omitted** when unset. |

`commit` and `build_time` are injected at build time (see `Dockerfile`, `.github/workflows/release.yml`, and `deploy-to-fly.sh`); `version` and `go_version` are always present. Unlike `version.current` in `/api/v1/auth/config`, this endpoint carries no update-check fields.

### Capabilities
```
GET /api/v1/capabilities
```

Advertises optional backend features so a newer CLI can gate behavior on what
this (possibly older, self-hosted) backend supports. No authentication required
and **no external dependency** — the signal is static for a given build. The
response body **is** the capabilities map (no outer wrapper).

**Response:**
```json
{
  "workflow_files": true,
  "workflow_journal": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `workflow_files` | bool | Backend resolves path-encoded workflow subagent file names (`subagents/workflows/<runId>/agent-<id>.jsonl`) and attributes their tokens. |
| `workflow_journal` | bool | Backend accepts, stores, and serves the `workflow_journal` file_type (the run journal), excluding it from analytics. See [Workflow files](#workflow-files). |

Both fields are always `true` on a build that ships this endpoint — its very
presence is the signal. **Older backends omit this endpoint entirely** (404);
the CLI reads that as "workflow files unsupported" and skips those uploads,
keeping the rollout order safe (backend first, then CLI). New capability fields
may be added over time; clients must treat any absent field as `false`.

### Model Pricing
```
GET /api/v1/pricing
```

Returns the effective per-million-token model price table. No authentication required. The frontend reads it at bootstrap to cost out token usage client-side; downstream self-hosted backends pull it from confabulous.dev to refresh their own table without a redeploy.

**Response:**
```json
{
  "schema_version": 0,
  "updated_at": "2026-05-29T00:00:00Z",
  "pricing": {
    "claude-code": {
      "opus-4-7": { "input": 5, "output": 25, "cacheWrite": 6.25, "cacheRead": 0.5 }
    },
    "codex": {
      "gpt-5": { "input": 1.25, "output": 10, "cacheWrite": 0, "cacheRead": 0.125 }
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `schema_version` | int | Document format version (currently `0`). A reader rejects a version higher than it understands and falls back to its embedded table. |
| `updated_at` | string (RFC 3339) | When the price data was last changed. Drives "freshest-wins": a self-hosted backend adopts a remote table only when it is strictly newer than its embedded copy. |
| `pricing` | object | Provider (`claude-code` / `codex`) → model family → rates. Rates are USD per million tokens. OpenAI cache writes are free (`cacheWrite: 0`); the cached-input rate is `cacheRead`. |

**Caching:** sent with `Cache-Control: public, max-age=<refresh-interval>` (default 7200s) — a tiny, edge-cacheable payload. The serving backend refreshes its own table from `PRICING_SOURCE_URL` lazily (2h on success, 15m on failure) and always returns a valid table (its embedded floor at worst). See [`internal/pricingsource`](internal/pricingsource/) for details.

---

## Utility Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check. Response: `{"status": "ok"}` |
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
- `403` - Forbidden (CSRF validation failure, insufficient permissions, email domain not permitted)
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
| External API | 30 req/sec | 60 |

Upload rate limiting is per-user (not per-IP) to support backfill scenarios.
External API rate limiting is per-user (keyed by authenticated user ID).

---

## Email Domain Restrictions

When `ALLOWED_EMAIL_DOMAINS` is set (comma-separated list of domains), only users with matching email domains can access the instance. This applies to all authentication methods.

| Auth Path | Rejection Response |
|-----------|--------------------|
| OAuth callbacks (GitHub, Google, OIDC) | Redirect to `/login?error=access_denied&error_description=Your email domain is not permitted...` |
| Password login | Redirect to `/login?error=Your email domain is not permitted...` |
| API key requests | `403 Forbidden` with body `"Email domain not permitted"` |
| Session-authenticated requests | `403 Forbidden` with body `"Email domain not permitted"` |
| Device code verification | `403 Forbidden` with HTML error `"Your email domain is not permitted"` |
| Device code token exchange | `403 Forbidden` with JSON `{"error": "access_denied"}` |
| Optional auth endpoints (session detail, analytics, sync file) | `401 Unauthorized` with `"Authentication required"` (anonymous access blocked) |
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

---

## Read-Only Identity (CF-483 Demo Mode)

When the server is configured with `DEMO_IDENTITY_EMAIL`, requests resolving to the demo user (whether via auto-impersonated session cookie or — should it occur — a leaked API key) get the following uniform error on any mutating method:

**Endpoints affected:** Any `POST`, `PUT`, `PATCH`, or `DELETE` under `/api/v1` when the resolved user has `read_only=true`.

**Response:** `403 Forbidden`

```json
{
  "error": "read_only_user",
  "message": "This identity is read-only."
}
```

GET/HEAD/OPTIONS requests pass through unchanged. The check fires from `EnforceReadOnly` middleware chained inside every auth middleware; it cannot be bypassed by adding a new route group. When `DEMO_IDENTITY_EMAIL` is unset, no user has `read_only=true` and this error never appears.
