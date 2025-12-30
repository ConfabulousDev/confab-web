-- Code Activity card: tracks file operations from Read/Write/Edit/Glob/Grep tools
CREATE TABLE session_card_code_activity (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    -- File operation counts
    files_read INT NOT NULL DEFAULT 0,
    files_modified INT NOT NULL DEFAULT 0,
    lines_added INT NOT NULL DEFAULT 0,
    lines_removed INT NOT NULL DEFAULT 0,
    search_count INT NOT NULL DEFAULT 0,

    -- Language breakdown (extension -> count)
    language_breakdown JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_code_activity_version ON session_card_code_activity(version);
