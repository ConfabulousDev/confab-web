# Plan: Consistent Metadata Nesting in Sync APIs

## Problem

The sync APIs have inconsistent structure for passing session metadata:

**Current Init API** (`POST /api/v1/sync/init`):
```json
{
  "external_id": "session-uuid",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/working/directory",
  "git_info": { "branch": "main", ... }
}
```

**Current Chunk API** (`POST /api/v1/sync/chunk`):
```json
{
  "session_id": "uuid",
  "file_name": "transcript.jsonl",
  "file_type": "transcript",
  "first_line": 1,
  "lines": ["..."],
  "metadata": {
    "git_info": { "branch": "main", ... },
    "summary": "...",
    "first_user_message": "..."
  }
}
```

The chunk API nests `git_info` under `metadata`, but init has it at the top level alongside `cwd`. This is inconsistent and makes the API harder to reason about.

## Proposed Change

Add a `metadata` wrapper to the init API to match chunk:

**New Init API**:
```json
{
  "external_id": "session-uuid",
  "transcript_path": "/path/to/transcript.jsonl",
  "metadata": {
    "cwd": "/working/directory",
    "git_info": { "branch": "main", ... }
  }
}
```

This groups session metadata consistently across both endpoints.

## Files to Change

### Backend (`confab-web/backend`)

1. **`internal/api/sync.go`**
   - Add `SyncInitMetadata` struct
   - Update `SyncInitRequest` to use nested metadata (keep old fields for backward compat)
   - Update `handleSyncInit` to read from either location

2. **`internal/api/sync_integration_test.go`**
   - Update tests to use new format
   - Add tests for backward compatibility with old format

3. **`API.md`**
   - Update sync/init documentation

### CLI (`confab` repo - separate PR)

4. **`pkg/sync/client.go`**
   - Add `InitMetadata` struct
   - Update `InitRequest` to use nested metadata

5. **`backend-api.md`**
   - Update documentation to match

## Implementation Details

### New Structs (sync.go)

```go
// SyncInitMetadata contains optional metadata for session initialization
type SyncInitMetadata struct {
    CWD     string          `json:"cwd,omitempty"`
    GitInfo json.RawMessage `json:"git_info,omitempty"`
}

// SyncInitRequest is the request body for POST /api/v1/sync/init
type SyncInitRequest struct {
    ExternalID     string            `json:"external_id"`
    TranscriptPath string            `json:"transcript_path"`
    Metadata       *SyncInitMetadata `json:"metadata,omitempty"`

    // Deprecated: Use Metadata.CWD instead. Kept for backward compatibility.
    CWD     string          `json:"cwd,omitempty"`
    // Deprecated: Use Metadata.GitInfo instead. Kept for backward compatibility.
    GitInfo json.RawMessage `json:"git_info,omitempty"`
}
```

### Handler Logic (handleSyncInit)

```go
// Extract metadata, preferring new nested format over deprecated top-level fields
cwd := req.CWD
gitInfo := req.GitInfo
if req.Metadata != nil {
    if req.Metadata.CWD != "" {
        cwd = req.Metadata.CWD
    }
    if req.Metadata.GitInfo != nil {
        gitInfo = req.Metadata.GitInfo
    }
}
```

### Validation

- Validate `cwd` length regardless of which field it comes from
- If both old and new fields are provided, new format takes precedence (no error)

## Backward Compatibility

The backend must accept both formats during the transition period:

| Client Version | Format Used | Backend Behavior |
|----------------|-------------|------------------|
| Old CLI | Top-level `cwd`, `git_info` | Works (reads from top-level) |
| New CLI | Nested `metadata.cwd`, `metadata.git_info` | Works (reads from metadata) |

After all CLI clients are updated, we can consider deprecation warnings or eventual removal of the top-level fields.

## Testing Checklist

- [ ] New format works: `metadata.cwd` and `metadata.git_info` are read correctly
- [ ] Old format still works: top-level `cwd` and `git_info` are read correctly
- [ ] Mixed format: `metadata` takes precedence over top-level fields
- [ ] Empty metadata: `metadata: {}` doesn't break anything
- [ ] Null metadata: `metadata: null` falls back to top-level fields
- [ ] Validation: `cwd` length limits apply regardless of field location

## Rollout Plan

1. **Backend first**: Deploy backend with backward-compatible changes
2. **CLI second**: Update CLI to use new format (separate PR in confab repo)
3. **Documentation**: Update API.md and backend-api.md

## Out of Scope

- Removing the deprecated top-level fields (future cleanup task)
- Changing the chunk API (already correct)
- Changing response formats (only request format changes)
