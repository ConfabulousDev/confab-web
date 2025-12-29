-- Reverse card consolidation
-- 1. Recreate cost table and migrate data back
-- 2. Recreate compaction table and migrate data back
-- 3. Remove added columns

-- Recreate cost table
CREATE TABLE session_card_cost (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    estimated_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0
);

COMMENT ON TABLE session_card_cost IS 'Cached cost estimates for sessions';

-- Migrate cost data back
INSERT INTO session_card_cost (session_id, version, computed_at, up_to_line, estimated_cost_usd)
SELECT session_id, version, computed_at, up_to_line, estimated_cost_usd
FROM session_card_tokens;

-- Recreate compaction table
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

-- Migrate compaction data back
INSERT INTO session_card_compaction (session_id, version, computed_at, up_to_line, auto_count, manual_count, avg_time_ms)
SELECT session_id, version, computed_at, up_to_line, compaction_auto, compaction_manual, compaction_avg_time_ms
FROM session_card_session;

-- Remove added columns
ALTER TABLE session_card_tokens DROP COLUMN estimated_cost_usd;
ALTER TABLE session_card_session DROP COLUMN compaction_auto;
ALTER TABLE session_card_session DROP COLUMN compaction_manual;
ALTER TABLE session_card_session DROP COLUMN compaction_avg_time_ms;
