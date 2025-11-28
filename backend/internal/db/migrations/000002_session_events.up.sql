-- Session events table for capturing lifecycle events (session_end, etc.)
CREATE TABLE IF NOT EXISTS session_events (
    id BIGSERIAL PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,          -- 'session_end', future: 'session_start', etc.
    event_timestamp TIMESTAMP NOT NULL,        -- When the event occurred (from client)
    received_at TIMESTAMP NOT NULL DEFAULT NOW(), -- When we received it
    payload JSONB,                             -- Full event payload (HookInput, etc.)
    CONSTRAINT valid_event_type CHECK (event_type IN ('session_end'))
);

CREATE INDEX idx_session_events_session_id ON session_events(session_id);
CREATE INDEX idx_session_events_event_type ON session_events(event_type);
CREATE INDEX idx_session_events_event_timestamp ON session_events(event_timestamp);
