package events

import (
	"context"
	"fmt"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// InsertSessionEvent inserts a new event into the session_events table
func (s *Store) InsertSessionEvent(ctx context.Context, params db.SessionEventParams) error {
	query := `
		INSERT INTO session_events (session_id, event_type, event_timestamp, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.conn().ExecContext(ctx, query, params.SessionID, params.EventType, params.EventTimestamp, params.Payload)
	if err != nil {
		return fmt.Errorf("failed to insert session event: %w", err)
	}
	return nil
}
