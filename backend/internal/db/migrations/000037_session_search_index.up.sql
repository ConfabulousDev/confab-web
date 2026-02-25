CREATE TABLE session_search_index (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL,
    content_text TEXT NOT NULL,
    search_vector TSVECTOR NOT NULL,
    indexed_up_to_line BIGINT NOT NULL,
    recap_indexed_at TIMESTAMPTZ,
    metadata_hash TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_search_vector ON session_search_index USING GIN (search_vector);
