-- Add password authentication support
-- Password is an identity provider, like GitHub/Google
-- user_identities row: provider='password', provider_id=email
-- Linked table stores password-specific data (hash, lockout)

CREATE TABLE identity_passwords (
    id BIGSERIAL PRIMARY KEY,
    identity_id BIGINT NOT NULL UNIQUE REFERENCES user_identities(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    failed_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Admin flag on users table
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for looking up password credentials by identity
CREATE INDEX idx_identity_passwords_identity ON identity_passwords(identity_id);
