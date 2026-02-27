-- Remove 'transcript' from the source CHECK constraint
-- NOTE: Will fail if rows with source='transcript' exist — operator must clean up manually
ALTER TABLE session_github_links DROP CONSTRAINT session_github_links_source_check;
ALTER TABLE session_github_links ADD CONSTRAINT session_github_links_source_check CHECK (source IN ('cli_hook', 'manual'));
