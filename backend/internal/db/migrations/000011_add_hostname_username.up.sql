-- Add hostname and username columns to sessions table
-- These capture the client machine context when a session is created/synced

ALTER TABLE sessions ADD COLUMN hostname VARCHAR(255);
ALTER TABLE sessions ADD COLUMN username VARCHAR(255);
