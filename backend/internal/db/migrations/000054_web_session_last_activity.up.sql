-- 60j6 (CF-425 audit A5): web session idle timeout / sliding window.
-- Add last_activity_at as a sliding gate alongside the absolute expires_at cap.
-- Nullable, no DB default (app convention: app-managed columns get no DB default);
-- the idle predicate uses COALESCE(last_activity_at, created_at) for rollout-gap
-- safety so old code that keeps INSERTing rows without the column stays valid.
ALTER TABLE web_sessions ADD COLUMN last_activity_at TIMESTAMP;

-- One-shot backfill: treat existing rows as last active at creation.
UPDATE web_sessions SET last_activity_at = created_at WHERE last_activity_at IS NULL;
