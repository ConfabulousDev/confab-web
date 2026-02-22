package db

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "db.get_user_by_id",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT id, email, name, avatar_url, status, created_at, updated_at FROM users WHERE id = $1`

	var user models.User
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// CountUsers returns the total number of users in the system
func (db *DB) CountUsers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(ctx, "db.count_users")
	defer span.End()

	query := `SELECT COUNT(*) FROM users`
	var count int
	err := db.conn.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	span.SetAttributes(attribute.Int("users.count", count))
	return count, nil
}

// UserExistsByEmail checks if a user exists with the given email
func (db *DB) UserExistsByEmail(ctx context.Context, email string) (bool, error) {
	ctx, span := tracer.Start(ctx, "db.user_exists_by_email")
	defer span.End()

	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	var exists bool
	err := db.conn.QueryRowContext(ctx, query, email).Scan(&exists)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("failed to check user exists: %w", err)
	}
	span.SetAttributes(attribute.Bool("user.exists", exists))
	return exists, nil
}

// ListAllUsers returns all users in the system with stats, ordered by ID
func (db *DB) ListAllUsers(ctx context.Context) ([]models.AdminUserStats, error) {
	ctx, span := tracer.Start(ctx, "db.list_all_users")
	defer span.End()

	query := `
		SELECT
			u.id, u.email, u.name, u.avatar_url, u.status, u.created_at, u.updated_at,
			COUNT(DISTINCT s.id) AS session_count,
			MAX(ak.last_used_at) AS last_api_key_used,
			MAX(ws.created_at) AS last_logged_in
		FROM users u
		LEFT JOIN sessions s ON s.user_id = u.id
		LEFT JOIN api_keys ak ON ak.user_id = u.id
		LEFT JOIN web_sessions ws ON ws.user_id = u.id
		GROUP BY u.id
		ORDER BY u.id`

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.AdminUserStats
	for rows.Next() {
		var user models.AdminUserStats
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Name,
			&user.AvatarURL,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.SessionCount,
			&user.LastAPIKeyUsed,
			&user.LastLoggedIn,
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	span.SetAttributes(attribute.Int("users.count", len(users)))
	return users, nil
}

// UpdateUserStatus updates the status of a user (active/inactive)
func (db *DB) UpdateUserStatus(ctx context.Context, userID int64, status models.UserStatus) error {
	ctx, span := tracer.Start(ctx, "db.update_user_status",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("user.status", string(status)),
		))
	defer span.End()

	query := `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`

	result, err := db.conn.ExecContext(ctx, query, status, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update user status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// DeleteUser permanently deletes a user and all associated data (via CASCADE)
// Note: S3 objects must be deleted separately before calling this function
func (db *DB) DeleteUser(ctx context.Context, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_user",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `DELETE FROM users WHERE id = $1`

	result, err := db.conn.ExecContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// HasOwnSessions checks if a user has any sessions they own
func (db *DB) HasOwnSessions(ctx context.Context, userID int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "db.has_own_sessions",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT EXISTS(SELECT 1 FROM sessions WHERE user_id = $1)`
	var exists bool
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("failed to check user sessions: %w", err)
	}
	span.SetAttributes(attribute.Bool("user.has_own_sessions", exists))
	return exists, nil
}

// HasAPIKeys checks if a user has any API keys
func (db *DB) HasAPIKeys(ctx context.Context, userID int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "db.has_api_keys",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT EXISTS(SELECT 1 FROM api_keys WHERE user_id = $1)`
	var exists bool
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("failed to check user API keys: %w", err)
	}
	span.SetAttributes(attribute.Bool("user.has_api_keys", exists))
	return exists, nil
}

// GetUserSessionIDs returns all session IDs (UUIDs) for a user
// Used for S3 cleanup before user deletion
func (db *DB) GetUserSessionIDs(ctx context.Context, userID int64) ([]string, error) {
	ctx, span := tracer.Start(ctx, "db.get_user_session_ids",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT id FROM sessions WHERE user_id = $1`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	var sessionIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan session ID: %w", err)
		}
		sessionIDs = append(sessionIDs, id)
	}

	if err = rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	span.SetAttributes(attribute.Int("sessions.count", len(sessionIDs)))
	return sessionIDs, nil
}
