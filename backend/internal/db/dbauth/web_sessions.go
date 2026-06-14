package dbauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// sessionActivityRefreshWindowSecs throttles the last_activity_at touch: the
// timestamp is bumped at most once per this window to avoid write amplification
// on every authenticated request (60j6).
const sessionActivityRefreshWindowSecs = 60

// CreateWebSession creates a new web session for a user
func (s *Store) CreateWebSession(ctx context.Context, sessionID string, userID int64, expiresAt time.Time) error {
	ctx, span := tracer.Start(ctx, "db.create_web_session",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// Store sha256(cookie value), never the raw token, so a DB read can't
	// replay the session (40hj). The cookie keeps the raw value.
	query := `INSERT INTO web_sessions (id, user_id, created_at, expires_at, last_activity_at) VALUES ($1, $2, NOW(), $3, NOW())`
	_, err := s.conn().ExecContext(ctx, query, db.HashToken(sessionID), userID, expiresAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to create web session: %w", err)
	}
	return nil
}

// GetWebSession retrieves a web session by ID and validates it against the
// absolute expiry cap AND, when idleTimeout > 0, the sliding idle-timeout gate
// (60j6). A non-positive idleTimeout disables the idle gate and the activity
// touch entirely — used for the CF-483 demo shared session, which sits idle
// between anonymous visitors and must never be force-expired. On a valid read
// with the idle gate active, last_activity_at is refreshed (throttled).
func (s *Store) GetWebSession(ctx context.Context, sessionID string, idleTimeout time.Duration) (*models.WebSession, error) {
	ctx, span := tracer.Start(ctx, "db.get_web_session")
	defer span.End()

	hashedID := db.HashToken(sessionID)
	applyIdle := idleTimeout > 0

	query := `
		SELECT ws.id, ws.user_id, u.email, u.status, u.read_only, ws.created_at, ws.expires_at, ws.last_activity_at
		FROM web_sessions ws
		JOIN users u ON ws.user_id = u.id
		WHERE ws.id = $1 AND ws.expires_at > NOW()`
	args := []any{hashedID}
	if applyIdle {
		// Idle = expired: a session inactive longer than the window is rejected
		// the same way as an absolutely-expired one. COALESCE handles rollout-gap
		// rows that predate last_activity_at.
		query += ` AND COALESCE(ws.last_activity_at, ws.created_at) > NOW() - make_interval(secs => $2)`
		args = append(args, idleTimeout.Seconds())
	}

	var session models.WebSession
	err := s.conn().QueryRowContext(ctx, query, args...).Scan(
		&session.ID,
		&session.UserID,
		&session.UserEmail,
		&session.UserStatus,
		&session.ReadOnly,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.LastActivityAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't record as error - expired/idle/missing session is expected
			return nil, fmt.Errorf("session not found or expired")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if applyIdle {
		s.touchSessionActivity(ctx, hashedID)
	}

	span.SetAttributes(attribute.Int64("user.id", session.UserID))
	return &session, nil
}

// touchSessionActivity advances last_activity_at to NOW(), but only when it is
// older than the refresh window — a single race-safe conditional UPDATE keyed on
// the PK. Best-effort: a failed touch must not fail the (already valid) read, so
// the error is recorded on the span but swallowed (60j6).
func (s *Store) touchSessionActivity(ctx context.Context, hashedID string) {
	query := `
		UPDATE web_sessions
		SET last_activity_at = NOW()
		WHERE id = $1 AND COALESCE(last_activity_at, created_at) < NOW() - make_interval(secs => $2)`
	if _, err := s.conn().ExecContext(ctx, query, hashedID, float64(sessionActivityRefreshWindowSecs)); err != nil {
		if span := trace.SpanFromContext(ctx); span != nil {
			span.RecordError(err)
		}
	}
}

// DeleteWebSession deletes a web session (logout)
func (s *Store) DeleteWebSession(ctx context.Context, sessionID string) error {
	ctx, span := tracer.Start(ctx, "db.delete_web_session")
	defer span.End()

	query := `DELETE FROM web_sessions WHERE id = $1`
	_, err := s.conn().ExecContext(ctx, query, db.HashToken(sessionID))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// UpsertSharedSession is the CF-483 single-shared-cookie helper. The
// row's ID is deterministically derived from CSRF_SECRET_KEY + demo
// email; bootstrap and the auto-impersonate fallback both call it so
// the demo user always has exactly one persistent session row.
func (s *Store) UpsertSharedSession(ctx context.Context, sessionID string, userID int64, expiresAt time.Time) error {
	ctx, span := tracer.Start(ctx, "db.upsert_shared_session",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// The demo row is exempt from the idle gate (GetWebSession is called with a
	// non-positive idleTimeout for it), so last_activity_at is set on insert for
	// completeness but never drives expiry and is not bumped on conflict.
	query := `
		INSERT INTO web_sessions (id, user_id, created_at, expires_at, last_activity_at)
		VALUES ($1, $2, NOW(), $3, NOW())
		ON CONFLICT (id) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at
	`
	if _, err := s.conn().ExecContext(ctx, query, db.HashToken(sessionID), userID, expiresAt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert shared web session: %w", err)
	}
	return nil
}

// DeleteOtherSessionsForUser deletes every web_sessions row for userID
// except the one with id=keepSessionID. Used by CF-483 bootstrap to
// guarantee the demo user has exactly one session row after flipping
// an existing real user. Returns the count of deleted rows.
func (s *Store) DeleteOtherSessionsForUser(ctx context.Context, userID int64, keepSessionID string) (int64, error) {
	ctx, span := tracer.Start(ctx, "db.delete_other_sessions_for_user",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `DELETE FROM web_sessions WHERE user_id = $1 AND id <> $2`
	res, err := s.conn().ExecContext(ctx, query, userID, db.HashToken(keepSessionID))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("delete other web sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
