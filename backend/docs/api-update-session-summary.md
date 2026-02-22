# Update Session Summary API

## Endpoint

```
PATCH /api/v1/sessions/{external_id}/summary
```

## Description

Updates the summary field for a session identified by its external_id (the Claude Code session ID). This endpoint is designed for CLI use to set/update the session summary after it has been computed.

## Authentication

Requires API key authentication via the `Authorization` header:

```
Authorization: Bearer <api_key>
```

## Path Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `external_id` | string | Yes | The Claude Code session ID (e.g., `01234567-89ab-cdef-0123-456789abcdef`) |

## Request Body

```json
{
  "summary": "string"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `summary` | string | Yes | The summary text to set. Can be empty string to clear. |

## Response

### Success (200 OK)

```json
{
  "status": "ok"
}
```

### Errors

| Status | Response | Description |
|--------|----------|-------------|
| 400 | `{"error": "external_id is required"}` | Missing external_id in URL |
| 400 | `{"error": "Invalid request body"}` | Malformed JSON or missing summary field |
| 401 | `{"error": "User not authenticated"}` | Missing or invalid API key |
| 403 | `{"error": "Access denied"}` | Session exists but belongs to another user |
| 404 | `{"error": "Session not found"}` | No session with this external_id exists |
| 500 | `{"error": "Failed to update summary"}` | Internal server error |

## Example

### Request

```bash
curl -X PATCH \
  'https://your-server.example.com/api/v1/sessions/01234567-89ab-cdef-0123-456789abcdef/summary' \
  -H 'Authorization: Bearer your_api_key_here' \
  -H 'Content-Type: application/json' \
  -d '{"summary": "Implemented dark mode toggle for settings page"}'
```

### Response

```json
{
  "status": "ok"
}
```

## Notes

- The session must already exist (created via `/api/v1/sync/init`)
- Only the session owner can update the summary
- Empty string clears the summary
- This updates only the `summary` field; use `/api/v1/sync/init` or `/api/v1/sync/chunk` to set `first_user_message`
