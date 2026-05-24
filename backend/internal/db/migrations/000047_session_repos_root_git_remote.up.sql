-- CF-494: extend the session_repos.root_source CHECK to include 'git_remote'.
--
-- CF-491 introduced root_source with three values; CF-494 adds a fourth for
-- mappings derived from CLI-shipped git remotes (definitive, no GitHub API
-- call). The old code never writes 'git_remote', so the wider constraint is
-- harmless during the deploy gap.

ALTER TABLE session_repos
    DROP CONSTRAINT session_repos_root_source_check,
    ADD CONSTRAINT session_repos_root_source_check
        CHECK (root_source IS NULL
               OR root_source IN ('pr_inference', 'github_api', 'manual', 'git_remote'));
