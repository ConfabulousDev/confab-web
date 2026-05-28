-- CF-510: drop the global filter-lookup tables.
--
-- The fork→upstream mapping is now resolved live in SQL from each session's own
-- git_info (db.RepoRootExpr reads repo_url + remotes + tracking_remote), and the
-- repo/branch/owner filter dropdowns derive from the viewer's visible sessions
-- (db.VisibleSessionsCTE). Neither table has any reader or writer left:
--   * session_repos.root_name fed the old cross-session COALESCE subquery, and
--     session_repos.repo_name fed the share-all dropdown — both retired.
--   * session_branches only ever fed the share-all branch dropdown.
--
-- No backfill: read-time resolution means existing sessions that carry
-- CLI-shipped git remotes collapse correctly on the very next read.

DROP TABLE IF EXISTS session_repos;
DROP TABLE IF EXISTS session_branches;
