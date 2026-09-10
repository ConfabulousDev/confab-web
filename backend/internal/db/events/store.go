package events

import (
	"database/sql"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// Store provides session event database operations.
type Store struct {
	DB *db.DB
}

// conn returns the underlying *sql.DB connection.
func (s *Store) conn() *sql.DB { return s.DB.Conn() }
