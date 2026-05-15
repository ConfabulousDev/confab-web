-- CF-385: codex_rollouts metadata sidecar for Codex parent-child thread trees.
--
-- Child rollouts upload their chunks under the root's hosted session
-- (file_type='transcript'). This table records the tree shape and per-thread
-- metadata (rollout path, cwd, model, agent attributes) without modifying
-- the sessions table.
--
-- Design notes:
--   * Composite PK (user_id, thread_uuid): UUIDs are user-local. Two users'
--     Codex SQLite DBs may legitimately mint the same UUID; they get
--     independent rows here.
--   * parent_thread_uuid has NO FK: child rollouts may be uploaded before
--     their parent, so an orphan parent reference must be allowed at write
--     time. Read paths handle missing parents with LEFT JOIN.
--   * ON DELETE CASCADE from users AND sessions: rollouts disappear when
--     their owner or hosted session goes away.
--   * hosted_file_name width matches sync_files.file_name (VARCHAR(512))
--     so a name accepted by sync/chunk can never fail this upsert on length.

CREATE TABLE codex_rollouts (
    thread_uuid         UUID NOT NULL,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_thread_uuid  UUID NULL,
    hosted_session_id   UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    hosted_file_name    VARCHAR(512) NOT NULL,
    rollout_path        VARCHAR(8192) NOT NULL,
    cwd                 VARCHAR(8192),
    model               VARCHAR(255),
    source              VARCHAR(64),
    thread_source       VARCHAR(255),
    agent_path          VARCHAR(8192),
    agent_role          VARCHAR(255),
    agent_nickname      VARCHAR(255),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, thread_uuid)
);

CREATE INDEX idx_codex_rollouts_user_parent ON codex_rollouts(user_id, parent_thread_uuid);
CREATE INDEX idx_codex_rollouts_session     ON codex_rollouts(hosted_session_id);
