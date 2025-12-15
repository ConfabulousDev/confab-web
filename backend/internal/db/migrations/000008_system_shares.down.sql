-- Revert system shares

DROP INDEX IF EXISTS idx_session_share_system_share_id;
DROP TABLE IF EXISTS session_share_system;
