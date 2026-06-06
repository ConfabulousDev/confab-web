-- Tokens v2 card table (hierarchical per-provider per-model breakdown)
-- Used by OpenCode sessions which may use models from multiple LLM providers.
-- Stores a nested JSON tree: total cost → by_provider → models → token breakdown.
-- Coexists with session_card_tokens (flat format) for backward compatibility.
CREATE TABLE session_card_tokens_v2 (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    data JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_tokens_v2_version ON session_card_tokens_v2(version);

COMMENT ON TABLE session_card_tokens_v2 IS 'Hierarchical token usage breakdown by provider and model (OpenCode)';
COMMENT ON COLUMN session_card_tokens_v2.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_tokens_v2.up_to_line IS 'JSONL line count when computed';
COMMENT ON COLUMN session_card_tokens_v2.data IS 'Nested JSON: {total_cost_usd, total_input, total_output, by_provider: {<provider>: {cost_usd, models: {<model>: {input, output, cache_read, cache_write, reasoning, cost_usd}}}}}';
