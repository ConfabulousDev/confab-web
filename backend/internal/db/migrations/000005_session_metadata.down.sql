-- Revert session metadata columns
DROP INDEX IF EXISTS idx_sessions_last_message;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_message_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_sync_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS git_info;
ALTER TABLE sessions DROP COLUMN IF EXISTS transcript_path;
ALTER TABLE sessions DROP COLUMN IF EXISTS cwd;
