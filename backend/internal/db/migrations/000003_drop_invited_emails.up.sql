-- Drop the invited_emails table
-- This table was used for login authorization via ALLOW_INVITED_EMAILS_AFTER_TS
-- which has been replaced by the simpler MAX_USERS cap system.
-- Share invites are tracked in session_share_invites table instead.

DROP INDEX IF EXISTS idx_invited_emails_email;
DROP TABLE IF EXISTS invited_emails;
