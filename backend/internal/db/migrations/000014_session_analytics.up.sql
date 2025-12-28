-- Session analytics cache table
-- Stores computed metrics from session JSONL files with caching
CREATE TABLE session_analytics (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    analytics_version INT NOT NULL DEFAULT 1,
    up_to_line BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL,

    -- Token stats (extracted for future aggregation)
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,

    -- Cost (extracted for future aggregation)
    estimated_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0,

    -- Compaction stats (extracted)
    compaction_auto INT NOT NULL DEFAULT 0,
    compaction_manual INT NOT NULL DEFAULT 0,
    compaction_avg_time_ms INT,

    -- Flexible JSONB for future expansion
    details JSONB NOT NULL DEFAULT '{}'
);

COMMENT ON TABLE session_analytics IS 'Cached computed metrics from session JSONL files';
COMMENT ON COLUMN session_analytics.analytics_version IS 'Version of analytics computation logic (for cache busting when logic changes)';
COMMENT ON COLUMN session_analytics.up_to_line IS 'JSONL line count when analytics were computed (for staleness detection)';
COMMENT ON COLUMN session_analytics.details IS 'Flexible JSONB for additional metrics without schema changes';
