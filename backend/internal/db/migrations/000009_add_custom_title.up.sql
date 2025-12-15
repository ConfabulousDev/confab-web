-- Add custom_title for manual title override (takes precedence over summary/first_user_message)
ALTER TABLE sessions ADD COLUMN custom_title VARCHAR(255);
