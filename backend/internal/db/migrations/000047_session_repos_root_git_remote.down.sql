ALTER TABLE session_repos
    DROP CONSTRAINT session_repos_root_source_check,
    ADD CONSTRAINT session_repos_root_source_check
        CHECK (root_source IS NULL
               OR root_source IN ('pr_inference', 'github_api', 'manual'));
