-- Initial schema for Confab backend
-- Flattened from migrations 000001-000005

-- Users table (OAuth-based authentication)
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255),
    avatar_url TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- User identities table (OAuth provider links)
-- Each user can have multiple OAuth identities (GitHub, Google, etc.)
CREATE TABLE IF NOT EXISTS user_identities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    provider_username VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_id)
);

-- Web sessions table (for browser authentication via OAuth)
CREATE TABLE IF NOT EXISTS web_sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash CHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMP
);

-- Sessions table (globally unique UUID for URLs, user_id+session_type+external_id for deduplication)
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    first_seen TIMESTAMP NOT NULL DEFAULT NOW(),
    title TEXT,
    session_type VARCHAR(50) NOT NULL DEFAULT 'Claude Code',
    cwd TEXT,
    transcript_path TEXT,
    git_info JSONB,
    last_sync_at TIMESTAMP,
    last_message_at TIMESTAMP,
    UNIQUE(user_id, session_type, external_id)
);

-- Runs table (execution instances)
CREATE TABLE IF NOT EXISTS runs (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    transcript_path TEXT NOT NULL,
    cwd TEXT NOT NULL,
    reason TEXT NOT NULL,
    end_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    s3_uploaded BOOLEAN NOT NULL DEFAULT FALSE,
    git_info JSONB,
    source VARCHAR(50) NOT NULL DEFAULT 'hook',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_activity TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Files table
CREATE TABLE IF NOT EXISTS files (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    size_bytes BIGINT NOT NULL,
    s3_key TEXT,
    s3_uploaded_at TIMESTAMP
);

-- Session shares table
CREATE TABLE IF NOT EXISTS session_shares (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    share_token CHAR(32) NOT NULL UNIQUE,
    visibility VARCHAR(20) NOT NULL,
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMP
);

-- Session share invites table (for private shares)
CREATE TABLE IF NOT EXISTS session_share_invites (
    id BIGSERIAL PRIMARY KEY,
    share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(share_id, email)
);

-- Session share accesses table (tracks which users accessed which shares)
CREATE TABLE IF NOT EXISTS session_share_accesses (
    id BIGSERIAL PRIMARY KEY,
    share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    first_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    access_count INT NOT NULL DEFAULT 1,
    UNIQUE(share_id, user_id)
);

-- Device codes table (for CLI device authorization flow)
CREATE TABLE IF NOT EXISTS device_codes (
    id BIGSERIAL PRIMARY KEY,
    device_code CHAR(64) NOT NULL UNIQUE,
    user_code CHAR(9) NOT NULL UNIQUE,
    key_name VARCHAR(255) NOT NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    authorized_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Track emails that have ever been invited to a private share
-- Used for login authorization via ALLOW_INVITED_EMAILS_AFTER_TS env var
CREATE TABLE IF NOT EXISTS invited_emails (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    first_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_invited_at TIMESTAMP NOT NULL DEFAULT NOW(),
    invite_count INT NOT NULL DEFAULT 1,
    UNIQUE(email)
);

-- Track sync state per file (high-water mark for incremental sync)
CREATE TABLE IF NOT EXISTS sync_files (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    last_synced_line INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(session_id, file_name)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_device_codes_device_code ON device_codes(device_code);
CREATE INDEX IF NOT EXISTS idx_device_codes_user_code ON device_codes(user_code);
CREATE INDEX IF NOT EXISTS idx_device_codes_expires ON device_codes(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_identities_user ON user_identities(user_id);
CREATE INDEX IF NOT EXISTS idx_user_identities_provider ON user_identities(provider, provider_id);
CREATE INDEX IF NOT EXISTS idx_web_sessions_user ON web_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_external_id ON sessions(user_id, session_type, external_id);
CREATE INDEX IF NOT EXISTS idx_sessions_last_message ON sessions(last_message_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_runs_session_id ON runs(session_id);
CREATE INDEX IF NOT EXISTS idx_runs_end_timestamp ON runs(end_timestamp);
CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at);
CREATE INDEX IF NOT EXISTS idx_runs_last_activity ON runs(last_activity);
CREATE INDEX IF NOT EXISTS idx_files_run ON files(run_id);
CREATE INDEX IF NOT EXISTS idx_files_run_type_size ON files(run_id, file_type, size_bytes);
CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token);
CREATE INDEX IF NOT EXISTS idx_session_shares_session_id ON session_shares(session_id);
CREATE INDEX IF NOT EXISTS idx_session_share_invites_share ON session_share_invites(share_id);
CREATE INDEX IF NOT EXISTS idx_session_share_invites_email ON session_share_invites(email);
CREATE INDEX IF NOT EXISTS idx_session_share_accesses_share ON session_share_accesses(share_id);
CREATE INDEX IF NOT EXISTS idx_session_share_accesses_user ON session_share_accesses(user_id);
CREATE INDEX IF NOT EXISTS idx_invited_emails_email ON invited_emails(LOWER(email));
CREATE INDEX IF NOT EXISTS idx_sync_files_session ON sync_files(session_id);
