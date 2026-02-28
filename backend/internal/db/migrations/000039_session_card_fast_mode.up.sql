CREATE TABLE session_card_fast_mode (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    computed_at TIMESTAMPTZ NOT NULL,
    up_to_line BIGINT NOT NULL,
    fast_turns INT NOT NULL DEFAULT 0,
    standard_turns INT NOT NULL DEFAULT 0,
    fast_cost_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    standard_cost_usd DECIMAL(20,10) NOT NULL DEFAULT 0
);

CREATE INDEX idx_session_card_fast_mode_version ON session_card_fast_mode(version);
