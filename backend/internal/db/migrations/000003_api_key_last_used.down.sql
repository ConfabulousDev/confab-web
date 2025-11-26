-- Remove last_used_at column from api_keys
ALTER TABLE api_keys DROP COLUMN last_used_at;
