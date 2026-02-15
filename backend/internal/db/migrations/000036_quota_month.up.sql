ALTER TABLE smart_recap_quota ADD COLUMN quota_month TEXT;

UPDATE smart_recap_quota
SET quota_month = TO_CHAR(
  COALESCE(quota_reset_at, last_compute_at, NOW()) AT TIME ZONE 'UTC',
  'YYYY-MM'
);

ALTER TABLE smart_recap_quota ALTER COLUMN quota_month SET NOT NULL;
ALTER TABLE smart_recap_quota DROP COLUMN quota_reset_at;

COMMENT ON COLUMN smart_recap_quota.quota_month IS 'Month this count applies to (YYYY-MM, UTC)';
