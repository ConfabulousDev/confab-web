-- nbrd: marker for the session_title invalidation target. POST
-- /admin/cards/invalidate sets it on title candidates (Codex with NULL
-- first_user_message, Cursor titles still wrapped in <user_query>); the
-- precompute worker re-derives first_user_message from stored data and clears it.
ALTER TABLE sessions ADD COLUMN title_recompute_requested_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sessions_title_recompute_requested_at
  ON sessions (title_recompute_requested_at)
  WHERE title_recompute_requested_at IS NOT NULL;
