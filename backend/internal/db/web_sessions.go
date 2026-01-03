package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// CreateWebSession creates a new web session for a user
func (db *DB) CreateWebSession(ctx context.Context, sessionID string, userID int64, expiresAt time.Time) error {
	ctx, span := tracer.Start(ctx, "db.create_web_session",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `INSERT INTO web_sessions (id, user_id, created_at, expires_at) VALUES ($1, $2, NOW(), $3)`
	_, err := db.conn.ExecContext(ctx, query, sessionID, userID, expiresAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to create web session: %w", err)
	}
	return nil
}

// GetWebSession retrieves a web session by ID and validates it's not expired
func (db *DB) GetWebSession(ctx context.Context, sessionID string) (*models.WebSession, error) {
	ctx, span := tracer.Start(ctx, "db.get_web_session")
	defer span.End()

	query := `
		SELECT ws.id, ws.user_id, u.email, u.status, ws.created_at, ws.expires_at
		FROM web_sessions ws
		JOIN users u ON ws.user_id = u.id
		WHERE ws.id = $1 AND ws.expires_at > NOW()
	`

	var session models.WebSession
	err := db.conn.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.UserEmail,
		&session.UserStatus,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't record as error - expired/missing session is expected
			return nil, fmt.Errorf("session not found or expired")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	span.SetAttributes(attribute.Int64("user.id", session.UserID))
	return &session, nil
}

// DeleteWebSession deletes a web session (logout)
func (db *DB) DeleteWebSession(ctx context.Context, sessionID string) error {
	ctx, span := tracer.Start(ctx, "db.delete_web_session")
	defer span.End()

	query := `DELETE FROM web_sessions WHERE id = $1`
	_, err := db.conn.ExecContext(ctx, query, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}
