-- Consolidate analytics cards:
-- 1. Merge cost into tokens card
-- 2. Merge compaction into session card

-- Add estimated_cost_usd to tokens card
ALTER TABLE session_card_tokens ADD COLUMN estimated_cost_usd DECIMAL(10,4) NOT NULL DEFAULT 0;

-- Migrate cost data from session_card_cost to session_card_tokens
UPDATE session_card_tokens t
SET estimated_cost_usd = c.estimated_cost_usd
FROM session_card_cost c
WHERE t.session_id = c.session_id;

-- Add compaction fields to session card
ALTER TABLE session_card_session ADD COLUMN compaction_auto INT NOT NULL DEFAULT 0;
ALTER TABLE session_card_session ADD COLUMN compaction_manual INT NOT NULL DEFAULT 0;
ALTER TABLE session_card_session ADD COLUMN compaction_avg_time_ms INT;

-- Migrate compaction data from session_card_compaction to session_card_session
UPDATE session_card_session s
SET compaction_auto = c.auto_count,
    compaction_manual = c.manual_count,
    compaction_avg_time_ms = c.avg_time_ms
FROM session_card_compaction c
WHERE s.session_id = c.session_id;

-- Drop the now-redundant tables
DROP TABLE session_card_cost;
DROP TABLE session_card_compaction;

-- Add comments for new columns
COMMENT ON COLUMN session_card_tokens.estimated_cost_usd IS 'Estimated API cost in USD based on token usage';
COMMENT ON COLUMN session_card_session.compaction_auto IS 'Number of auto-triggered compactions';
COMMENT ON COLUMN session_card_session.compaction_manual IS 'Number of manually-triggered compactions';
COMMENT ON COLUMN session_card_session.compaction_avg_time_ms IS 'Average auto-compaction time in milliseconds';
