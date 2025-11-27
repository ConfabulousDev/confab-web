# Sync API Migration Plan - Frontend & Backend

## Overview

Fully migrate from legacy `runs`/`files` model to sync-only model. Remove all legacy code.

## Current State

### Legacy Model (to be removed)
- **DB Tables**: `runs`, `files`
- **S3 Structure**: `{user_id}/claude-code/{external_id}/{run_id}/file.jsonl`
- **API**:
  - `GET /api/v1/sessions/{id}` returns `{ runs: [{ files: [...] }] }`
  - `GET /api/v1/runs/{runId}/files/{fileId}/content`
- **Frontend**: Uses `run.id` + `file.id` to fetch content

### Sync Model (target)
- **DB Tables**: `sessions`, `sync_files` (extended)
- **S3 Structure**: `{user_id}/claude-code/{external_id}/chunks/{file_name}/chunk_*.jsonl`
- **API**:
  - `GET /api/v1/sessions/{id}` returns `{ files: [...] }` (no runs)
  - `GET /api/v1/sync/file?session_id=...&file_name=...`
- **Frontend**: Uses `session_id` + `file_name` to fetch content

## Data Model Changes

### sync_files table (extend)
Current:
```sql
CREATE TABLE sync_files (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    last_synced_line INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(session_id, file_name)
);
```

Add columns to sync_files OR sessions table:
- `cwd TEXT` - working directory
- `transcript_path TEXT` - original transcript path
- `git_info JSONB` - git metadata
- `reason TEXT` - end reason (optional for sync, may not apply)

**Decision**: Store `cwd`, `transcript_path` on sessions table (already exists for transcript_path via sync/init). Store `git_info` on sessions table too since it's session-level metadata.

### sessions table (extend)
Add:
```sql
ALTER TABLE sessions ADD COLUMN cwd TEXT;
ALTER TABLE sessions ADD COLUMN git_info JSONB;
```

## API Changes

### Session List (`GET /api/v1/sessions`)

**Current Response** (per session):
```json
{
  "id": "uuid",
  "external_id": "...",
  "run_count": 3,
  "last_run_time": "...",
  "max_transcript_size": 12345,
  ...
}
```

**New Response** (per session):
```json
{
  "id": "uuid",
  "external_id": "...",
  "file_count": 2,
  "last_sync_time": "...",
  "total_lines": 5000,
  ...
}
```

Changes:
- `run_count` → `file_count` (from sync_files)
- `last_run_time` → `last_sync_time` (from sync_files.updated_at)
- `max_transcript_size` → `total_lines` (sum of last_synced_line)

### Session Detail (`GET /api/v1/sessions/{id}`)

**Current Response**:
```json
{
  "id": "uuid",
  "external_id": "...",
  "runs": [
    {
      "id": 123,
      "cwd": "/path",
      "git_info": {...},
      "files": [
        {"id": 456, "file_type": "transcript", "file_path": "...", "size_bytes": 1234}
      ]
    }
  ]
}
```

**New Response**:
```json
{
  "id": "uuid",
  "external_id": "...",
  "cwd": "/path",
  "transcript_path": "...",
  "git_info": {...},
  "files": [
    {"file_name": "transcript.jsonl", "file_type": "transcript", "last_synced_line": 5000}
  ]
}
```

Changes:
- Remove `runs` array entirely
- Flatten: `cwd`, `transcript_path`, `git_info` at session level
- `files` array from sync_files (no `id`, use `file_name` as key)

### File Content Read

**Current**: `GET /api/v1/runs/{runId}/files/{fileId}/content`

**New**: `GET /api/v1/sync/file?session_id={uuid}&file_name={name}`
(Already exists!)

### Shared File Content Read

**Current**: `GET /api/v1/sessions/{id}/shared/{token}/files/{fileId}/content`

**New**: `GET /api/v1/sessions/{id}/shared/{token}/sync/file?file_name={name}`
(Need to add)

### Delete Session

**Current**: Already handles sync chunks cleanup via `DeleteAllSessionChunks`
**New**: Remove runs/files cleanup code (no longer needed)

## Backend Implementation

