-- Add index to support trends queries
-- This index optimizes queries filtering sessions by user_id with ordering by first_seen
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_first_seen
    ON sessions(user_id, first_seen DESC);
