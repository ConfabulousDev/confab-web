package access

import (
	"database/sql"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("confab/db/access")

// Store provides session access and sharing database operations.
type Store struct {
	DB *db.DB
}

// conn returns the underlying *sql.DB connection.
func (s *Store) conn() *sql.DB { return s.DB.Conn() }
