-- Matches the session-list sort key (ORDER BY COALESCE(last_message_at,
-- first_seen) DESC, id DESC) so the share-all list query can walk sessions in
-- order and stop at LIMIT instead of sorting every session.
CREATE INDEX IF NOT EXISTS idx_sessions_list_order
  ON sessions ((COALESCE(last_message_at, first_seen)) DESC, id DESC);
