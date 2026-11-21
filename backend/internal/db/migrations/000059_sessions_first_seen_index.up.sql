-- c3ts: support the admin Session Insights tab, which range-scans every session
-- on the instance by first_seen (no user_id prefix, so idx_sessions_user_first_seen
-- from 000033 can't serve it).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_first_seen
    ON sessions(first_seen DESC);
