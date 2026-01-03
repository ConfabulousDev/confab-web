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

// FindOrCreateUserByOAuth finds or creates a user by OAuth provider identity.
// It handles account linking: if an identity doesn't exist but the email matches
// an existing user, it links the new identity to that user.
func (db *DB) FindOrCreateUserByOAuth(ctx context.Context, info models.OAuthUserInfo) (*models.User, error) {
	ctx, span := tracer.Start(ctx, "db.find_or_create_user_by_oauth",
		trace.WithAttributes(attribute.String("oauth.provider", string(info.Provider))))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Try to find existing user by provider identity
	query := `
		SELECT u.id, u.email, u.name, u.avatar_url, u.status, u.created_at, u.updated_at
		FROM users u
		JOIN user_identities i ON u.id = i.user_id
		WHERE i.provider = $1 AND i.provider_id = $2
	`
	var user models.User
	err = tx.QueryRowContext(ctx, query, info.Provider, info.ProviderID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		// User found via identity - update profile info and username
		updateSQL := `UPDATE users SET email = $1, name = $2, avatar_url = $3, updated_at = NOW() WHERE id = $4`
		if _, err = tx.ExecContext(ctx, updateSQL, info.Email, info.Name, info.AvatarURL, user.ID); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		// Update provider username if changed
		if info.ProviderUsername != "" {
			updateIdentitySQL := `UPDATE user_identities SET provider_username = $1 WHERE user_id = $2 AND provider = $3`
			if _, err = tx.ExecContext(ctx, updateIdentitySQL, info.ProviderUsername, user.ID, info.Provider); err != nil {
				return nil, fmt.Errorf("failed to update identity: %w", err)
			}
		}

		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user by identity: %w", err)
	}

	// 2. Identity not found - check if email exists (for account linking)
	emailQuery := `SELECT id, email, name, avatar_url, status, created_at, updated_at FROM users WHERE email = $1`
	err = tx.QueryRowContext(ctx, emailQuery, info.Email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		// User exists with same email - link this identity to their account
		linkSQL := `INSERT INTO user_identities (user_id, provider, provider_id, provider_username, created_at)
		            VALUES ($1, $2, $3, $4, NOW())`
		var username *string
		if info.ProviderUsername != "" {
			username = &info.ProviderUsername
		}
		if _, err = tx.ExecContext(ctx, linkSQL, user.ID, info.Provider, info.ProviderID, username); err != nil {
			return nil, fmt.Errorf("failed to link identity: %w", err)
		}

		// Update profile with latest info
		updateSQL := `UPDATE users SET name = $1, avatar_url = $2, updated_at = NOW() WHERE id = $3`
		if _, err = tx.ExecContext(ctx, updateSQL, info.Name, info.AvatarURL, user.ID); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	// 3. New user - create user and identity
	insertUserSQL := `
		INSERT INTO users (email, name, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, 'active', NOW(), NOW())
		RETURNING id, email, name, avatar_url, status, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, insertUserSQL, info.Email, info.Name, info.AvatarURL).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create identity
	insertIdentitySQL := `INSERT INTO user_identities (user_id, provider, provider_id, provider_username, created_at)
	                      VALUES ($1, $2, $3, $4, NOW())`
	var username *string
	if info.ProviderUsername != "" {
		username = &info.ProviderUsername
	}
	if _, err = tx.ExecContext(ctx, insertIdentitySQL, user.ID, info.Provider, info.ProviderID, username); err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	// Resolve pending share recipients for the new user
	// This links any shares that were created with this user's email before they signed up
	resolvePendingSQL := `UPDATE session_share_recipients SET user_id = $1 WHERE LOWER(email) = LOWER($2) AND user_id IS NULL`
	if _, err = tx.ExecContext(ctx, resolvePendingSQL, user.ID, info.Email); err != nil {
		return nil, fmt.Errorf("failed to resolve pending share recipients: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &user, nil
}
