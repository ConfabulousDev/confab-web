-- Revert: rename summary back to title, drop first_user_message
ALTER TABLE sessions DROP COLUMN first_user_message;
ALTER TABLE sessions RENAME COLUMN summary TO title;
