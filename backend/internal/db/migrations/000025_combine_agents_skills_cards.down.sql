-- Recreate separate tables
CREATE TABLE session_card_agents (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    total_invocations INT NOT NULL DEFAULT 0,
    agent_stats JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_agents_version ON session_card_agents(version);

CREATE TABLE session_card_skills (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    total_invocations INT NOT NULL DEFAULT 0,
    skill_stats JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_skills_version ON session_card_skills(version);

-- Migrate data back from combined table
INSERT INTO session_card_agents (session_id, version, computed_at, up_to_line, total_invocations, agent_stats)
SELECT session_id, version, computed_at, up_to_line, agent_invocations, agent_stats
FROM session_card_agents_and_skills
WHERE agent_invocations > 0 OR agent_stats != '{}';

INSERT INTO session_card_skills (session_id, version, computed_at, up_to_line, total_invocations, skill_stats)
SELECT session_id, version, computed_at, up_to_line, skill_invocations, skill_stats
FROM session_card_agents_and_skills
WHERE skill_invocations > 0 OR skill_stats != '{}';

-- Drop combined table
DROP TABLE session_card_agents_and_skills;
