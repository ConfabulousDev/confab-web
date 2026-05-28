-- CF-233 rollback: restore the CHECK constraint in its CF-494-era form.
--
-- The data side of the UP migration (clearing pr_inference stamps) is
-- irreversible — we cannot reconstruct mappings we deleted. Rows whose
-- root_name was nulled remain nulled after this DOWN. New stamps written
-- by the rolled-back code (pr_inference fallback) will accumulate
-- normally from this point forward.

ALTER TABLE session_repos
    ADD CONSTRAINT session_repos_root_source_check
        CHECK (root_source IS NULL
               OR root_source IN ('pr_inference', 'github_api', 'manual', 'git_remote'));
