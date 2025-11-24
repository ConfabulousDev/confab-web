package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// DB wraps a PostgreSQL database connection
type DB struct {
	conn *sql.DB
}

// Connect establishes a connection to PostgreSQL
func Connect(dsn string) (*DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	// MaxOpenConns: Limit total connections to avoid overwhelming the database
	conn.SetMaxOpenConns(500)
	// MaxIdleConns: Keep some connections ready for reuse, but not too many
	conn.SetMaxIdleConns(100)
	// ConnMaxLifetime: Recycle connections periodically to avoid stale connections
	conn.SetConnMaxLifetime(20 * time.Minute)

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
		email VARCHAR(255) NOT NULL UNIQUE,
		name VARCHAR(255),
		avatar_url TEXT,
		github_id VARCHAR(255) UNIQUE,
		github_username VARCHAR(255),
		google_id VARCHAR(255) UNIQUE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Web sessions table (for browser authentication via OAuth)
	CREATE TABLE IF NOT EXISTS web_sessions (
		id VARCHAR(64) PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMP NOT NULL
	);

	-- API Keys table
	CREATE TABLE IF NOT EXISTS api_keys (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		key_hash CHAR(64) NOT NULL UNIQUE,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		session_id VARCHAR(255) PRIMARY KEY,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		first_seen TIMESTAMP NOT NULL DEFAULT NOW(),
		title TEXT,
		session_type VARCHAR(50) NOT NULL DEFAULT 'Claude Code'
	);

	-- Runs table (execution instances)
	CREATE TABLE IF NOT EXISTS runs (
		id BIGSERIAL PRIMARY KEY,
		session_id VARCHAR(255) NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		transcript_path TEXT NOT NULL,
		cwd TEXT NOT NULL,
		reason TEXT NOT NULL,
		end_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
		s3_uploaded BOOLEAN NOT NULL DEFAULT FALSE,
		git_info JSONB,
		source VARCHAR(50) NOT NULL DEFAULT 'hook',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_activity TIMESTAMP NOT NULL DEFAULT NOW()
	);

	-- Files table
	CREATE TABLE IF NOT EXISTS files (
		id BIGSERIAL PRIMARY KEY,
		run_id BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
		file_path TEXT NOT NULL,
		file_type VARCHAR(50) NOT NULL,
		size_bytes BIGINT NOT NULL,
		s3_key TEXT,
		s3_uploaded_at TIMESTAMP
	);

	-- Session shares table
	CREATE TABLE IF NOT EXISTS session_shares (
		id BIGSERIAL PRIMARY KEY,
		session_id VARCHAR(255) NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		share_token CHAR(32) NOT NULL UNIQUE,
		visibility VARCHAR(20) NOT NULL,
		expires_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_accessed_at TIMESTAMP
	);

	-- Session share invites table (for private shares)
	CREATE TABLE IF NOT EXISTS session_share_invites (
		id BIGSERIAL PRIMARY KEY,
		share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
		email VARCHAR(255) NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(share_id, email)
	);

	-- Session share accesses table (tracks which users accessed which shares)
	CREATE TABLE IF NOT EXISTS session_share_accesses (
		id BIGSERIAL PRIMARY KEY,
		share_id BIGINT NOT NULL REFERENCES session_shares(id) ON DELETE CASCADE,
		user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		first_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_accessed_at TIMESTAMP NOT NULL DEFAULT NOW(),
		access_count INT NOT NULL DEFAULT 1,
		UNIQUE(share_id, user_id)
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
	CREATE INDEX IF NOT EXISTS idx_runs_last_activity ON runs(last_activity);
	CREATE INDEX IF NOT EXISTS idx_files_run ON files(run_id);
	CREATE INDEX IF NOT EXISTS idx_files_run_type_size ON files(run_id, file_type, size_bytes);
	CREATE INDEX IF NOT EXISTS idx_session_shares_token ON session_shares(share_token);
	CREATE INDEX IF NOT EXISTS idx_session_shares_session ON session_shares(session_id, user_id);
	CREATE INDEX IF NOT EXISTS idx_session_share_invites_share ON session_share_invites(share_id);
	CREATE INDEX IF NOT EXISTS idx_session_share_invites_email ON session_share_invites(email);
	CREATE INDEX IF NOT EXISTS idx_session_share_accesses_share ON session_share_accesses(share_id);
	CREATE INDEX IF NOT EXISTS idx_session_share_accesses_user ON session_share_accesses(user_id);
	`

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Backfill last_activity from created_at for existing rows
	backfillLastActivity := `UPDATE runs SET last_activity = created_at WHERE last_activity IS NULL;`
	if _, err := db.conn.Exec(backfillLastActivity); err != nil {
		return fmt.Errorf("failed to backfill last_activity: %w", err)
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

	now := time.Now().UTC()

	// Insert session if doesn't exist, or update title/session_type if provided
	sessionSQL := `
		INSERT INTO sessions (session_id, user_id, first_seen, title, session_type)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id) DO UPDATE SET
			title = CASE WHEN EXCLUDED.title IS NOT NULL AND EXCLUDED.title != '' THEN EXCLUDED.title ELSE sessions.title END,
			session_type = CASE WHEN EXCLUDED.session_type IS NOT NULL AND EXCLUDED.session_type != '' THEN EXCLUDED.session_type ELSE sessions.session_type END
	`
	sessionType := req.SessionType
	if sessionType == "" {
		sessionType = "Claude Code" // Default
	}
	_, err = tx.ExecContext(ctx, sessionSQL, req.SessionID, userID, now, req.Title, sessionType)
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
		INSERT INTO runs (session_id, user_id, transcript_path, cwd, reason, end_timestamp, s3_uploaded, git_info, source, last_activity)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
		req.LastActivity, // CLI always provides this field
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

// extractRepoName extracts the org/repo from a git URL
// Examples:
//   - "https://github.com/santaclaude2025/confab.git" -> "santaclaude2025/confab"
//   - "git@github.com:santaclaude2025/confab.git" -> "santaclaude2025/confab"
func extractRepoName(repoURL string) *string {
	// Remove .git suffix if present
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle HTTPS URLs: https://github.com/org/repo
	if strings.Contains(repoURL, "://") {
		parts := strings.Split(repoURL, "/")
		if len(parts) >= 2 {
			result := parts[len(parts)-2] + "/" + parts[len(parts)-1]
			return &result
		}
	}

	// Handle SSH URLs: git@github.com:org/repo
	if strings.Contains(repoURL, "@") && strings.Contains(repoURL, ":") {
		parts := strings.Split(repoURL, ":")
		if len(parts) == 2 {
			return &parts[1]
		}
	}

	// Fallback: return the original URL
	return &repoURL
}

// SessionListItem represents a session in the list view
type SessionListItem struct {
	SessionID         string    `json:"session_id"`
	FirstSeen         time.Time `json:"first_seen"`
	RunCount          int       `json:"run_count"`
	LastRunTime       time.Time `json:"last_run_time"`
	Title             *string   `json:"title,omitempty"`
	SessionType       string    `json:"session_type"`
	MaxTranscriptSize int64     `json:"max_transcript_size"`       // Max transcript size across all runs (0 = empty session)
	GitRepo           *string   `json:"git_repo,omitempty"`        // Git repository from latest run (e.g., "org/repo") - extracted from git_info JSONB
	GitBranch         *string   `json:"git_branch,omitempty"`      // Git branch from latest run - extracted from git_info JSONB
	IsOwner           bool      `json:"is_owner"`                  // true if user owns this session
	AccessType        string    `json:"access_type"`               // "owner" | "private_share" | "public_share"
	ShareToken        *string   `json:"share_token,omitempty"`     // share token if accessed via share
	SharedByEmail     *string   `json:"shared_by_email,omitempty"` // email of user who shared (if not owner)
}

// ListUserSessions returns all sessions for a user
// If includeShared is true, also includes sessions shared with the user (private shares and accessed public shares)
func (db *DB) ListUserSessions(ctx context.Context, userID int64, includeShared bool) ([]SessionListItem, error) {
	// Get user's email for private share matching
	var userEmail string
	emailQuery := `SELECT email FROM users WHERE id = $1`
	if err := db.conn.QueryRowContext(ctx, emailQuery, userID).Scan(&userEmail); err != nil {
		return nil, fmt.Errorf("failed to get user email: %w", err)
	}

	var query string
	if !includeShared {
		// Original query - only owned sessions
		query = `
			WITH latest_runs AS (
				SELECT DISTINCT ON (session_id)
					session_id,
					git_info->>'repo_url' as git_repo_url,
					git_info->>'branch' as git_branch
				FROM runs
				ORDER BY session_id, last_activity DESC
			)
			SELECT
				s.session_id,
				s.first_seen,
				COUNT(r.id) as run_count,
				COALESCE(MAX(r.last_activity), s.first_seen) as last_run_time,
				s.title,
				s.session_type,
				COALESCE(MAX(transcript_sizes.max_size), 0) as max_transcript_size,
				latest_runs.git_repo_url,
				latest_runs.git_branch,
				true as is_owner,
				'owner' as access_type,
				NULL::text as share_token,
				NULL::text as shared_by_email
			FROM sessions s
			LEFT JOIN runs r ON s.session_id = r.session_id
			LEFT JOIN (
				SELECT run_id, MAX(size_bytes) as max_size
				FROM files
				WHERE file_type = 'transcript'
				GROUP BY run_id
			) transcript_sizes ON r.id = transcript_sizes.run_id
			LEFT JOIN latest_runs ON s.session_id = latest_runs.session_id
			WHERE s.user_id = $1
			GROUP BY s.session_id, s.first_seen, s.title, s.session_type, latest_runs.git_repo_url, latest_runs.git_branch
			ORDER BY last_run_time DESC
		`
	} else {
		// Include owned + shared sessions via UNION
		query = `
			WITH latest_runs AS (
				SELECT DISTINCT ON (session_id)
					session_id,
					git_info->>'repo_url' as git_repo_url,
					git_info->>'branch' as git_branch
				FROM runs
				ORDER BY session_id, last_activity DESC
			),
			-- User's own sessions
			owned_sessions AS (
				SELECT
					s.session_id,
					s.first_seen,
					COUNT(r.id) as run_count,
					COALESCE(MAX(r.last_activity), s.first_seen) as last_run_time,
					s.title,
					s.session_type,
					COALESCE(MAX(transcript_sizes.max_size), 0) as max_transcript_size,
					latest_runs.git_repo_url,
					latest_runs.git_branch,
					true as is_owner,
					'owner' as access_type,
					NULL::text as share_token,
					NULL::text as shared_by_email
				FROM sessions s
				LEFT JOIN runs r ON s.session_id = r.session_id
				LEFT JOIN (
					SELECT run_id, MAX(size_bytes) as max_size
					FROM files
					WHERE file_type = 'transcript'
					GROUP BY run_id
				) transcript_sizes ON r.id = transcript_sizes.run_id
				LEFT JOIN latest_runs ON s.session_id = latest_runs.session_id
				WHERE s.user_id = $1
				GROUP BY s.session_id, s.first_seen, s.title, s.session_type,
				         latest_runs.git_repo_url, latest_runs.git_branch
			),
			-- Private shares (invited by email)
			private_shares AS (
				SELECT
					s.session_id,
					s.first_seen,
					COUNT(r.id) as run_count,
					COALESCE(MAX(r.last_activity), s.first_seen) as last_run_time,
					s.title,
					s.session_type,
					COALESCE(MAX(transcript_sizes.max_size), 0) as max_transcript_size,
					latest_runs.git_repo_url,
					latest_runs.git_branch,
					false as is_owner,
					'private_share' as access_type,
					sh.share_token,
					u.email as shared_by_email
				FROM sessions s
				JOIN session_shares sh ON s.session_id = sh.session_id
				JOIN session_share_invites si ON sh.id = si.share_id
				JOIN users u ON sh.user_id = u.id
				LEFT JOIN runs r ON s.session_id = r.session_id
				LEFT JOIN (
					SELECT run_id, MAX(size_bytes) as max_size
					FROM files
					WHERE file_type = 'transcript'
					GROUP BY run_id
				) transcript_sizes ON r.id = transcript_sizes.run_id
				LEFT JOIN latest_runs ON s.session_id = latest_runs.session_id
				WHERE sh.visibility = 'private'
				  AND LOWER(si.email) = LOWER($2)
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				GROUP BY s.session_id, s.first_seen, s.title, s.session_type,
				         latest_runs.git_repo_url, latest_runs.git_branch, sh.share_token, u.email
			),
			-- Public shares accessed by user
			public_shares AS (
				SELECT
					s.session_id,
					s.first_seen,
					COUNT(r.id) as run_count,
					COALESCE(MAX(r.last_activity), s.first_seen) as last_run_time,
					s.title,
					s.session_type,
					COALESCE(MAX(transcript_sizes.max_size), 0) as max_transcript_size,
					latest_runs.git_repo_url,
					latest_runs.git_branch,
					false as is_owner,
					'public_share' as access_type,
					sh.share_token,
					u.email as shared_by_email
				FROM sessions s
				JOIN session_shares sh ON s.session_id = sh.session_id
				JOIN session_share_accesses sa ON sh.id = sa.share_id
				JOIN users u ON sh.user_id = u.id
				LEFT JOIN runs r ON s.session_id = r.session_id
				LEFT JOIN (
					SELECT run_id, MAX(size_bytes) as max_size
					FROM files
					WHERE file_type = 'transcript'
					GROUP BY run_id
				) transcript_sizes ON r.id = transcript_sizes.run_id
				LEFT JOIN latest_runs ON s.session_id = latest_runs.session_id
				WHERE sh.visibility = 'public'
				  AND sa.user_id = $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				GROUP BY s.session_id, s.first_seen, s.title, s.session_type,
				         latest_runs.git_repo_url, latest_runs.git_branch, sh.share_token, u.email
			)
			SELECT * FROM owned_sessions
			UNION ALL
			SELECT * FROM private_shares
			UNION ALL
			SELECT * FROM public_shares
			ORDER BY last_run_time DESC
		`
	}

	var rows *sql.Rows
	var err error
	if !includeShared {
		rows, err = db.conn.QueryContext(ctx, query, userID)
	} else {
		rows, err = db.conn.QueryContext(ctx, query, userID, userEmail)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionListItem
	for rows.Next() {
		var session SessionListItem
		var gitRepoURL *string // Full URL from git_info JSONB
		if err := rows.Scan(
			&session.SessionID,
			&session.FirstSeen,
			&session.RunCount,
			&session.LastRunTime,
			&session.Title,
			&session.SessionType,
			&session.MaxTranscriptSize,
			&gitRepoURL,
			&session.GitBranch,
			&session.IsOwner,
			&session.AccessType,
			&session.ShareToken,
			&session.SharedByEmail,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// Extract org/repo from full git URL (e.g., "https://github.com/org/repo.git" -> "org/repo")
		if gitRepoURL != nil && *gitRepoURL != "" {
			session.GitRepo = extractRepoName(*gitRepoURL)
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
		defer fileRows.Close()

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
			defer emailRows.Close()

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

// ShareWithSessionInfo includes both share and session details
type ShareWithSessionInfo struct {
	SessionShare
	SessionTitle *string `json:"session_title,omitempty"`
}

// ListAllUserShares returns all shares for a user across all sessions
func (db *DB) ListAllUserShares(ctx context.Context, userID int64) ([]ShareWithSessionInfo, error) {
	// Get all shares for the user with session info
	query := `
		SELECT
			ss.id, ss.session_id, ss.user_id, ss.share_token, ss.visibility,
			ss.expires_at, ss.created_at, ss.last_accessed_at,
			s.title
		FROM session_shares ss
		JOIN sessions s ON ss.session_id = s.session_id
		WHERE ss.user_id = $1
		ORDER BY ss.created_at DESC
	`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	var shares []ShareWithSessionInfo
	for rows.Next() {
		var share ShareWithSessionInfo
		err := rows.Scan(
			&share.ID, &share.SessionID, &share.UserID, &share.ShareToken,
			&share.Visibility, &share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt,
			&share.SessionTitle,
		)
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
			defer emailRows.Close()

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
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now().UTC()) {
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
		defer fileRows.Close()

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

// RecordShareAccess records that a user accessed a share
// This is used to track public shares accessed by logged-in users
// so they can see them in their session list
func (db *DB) RecordShareAccess(ctx context.Context, shareToken string, userID int64) error {
	// First get the share ID
	var shareID int64
	query := `SELECT id FROM session_shares WHERE share_token = $1`
	err := db.conn.QueryRowContext(ctx, query, shareToken).Scan(&shareID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrShareNotFound
		}
		return fmt.Errorf("failed to get share: %w", err)
	}

	// Record access (INSERT or UPDATE existing record)
	upsertQuery := `
		INSERT INTO session_share_accesses (share_id, user_id, first_accessed_at, last_accessed_at, access_count)
		VALUES ($1, $2, NOW(), NOW(), 1)
		ON CONFLICT (share_id, user_id)
		DO UPDATE SET
			last_accessed_at = NOW(),
			access_count = session_share_accesses.access_count + 1
	`
	_, err = db.conn.ExecContext(ctx, upsertQuery, shareID, userID)
	if err != nil {
		return fmt.Errorf("failed to record share access: %w", err)
	}

	return nil
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
	if share.ExpiresAt != nil && share.ExpiresAt.Before(time.Now().UTC()) {
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

// GetRunS3Keys retrieves all S3 keys for files in a specific run
// Also verifies ownership and returns the session ID and run count
func (db *DB) GetRunS3Keys(ctx context.Context, runID int64, userID int64) (sessionID string, runCount int, s3Keys []string, err error) {
	// Verify ownership by joining through sessions and get run count
	ownershipQuery := `
		SELECT s.session_id, COUNT(r2.id) as run_count
		FROM runs r
		JOIN sessions s ON r.session_id = s.session_id
		LEFT JOIN runs r2 ON s.session_id = r2.session_id
		WHERE r.id = $1 AND s.user_id = $2
		GROUP BY s.session_id
	`
	err = db.conn.QueryRowContext(ctx, ownershipQuery, runID, userID).Scan(&sessionID, &runCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, nil, ErrUnauthorized
		}
		return "", 0, nil, fmt.Errorf("failed to verify ownership: %w", err)
	}

	// Get all S3 keys for files in this run
	query := `SELECT s3_key FROM files WHERE run_id = $1 AND s3_key IS NOT NULL`
	rows, err := db.conn.QueryContext(ctx, query, runID)
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to query S3 keys: %w", err)
	}
	defer rows.Close()

	s3Keys = []string{}
	for rows.Next() {
		var s3Key string
		if err := rows.Scan(&s3Key); err != nil {
			return "", 0, nil, fmt.Errorf("failed to scan S3 key: %w", err)
		}
		s3Keys = append(s3Keys, s3Key)
	}

	if err := rows.Err(); err != nil {
		return "", 0, nil, fmt.Errorf("error iterating S3 keys: %w", err)
	}

	return sessionID, runCount, s3Keys, nil
}

// GetSessionS3Keys retrieves all S3 keys for all files in all runs of a session
// Also verifies ownership
func (db *DB) GetSessionS3Keys(ctx context.Context, sessionID string, userID int64) ([]string, error) {
	// Verify ownership
	var ownerID int64
	ownershipQuery := `SELECT user_id FROM sessions WHERE session_id = $1`
	err := db.conn.QueryRowContext(ctx, ownershipQuery, sessionID).Scan(&ownerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify ownership: %w", err)
	}

	if ownerID != userID {
		return nil, ErrUnauthorized
	}

	// Get all S3 keys for all files in all runs of this session
	query := `
		SELECT f.s3_key
		FROM files f
		JOIN runs r ON f.run_id = r.id
		WHERE r.session_id = $1 AND f.s3_key IS NOT NULL
	`
	rows, err := db.conn.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query S3 keys: %w", err)
	}
	defer rows.Close()

	s3Keys := []string{}
	for rows.Next() {
		var s3Key string
		if err := rows.Scan(&s3Key); err != nil {
			return nil, fmt.Errorf("failed to scan S3 key: %w", err)
		}
		s3Keys = append(s3Keys, s3Key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating S3 keys: %w", err)
	}

	return s3Keys, nil
}

// DeleteRunFromDB deletes a single run from the database
// S3 objects must be deleted BEFORE calling this function
// If this is the only run for the session, the session is also deleted
func (db *DB) DeleteRunFromDB(ctx context.Context, runID int64, sessionID string, runCount int) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete the run (CASCADE will delete files)
	deleteRunQuery := `DELETE FROM runs WHERE id = $1`
	result, err := tx.ExecContext(ctx, deleteRunQuery, runID)
	if err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrRunNotFound
	}

	// If this was the only run, delete the session too
	if runCount == 1 {
		deleteSessionQuery := `DELETE FROM sessions WHERE session_id = $1`
		_, err = tx.ExecContext(ctx, deleteSessionQuery, sessionID)
		if err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteSessionFromDB deletes an entire session and all its runs from the database
// S3 objects must be deleted BEFORE calling this function
func (db *DB) DeleteSessionFromDB(ctx context.Context, sessionID string) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete the session (CASCADE will delete runs, files, shares, and share invites)
	deleteSessionQuery := `DELETE FROM sessions WHERE session_id = $1`
	result, err := tx.ExecContext(ctx, deleteSessionQuery, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrSessionNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CountUserRunsInLastWeek returns the number of runs uploaded by a user in the last 7 days
func (db *DB) CountUserRunsInLastWeek(ctx context.Context, userID int64) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM runs
		WHERE user_id = $1 AND created_at >= NOW() - INTERVAL '7 days'
	`

	var count int
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count runs: %w", err)
	}

	return count, nil
}

// WeeklyUsage represents a user's weekly upload usage statistics
type WeeklyUsage struct {
	CurrentCount int       `json:"runs_uploaded"`
	Limit        int       `json:"limit"`
	Remaining    int       `json:"remaining"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
}

// GetUserWeeklyUsage returns detailed weekly usage statistics for a user
func (db *DB) GetUserWeeklyUsage(ctx context.Context, userID int64, maxRunsPerWeek int) (*WeeklyUsage, error) {
	count, err := db.CountUserRunsInLastWeek(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	periodStart := now.Add(-7 * 24 * time.Hour)

	remaining := maxRunsPerWeek - count
	if remaining < 0 {
		remaining = 0
	}

	return &WeeklyUsage{
		CurrentCount: count,
		Limit:        maxRunsPerWeek,
		Remaining:    remaining,
		PeriodStart:  periodStart,
		PeriodEnd:    now,
	}, nil
}
