package dbauth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// CreateDeviceCode creates a new device code for CLI authentication
func (s *Store) CreateDeviceCode(ctx context.Context, deviceCode, userCode, keyName string, expiresAt time.Time) error {
	// Store sha256(device_code) so a DB read can't replay it (40hj). user_code
	// stays plaintext (D4): low-entropy + short-lived, defended by the 8epk
	// per-verifier throttle rather than at-rest hashing.
	query := `INSERT INTO device_codes (device_code, user_code, key_name, expires_at) VALUES ($1, $2, $3, $4)`
	_, err := s.conn().ExecContext(ctx, query, db.HashToken(deviceCode), userCode, keyName, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create device code: %w", err)
	}
	return nil
}

// GetDeviceCodeByUserCode retrieves a device code by user code (for web verification page)
func (s *Store) GetDeviceCodeByUserCode(ctx context.Context, userCode string) (*db.DeviceCode, error) {
	query := `SELECT id, device_code, user_code, key_name, user_id, expires_at, authorized_at, created_at
	          FROM device_codes WHERE user_code = $1 AND expires_at > NOW()`

	var dc db.DeviceCode
	err := s.conn().QueryRowContext(ctx, query, userCode).Scan(
		&dc.ID, &dc.DeviceCode, &dc.UserCode, &dc.KeyName,
		&dc.UserID, &dc.ExpiresAt, &dc.AuthorizedAt, &dc.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDeviceCodeNotFound
		}
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// GetDeviceCodeByDeviceCode retrieves a device code by device code (for CLI polling)
func (s *Store) GetDeviceCodeByDeviceCode(ctx context.Context, deviceCode string) (*db.DeviceCode, error) {
	query := `SELECT id, device_code, user_code, key_name, user_id, expires_at, authorized_at, created_at
	          FROM device_codes WHERE device_code = $1`

	var dc db.DeviceCode
	err := s.conn().QueryRowContext(ctx, query, db.HashToken(deviceCode)).Scan(
		&dc.ID, &dc.DeviceCode, &dc.UserCode, &dc.KeyName,
		&dc.UserID, &dc.ExpiresAt, &dc.AuthorizedAt, &dc.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDeviceCodeNotFound
		}
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// AuthorizeDeviceCode marks a device code as authorized by a user
func (s *Store) AuthorizeDeviceCode(ctx context.Context, userCode string, userID int64) error {
	query := `UPDATE device_codes SET user_id = $1, authorized_at = NOW()
	          WHERE user_code = $2 AND expires_at > NOW() AND authorized_at IS NULL`

	result, err := s.conn().ExecContext(ctx, query, userID, userCode)
	if err != nil {
		return fmt.Errorf("failed to authorize device code: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return db.ErrDeviceCodeNotFound
	}
	return nil
}

// DeleteDeviceCode removes a device code (expired or domain-denied codes in
// HandleDeviceToken; a successful exchange consumes the code inside
// ConsumeDeviceCodeAndReplaceKey instead).
func (s *Store) DeleteDeviceCode(ctx context.Context, deviceCode string) error {
	query := `DELETE FROM device_codes WHERE device_code = $1`
	_, err := s.conn().ExecContext(ctx, query, db.HashToken(deviceCode))
	if err != nil {
		return fmt.Errorf("failed to delete device code: %w", err)
	}
	return nil
}

// ConsumedDeviceCode is the result of a successful device-code exchange: the
// newly issued API key plus the user and key name the code was bound to.
type ConsumedDeviceCode struct {
	KeyID     int64
	CreatedAt time.Time
	UserID    int64
	KeyName   string
}

// ConsumeDeviceCodeAndReplaceKey atomically claims an authorized, unexpired
// device code and issues the API key for it, in one transaction:
//
//  1. DELETE … RETURNING claims the row. Concurrent callers serialize on the
//     row lock; once the winner commits, every other caller's DELETE matches
//     no row and gets db.ErrDeviceCodeNotFound (as do pending, expired, or
//     missing codes).
//  2. The key is issued with ReplaceAPIKey semantics under the code's
//     user_id and key_name.
//
// Any failure — including db.ErrAPIKeyLimitExceeded — rolls back both steps,
// so the code is not consumed and no key is minted.
func (s *Store) ConsumeDeviceCodeAndReplaceKey(ctx context.Context, deviceCode, keyHash string) (*ConsumedDeviceCode, error) {
	tx, err := s.conn().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var res ConsumedDeviceCode
	err = tx.QueryRowContext(ctx,
		`DELETE FROM device_codes
		 WHERE device_code = $1 AND authorized_at IS NOT NULL AND user_id IS NOT NULL AND expires_at > NOW()
		 RETURNING user_id, key_name`,
		db.HashToken(deviceCode)).Scan(&res.UserID, &res.KeyName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrDeviceCodeNotFound
		}
		return nil, fmt.Errorf("failed to claim device code: %w", err)
	}

	res.KeyID, res.CreatedAt, err = replaceAPIKeyTx(ctx, tx, res.UserID, keyHash, res.KeyName)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	return &res, nil
}
