-- Smart Recap quota tracking per user
-- Limits the number of AI recap generations per month
CREATE TABLE smart_recap_quota (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    compute_count INT NOT NULL DEFAULT 0,
    last_compute_at TIMESTAMPTZ,
    quota_reset_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE smart_recap_quota IS 'Monthly quota tracking for AI recap generation';
COMMENT ON COLUMN smart_recap_quota.compute_count IS 'Number of recaps generated this month';
COMMENT ON COLUMN smart_recap_quota.last_compute_at IS 'Timestamp of last recap generation';
COMMENT ON COLUMN smart_recap_quota.quota_reset_at IS 'Timestamp when quota was last reset';
