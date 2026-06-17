-- zsr6: cursor_session_meta sidecar — persists the per-session model name the
-- Cursor CLI sends as metadata.model on sync chunks (sourced from
-- ~/.cursor/.../state.vscdb composerData.modelConfig.modelName).
--
-- Cursor JSONL transcripts carry no model field, so analytics cannot recover
-- the model from S3 content alone. This sidecar gives the analytics step a
-- per-session read-back path (mirrors the codex_rollouts precedent) so the
-- session card's models_used can be populated without a column on the generic
-- sessions table.
--
-- Design notes:
--   * session_id is the PK and FKs to sessions with ON DELETE CASCADE: the row
--     disappears when its session does.
--   * model is NOT NULL: the app only ever inserts a non-empty model (the sync
--     handler skips the upsert when metadata.model is absent/empty), per the
--     project convention that enum-like/required values are supplied by the app
--     rather than defaulted in the DB.
CREATE TABLE cursor_session_meta (
    session_id UUID PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    model      VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
