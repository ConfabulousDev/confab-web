package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/santaclaude/confab/pkg/types"
)

const (
	dbFileName = "sessions.db"
	dbDirName  = ".confab"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
	path string
}

// Open opens or creates the confab database
func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbDir := filepath.Join(home, dbDirName)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, dbFileName)

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: dbPath,
	}

	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, err
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// initSchema creates tables if they don't exist
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		transcript_path TEXT NOT NULL,
		cwd TEXT NOT NULL,
		reason TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		file_count INTEGER NOT NULL,
		total_size_bytes INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		file_path TEXT NOT NULL,
		file_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(session_id)
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_timestamp ON sessions(timestamp);
	CREATE INDEX IF NOT EXISTS idx_files_session ON files(session_id);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// InsertSession stores a session and its files
func (db *DB) InsertSession(hookInput *types.HookInput, files []types.SessionFile) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Calculate total size
	var totalSize int64
	for _, f := range files {
		totalSize += f.SizeBytes
	}

	// Insert session
	sessionSQL := `
		INSERT INTO sessions (session_id, transcript_path, cwd, reason, timestamp, file_count, total_size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(sessionSQL,
		hookInput.SessionID,
		hookInput.TranscriptPath,
		hookInput.CWD,
		hookInput.Reason,
		time.Now(),
		len(files),
		totalSize,
	)
	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	// Insert files
	fileSQL := `
		INSERT INTO files (session_id, file_path, file_type, size_bytes)
		VALUES (?, ?, ?, ?)
	`
	for _, f := range files {
		_, err = tx.Exec(fileSQL, hookInput.SessionID, f.Path, f.Type, f.SizeBytes)
		if err != nil {
			return fmt.Errorf("failed to insert file: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetRecentSessions returns the N most recent sessions
func (db *DB) GetRecentSessions(limit int) ([]types.Session, error) {
	query := `
		SELECT session_id, transcript_path, cwd, reason, timestamp, file_count, total_size_bytes
		FROM sessions
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []types.Session
	for rows.Next() {
		var s types.Session
		if err := rows.Scan(
			&s.SessionID,
			&s.TranscriptPath,
			&s.CWD,
			&s.Reason,
			&s.Timestamp,
			&s.FileCount,
			&s.TotalSizeBytes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// GetSessionCount returns the total number of sessions
func (db *DB) GetSessionCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	return count, err
}
