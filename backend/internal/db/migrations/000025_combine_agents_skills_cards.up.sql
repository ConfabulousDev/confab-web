-- Create combined agents_and_skills card table
CREATE TABLE session_card_agents_and_skills (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    agent_invocations INT NOT NULL DEFAULT 0,
    skill_invocations INT NOT NULL DEFAULT 0,
    agent_stats JSONB NOT NULL DEFAULT '{}',
    skill_stats JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_session_card_agents_and_skills_version ON session_card_agents_and_skills(version);

-- Migrate data from existing tables
-- Use COALESCE to handle sessions that only have one type of data
INSERT INTO session_card_agents_and_skills (
    session_id, version, computed_at, up_to_line,
    agent_invocations, skill_invocations, agent_stats, skill_stats
)
SELECT
    COALESCE(a.session_id, s.session_id) as session_id,
    1 as version,  -- Reset version since schema changed
    COALESCE(a.computed_at, s.computed_at) as computed_at,
    COALESCE(a.up_to_line, s.up_to_line) as up_to_line,
    COALESCE(a.total_invocations, 0) as agent_invocations,
    COALESCE(s.total_invocations, 0) as skill_invocations,
    COALESCE(a.agent_stats, '{}') as agent_stats,
    COALESCE(s.skill_stats, '{}') as skill_stats
FROM session_card_agents a
FULL OUTER JOIN session_card_skills s ON a.session_id = s.session_id;

-- Drop old tables
DROP TABLE session_card_agents;
DROP TABLE session_card_skills;
