-- Rename title to summary (backfills existing data)
-- Add first_user_message for higher fidelity title data
ALTER TABLE sessions RENAME COLUMN title TO summary;
ALTER TABLE sessions ADD COLUMN first_user_message TEXT;
