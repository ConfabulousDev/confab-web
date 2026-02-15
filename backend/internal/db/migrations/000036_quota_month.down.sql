ALTER TABLE smart_recap_quota ADD COLUMN quota_reset_at TIMESTAMPTZ;
UPDATE smart_recap_quota SET quota_reset_at = (quota_month || '-01')::DATE;
ALTER TABLE smart_recap_quota DROP COLUMN quota_month;
