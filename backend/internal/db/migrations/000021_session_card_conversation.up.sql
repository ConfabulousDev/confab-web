-- Conversation card: tracks timing metrics for conversational turns
CREATE TABLE session_card_conversation (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,

    -- Turn counts (for context)
    user_turns INT NOT NULL DEFAULT 0,
    assistant_turns INT NOT NULL DEFAULT 0,

    -- Timing metrics (nullable - may not be computable for short sessions)
    avg_assistant_turn_ms BIGINT,
    avg_user_thinking_ms BIGINT
);

CREATE INDEX idx_session_card_conversation_version ON session_card_conversation(version);
