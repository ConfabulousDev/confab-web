-- CF-347 rollback: restore the historical 'Claude Code' default. Any rows
-- that were written with the canonical 'claude-code' (or 'codex') value stay
-- as-is — only the column default reverts. The application can still read
-- those rows via the read-side normalizer for as long as it remains in code.

ALTER TABLE sessions ALTER COLUMN session_type SET DEFAULT 'Claude Code';
