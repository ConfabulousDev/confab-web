-- Lookup tables for session filter dropdown values.
-- These are append-only, global (not per-user) tables.
-- Trade-off: users may see repo/branch names from sessions they can't access,
-- but these are just names, not session data.

CREATE TABLE session_repos (
    repo_name TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE session_branches (
    branch_name TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Backfill from existing sessions
INSERT INTO session_repos (repo_name)
SELECT DISTINCT regexp_replace(
    regexp_replace(git_info->>'repo_url', '\.git$', ''),
    '^.*[/:]([^/:]+/[^/:]+)$', '\1'
)
FROM sessions
WHERE git_info->>'repo_url' IS NOT NULL AND git_info->>'repo_url' != ''
ON CONFLICT DO NOTHING;

INSERT INTO session_branches (branch_name)
SELECT DISTINCT git_info->>'branch'
FROM sessions
WHERE git_info->>'branch' IS NOT NULL AND git_info->>'branch' != ''
ON CONFLICT DO NOTHING;
