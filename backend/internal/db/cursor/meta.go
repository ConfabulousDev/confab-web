package cursor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// UpsertModel records the model name for a Cursor session.
//
// Semantics (mirrors the codex_rollouts free-form-field rule):
//   - session_id is the PK; one row per session.
//   - First non-empty model wins: a re-upsert with a different model never
//     clobbers an already-stored value (COALESCE/NULLIF). This matches the
//     wire contract — the CLI's metadata.model is best-effort, so the first
//     time it recovers a name we keep it.
//   - updated_at advances on every successful call (NOW()).
//
// The caller must pass a non-empty model — the sync handler guards on this so
// an absent/empty metadata.model never reaches the table (the NOT NULL column
// would reject "" anyway, but the guard keeps the no-op path off the DB).
func (s *Store) UpsertModel(ctx context.Context, sessionID, model string) error {
	ctx, span := tracer.Start(ctx, "db.upsert_cursor_session_meta",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		))
	defer span.End()

	const query = `
		INSERT INTO cursor_session_meta (session_id, model)
		VALUES ($1, $2)
		ON CONFLICT (session_id) DO UPDATE SET
			model      = COALESCE(NULLIF(cursor_session_meta.model, ''), EXCLUDED.model),
			updated_at = NOW()
	`
	if _, err := s.conn().ExecContext(ctx, query, sessionID, model); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert cursor session meta: %w", err)
	}
	return nil
}

// GetModel returns the stored model for a Cursor session. The bool reports
// whether a row exists; (", false, nil) means no model has been persisted yet
// and the caller should leave models_used empty (never invent a model).
func (s *Store) GetModel(ctx context.Context, sessionID string) (string, bool, error) {
	ctx, span := tracer.Start(ctx, "db.get_cursor_session_meta",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		))
	defer span.End()

	var model string
	row := s.conn().QueryRowContext(ctx,
		`SELECT model FROM cursor_session_meta WHERE session_id = $1`, sessionID)
	if err := row.Scan(&model); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", false, fmt.Errorf("get cursor session meta: %w", err)
	}
	return model, true, nil
}
