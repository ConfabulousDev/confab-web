-- 6kbe: per-(session, bucket) precompute failure tracking for the worker.
-- A row exists while a session has an unsettled attempt (in_progress_since set)
-- or consecutive failures in a bucket; success deletes it. After
-- analytics.QuarantineAfterAttempts failures quarantined_at is set and the
-- bucket's Find* query excludes the session until an admin (or an on-view
-- success) releases it. bucket is app-validated (regular_cards, smart_recap,
-- search_index, session_title); every insert supplies every NOT NULL column.
CREATE TABLE IF NOT EXISTS session_precompute_failures (
    session_id        UUID        NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    bucket            TEXT        NOT NULL,
    attempts          INT         NOT NULL,
    last_error        TEXT,
    in_progress_since TIMESTAMPTZ,
    quarantined_at    TIMESTAMPTZ,
    updated_at        TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (session_id, bucket)
);
