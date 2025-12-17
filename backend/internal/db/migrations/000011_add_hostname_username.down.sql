-- Remove hostname and username columns from sessions table
ALTER TABLE sessions DROP COLUMN IF EXISTS hostname;
ALTER TABLE sessions DROP COLUMN IF EXISTS username;
