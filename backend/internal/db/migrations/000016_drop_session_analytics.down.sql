-- Recreate session_analytics table for rollback
-- Note: Data will need to be recomputed after rollback

CREATE TABLE session_analytics (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    analytics_version INT NOT NULL DEFAULT 1,
    up_to_line BIGINT NOT NULL DEFAULT 0,
    computed_at TIMESTAMPTZ NOT NULL,

    -- Token stats
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,

    -- Cost
    estimated_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0,

    -- Compaction stats
    compaction_auto INT NOT NULL DEFAULT 0,
    compaction_manual INT NOT NULL DEFAULT 0,
    compaction_avg_time_ms INT,

    -- Flexible JSONB for future expansion
    details JSONB NOT NULL DEFAULT '{}'
);

COMMENT ON TABLE session_analytics IS 'Cached computed metrics from session JSONL files';
COMMENT ON COLUMN session_analytics.analytics_version IS 'Version of analytics computation logic';
COMMENT ON COLUMN session_analytics.up_to_line IS 'JSONL line count when analytics were computed';
COMMENT ON COLUMN session_analytics.details IS 'Flexible JSONB for additional metrics';

-- Migrate data back from card tables
INSERT INTO session_analytics (
    session_id, analytics_version, up_to_line, computed_at,
    input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
    estimated_cost_usd,
    compaction_auto, compaction_manual, compaction_avg_time_ms
)
SELECT
    t.session_id, t.version, t.up_to_line, t.computed_at,
    t.input_tokens, t.output_tokens, t.cache_creation_tokens, t.cache_read_tokens,
    c.estimated_cost_usd,
    comp.auto_count, comp.manual_count, comp.avg_time_ms
FROM session_card_tokens t
JOIN session_card_cost c ON c.session_id = t.session_id
JOIN session_card_compaction comp ON comp.session_id = t.session_id;
