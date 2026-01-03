-- Redactions card table (line-based invalidation)
-- Tracks counts of redacted sensitive data by type (e.g., GITHUB_TOKEN, AWS_KEY)
CREATE TABLE session_card_redactions (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    total_redactions INT NOT NULL DEFAULT 0,
    redaction_counts JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_redactions_version ON session_card_redactions(version);

COMMENT ON TABLE session_card_redactions IS 'Cached redaction counts for sessions';
COMMENT ON COLUMN session_card_redactions.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_redactions.up_to_line IS 'JSONL line count when computed';
COMMENT ON COLUMN session_card_redactions.total_redactions IS 'Total number of redactions across all types';
COMMENT ON COLUMN session_card_redactions.redaction_counts IS 'JSON object mapping redaction type to count (e.g., {"GITHUB_TOKEN": 5, "AWS_KEY": 2})';
