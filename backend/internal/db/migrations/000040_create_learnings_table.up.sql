-- Create learnings table for capturing reusable knowledge from sessions
CREATE TYPE learning_status AS ENUM ('draft', 'confirmed', 'exported', 'archived');
CREATE TYPE learning_source AS ENUM ('manual_session', 'manual_review', 'ai_extracted');

CREATE TABLE learnings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    tags TEXT[] NOT NULL DEFAULT '{}',
    status learning_status NOT NULL DEFAULT 'draft',
    source learning_source NOT NULL,
    session_ids UUID[] NOT NULL DEFAULT '{}',
    transcript_range JSONB,
    confluence_page_id TEXT,
    exported_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_learnings_user_id ON learnings(user_id);
CREATE INDEX idx_learnings_status ON learnings(status);
CREATE INDEX idx_learnings_tags ON learnings USING GIN(tags);
CREATE INDEX idx_learnings_session_ids ON learnings USING GIN(session_ids);
CREATE INDEX idx_learnings_created_at ON learnings(created_at DESC);

-- Full-text search on title + body
ALTER TABLE learnings ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(body, '')), 'B')
    ) STORED;

CREATE INDEX idx_learnings_search ON learnings USING GIN(search_vector);
