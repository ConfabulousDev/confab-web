-- Add last_used_at column to track when API keys were last used
ALTER TABLE api_keys ADD COLUMN last_used_at TIMESTAMP;
