-- CF-510 rollback: recreate the lookup tables in their final pre-drop shape.
--
-- DATA IS NOT RESTORED. The rows were derivable from sessions.git_info; the
-- rolled-back write path (upsertFilterLookups + RecordRepoRoot) repopulates them
-- on subsequent syncs. session_repos carries root_name/root_source (added in
-- migration 046); the root_source CHECK was dropped in migration 048, so none
-- is recreated here.

CREATE TABLE session_repos (
    repo_name   TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    root_name   TEXT,
    root_source TEXT
);

CREATE TABLE session_branches (
    branch_name TEXT PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
