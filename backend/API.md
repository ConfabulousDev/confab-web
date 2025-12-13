# Confab Backend API Reference

This document describes the backend API surface for the Confab web application. It is intended for Claude Code working on the frontend or CLI.

## Authentication

The API uses two authentication methods:

### 1. API Key Authentication (CLI)
Used by CLI tools. Pass the API key in the Authorization header:
```
Authorization: Bearer cfb_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

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

### Check Sessions Exist
Check which sessions already exist (for backfill deduplication).

```
POST /api/v1/sessions/check
Authorization: Bearer <api_key>
Content-Type: application/json
```

**Request:**
```json
{
  "external_ids": ["session-uuid-1", "session-uuid-2", ...]
}
```
- Max 1000 external IDs per request

**Response:**
```json
{
  "existing": ["session-uuid-1"],
  "missing": ["session-uuid-2"]
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

**Request:**
```json
{
  "external_id": "session-uuid",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/working/directory",
  "git_info": { ... }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `external_id` | string | Yes | Unique session identifier (UUID from Claude Code) |
| `transcript_path` | string | Yes | Path to transcript file on user's machine |
| `cwd` | string | No | Current working directory |
| `git_info` | object | No | Git repository metadata (branch, remote, etc.) |

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

### Read Sync File
Read the full contents of a synced file.

```
GET /api/v1/sync/file?session_id=<uuid>&file_name=<name>
Authorization: Bearer <api_key>
```

**Response:** `text/plain` - concatenated file contents

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
GET /api/v1/sessions?include_shared=true
```

| Query Param | Type | Default | Description |
|-------------|------|---------|-------------|
| `include_shared` | bool | false | Include sessions shared with user |

**Response:**
```json
[
  {
    "id": "uuid",
    "external_id": "session-uuid",
    "summary": "Session summary",
    "first_user_message": "First message",
    "first_seen": "2024-01-15T10:00:00Z",
    "last_message_at": "2024-01-15T11:30:00Z",
    "cwd": "/project/path",
    "git_info": { ... },
    "is_shared": false
  }
]
```

#### Get Session Detail
```
GET /api/v1/sessions/{id}
```

**Response:**
```json
{
  "id": "uuid",
  "external_id": "session-uuid",
  "summary": "Session summary",
  "first_user_message": "First message",
  "first_seen": "2024-01-15T10:00:00Z",
  "last_message_at": "2024-01-15T11:30:00Z",
  "cwd": "/project/path",
  "git_info": { ... },
  "files": [
    {
      "file_name": "transcript.jsonl",
      "file_type": "transcript",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ],
  "shares": [
    {
      "share_token": "hex32chars",
      "visibility": "public",
      "expires_at": null
    }
  ]
}
```

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
  "visibility": "public",
  "invited_emails": [],
  "expires_in_days": 30
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `visibility` | string | Yes | `"public"` or `"private"` |
| `invited_emails` | string[] | For private | Email addresses to invite (max 50) |
| `expires_in_days` | int | No | Days until expiration (null = never) |

**Response:**
```json
{
  "share_token": "hex32chars",
  "share_url": "https://confab.dev/sessions/{id}/shared/{token}",
  "visibility": "public",
  "invited_emails": [],
  "expires_at": "2024-02-15T10:00:00Z",
  "emails_sent": true,
  "email_failures": []
}
```

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
DELETE /api/v1/shares/{shareToken}
X-CSRF-Token: <token>
```

**Response:** `204 No Content`

---

### Access Shared Session (Public)

These endpoints don't require authentication (for public shares) or require the viewer to be logged in and on the invite list (for private shares).

#### Get Shared Session
```
GET /api/v1/sessions/{id}/shared/{shareToken}
```

#### Read Shared Sync File
```
GET /api/v1/sessions/{id}/shared/{shareToken}/sync/file?file_name=<name>
```

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

### Device Code Flow (CLI on headless machines)

| Endpoint | Description |
|----------|-------------|
| `POST /auth/device/code` | Request device code |
| `POST /auth/device/token` | Poll for access token |
| `GET /auth/device` | User verification page |
| `POST /auth/device/verify` | Submit user code |

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
| L | 2 MB | Batch operations (sessions/check) |
| XL | 16 MB | Sync chunk uploads |
