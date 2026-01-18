-- Smart Recap card table (time-based invalidation due to LLM cost)
-- Stores AI-generated session analysis from Claude Haiku
CREATE TABLE session_card_smart_recap (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    -- LLM-generated content
    recap TEXT NOT NULL,
    went_well JSONB NOT NULL DEFAULT '[]',
    went_bad JSONB NOT NULL DEFAULT '[]',
    human_suggestions JSONB NOT NULL DEFAULT '[]',
    environment_suggestions JSONB NOT NULL DEFAULT '[]',
    default_context_suggestions JSONB NOT NULL DEFAULT '[]',

    -- LLM metadata
    model_used VARCHAR(100) NOT NULL,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    generation_time_ms INT,

    -- Race prevention (optimistic lock)
    computing_started_at TIMESTAMPTZ
);

CREATE INDEX idx_session_card_smart_recap_version ON session_card_smart_recap(version);

COMMENT ON TABLE session_card_smart_recap IS 'Cached AI-generated session recaps';
COMMENT ON COLUMN session_card_smart_recap.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_smart_recap.up_to_line IS 'JSONL line count when computed';
COMMENT ON COLUMN session_card_smart_recap.recap IS 'Short 2-3 sentence recap of the session';
COMMENT ON COLUMN session_card_smart_recap.went_well IS 'JSON array of things that went well (max 3)';
COMMENT ON COLUMN session_card_smart_recap.went_bad IS 'JSON array of things that did not go well (max 3)';
COMMENT ON COLUMN session_card_smart_recap.human_suggestions IS 'JSON array of human technique improvements (max 3)';
COMMENT ON COLUMN session_card_smart_recap.environment_suggestions IS 'JSON array of environment improvements (max 3)';
COMMENT ON COLUMN session_card_smart_recap.default_context_suggestions IS 'JSON array of CLAUDE.md/system context improvements (max 3)';
COMMENT ON COLUMN session_card_smart_recap.model_used IS 'LLM model identifier used for generation';
COMMENT ON COLUMN session_card_smart_recap.input_tokens IS 'Number of input tokens used';
COMMENT ON COLUMN session_card_smart_recap.output_tokens IS 'Number of output tokens generated';
COMMENT ON COLUMN session_card_smart_recap.generation_time_ms IS 'Time taken to generate the recap in milliseconds';
COMMENT ON COLUMN session_card_smart_recap.computing_started_at IS 'Lock timestamp to prevent concurrent generation';