### 1. Database Migration
```sql
-- Add session-level metadata
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cwd TEXT;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS git_info JSONB;

-- Update sync/init to store these fields
```

### 2. Update sync/init handler
- Accept `git_info` in request
- Store `cwd`, `git_info` on session record

### 3. Update ListUserSessions query
- Query sync_files instead of runs/files
- Calculate file_count, last_sync_time, total_lines from sync_files

### 4. Update GetSessionDetail query
- Return session with `cwd`, `git_info`, `transcript_path`
- Return sync_files as `files` array

### 5. Add shared sync file endpoint
- `GET /api/v1/sessions/{id}/shared/{token}/sync/file?file_name={name}`
- Verify share access, then read sync chunks

### 6. Clean up
- Remove `handleSaveSession` (legacy upload)
- Remove `HandleGetFileContent` (legacy file read)
- Remove runs/files related DB queries
- Keep runs/files tables for now (separate cleanup migration)

## Frontend Implementation

### 1. Update Types (`src/schemas/api.ts`)

Remove:
- `RunDetailSchema`
- `FileDetailSchema`

Add:
```typescript
export const SyncFileSchema = z.object({
  file_name: z.string(),
  file_type: z.string(),
  last_synced_line: z.number(),
});

export const SessionDetailSchema = z.object({
  id: z.string(),
  external_id: z.string(),
  first_seen: z.string(),
  cwd: z.string().optional(),
  transcript_path: z.string().optional(),
  git_info: GitInfoSchema.optional(),
  files: z.array(SyncFileSchema),
});
```

### 2. Update API Client (`src/services/api.ts`)

Add sync file read method:
```typescript
export const syncAPI = {
  getFileContent: (sessionId: string, fileName: string): Promise<string> =>
    api.getString(`/sync/file?session_id=${sessionId}&file_name=${encodeURIComponent(fileName)}`),

  getSharedFileContent: (sessionId: string, shareToken: string, fileName: string): Promise<string> =>
    api.getString(`/sessions/${sessionId}/shared/${shareToken}/sync/file?file_name=${encodeURIComponent(fileName)}`),
};
```

### 3. Update TranscriptService (`src/services/transcriptService.ts`)

Change `fetchTranscriptContent`:
```typescript
export async function fetchTranscriptContent(
  sessionId: string,
  fileName: string,
  options?: { shareToken?: string }
): Promise<string> {
  let url: string;
  if (options?.shareToken) {
    url = `/api/v1/sessions/${sessionId}/shared/${options.shareToken}/sync/file?file_name=${encodeURIComponent(fileName)}`;
  } else {
    url = `/api/v1/sync/file?session_id=${sessionId}&file_name=${encodeURIComponent(fileName)}`;
  }
  // ... fetch
}
```

### 4. Rename RunCard → SessionCard

- Props change from `run: RunDetail` to session-level props
- Remove run-specific fields
- Update file iteration to use `file_name` instead of `file.id`

### 5. Update SessionDetailPage

- Remove `runs` array handling
- Display session-level metadata directly
- Pass session data to SessionCard

### 6. Update TranscriptViewer

- Props change: `{ sessionId, fileName, shareToken? }` instead of `{ run, ... }`
- Use `fileName` for file lookup instead of `file.id`

### 7. Update useTodos hook

- Change from `run.files` to session `files`
- Use sync file read endpoint with `file_name`

## Migration Order

1. **Backend**: Add columns to sessions table (non-breaking)
2. **Backend**: Update sync/init to store metadata
3. **Backend**: Add shared sync file endpoint
4. **Backend**: Update session list query
5. **Backend**: Update session detail query
6. **Frontend**: Update types/schemas
7. **Frontend**: Update API client
8. **Frontend**: Update components (SessionDetailPage, RunCard→SessionCard, TranscriptViewer, useTodos)
9. **Backend**: Remove legacy handlers/queries (breaking change)
10. **Cleanup**: Database migration to drop runs/files tables (future)

## Testing

- Backend unit tests for new queries
- Backend integration tests for sync endpoints
- Frontend build/lint/test
- Manual E2E: list sessions, view session, view transcript, share session, delete session
