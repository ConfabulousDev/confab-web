package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/models"
	_ "github.com/lib/pq"
)

// DB wraps a PostgreSQL database connection
type DB struct {
	conn *sql.DB
}

// Connect establishes a connection to PostgreSQL
func Connect(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// RunMigrations applies database migrations
func (db *DB) RunMigrations() error {
	schema := `
	-- Users table (OAuth-based authentication)
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		name TEXT,
		avatar_url TEXT,
		github_id TEXT UNIQUE,
		google_id TEXT UNIQUE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Web sessions table (for browser authentication via OAuth)
	CREATE TABLE IF NOT EXISTS web_sessions (
		id TEXT PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL
	);

	-- API Keys table
	CREATE TABLE IF NOT EXISTS api_keys (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		key_hash TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Sessions table (mirrors SQLite)
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		first_seen TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Runs table (execution instances)
	CREATE TABLE IF NOT EXISTS runs (
		id BIGSERIAL PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		transcript_path TEXT NOT NULL,
		cwd TEXT NOT NULL,
		reason TEXT NOT NULL,
		end_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
		s3_uploaded BOOLEAN NOT NULL DEFAULT FALSE
	);

	-- Files table
	CREATE TABLE IF NOT EXISTS files (
		id BIGSERIAL PRIMARY KEY,
		run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
		file_path TEXT NOT NULL,
		file_type TEXT NOT NULL,
		size_bytes BIGINT NOT NULL,
		s3_key TEXT,
		s3_uploaded_at TIMESTAMP
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_users_github_id ON users(github_id);
	CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id);
	CREATE INDEX IF NOT EXISTS idx_web_sessions_user ON web_sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_web_sessions_expires ON web_sessions(expires_at);
	CREATE INDEX IF NOT EXISTS idx_api_keys_user ON api_keys(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_runs_session ON runs(session_id);
	CREATE INDEX IF NOT EXISTS idx_runs_user ON runs(user_id);
	CREATE INDEX IF NOT EXISTS idx_runs_end_timestamp ON runs(end_timestamp);
	CREATE INDEX IF NOT EXISTS idx_files_run ON files(run_id);

	-- Create default user if not exists
	INSERT INTO users (id, email, created_at)
	VALUES (1, 'default@confab.local', NOW())
	ON CONFLICT (id) DO NOTHING;
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `SELECT id, email, created_at FROM users WHERE id = $1`

	var user models.User
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// ValidateAPIKey checks if an API key is valid and returns the associated user ID
func (db *DB) ValidateAPIKey(ctx context.Context, keyHash string) (int64, error) {
	query := `SELECT user_id FROM api_keys WHERE key_hash = $1`

	var userID int64
	err := db.conn.QueryRowContext(ctx, query, keyHash).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("invalid API key")
		}
		return 0, fmt.Errorf("failed to validate API key: %w", err)
	}

	return userID, nil
}

// CreateAPIKey creates a new API key for a user
func (db *DB) CreateAPIKey(ctx context.Context, userID int64, keyHash, name string) error {
	query := `INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3)`

	_, err := db.conn.ExecContext(ctx, query, userID, keyHash, name)
	if err != nil {
		return fmt.Errorf("failed to create API key: %w", err)
	}

	return nil
}

// SaveSession stores a session with its run and files in a transaction
func (db *DB) SaveSession(ctx context.Context, userID int64, req *models.SaveSessionRequest, s3Keys map[string]string) (int64, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Insert session if doesn't exist (first time seeing this session_id)
	sessionSQL := `INSERT INTO sessions (session_id, user_id, first_seen) VALUES ($1, $2, $3) ON CONFLICT (session_id) DO NOTHING`
	_, err = tx.ExecContext(ctx, sessionSQL, req.SessionID, userID, now)
	if err != nil {
		return 0, fmt.Errorf("failed to insert session: %w", err)
	}

	// Always insert a new run
	runSQL := `
		INSERT INTO runs (session_id, user_id, transcript_path, cwd, reason, end_timestamp, s3_uploaded)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	var runID int64
	err = tx.QueryRowContext(ctx, runSQL,
		req.SessionID,
		userID,
		req.TranscriptPath,
		req.CWD,
		req.Reason,
		now,
		len(s3Keys) > 0,
	).Scan(&runID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert run: %w", err)
	}

	// Insert files linked to this run
	fileSQL := `
		INSERT INTO files (run_id, file_path, file_type, size_bytes, s3_key, s3_uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	for _, f := range req.Files {
		s3Key, uploaded := s3Keys[f.Path]
		var s3UploadedAt *time.Time
		var s3KeyPtr *string
		if uploaded {
			s3UploadedAt = &now
			s3KeyPtr = &s3Key
		}

		_, err = tx.ExecContext(ctx, fileSQL, runID, f.Path, f.Type, f.SizeBytes, s3KeyPtr, s3UploadedAt)
		if err != nil {
			return 0, fmt.Errorf("failed to insert file: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return runID, nil
}

// FindOrCreateUserByGitHub finds or creates a user by GitHub ID
func (db *DB) FindOrCreateUserByGitHub(ctx context.Context, githubID, email, name, avatarURL string) (*models.User, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Try to find existing user by GitHub ID
	query := `SELECT id, email, name, avatar_url, github_id, google_id, created_at, updated_at
	          FROM users WHERE github_id = $1`

	var user models.User
	err = tx.QueryRowContext(ctx, query, githubID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.GitHubID,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == nil {
		// User exists, update their info
		updateSQL := `UPDATE users SET email = $1, name = $2, avatar_url = $3, updated_at = NOW() WHERE id = $4`
		_, err = tx.ExecContext(ctx, updateSQL, email, name, avatarURL, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// User doesn't exist, create new one
	insertSQL := `INSERT INTO users (email, name, avatar_url, github_id, created_at, updated_at)
	              VALUES ($1, $2, $3, $4, NOW(), NOW())
	              RETURNING id, email, name, avatar_url, github_id, google_id, created_at, updated_at`

	err = tx.QueryRowContext(ctx, insertSQL, email, name, avatarURL, githubID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.GitHubID,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &user, nil
}

// CreateWebSession creates a new web session for a user
func (db *DB) CreateWebSession(ctx context.Context, sessionID string, userID int64, expiresAt time.Time) error {
	query := `INSERT INTO web_sessions (id, user_id, created_at, expires_at) VALUES ($1, $2, NOW(), $3)`
	_, err := db.conn.ExecContext(ctx, query, sessionID, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create web session: %w", err)
	}
	return nil
}

// GetWebSession retrieves a web session by ID and validates it's not expired
func (db *DB) GetWebSession(ctx context.Context, sessionID string) (*models.WebSession, error) {
	query := `SELECT id, user_id, created_at, expires_at
	          FROM web_sessions
	          WHERE id = $1 AND expires_at > NOW()`

	var session models.WebSession
	err := db.conn.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// DeleteWebSession deletes a web session (logout)
func (db *DB) DeleteWebSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM web_sessions WHERE id = $1`
	_, err := db.conn.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// CleanupExpiredSessions removes expired web sessions
func (db *DB) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	query := `DELETE FROM web_sessions WHERE expires_at < NOW()`
	result, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sessions: %w", err)
	}
	count, _ := result.RowsAffected()
	return count, nil
}
