-- Restore TILs table (undo feature removal)
CREATE TABLE IF NOT EXISTS tils (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(500) NOT NULL,
    summary TEXT NOT NULL,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_uuid TEXT,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tils_owner_created ON tils(owner_id, created_at DESC);
CREATE INDEX idx_tils_session ON tils(session_id);
CREATE INDEX idx_tils_search ON tils USING GIN(to_tsvector('english', title || ' ' || summary));
