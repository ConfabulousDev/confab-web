-- Recreate invited_emails table (for rollback)
CREATE TABLE IF NOT EXISTS invited_emails (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    first_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    invite_count INT NOT NULL DEFAULT 1,
    UNIQUE(email)
);

CREATE INDEX IF NOT EXISTS idx_invited_emails_email ON invited_emails(LOWER(email));
