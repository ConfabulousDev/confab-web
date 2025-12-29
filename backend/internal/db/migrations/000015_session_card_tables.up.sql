-- Session card tables (card-per-table architecture)
-- Each card has its own table with independent versioning and invalidation

-- Tokens card (line-based invalidation)
CREATE TABLE session_card_tokens (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE session_card_tokens IS 'Cached token usage metrics for sessions';
COMMENT ON COLUMN session_card_tokens.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_tokens.up_to_line IS 'JSONL line count when computed';

-- Cost card (line-based invalidation)
CREATE TABLE session_card_cost (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    estimated_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0
);

COMMENT ON TABLE session_card_cost IS 'Cached cost estimates for sessions';
COMMENT ON COLUMN session_card_cost.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_cost.up_to_line IS 'JSONL line count when computed';

-- Compaction card (line-based invalidation)
CREATE TABLE session_card_compaction (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    auto_count INT NOT NULL DEFAULT 0,
    manual_count INT NOT NULL DEFAULT 0,
    avg_time_ms INT
);

COMMENT ON TABLE session_card_compaction IS 'Cached compaction statistics for sessions';
COMMENT ON COLUMN session_card_compaction.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_compaction.up_to_line IS 'JSONL line count when computed';

-- Migrate existing data from session_analytics
INSERT INTO session_card_tokens (session_id, version, computed_at, up_to_line, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens)
SELECT session_id, analytics_version, computed_at, up_to_line, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens
FROM session_analytics;

INSERT INTO session_card_cost (session_id, version, computed_at, up_to_line, estimated_cost_usd)
SELECT session_id, analytics_version, computed_at, up_to_line, estimated_cost_usd
FROM session_analytics;

INSERT INTO session_card_compaction (session_id, version, computed_at, up_to_line, auto_count, manual_count, avg_time_ms)
SELECT session_id, analytics_version, computed_at, up_to_line, compaction_auto, compaction_manual, compaction_avg_time_ms
FROM session_analytics;
