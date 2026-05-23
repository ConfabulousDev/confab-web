-- CF-491: Collapse forks into their upstream root in repo filter chips.
--
-- Adds two nullable columns to session_repos:
--   * root_name   — the upstream owner/repo when this row is known to be a fork
--   * root_source — provenance of the mapping ('pr_inference' for v1)
--
-- Mappings are observed per-session: if a session's git_info.repo_url extracts
-- to one owner/repo but its PR links point to a different owner/repo, the PR's
-- owner/repo is the upstream. Repos with no PR-link observation keep NULL
-- root_name and pass through COALESCE unchanged at read time.
--
-- Casing note: GitHub treats owner/repo as case-insensitive but stores
-- case-as-typed. We accept that two casings would produce two chips if both
-- appear; in practice git remote URLs are stable.

ALTER TABLE session_repos
    ADD COLUMN root_name TEXT,
    ADD COLUMN root_source TEXT
        CHECK (root_source IS NULL
               OR root_source IN ('pr_inference', 'github_api', 'manual'));

-- Backfill: for every session_repos row whose name corresponds to a session
-- with a PR link to a different owner/repo, stamp the PR's owner/repo as the
-- root. First-write-wins via `root_name IS NULL` guard.
UPDATE session_repos sr
SET root_name = derived.root_repo,
    root_source = 'pr_inference'
FROM (
    SELECT DISTINCT
        regexp_replace(regexp_replace(s.git_info->>'repo_url', '\.git$', ''),
                       '^.*[/:]([^/:]+/[^/:]+)$', '\1') AS fork_repo,
        sgl.owner || '/' || sgl.repo AS root_repo
    FROM session_github_links sgl
    JOIN sessions s ON s.id = sgl.session_id
    WHERE sgl.link_type = 'pull_request'
      AND s.git_info->>'repo_url' IS NOT NULL
      AND s.git_info->>'repo_url' <> ''
      AND regexp_replace(regexp_replace(s.git_info->>'repo_url', '\.git$', ''),
                         '^.*[/:]([^/:]+/[^/:]+)$', '\1')
          <> (sgl.owner || '/' || sgl.repo)
) derived
WHERE sr.repo_name = derived.fork_repo
  AND sr.root_name IS NULL;
