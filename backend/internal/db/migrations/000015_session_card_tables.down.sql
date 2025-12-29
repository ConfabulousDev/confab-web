-- Reverse migration: drop card tables
-- Note: Data will be lost; session_analytics must still exist for rollback to work

DROP TABLE IF EXISTS session_card_compaction;
DROP TABLE IF EXISTS session_card_cost;
DROP TABLE IF EXISTS session_card_tokens;
