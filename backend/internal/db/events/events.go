package events

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// InsertSessionEvent inserts a new event into the session_events table
func (s *Store) InsertSessionEvent(ctx context.Context, params db.SessionEventParams) error {
	ctx, span := tracer.Start(ctx, "db.insert_session_event",
		trace.WithAttributes(
			attribute.String("session.id", params.SessionID),
			attribute.String("event.type", params.EventType),
		))
	defer span.End()

	query := `
		INSERT INTO session_events (session_id, event_type, event_timestamp, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.conn().ExecContext(ctx, query, params.SessionID, params.EventType, params.EventTimestamp, params.Payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to insert session event: %w", err)
	}
	return nil
}
