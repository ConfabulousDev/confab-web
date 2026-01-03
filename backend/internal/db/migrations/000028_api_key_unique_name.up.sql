-- First, delete duplicate API keys (keep the most recent one per user/name)
-- Uses a CTE to identify duplicates and delete all but the most recent
DELETE FROM api_keys
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY user_id, name ORDER BY created_at DESC) as rn
        FROM api_keys
    ) ranked
    WHERE rn > 1
);

-- Add unique constraint on (user_id, name)
-- This ensures each user can only have one API key with a given name
ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_name_unique UNIQUE (user_id, name);
