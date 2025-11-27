-- Add session-level metadata for sync-only model
-- Previously stored per-run, now stored at session level

-- Working directory for the session
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS cwd TEXT;

-- Original transcript path
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS transcript_path TEXT;

-- Git information (repo, branch, commit, etc.)
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS git_info JSONB;

-- Last sync timestamp (updated when any file is synced)
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMP;

-- Last message timestamp (extracted from transcript JSONL during sync)
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMP;

-- Index for sorting by last message (activity)
CREATE INDEX IF NOT EXISTS idx_sessions_last_message ON sessions(last_message_at DESC NULLS LAST);
