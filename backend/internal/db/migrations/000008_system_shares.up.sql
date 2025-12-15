-- System shares: shares visible to all authenticated users
-- Used by admins to share sessions with the entire user base (including future users)

CREATE TABLE IF NOT EXISTS session_share_system (
    id BIGSERIAL PRIMARY KEY,
    share_id BIGINT NOT NULL UNIQUE REFERENCES session_shares(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_share_system_share_id ON session_share_system(share_id);
