package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"github.com/santaclaude2025/confab/backend/internal/models"
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

	// Configure connection pool
	// MaxOpenConns: Limit total connections to avoid overwhelming the database
	conn.SetMaxOpenConns(25)
	// MaxIdleConns: Keep some connections ready for reuse, but not too many
	conn.SetMaxIdleConns(5)
	// ConnMaxLifetime: Recycle connections periodically to avoid stale connections
	conn.SetConnMaxLifetime(5 * time.Minute)

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Exec executes a query without returning rows (for testing/migrations)
func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row (for testing)
func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
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
		github_username TEXT,
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
		s3_uploaded BOOLEAN NOT NULL DEFAULT FALSE,
		git_info JSONB,
		source TEXT NOT NULL DEFAULT 'hook',
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
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

	-- Session shares table
	CREATE TABLE IF NOT EXISTS session_shares (
		id BIGSERIAL PRIMARY KEY,
		session_id TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		share_token TEXT NOT NULL UNIQUE,
		visibility TEXT NOT NULL,
		expires_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_accessed_at TIMESTAMP
	);

	-- Session share invites table (for private shares)
	CREATE TABLE IF NOT EXISTS session_share_invites (
		id BIGSERIAL PRIMARY KEY,
		share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
		email TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(share_id, email)
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
	CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs(created_at);
	CREATE INDEX IF NOT EXISTS idx_files_run ON files(run_id);
	CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token);
	CREATE INDEX IF NOT EXISTS idx_session_shares_session ON session_shares(session_id, user_id);
	CREATE INDEX IF NOT EXISTS idx_session_share_invites_share ON session_share_invites(share_id);
	CREATE INDEX IF NOT EXISTS idx_session_share_invites_email ON session_share_invites(email);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Add git_info column to existing runs table (for databases created before this migration)
	alterGitInfo := `ALTER TABLE runs ADD COLUMN IF NOT EXISTS git_info JSONB;`
	if _, err := db.conn.Exec(alterGitInfo); err != nil {
		return fmt.Errorf("failed to add git_info column: %w", err)
	}

	// Add source column to existing runs table (for databases created before this migration)
	alterSource := `ALTER TABLE runs ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'hook';`
	if _, err := db.conn.Exec(alterSource); err != nil {
		return fmt.Errorf("failed to add source column: %w", err)
	}

	// Add created_at column to existing runs table (for databases created before this migration)
	// First add the column without NOT NULL constraint
	alterCreatedAt := `ALTER TABLE runs ADD COLUMN IF NOT EXISTS created_at TIMESTAMP;`
	if _, err := db.conn.Exec(alterCreatedAt); err != nil {
		return fmt.Errorf("failed to add created_at column: %w", err)
	}

	// Backfill created_at with end_timestamp for existing rows
	backfillCreatedAt := `UPDATE runs SET created_at = end_timestamp WHERE created_at IS NULL;`
	if _, err := db.conn.Exec(backfillCreatedAt); err != nil {
		return fmt.Errorf("failed to backfill created_at: %w", err)
	}

	// Now make it NOT NULL with default for new rows
	alterCreatedAtNotNull := `ALTER TABLE runs ALTER COLUMN created_at SET NOT NULL, ALTER COLUMN created_at SET DEFAULT NOW();`
	if _, err := db.conn.Exec(alterCreatedAtNotNull); err != nil {
		return fmt.Errorf("failed to set created_at constraints: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `SELECT id, email, name, avatar_url, github_id, github_username, google_id, created_at, updated_at FROM users WHERE id = $1`

	var user models.User
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.GitHubID,
		&user.GitHubUsername,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
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

// CreateAPIKeyWithReturn creates a new API key and returns the key ID and created_at
func (db *DB) CreateAPIKeyWithReturn(ctx context.Context, userID int64, keyHash, name string) (int64, time.Time, error) {
	query := `INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, created_at`

	var keyID int64
	var createdAt time.Time
	err := db.conn.QueryRowContext(ctx, query, userID, keyHash, name).Scan(&keyID, &createdAt)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to create API key: %w", err)
	}

	return keyID, createdAt, nil
}

// ListAPIKeys returns all API keys for a user (without hashes)
func (db *DB) ListAPIKeys(ctx context.Context, userID int64) ([]models.APIKey, error) {
	query := `SELECT id, user_id, name, created_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var key models.APIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan API key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// DeleteAPIKey deletes an API key
func (db *DB) DeleteAPIKey(ctx context.Context, userID, keyID int64) error {
	query := `DELETE FROM api_keys WHERE id = $1 AND user_id = $2`

	result, err := db.conn.ExecContext(ctx, query, keyID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrAPIKeyNotFound
	}

	return nil
}

// SaveSession stores a session with its run and files in a transaction
func (db *DB) SaveSession(ctx context.Context, userID int64, req *models.SaveSessionRequest, s3Keys map[string]string, source string) (int64, error) {
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

	// Marshal GitInfo to JSON for JSONB column
	var gitInfoJSON interface{} = nil // Explicitly nil for NULL in database
	if req.GitInfo != nil {
		jsonBytes, err := json.Marshal(req.GitInfo)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal git_info: %w", err)
		}
		gitInfoJSON = jsonBytes
	}

	// Always insert a new run
	runSQL := `
		INSERT INTO runs (session_id, user_id, transcript_path, cwd, reason, end_timestamp, s3_uploaded, git_info, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	if source == "" {
		source = "hook"
	}
	var runID int64
	err = tx.QueryRowContext(ctx, runSQL,
		req.SessionID,
		userID,
		req.TranscriptPath,
		req.CWD,
		req.Reason,
		now,
		len(s3Keys) > 0,
		gitInfoJSON,
		source,
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
func (db *DB) FindOrCreateUserByGitHub(ctx context.Context, githubID, githubUsername, email, name, avatarURL string) (*models.User, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer rollback - will be a no-op if commit succeeds
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Try to find existing user by GitHub ID
	query := `SELECT id, email, name, avatar_url, github_id, github_username, google_id, created_at, updated_at
	          FROM users WHERE github_id = $1`

	var user models.User
	err = tx.QueryRowContext(ctx, query, githubID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.GitHubID,
		&user.GitHubUsername,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == nil {
		// User exists, update their info
		updateSQL := `UPDATE users SET email = $1, name = $2, avatar_url = $3, github_username = $4, updated_at = NOW() WHERE id = $5`
		_, err = tx.ExecContext(ctx, updateSQL, email, name, avatarURL, githubUsername, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
		err = tx.Commit()
		if err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// User doesn't exist, create new one
	insertSQL := `INSERT INTO users (email, name, avatar_url, github_id, github_username, created_at, updated_at)
	              VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	              RETURNING id, email, name, avatar_url, github_id, github_username, google_id, created_at, updated_at`

	err = tx.QueryRowContext(ctx, insertSQL, email, name, avatarURL, githubID, githubUsername).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
		&user.GitHubID,
		&user.GitHubUsername,
		&user.GoogleID,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	err = tx.Commit()
	if err != nil {
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

// SessionListItem represents a session in the list view
type SessionListItem struct {
	SessionID   string    `json:"session_id"`
	FirstSeen   time.Time `json:"first_seen"`
	RunCount    int       `json:"run_count"`
	LastRunTime time.Time `json:"last_run_time"`
}

// ListUserSessions returns all sessions for a user
func (db *DB) ListUserSessions(ctx context.Context, userID int64) ([]SessionListItem, error) {
	query := `
		SELECT
			s.session_id,
			s.first_seen,
			COUNT(r.id) as run_count,
			COALESCE(MAX(r.created_at), s.first_seen) as last_run_time
		FROM sessions s
		LEFT JOIN runs r ON s.session_id = r.session_id
		WHERE s.user_id = $1
		GROUP BY s.session_id, s.first_seen
		ORDER BY last_run_time DESC
	`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionListItem
	for rows.Next() {
		var session SessionListItem
		if err := rows.Scan(&session.SessionID, &session.FirstSeen, &session.RunCount, &session.LastRunTime); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// SessionDetail represents detailed session information
type SessionDetail struct {
	SessionID string      `json:"session_id"`
	FirstSeen time.Time   `json:"first_seen"`
	Runs      []RunDetail `json:"runs"`
}

// RunDetail represents a run with its files
type RunDetail struct {
	ID             int64        `json:"id"`
	EndTimestamp   time.Time    `json:"end_timestamp"`
	CWD            string       `json:"cwd"`
	Reason         string       `json:"reason"`
	TranscriptPath string       `json:"transcript_path"`
	S3Uploaded     bool         `json:"s3_uploaded"`
	GitInfo        interface{}  `json:"git_info,omitempty"`
	Files          []FileDetail `json:"files"`
}

// FileDetail represents a file in a run
type FileDetail struct {
	ID           int64      `json:"id"`
	FilePath     string     `json:"file_path"`
	FileType     string     `json:"file_type"`
	SizeBytes    int64      `json:"size_bytes"`
	S3Key        *string    `json:"s3_key,omitempty"`
	S3UploadedAt *time.Time `json:"s3_uploaded_at,omitempty"`
}

// GetSessionDetail returns detailed information about a session
func (db *DB) GetSessionDetail(ctx context.Context, sessionID string, userID int64) (*SessionDetail, error) {
	// First, get the session and verify ownership
	var session SessionDetail
	sessionQuery := `SELECT session_id, first_seen FROM sessions WHERE session_id = $1 AND user_id = $2`
	err := db.conn.QueryRowContext(ctx, sessionQuery, sessionID, userID).Scan(&session.SessionID, &session.FirstSeen)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get all runs for this session
	runsQuery := `
		SELECT id, end_timestamp, cwd, reason, transcript_path, s3_uploaded, git_info
		FROM runs
		WHERE session_id = $1 AND user_id = $2
		ORDER BY end_timestamp ASC
	`

	rows, err := db.conn.QueryContext(ctx, runsQuery, sessionID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var run RunDetail
		var gitInfoBytes []byte
		if err := rows.Scan(&run.ID, &run.EndTimestamp, &run.CWD, &run.Reason, &run.TranscriptPath, &run.S3Uploaded, &gitInfoBytes); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		// Unmarshal git_info JSONB if present
		if len(gitInfoBytes) > 0 {
			if err := json.Unmarshal(gitInfoBytes, &run.GitInfo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal git_info: %w", err)
			}
		}

		// Get files for this run
		filesQuery := `
			SELECT id, file_path, file_type, size_bytes, s3_key, s3_uploaded_at
			FROM files
			WHERE run_id = $1
			ORDER BY file_type DESC, file_path ASC
		`

		fileRows, err := db.conn.QueryContext(ctx, filesQuery, run.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query files: %w", err)
		}

		for fileRows.Next() {
			var file FileDetail
			if err := fileRows.Scan(&file.ID, &file.FilePath, &file.FileType, &file.SizeBytes, &file.S3Key, &file.S3UploadedAt); err != nil {
				fileRows.Close()
				return nil, fmt.Errorf("failed to scan file: %w", err)
			}
			run.Files = append(run.Files, file)
		}

		if err := fileRows.Err(); err != nil {
			fileRows.Close()
			return nil, fmt.Errorf("error iterating files: %w", err)
		}

		fileRows.Close()

		session.Runs = append(session.Runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating runs: %w", err)
	}

	return &session, nil
}

// CheckSessionsExist checks which session IDs exist for a user
// Returns the list of session IDs that already exist in the database
func (db *DB) CheckSessionsExist(ctx context.Context, userID int64, sessionIDs []string) ([]string, error) {
	if len(sessionIDs) == 0 {
		return []string{}, nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(sessionIDs))
	args := make([]interface{}, len(sessionIDs)+1)
	args[0] = userID
	for i, id := range sessionIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf(`
		SELECT session_id FROM sessions
		WHERE user_id = $1 AND session_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to check sessions: %w", err)
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var sessionID string
		if err := rows.Scan(&sessionID); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		existing = append(existing, sessionID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating existing sessions: %w", err)
	}

	return existing, nil
}

// SessionShare represents a share link
type SessionShare struct {
	ID             int64      `json:"id"`
	SessionID      string     `json:"session_id"`
	UserID         int64      `json:"user_id"`
	ShareToken     string     `json:"share_token"`
	Visibility     string     `json:"visibility"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	InvitedEmails  []string   `json:"invited_emails,omitempty"`
}

// CreateShare creates a new share link
func (db *DB) CreateShare(ctx context.Context, sessionID string, userID int64, shareToken, visibility string, expiresAt *time.Time, invitedEmails []string) (*SessionShare, error) {
	// Verify session ownership
	var ownerID int64
	err := db.conn.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify ownership: %w", err)
	}

	if ownerID != userID {
		return nil, ErrUnauthorized
	}

	// Insert share
	query := `INSERT INTO session_shares (session_id, user_id, share_token, visibility, expires_at)
	          VALUES ($1, $2, $3, $4, $5)
	          RETURNING id, created_at`

	var share SessionShare
	share.SessionID = sessionID
	share.UserID = userID
	share.ShareToken = shareToken
	share.Visibility = visibility
	share.ExpiresAt = expiresAt

	err = db.conn.QueryRowContext(ctx, query, sessionID, userID, shareToken, visibility, expiresAt).Scan(&share.ID, &share.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	// Insert invites for private shares
	if visibility == "private" && len(invitedEmails) > 0 {
		for _, email := range invitedEmails {
			_, err := db.conn.ExecContext(ctx,
				`INSERT INTO session_share_invites (share_id, email) VALUES ($1, $2)`,
				share.ID, email)
			if err != nil {
				// Rollback share if invite fails
				db.conn.ExecContext(ctx, `DELETE FROM session_shares WHERE id = $1`, share.ID)
				return nil, fmt.Errorf("failed to create invite: %w", err)
			}
		}
		share.InvitedEmails = invitedEmails
	}

	return &share, nil
}

// ListShares returns all shares for a session
func (db *DB) ListShares(ctx context.Context, sessionID string, userID int64) ([]SessionShare, error) {
	// Verify ownership
	var ownerID int64
	err := db.conn.QueryRowContext(ctx, `SELECT user_id FROM sessions WHERE session_id = $1`, sessionID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify ownership: %w", err)
	}

	if ownerID != userID {
		return nil, ErrUnauthorized
	}

	// Get shares
	query := `SELECT id, session_id, user_id, share_token, visibility, expires_at, created_at, last_accessed_at
	          FROM session_shares
	          WHERE session_id = $1 AND user_id = $2
	          ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, sessionID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	var shares []SessionShare
	for rows.Next() {
		var share SessionShare
		err := rows.Scan(&share.ID, &share.SessionID, &share.UserID, &share.ShareToken,
			&share.Visibility, &share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}

		// Get invited emails for private shares
		if share.Visibility == "private" {
			emailRows, err := db.conn.QueryContext(ctx,
				`SELECT email FROM session_share_invites WHERE share_id = $1 ORDER BY email`,
				share.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get invites: %w", err)
			}

			var emails []string
			for emailRows.Next() {
				var email string
				if err := emailRows.Scan(&email); err != nil {
					emailRows.Close()
					return nil, fmt.Errorf("failed to scan email: %w", err)
				}
				emails = append(emails, email)
			}

			if err := emailRows.Err(); err != nil {
				emailRows.Close()
				return nil, fmt.Errorf("error iterating emails: %w", err)
			}

			emailRows.Close()
			share.InvitedEmails = emails
		}

		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shares: %w", err)
	}

	return shares, nil
}

// RevokeShare deletes a share
func (db *DB) RevokeShare(ctx context.Context, shareToken string, userID int64) error {
	// Verify ownership and delete
	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM session_shares WHERE share_token = $1 AND user_id = $2`,
		shareToken, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke share: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Could be either not found or unauthorized - keeping combined error for security
		return ErrUnauthorized
	}

	return nil
}

// GetSharedSession returns session detail via share token
func (db *DB) GetSharedSession(ctx context.Context, sessionID, shareToken string, viewerEmail *string) (*SessionDetail, error) {
	// Get share
	var share SessionShare
	query := `SELECT id, session_id, user_id, visibility, expires_at, last_accessed_at
	          FROM session_shares
	          WHERE share_token = $1`

	err := db.conn.QueryRowContext(ctx, query, shareToken).Scan(
		&share.ID, &share.SessionID, &share.UserID, &share.Visibility,
		&share.ExpiresAt, &share.LastAccessedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrShareNotFound
		}
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	// Verify token belongs to this session
	if share.SessionID != sessionID {
		return nil, ErrShareNotFound
	}

	// Check expiration
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		return nil, ErrShareExpired
	}

	// Check authorization for private shares
	if share.Visibility == "private" {
		if viewerEmail == nil {
			return nil, ErrUnauthorized
		}

		// Check if email is invited
		var count int
		err := db.conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM session_share_invites WHERE share_id = $1 AND LOWER(email) = LOWER($2)`,
			share.ID, *viewerEmail).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("failed to check authorization: %w", err)
		}

		if count == 0 {
			return nil, ErrForbidden
		}
	}

	// Update last accessed
	db.conn.ExecContext(ctx,
		`UPDATE session_shares SET last_accessed_at = NOW() WHERE id = $1`,
		share.ID)

	// Get session detail (no ownership check since share verified)
	var session SessionDetail
	sessionQuery := `SELECT session_id, first_seen FROM sessions WHERE session_id = $1`
	err = db.conn.QueryRowContext(ctx, sessionQuery, sessionID).Scan(&session.SessionID, &session.FirstSeen)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Get runs
	runsQuery := `
		SELECT id, end_timestamp, cwd, reason, transcript_path, s3_uploaded, git_info
		FROM runs
		WHERE session_id = $1
		ORDER BY end_timestamp ASC
	`

	rows, err := db.conn.QueryContext(ctx, runsQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query runs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var run RunDetail
		var gitInfoBytes []byte
		if err := rows.Scan(&run.ID, &run.EndTimestamp, &run.CWD, &run.Reason, &run.TranscriptPath, &run.S3Uploaded, &gitInfoBytes); err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}

		// Unmarshal git_info JSONB if present
		if len(gitInfoBytes) > 0 {
			if err := json.Unmarshal(gitInfoBytes, &run.GitInfo); err != nil {
				return nil, fmt.Errorf("failed to unmarshal git_info: %w", err)
			}
		}

		// Get files
		filesQuery := `
			SELECT id, file_path, file_type, size_bytes, s3_key, s3_uploaded_at
			FROM files
			WHERE run_id = $1
			ORDER BY file_type DESC, file_path ASC
		`

		fileRows, err := db.conn.QueryContext(ctx, filesQuery, run.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to query files: %w", err)
		}

		for fileRows.Next() {
			var file FileDetail
			if err := fileRows.Scan(&file.ID, &file.FilePath, &file.FileType, &file.SizeBytes, &file.S3Key, &file.S3UploadedAt); err != nil {
				fileRows.Close()
				return nil, fmt.Errorf("failed to scan file: %w", err)
			}
			run.Files = append(run.Files, file)
		}

		if err := fileRows.Err(); err != nil {
			fileRows.Close()
			return nil, fmt.Errorf("error iterating files: %w", err)
		}

		fileRows.Close()

		session.Runs = append(session.Runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating runs: %w", err)
	}

	return &session, nil
}

// GetFileByID retrieves a file by ID with ownership verification
func (db *DB) GetFileByID(ctx context.Context, fileID int64, userID int64) (*FileDetail, error) {
	// Join through runs and sessions to verify ownership
	query := `
		SELECT f.id, f.file_path, f.file_type, f.size_bytes, f.s3_key, f.s3_uploaded_at
		FROM files f
		JOIN runs r ON f.run_id = r.id
		JOIN sessions s ON r.session_id = s.session_id
		WHERE f.id = $1 AND s.user_id = $2
	`

	var file FileDetail
	err := db.conn.QueryRowContext(ctx, query, fileID, userID).Scan(
		&file.ID,
		&file.FilePath,
		&file.FileType,
		&file.SizeBytes,
		&file.S3Key,
		&file.S3UploadedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Could be either not found or unauthorized - keeping combined for security
			return nil, ErrUnauthorized
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &file, nil
}

// GetSharedFileByID retrieves a file by ID via share token (for shared sessions)
func (db *DB) GetSharedFileByID(ctx context.Context, sessionID, shareToken string, fileID int64, viewerEmail *string) (*FileDetail, error) {
	// First validate the share token
	var share SessionShare
	shareQuery := `SELECT id, session_id, user_id, visibility, expires_at
	               FROM session_shares
	               WHERE share_token = $1 AND session_id = $2`

	err := db.conn.QueryRowContext(ctx, shareQuery, shareToken, sessionID).Scan(
		&share.ID, &share.SessionID, &share.UserID, &share.Visibility, &share.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrShareNotFound
		}
		return nil, fmt.Errorf("failed to get share: %w", err)
	}

	// Check expiration
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now()) {
		return nil, ErrShareExpired
	}

	// Check private share access
	if share.Visibility == "private" {
		if viewerEmail == nil {
			return nil, ErrUnauthorized
		}
		// Check if viewer is invited
		var count int
		inviteQuery := `SELECT COUNT(*) FROM session_share_invites
		                WHERE share_id = $1 AND email = $2`
		err = db.conn.QueryRowContext(ctx, inviteQuery, share.ID, *viewerEmail).Scan(&count)
		if err != nil || count == 0 {
			return nil, ErrUnauthorized
		}
	}

	// Get file, verifying it belongs to the shared session
	fileQuery := `
		SELECT f.id, f.file_path, f.file_type, f.size_bytes, f.s3_key, f.s3_uploaded_at
		FROM files f
		JOIN runs r ON f.run_id = r.id
		WHERE f.id = $1 AND r.session_id = $2
	`

	var file FileDetail
	err = db.conn.QueryRowContext(ctx, fileQuery, fileID, sessionID).Scan(
		&file.ID,
		&file.FilePath,
		&file.FileType,
		&file.SizeBytes,
		&file.S3Key,
		&file.S3UploadedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	// Update last accessed timestamp
	updateQuery := `UPDATE session_shares SET last_accessed_at = NOW() WHERE id = $1`
	db.conn.ExecContext(ctx, updateQuery, share.ID)

	return &file, nil
}
