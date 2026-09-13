DROP INDEX IF EXISTS idx_sessions_title_recompute_requested_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS title_recompute_requested_at;
