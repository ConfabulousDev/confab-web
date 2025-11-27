-- Track sync state per file (high-water mark for incremental sync)
CREATE TABLE sync_files (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL,              -- e.g., "transcript.jsonl", "agent-abc123.jsonl"
    file_type VARCHAR(50) NOT NULL,       -- "transcript", "agent", "todo"
    last_synced_line INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(session_id, file_name)
);

-- Index for fast lookup by session
CREATE INDEX idx_sync_files_session ON sync_files(session_id);
