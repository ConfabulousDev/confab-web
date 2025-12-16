-- Convert TEXT columns to VARCHAR with explicit size limits
-- This provides bounded storage and enables consistent validation

-- users
ALTER TABLE users ALTER COLUMN avatar_url TYPE VARCHAR(4096);

-- sessions
ALTER TABLE sessions ALTER COLUMN external_id TYPE VARCHAR(512);
ALTER TABLE sessions ALTER COLUMN summary TYPE VARCHAR(2048);
ALTER TABLE sessions ALTER COLUMN first_user_message TYPE VARCHAR(8192);
ALTER TABLE sessions ALTER COLUMN cwd TYPE VARCHAR(8192);
ALTER TABLE sessions ALTER COLUMN transcript_path TYPE VARCHAR(8192);

-- runs
ALTER TABLE runs ALTER COLUMN transcript_path TYPE VARCHAR(8192);
ALTER TABLE runs ALTER COLUMN cwd TYPE VARCHAR(8192);
ALTER TABLE runs ALTER COLUMN reason TYPE VARCHAR(2048);

-- files
ALTER TABLE files ALTER COLUMN file_path TYPE VARCHAR(8192);
ALTER TABLE files ALTER COLUMN s3_key TYPE VARCHAR(2048);

-- sync_files
ALTER TABLE sync_files ALTER COLUMN file_name TYPE VARCHAR(512);
