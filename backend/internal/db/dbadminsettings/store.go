package dbadminsettings

import (
	"context"
	"database/sql"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// Setting represents a row in the admin_settings table.
type Setting struct {
	Key       string
	Value     string
	UpdatedAt time.Time
}

// Store provides admin settings database operations.
type Store struct {
	DB *db.DB
}

// conn returns the underlying *sql.DB connection.
func (s *Store) conn() *sql.DB { return s.DB.Conn() }

// Get retrieves a setting by key. Returns nil, nil when the key doesn't exist.
// A row with Value="" is returned as &Setting{Value: ""} (distinct from nil).
func (s *Store) Get(ctx context.Context, key string) (*Setting, error) {
	var setting Setting
	err := s.conn().QueryRowContext(ctx,
		`SELECT key, value, updated_at FROM admin_settings WHERE key = $1`, key,
	).Scan(&setting.Key, &setting.Value, &setting.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// Set creates or updates a setting. Uses upsert to handle both cases atomically.
func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO admin_settings (key, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()
	`, key, value)
	return err
}

// Delete removes a setting by key. No error if the key doesn't exist.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.conn().ExecContext(ctx,
		`DELETE FROM admin_settings WHERE key = $1`, key,
	)
	return err
}
