CREATE TABLE session_card_agents (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    total_invocations INT NOT NULL DEFAULT 0,
    agent_stats JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_agents_version ON session_card_agents(version);
