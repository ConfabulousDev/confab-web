-- Remove share_token column from session_shares table
-- Share tokens were used for URL-based access (/share/{token}) which has been
-- replaced by canonical session URLs (/sessions/{id}). Revocation now uses
-- the share ID instead of the token.

-- Drop the index first
DROP INDEX IF EXISTS idx_session_shares_token;

-- Remove the column
ALTER TABLE session_shares DROP COLUMN IF EXISTS share_token;

-- Also remove the legacy visibility column if it exists (was replaced by type tables)
ALTER TABLE session_shares DROP COLUMN IF EXISTS visibility;
