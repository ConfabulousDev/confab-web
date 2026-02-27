-- Add 'transcript' to the source CHECK constraint on session_github_links
-- This allows pr-link messages extracted from transcript chunks during sync
ALTER TABLE session_github_links DROP CONSTRAINT session_github_links_source_check;
ALTER TABLE session_github_links ADD CONSTRAINT session_github_links_source_check CHECK (source IN ('cli_hook', 'manual', 'transcript'));
