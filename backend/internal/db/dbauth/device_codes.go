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

// DeleteDeviceCode removes a device code (after successful token exchange or expiration)
func (s *Store) DeleteDeviceCode(ctx context.Context, deviceCode string) error {
	query := `DELETE FROM device_codes WHERE device_code = $1`
	_, err := s.conn().ExecContext(ctx, query, db.HashToken(deviceCode))
	if err != nil {
		return fmt.Errorf("failed to delete device code: %w", err)
	}
	return nil
}
