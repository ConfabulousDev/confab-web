-- Revert VARCHAR columns back to TEXT

-- users
ALTER TABLE users ALTER COLUMN avatar_url TYPE TEXT;

-- sessions
ALTER TABLE sessions ALTER COLUMN external_id TYPE TEXT;
ALTER TABLE sessions ALTER COLUMN summary TYPE TEXT;
ALTER TABLE sessions ALTER COLUMN first_user_message TYPE TEXT;
ALTER TABLE sessions ALTER COLUMN cwd TYPE TEXT;
ALTER TABLE sessions ALTER COLUMN transcript_path TYPE TEXT;

-- runs
ALTER TABLE runs ALTER COLUMN transcript_path TYPE TEXT;
ALTER TABLE runs ALTER COLUMN cwd TYPE TEXT;
ALTER TABLE runs ALTER COLUMN reason TYPE TEXT;

-- files
ALTER TABLE files ALTER COLUMN file_path TYPE TEXT;
ALTER TABLE files ALTER COLUMN s3_key TYPE TEXT;

-- sync_files
ALTER TABLE sync_files ALTER COLUMN file_name TYPE TEXT;
