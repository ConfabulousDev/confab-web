// Package cursor provides database operations for the cursor_session_meta
// sidecar table — per-session Cursor metadata (the model name) that has no home
// on the generic sessions table and no representation in the synced JSONL.
//
// It mirrors the internal/db/codex package shape: a Store wrapping *db.DB with a
// conn() helper, one upsert with first-non-empty-wins semantics, and a read-back
// the analytics step uses to populate the session card's models_used.
package cursor

import (
	"database/sql"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("confab/db/cursor")

// Store provides cursor_session_meta database operations.
type Store struct {
	DB *db.DB
}

// conn returns the underlying *sql.DB connection.
func (s *Store) conn() *sql.DB { return s.DB.Conn() }
