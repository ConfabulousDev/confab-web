-- Re-add share_token column to session_shares table
ALTER TABLE session_shares ADD COLUMN IF NOT EXISTS share_token CHAR(32);

-- Note: This will leave share_token as NULL for existing rows.
-- A full rollback would require regenerating tokens, which is not practical.

-- Re-add the index
CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token);
