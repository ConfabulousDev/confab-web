-- Workflows card table (line-based invalidation)
-- Tracks per-run aggregates for Claude Code workflow subagent runs:
-- agent count, token breakdown + cost, journal-derived success count, and a
-- duration span. Runs are grouped by the <runId> path segment of each
-- subagents/workflows/<runId>/agent-<id>.jsonl file (see analytics.ExtractWorkflowRunID).
CREATE TABLE session_card_workflows (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    runs JSONB NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_session_card_workflows_version ON session_card_workflows(version);

COMMENT ON TABLE session_card_workflows IS 'Cached per-run workflow subagent aggregates for sessions';
COMMENT ON COLUMN session_card_workflows.version IS 'Compute logic version for cache invalidation';
COMMENT ON COLUMN session_card_workflows.up_to_line IS 'JSONL line count (across transcript+agent files) when computed';
COMMENT ON COLUMN session_card_workflows.runs IS 'JSON array of per-run aggregates: run_id, agent_count, token breakdown, estimated_usd, succeeded_agents, has_journal, duration_ms';
