-- TILs (Today I Learned) table
-- Stores user-created learning notes linked to session transcript positions
CREATE TABLE IF NOT EXISTS tils (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    summary TEXT NOT NULL,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_uuid TEXT,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for listing TILs by owner (most common query)
CREATE INDEX idx_tils_owner_created ON tils(owner_id, created_at DESC);

-- Index for querying TILs by session (transcript viewer markers)
CREATE INDEX idx_tils_session ON tils(session_id);

-- Full-text search index on title and summary
CREATE INDEX idx_tils_search ON tils USING GIN(to_tsvector('english', title || ' ' || summary));
