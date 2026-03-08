# models

Shared domain types used across the backend. This is a leaf package with no internal dependencies.

## Files

| File | Role |
|------|------|
| `models.go` | All domain model structs, enums, and API request/response types |

## Key Types

### User domain

- **`User`** -- Core user record with ID, email, optional name/avatar, status, and timestamps.
- **`UserStatus`** -- String enum: `"active"` or `"inactive"`.
- **`AdminUserStats`** -- Embeds `User` with session count, last API key usage, and last login timestamps. Used by the admin UI.
- **`OAuthProvider`** -- String enum: `"github"`, `"google"`, `"oidc"`.
- **`UserIdentity`** -- An OAuth identity linked to a user (provider, provider ID, optional username).
- **`OAuthUserInfo`** -- User info fetched from an OAuth provider during login.
- **`WebSession`** -- Browser session for OAuth-authenticated users. Includes `UserEmail` and `UserStatus` fields (not serialized to JSON) for tracing and auth checks.
- **`APIKey`** -- An API key record. `KeyHash` is tagged `json:"-"` to prevent exposure.

### Session domain

- **`Session`** -- A Claude Code session with UUID primary key, external ID, user ID, optional `CustomTitle`/`Summary`/`FirstUserMessage`, and session type.
- **`Run`** -- A single execution/resumption of a session with transcript path, working directory, reason, end timestamp, and S3 upload flag.
- **`File`** -- A session file (transcript, agent sidechain, or todo) with optional S3 key and upload timestamp.

### API types

- **`SaveSessionRequest`** -- API request for saving a session, including files as `FileUpload` entries, optional summary/first user message/session type, and a required `LastActivity` timestamp.
- **`FileUpload`** -- A file to upload with path, type, size, and content (`[]byte`, base64-encoded in JSON).
- **`SaveSessionResponse`** -- API response with success flag, session UUID, external ID, run ID, session URL, and optional message.

### GitHub integration

- **`GitHubLinkType`** -- String enum: `"commit"` or `"pull_request"`.
- **`GitHubLinkSource`** -- String enum: `"cli_hook"`, `"manual"`, `"transcript"`.
- **`GitHubLink`** -- A link between a session and a GitHub artifact (commit or PR).
- **`CreateGitHubLinkRequest`** -- API request for creating a GitHub link.

## How to Extend

### Adding a new domain type

1. Define the struct in `models.go` with appropriate JSON tags.
2. Use pointer types for optional fields (`*string`, `*time.Time`).
3. Use `json:"-"` for fields that must never be serialized (e.g., secrets, hashes).
4. If the type has a fixed set of values, define a string type and constants (see `UserStatus`, `OAuthProvider`).

## Invariants

- **No internal imports.** This package depends only on the standard library (`time`). It is a leaf in the dependency graph, importable by any other package without risk of cycles.
- **JSON tags on all serialized fields.** Every struct field that crosses an API boundary has explicit `json` tags, including `omitempty` where appropriate.
- **Sensitive fields excluded from JSON.** `APIKey.KeyHash` and `WebSession.UserEmail`/`UserStatus` use `json:"-"` to prevent accidental exposure in API responses.

## Design Decisions

**Single file for all models.** The domain is small enough that splitting into multiple files would add navigation overhead without improving clarity. Types are grouped by domain area with comments.

**String enums instead of iota.** `UserStatus`, `OAuthProvider`, `GitHubLinkType`, and `GitHubLinkSource` use string constants that match their database values. This avoids the fragility of integer enums and makes database queries and JSON payloads self-documenting.

**Dual ID scheme for sessions.** Sessions have both a UUID `ID` (internal primary key) and an `ExternalID` (the CLI's session identifier). The UUID is used in URLs and database references; the external ID maps back to the originating tool.

## Testing

```bash
go test ./internal/models/...
```

This package has no tests because it contains only type definitions and constants with no logic.

## Dependencies

**Uses:** (standard library only: `time`)

**Used by:** `internal/admin`, `internal/api`, `internal/auth`, `internal/db/*`, `internal/analytics`, `internal/testutil`
