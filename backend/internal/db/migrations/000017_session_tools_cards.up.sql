-- Session card table (line-based invalidation)
CREATE TABLE session_card_session (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    user_turns INT NOT NULL DEFAULT 0,
    assistant_turns INT NOT NULL DEFAULT 0,
    duration_ms BIGINT,
    models_used JSONB NOT NULL DEFAULT '[]'
);

COMMENT ON TABLE session_card_session IS 'Cached session metrics (turns, duration, models used)';
COMMENT ON COLUMN session_card_session.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_session.up_to_line IS 'JSONL line count when computed';
COMMENT ON COLUMN session_card_session.duration_ms IS 'Session duration in milliseconds (null if not computable)';
COMMENT ON COLUMN session_card_session.models_used IS 'JSON array of unique model IDs used in the session';

-- Tools card table (line-based invalidation)
CREATE TABLE session_card_tools (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    total_calls INT NOT NULL DEFAULT 0,
    tool_breakdown JSONB NOT NULL DEFAULT '{}',
    error_count INT NOT NULL DEFAULT 0
);

COMMENT ON TABLE session_card_tools IS 'Cached tool usage metrics for sessions';
COMMENT ON COLUMN session_card_tools.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_tools.up_to_line IS 'JSONL line count when computed';
COMMENT ON COLUMN session_card_tools.tool_breakdown IS 'JSON object mapping tool names to call counts';
