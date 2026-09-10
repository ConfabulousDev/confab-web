package user

import (
	"database/sql"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// Store provides user database operations.
type Store struct {
	DB *db.DB
}

// conn returns the underlying *sql.DB connection.
func (s *Store) conn() *sql.DB { return s.DB.Conn() }
