-- CF-233: retire the pr_inference fork→root resolver.
--
-- CF-491 introduced the pr_inference heuristic: if a session's
-- git_info.repo_url extracted to one owner/repo but its PR links pointed at
-- a different owner/repo, the PR target was stamped as the upstream. This
-- was too aggressive — cross-repo PRs (sibling, dependency, downstream
-- consumer, cross-org) all got treated as upstream evidence and silently
-- misclassified working repos under unrelated roots, surfacing as wrong
-- numbers under repo filters in /sessions, /org, and /trends.
--
-- CF-494's git_remote signal (CLI-shipped remotes + tracking_remote) is
-- definitive; pr_inference is no longer trusted. The chunk handler stops
-- writing it (see api/sync.go) and this migration clears the existing
-- stamps. Future syncs from CF-494-capable CLIs re-stamp via git_remote.
-- Sessions from older CLIs stay un-collapsed (each raw repo as its own
-- filter chip), which is the accurate posture absent a real upstream
-- signal — and the same behavior the product shipped with for months
-- before CF-491.

UPDATE session_repos
   SET root_name = NULL, root_source = NULL
   WHERE root_source = 'pr_inference';

-- Also drop the CHECK constraint on root_source. Per the project's
-- "db constraints in app" preference, allowed-value enums live in the
-- application; the DB enforces structural integrity only. After CF-233
-- only 'git_remote' is ever written, but the validator for that lives in
-- helpers.go::RecordRepoRoot rather than here.
ALTER TABLE session_repos
    DROP CONSTRAINT session_repos_root_source_check;
