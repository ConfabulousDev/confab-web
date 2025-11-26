-- Track emails that have ever been invited to a private share
-- This table persists invitation history independent of share lifecycle
-- Used for login authorization via ALLOW_INVITED_EMAILS_AFTER_TS env var
CREATE TABLE IF NOT EXISTS invited_emails (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    first_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    invite_count INT NOT NULL DEFAULT 1,
    UNIQUE(email)
);

-- Index for fast lookup during OAuth (case-insensitive)
CREATE INDEX IF NOT EXISTS idx_invited_emails_email ON invited_emails(LOWER(email));

-- Backfill from existing invites
INSERT INTO invited_emails (email, first_invited_at, last_invited_at, invite_count)
SELECT
    LOWER(email) as email,
    MIN(created_at) as first_invited_at,
    MAX(created_at) as last_invited_at,
    COUNT(*) as invite_count
FROM session_share_invites
GROUP BY LOWER(email)
ON CONFLICT (email) DO NOTHING;
