-- GitHub artifact links for sessions (commits, PRs)
-- Enables bidirectional navigation between Confabulous sessions and GitHub

CREATE TABLE session_github_links (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    link_type VARCHAR(20) NOT NULL CHECK (link_type IN ('commit', 'pull_request')),
    url VARCHAR(2048) NOT NULL,
    -- Parsed from URL for efficient querying
    owner VARCHAR(255) NOT NULL,
    repo VARCHAR(255) NOT NULL,
    -- For commits: SHA, for PRs: PR number
    ref VARCHAR(255) NOT NULL,
    -- Optional metadata
    title VARCHAR(500),
    -- Tracking
    source VARCHAR(50) NOT NULL CHECK (source IN ('cli_hook', 'manual')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Prevent duplicate links
    UNIQUE (session_id, link_type, owner, repo, ref)
);

CREATE INDEX idx_github_links_session ON session_github_links(session_id);
CREATE INDEX idx_github_links_repo ON session_github_links(owner, repo);
