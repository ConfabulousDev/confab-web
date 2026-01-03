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

// ValidateAPIKey checks if an API key is valid and returns the associated user ID, key ID, user email, and user status
func (db *DB) ValidateAPIKey(ctx context.Context, keyHash string) (userID int64, keyID int64, userEmail string, userStatus models.UserStatus, err error) {
	ctx, span := tracer.Start(ctx, "db.validate_api_key")
	defer span.End()

	query := `
		SELECT ak.id, ak.user_id, u.email, u.status
		FROM api_keys ak
		JOIN users u ON ak.user_id = u.id
		WHERE ak.key_hash = $1
	`

	err = db.conn.QueryRowContext(ctx, query, keyHash).Scan(&keyID, &userID, &userEmail, &userStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			// Don't record as error - invalid key is expected behavior
			return 0, 0, "", "", fmt.Errorf("invalid API key")
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, 0, "", "", fmt.Errorf("failed to validate API key: %w", err)
	}

	span.SetAttributes(attribute.Int64("user.id", userID))
	return userID, keyID, userEmail, userStatus, nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key
func (db *DB) UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error {
	ctx, span := tracer.Start(ctx, "db.update_api_key_last_used",
		trace.WithAttributes(attribute.Int64("key.id", keyID)))
	defer span.End()

	query := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := db.conn.ExecContext(ctx, query, keyID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

// CountAPIKeys returns the number of API keys for a user
func (db *DB) CountAPIKeys(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM api_keys WHERE user_id = $1`
	var count int
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}
	return count, nil
}

// CreateAPIKeyWithReturn creates a new API key and returns the key ID and created_at
// Returns ErrAPIKeyLimitExceeded if the user already has MaxAPIKeysPerUser keys
func (db *DB) CreateAPIKeyWithReturn(ctx context.Context, userID int64, keyHash, name string) (int64, time.Time, error) {
	ctx, span := tracer.Start(ctx, "db.create_api_key",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// Check if user has reached the limit
	count, err := db.CountAPIKeys(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, time.Time{}, err
	}
	if count >= MaxAPIKeysPerUser {
		return 0, time.Time{}, ErrAPIKeyLimitExceeded
	}

	query := `INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, created_at`

	var keyID int64
	var createdAt time.Time
	err = db.conn.QueryRowContext(ctx, query, userID, keyHash, name).Scan(&keyID, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, time.Time{}, ErrAPIKeyNameExists
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, time.Time{}, fmt.Errorf("failed to create API key: %w", err)
	}

	span.SetAttributes(attribute.Int64("key.id", keyID))
	return keyID, createdAt, nil
}

// ListAPIKeys returns all API keys for a user (without hashes)
func (db *DB) ListAPIKeys(ctx context.Context, userID int64) ([]models.APIKey, error) {
	ctx, span := tracer.Start(ctx, "db.list_api_keys",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `SELECT id, user_id, name, created_at, last_used_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var key models.APIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.CreatedAt, &key.LastUsedAt); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	span.SetAttributes(attribute.Int("keys.count", len(keys)))
	return keys, nil
}

// DeleteAPIKey deletes an API key
func (db *DB) DeleteAPIKey(ctx context.Context, userID, keyID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_api_key",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.Int64("key.id", keyID),
		))
	defer span.End()

	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	result, err := db.conn.ExecContext(ctx, query, keyID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// ReplaceAPIKey atomically replaces an existing API key with the same name, or creates a new one.
// If a key with the same name exists for the user, it is deleted and a new key is created.
// If no key with the same name exists, a new key is created (subject to MaxAPIKeysPerUser limit).
// Returns the new key ID and created_at timestamp.
func (db *DB) ReplaceAPIKey(ctx context.Context, userID int64, keyHash, name string) (int64, time.Time, error) {
	ctx, span := tracer.Start(ctx, "db.replace_api_key",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, time.Time{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if a key with the same name already exists
	var existingKeyID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM api_keys WHERE user_id = $1 AND name = $2`,
		userID, name).Scan(&existingKeyID)

	keyExists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return 0, time.Time{}, fmt.Errorf("failed to check existing key: %w", err)
	}

	// If no existing key, check the limit
	if !keyExists {
		var count int
		err = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM api_keys WHERE user_id = $1`,
			userID).Scan(&count)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("failed to count API keys: %w", err)
		}
		if count >= MaxAPIKeysPerUser {
			return 0, time.Time{}, ErrAPIKeyLimitExceeded
		}
	}

	// Delete existing key if it exists
	if keyExists {
		_, err = tx.ExecContext(ctx,
			`DELETE FROM api_keys WHERE id = $1`,
			existingKeyID)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("failed to delete existing key: %w", err)
		}
	}

	// Create new key
	var keyID int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, created_at`,
		userID, keyHash, name).Scan(&keyID, &createdAt)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to create API key: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to commit: %w", err)
	}

	return keyID, createdAt, nil
}
