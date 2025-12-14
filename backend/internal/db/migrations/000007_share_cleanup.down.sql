-- Reverse the share system cleanup

-- Re-add visibility column
ALTER TABLE session_shares ADD COLUMN visibility VARCHAR(20) NOT NULL DEFAULT 'private';

-- Restore visibility from session_share_public
UPDATE session_shares ss
SET visibility = 'public'
FROM session_share_public ssp
WHERE ss.id = ssp.share_id;

-- Drop session_share_public
DROP INDEX IF EXISTS idx_session_share_public_share_id;
DROP TABLE IF EXISTS session_share_public;

-- Remove user_id and indexes from session_share_recipients
DROP INDEX IF EXISTS idx_session_share_recipients_pending;
DROP INDEX IF EXISTS idx_session_share_recipients_user_id;
ALTER TABLE session_share_recipients DROP COLUMN IF EXISTS user_id;

-- Rename back to session_share_invites
ALTER TABLE session_share_recipients RENAME TO session_share_invites;

-- Recreate session_share_accesses table
CREATE TABLE IF NOT EXISTS session_share_accesses (
    id BIGSERIAL PRIMARY KEY,
    share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    first_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    access_count INT NOT NULL DEFAULT 1,
    UNIQUE(share_id, user_id)
);

CREATE INDEX idx_session_share_accesses_share ON session_share_accesses(share_id);
CREATE INDEX idx_session_share_accesses_user ON session_share_accesses(user_id);
