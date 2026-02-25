CREATE TABLE session_search_index (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    content_text TEXT NOT NULL DEFAULT '',
    search_vector TSVECTOR NOT NULL,
    indexed_up_to_line BIGINT NOT NULL DEFAULT 0,
    recap_indexed_at TIMESTAMPTZ,
    metadata_hash TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_search_vector ON session_search_index USING GIN (search_vector);
