package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// CreateDeviceCode creates a new device code for CLI authentication
func (db *DB) CreateDeviceCode(ctx context.Context, deviceCode, userCode, keyName string, expiresAt time.Time) error {
	ctx, span := tracer.Start(ctx, "db.create_device_code")
	defer span.End()

	query := `INSERT INTO device_codes (device_code, user_code, key_name, expires_at) VALUES ($1, $2, $3, $4)`
	_, err := db.conn.ExecContext(ctx, query, deviceCode, userCode, keyName, expiresAt)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to create device code: %w", err)
	}
	return nil
}

// GetDeviceCodeByUserCode retrieves a device code by user code (for web verification page)
func (db *DB) GetDeviceCodeByUserCode(ctx context.Context, userCode string) (*DeviceCode, error) {
	ctx, span := tracer.Start(ctx, "db.get_device_code_by_user_code")
	defer span.End()

	query := `SELECT id, device_code, user_code, key_name, user_id, expires_at, authorized_at, created_at
	          FROM device_codes WHERE user_code = $1 AND expires_at > NOW()`

	var dc DeviceCode
	err := db.conn.QueryRowContext(ctx, query, userCode).Scan(
		&dc.ID, &dc.DeviceCode, &dc.UserCode, &dc.KeyName,
		&dc.UserID, &dc.ExpiresAt, &dc.AuthorizedAt, &dc.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDeviceCodeNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// GetDeviceCodeByDeviceCode retrieves a device code by device code (for CLI polling)
func (db *DB) GetDeviceCodeByDeviceCode(ctx context.Context, deviceCode string) (*DeviceCode, error) {
	ctx, span := tracer.Start(ctx, "db.get_device_code_by_device_code")
	defer span.End()

	query := `SELECT id, device_code, user_code, key_name, user_id, expires_at, authorized_at, created_at
	          FROM device_codes WHERE device_code = $1`

	var dc DeviceCode
	err := db.conn.QueryRowContext(ctx, query, deviceCode).Scan(
		&dc.ID, &dc.DeviceCode, &dc.UserCode, &dc.KeyName,
		&dc.UserID, &dc.ExpiresAt, &dc.AuthorizedAt, &dc.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrDeviceCodeNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// AuthorizeDeviceCode marks a device code as authorized by a user
func (db *DB) AuthorizeDeviceCode(ctx context.Context, userCode string, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.authorize_device_code",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `UPDATE device_codes SET user_id = $1, authorized_at = NOW()
	          WHERE user_code = $2 AND expires_at > NOW() AND authorized_at IS NULL`

	result, err := db.conn.ExecContext(ctx, query, userID, userCode)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to authorize device code: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeviceCodeNotFound
	}
	return nil
}

// DeleteDeviceCode removes a device code (after successful token exchange or expiration)
func (db *DB) DeleteDeviceCode(ctx context.Context, deviceCode string) error {
	ctx, span := tracer.Start(ctx, "db.delete_device_code")
	defer span.End()

	query := `DELETE FROM device_codes WHERE device_code = $1`
	_, err := db.conn.ExecContext(ctx, query, deviceCode)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete device code: %w", err)
	}
	return nil
}

// CleanupExpiredDeviceCodes removes expired device codes
func (db *DB) CleanupExpiredDeviceCodes(ctx context.Context) (int64, error) {
	ctx, span := tracer.Start(ctx, "db.cleanup_expired_device_codes")
	defer span.End()

	query := `DELETE FROM device_codes WHERE expires_at < NOW()`
	result, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, fmt.Errorf("failed to cleanup expired device codes: %w", err)
	}
	rows, _ := result.RowsAffected()
	span.SetAttributes(attribute.Int64("codes.deleted", rows))
	return rows, nil
}
