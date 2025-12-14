-- Share system cleanup
-- Design: user_id is source of truth for identity/auth; email is for discovery/growth
--
-- Share target model: consistent tables for each target type
--   session_share_recipients - individual users
--   session_share_public - world (anyone with link)
--
-- Teams support will be added in a future migration.

-- ============================================================================
-- Clean up existing share system
-- ============================================================================

-- Drop session_share_accesses - no longer tracking individual accesses
DROP INDEX IF EXISTS idx_session_share_accesses_user;
DROP INDEX IF EXISTS idx_session_share_accesses_share;
DROP TABLE IF EXISTS session_share_accesses;

-- Rename session_share_invites to reflect true purpose (recipients, not invites)
ALTER TABLE session_share_invites RENAME TO session_share_recipients;

-- Add user_id for resolved recipients (NULL = pending, awaiting signup)
ALTER TABLE session_share_recipients
    ADD COLUMN user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;

-- Backfill user_id for existing recipients where email matches a user
UPDATE session_share_recipients ssr
SET user_id = u.id
FROM users u
WHERE LOWER(ssr.email) = LOWER(u.email);

-- Index for access checks (by user_id)
CREATE INDEX idx_session_share_recipients_user_id ON session_share_recipients(user_id)
    WHERE user_id IS NOT NULL;

-- Index for resolving pending on signup (by email, only pending)
CREATE INDEX idx_session_share_recipients_pending ON session_share_recipients(LOWER(email))
    WHERE user_id IS NULL;

-- Create session_share_public table (existence of row = public share)
CREATE TABLE IF NOT EXISTS session_share_public (
    id BIGSERIAL PRIMARY KEY,
    share_id BIGINT NOT NULL UNIQUE REFERENCES session_shares(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_share_public_share_id ON session_share_public(share_id);

-- Migrate existing public shares to session_share_public
INSERT INTO session_share_public (share_id, created_at)
SELECT id, created_at FROM session_shares WHERE visibility = 'public';

-- Now safe to drop visibility column
ALTER TABLE session_shares DROP COLUMN visibility;
