-- 40hj: web_sessions.id and device_codes.device_code are now stored as
-- sha256(token) (hex) instead of the raw value, so a read of the database
-- (backup leak, SQLi, replica, support tooling) cannot replay live, high-entropy
-- session/device tokens.
--
-- The lookup-key format changed and is NOT backward compatible across the
-- migrate-then-deploy rollout gap, so delete all existing rows rather than
-- hash them in place: every user re-logs in once, the demo shared session is
-- re-provisioned by bootstrap on next start, and any in-flight device-code
-- flow expires within its short (5-minute) window.
--
-- No column type change: web_sessions.id is VARCHAR(64) and
-- device_codes.device_code is CHAR(64) — both already hold a 64-char hex digest.
DELETE FROM web_sessions;
DELETE FROM device_codes;
