-- Remove the unique constraint on (user_id, name)
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_user_id_name_unique;
