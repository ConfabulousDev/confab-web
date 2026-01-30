package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// Password authentication errors
var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountLocked      = errors.New("account is temporarily locked")
)

// Password authentication constants
const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
)

// PasswordCredentials represents a password identity's credentials
type PasswordCredentials struct {
	ID             int64
	IdentityID     int64
	PasswordHash   string
	FailedAttempts int
	LockedUntil    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AuthenticatePassword verifies email/password and returns the user if valid.
// Handles account lockout after too many failed attempts.
func (db *DB) AuthenticatePassword(ctx context.Context, email, password string) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "db.authenticate_password",
		trace.WithAttributes(attribute.String("email", email)))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Find password identity and credentials by email
	query := `
		SELECT
			u.id, u.email, u.name, u.avatar_url, u.status, u.created_at, u.updated_at,
			i.id as identity_id,
			p.password_hash, p.failed_attempts, p.locked_until
		FROM users u
		JOIN user_identities i ON u.id = i.user_id
		JOIN identity_passwords p ON i.id = p.identity_id
		WHERE i.provider = 'password' AND i.provider_id = $1
	`

	var user models.User
	var identityID int64
	var passwordHash string
	var failedAttempts int
	var lockedUntil *time.Time

	err = tx.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
		&identityID,
		&passwordHash, &failedAttempts, &lockedUntil,
	)

	if err == sql.ErrNoRows {
		// No such user - but use constant time to prevent timing attacks
		bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy.hash.to.prevent.timing.attacks."), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to query password identity: %w", err)
	}

	// Check if account is locked
	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		return nil, ErrAccountLocked
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		// Increment failed attempts
		newAttempts := failedAttempts + 1
		var newLockedUntil *time.Time
		if newAttempts >= MaxFailedAttempts {
			lockTime := time.Now().Add(LockoutDuration)
			newLockedUntil = &lockTime
		}

		updateSQL := `UPDATE identity_passwords SET failed_attempts = $1, locked_until = $2, updated_at = NOW() WHERE identity_id = $3`
		if _, err = tx.ExecContext(ctx, updateSQL, newAttempts, newLockedUntil, identityID); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("failed to update failed attempts: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}

		if newLockedUntil != nil {
			return nil, ErrAccountLocked
		}
		return nil, ErrInvalidCredentials
	}

	// Check if user is inactive
	if user.Status == models.UserStatusInactive {
		return nil, ErrInvalidCredentials
	}

	// Success - reset failed attempts
	resetSQL := `UPDATE identity_passwords SET failed_attempts = 0, locked_until = NULL, updated_at = NOW() WHERE identity_id = $1`
	if _, err = tx.ExecContext(ctx, resetSQL, identityID); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to reset failed attempts: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	span.SetAttributes(attribute.Int64("user.id", user.ID))
	return &user, nil
}

// CreatePasswordUser creates a new user with password authentication.
// Creates entries in users, user_identities, and identity_passwords tables.
func (db *DB) CreatePasswordUser(ctx context.Context, email, passwordHash string, isAdmin bool) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "db.create_password_user",
		trace.WithAttributes(attribute.String("email", email), attribute.Bool("is_admin", isAdmin)))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if email already exists
	var exists bool
	checkSQL := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	if err = tx.QueryRowContext(ctx, checkSQL, email).Scan(&exists); err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("user with email %s already exists", email)
	}

	// Create user
	var user models.User
	insertUserSQL := `
		INSERT INTO users (email, name, status, is_admin, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, NOW(), NOW())
		RETURNING id, email, name, avatar_url, status, created_at, updated_at
	`
	// Use email prefix as default name
	name := email
	if atIdx := len(email); atIdx > 0 {
		for i, c := range email {
			if c == '@' {
				name = email[:i]
				break
			}
		}
	}

	err = tx.QueryRowContext(ctx, insertUserSQL, email, name, isAdmin).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create password identity
	var identityID int64
	insertIdentitySQL := `
		INSERT INTO user_identities (user_id, provider, provider_id, created_at)
		VALUES ($1, 'password', $2, NOW())
		RETURNING id
	`
	err = tx.QueryRowContext(ctx, insertIdentitySQL, user.ID, email).Scan(&identityID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create password identity: %w", err)
	}

	// Create password credentials
	insertCredsSQL := `
		INSERT INTO identity_passwords (identity_id, password_hash, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
	`
	if _, err = tx.ExecContext(ctx, insertCredsSQL, identityID, passwordHash); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to create password credentials: %w", err)
	}

	// Resolve pending share recipients (same as OAuth)
	resolvePendingSQL := `UPDATE session_share_recipients SET user_id = $1 WHERE LOWER(email) = LOWER($2) AND user_id IS NULL`
	if _, err = tx.ExecContext(ctx, resolvePendingSQL, user.ID, email); err != nil {
		return nil, fmt.Errorf("failed to resolve pending share recipients: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	span.SetAttributes(attribute.Int64("user.id", user.ID))
	return &user, nil
}

// UpdateUserPassword updates a user's password hash
func (db *DB) UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error {
	ctx, span := tracer.Start(ctx, "db.update_user_password",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `
		UPDATE identity_passwords p
		SET password_hash = $1, failed_attempts = 0, locked_until = NULL, updated_at = NOW()
		FROM user_identities i
		WHERE p.identity_id = i.id AND i.user_id = $2 AND i.provider = 'password'
	`

	result, err := db.conn.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no password identity found for user %d", userID)
	}

	return nil
}

// GetUserByEmail retrieves a user by email address
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "db.get_user_by_email")
	defer span.End()

	query := `SELECT id, email, name, avatar_url, status, created_at, updated_at FROM users WHERE email = $1`

	var user models.User
	err := db.conn.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
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

// IsUserAdmin checks if a user has admin privileges
func (db *DB) IsUserAdmin(ctx context.Context, userID int64) (bool, error) {
	ctx, span := tracer.Start(ctx, "db.is_user_admin",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT is_admin FROM users WHERE id = $1`

	var isAdmin bool
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&isAdmin)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrUserNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("failed to check admin status: %w", err)
	}

	return isAdmin, nil
}
