package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

// Conn returns the underlying *sql.DB connection.
// Used by testutil to run migrations in tests.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	query := `SELECT id, email, name, avatar_url, created_at, updated_at FROM users WHERE id = $1`

	var user models.User
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.AvatarURL,
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

// ValidateAPIKey checks if an API key is valid and returns the associated user ID and key ID
func (db *DB) ValidateAPIKey(ctx context.Context, keyHash string) (userID int64, keyID int64, err error) {
	query := `SELECT id, user_id FROM api_keys WHERE key_hash = $1`

	err = db.conn.QueryRowContext(ctx, query, keyHash).Scan(&keyID, &userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, fmt.Errorf("invalid API key")
		}
		return 0, 0, fmt.Errorf("failed to validate API key: %w", err)
	}

	return userID, keyID, nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key
func (db *DB) UpdateAPIKeyLastUsed(ctx context.Context, keyID int64) error {
	query := `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`
	_, err := db.conn.ExecContext(ctx, query, keyID)
	if err != nil {
		return fmt.Errorf("failed to update API key last used: %w", err)
	}
	return nil
}

// MaxAPIKeysPerUser is the maximum number of API keys a user can have
const MaxAPIKeysPerUser = 100

// CountAPIKeys returns the number of API keys for a user
func (db *DB) CountAPIKeys(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM api_keys WHERE user_id = $1`
	var count int
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count API keys: %w", err)
	}
	return count, nil
}

// CreateAPIKeyWithReturn creates a new API key and returns the key ID and created_at
// Returns ErrAPIKeyLimitExceeded if the user already has MaxAPIKeysPerUser keys
func (db *DB) CreateAPIKeyWithReturn(ctx context.Context, userID int64, keyHash, name string) (int64, time.Time, error) {
	// Check if user has reached the limit
	count, err := db.CountAPIKeys(ctx, userID)
	if err != nil {
		return 0, time.Time{}, err
	}
	if count >= MaxAPIKeysPerUser {
		return 0, time.Time{}, ErrAPIKeyLimitExceeded
	}

	query := `INSERT INTO api_keys (user_id, key_hash, name) VALUES ($1, $2, $3) RETURNING id, created_at`

	var keyID int64
	var createdAt time.Time
	err = db.conn.QueryRowContext(ctx, query, userID, keyHash, name).Scan(&keyID, &createdAt)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed to create API key: %w", err)
	}

	return keyID, createdAt, nil
}

// ListAPIKeys returns all API keys for a user (without hashes)
func (db *DB) ListAPIKeys(ctx context.Context, userID int64) ([]models.APIKey, error) {
	query := `SELECT id, user_id, name, created_at, last_used_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	defer rows.Close()

	var keys []models.APIKey
	for rows.Next() {
		var key models.APIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.Name, &key.CreatedAt, &key.LastUsedAt); err != nil {
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

// NOTE: Legacy CreateSessionRun, AddFilesToRun, DeleteRun removed - using sync API instead

// FindOrCreateUserByOAuth finds or creates a user by OAuth provider identity.
// It handles account linking: if an identity doesn't exist but the email matches
// an existing user, it links the new identity to that user.
func (db *DB) FindOrCreateUserByOAuth(ctx context.Context, info models.OAuthUserInfo) (*models.User, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Try to find existing user by provider identity
	query := `
		SELECT u.id, u.email, u.name, u.avatar_url, u.created_at, u.updated_at
		FROM users u
		JOIN user_identities i ON u.id = i.user_id
		WHERE i.provider = $1 AND i.provider_id = $2
	`
	var user models.User
	err = tx.QueryRowContext(ctx, query, info.Provider, info.ProviderID).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		// User found via identity - update profile info and username
		updateSQL := `UPDATE users SET email = $1, name = $2, avatar_url = $3, updated_at = NOW() WHERE id = $4`
		if _, err = tx.ExecContext(ctx, updateSQL, info.Email, info.Name, info.AvatarURL, user.ID); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		// Update provider username if changed
		if info.ProviderUsername != "" {
			updateIdentitySQL := `UPDATE user_identities SET provider_username = $1 WHERE user_id = $2 AND provider = $3`
			if _, err = tx.ExecContext(ctx, updateIdentitySQL, info.ProviderUsername, user.ID, info.Provider); err != nil {
				return nil, fmt.Errorf("failed to update identity: %w", err)
			}
		}

		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user by identity: %w", err)
	}

	// 2. Identity not found - check if email exists (for account linking)
	emailQuery := `SELECT id, email, name, avatar_url, created_at, updated_at FROM users WHERE email = $1`
	err = tx.QueryRowContext(ctx, emailQuery, info.Email).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == nil {
		// User exists with same email - link this identity to their account
		linkSQL := `INSERT INTO user_identities (user_id, provider, provider_id, provider_username, created_at)
		            VALUES ($1, $2, $3, $4, NOW())`
		var username *string
		if info.ProviderUsername != "" {
			username = &info.ProviderUsername
		}
		if _, err = tx.ExecContext(ctx, linkSQL, user.ID, info.Provider, info.ProviderID, username); err != nil {
			return nil, fmt.Errorf("failed to link identity: %w", err)
		}

		// Update profile with latest info
		updateSQL := `UPDATE users SET name = $1, avatar_url = $2, updated_at = NOW() WHERE id = $3`
		if _, err = tx.ExecContext(ctx, updateSQL, info.Name, info.AvatarURL, user.ID); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}

		if err = tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}

	// 3. New user - create user and identity
	insertUserSQL := `
		INSERT INTO users (email, name, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, email, name, avatar_url, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, insertUserSQL, info.Email, info.Name, info.AvatarURL).Scan(
		&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create identity
	insertIdentitySQL := `INSERT INTO user_identities (user_id, provider, provider_id, provider_username, created_at)
	                      VALUES ($1, $2, $3, $4, NOW())`
	var username *string
	if info.ProviderUsername != "" {
		username = &info.ProviderUsername
	}
	if _, err = tx.ExecContext(ctx, insertIdentitySQL, user.ID, info.Provider, info.ProviderID, username); err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	if err = tx.Commit(); err != nil {
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
	ID            string     `json:"id"`                        // UUID primary key for URL routing
	ExternalID    string     `json:"external_id"`               // External system's session ID (e.g., Claude Code's ID)
	FirstSeen     time.Time  `json:"first_seen"`
	FileCount     int        `json:"file_count"`                // Number of sync files
	LastSyncTime  *time.Time `json:"last_sync_time,omitempty"`  // Last sync timestamp
	Title         *string    `json:"title,omitempty"`
	SessionType   string     `json:"session_type"`
	TotalLines    int64      `json:"total_lines"`               // Sum of last_synced_line across all files
	GitRepo       *string    `json:"git_repo,omitempty"`        // Git repository (e.g., "org/repo") - extracted from git_info JSONB
	GitBranch     *string    `json:"git_branch,omitempty"`      // Git branch - extracted from git_info JSONB
	IsOwner       bool       `json:"is_owner"`                  // true if user owns this session
	AccessType    string     `json:"access_type"`               // "owner" | "private_share" | "public_share"
	ShareToken    *string    `json:"share_token,omitempty"`     // share token if accessed via share
	SharedByEmail *string    `json:"shared_by_email,omitempty"` // email of user who shared (if not owner)
}

// ListUserSessions returns all sessions for a user
// If includeShared is true, also includes sessions shared with the user (private shares and accessed public shares)
// Uses sync_files table for file counts and sync state
func (db *DB) ListUserSessions(ctx context.Context, userID int64, includeShared bool) ([]SessionListItem, error) {
	// Get user's email for private share matching
	var userEmail string
	emailQuery := `SELECT email FROM users WHERE id = $1`
	if err := db.conn.QueryRowContext(ctx, emailQuery, userID).Scan(&userEmail); err != nil {
		return nil, fmt.Errorf("failed to get user email: %w", err)
	}

	var query string
	if !includeShared {
		// Only owned sessions - using sync_files for file counts
		query = `
			SELECT
				s.id,
				s.external_id,
				s.first_seen,
				COALESCE(sf_stats.file_count, 0) as file_count,
				s.last_message_at,
				s.title,
				s.session_type,
				COALESCE(sf_stats.total_lines, 0) as total_lines,
				s.git_info->>'repo_url' as git_repo_url,
				s.git_info->>'branch' as git_branch,
				true as is_owner,
				'owner' as access_type,
				NULL::text as share_token,
				NULL::text as shared_by_email
			FROM sessions s
			LEFT JOIN (
				SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
				FROM sync_files
				GROUP BY session_id
			) sf_stats ON s.id = sf_stats.session_id
			WHERE s.user_id = $1
			ORDER BY COALESCE(s.last_message_at, s.first_seen) DESC
		`
	} else {
		// Include owned + shared sessions via UNION
		query = `
			WITH
			-- User's own sessions
			owned_sessions AS (
				SELECT
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.title,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					true as is_owner,
					'owner' as access_type,
					NULL::text as share_token,
					NULL::text as shared_by_email
				FROM sessions s
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				WHERE s.user_id = $1
			),
			-- Private shares (invited by email)
			private_shares AS (
				SELECT
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.title,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					false as is_owner,
					'private_share' as access_type,
					sh.share_token,
					u.email as shared_by_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_invites si ON sh.id = si.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				WHERE sh.visibility = 'private'
				  AND LOWER(si.email) = LOWER($2)
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
			),
			-- Public shares accessed by user
			public_shares AS (
				SELECT
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.title,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					false as is_owner,
					'public_share' as access_type,
					sh.share_token,
					u.email as shared_by_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_accesses sa ON sh.id = sa.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				WHERE sh.visibility = 'public'
				  AND sa.user_id = $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
			)
			SELECT * FROM (
				SELECT * FROM owned_sessions
				UNION ALL
				SELECT * FROM private_shares
				UNION ALL
				SELECT * FROM public_shares
			) combined
			ORDER BY COALESCE(last_message_at, first_seen) DESC
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

	sessions := make([]SessionListItem, 0)
	for rows.Next() {
		var session SessionListItem
		var gitRepoURL *string // Full URL from git_info JSONB
		if err := rows.Scan(
			&session.ID,
			&session.ExternalID,
			&session.FirstSeen,
			&session.FileCount,
			&session.LastSyncTime,
			&session.Title,
			&session.SessionType,
			&session.TotalLines,
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

// SessionDetail represents detailed session information (sync-based model)
type SessionDetail struct {
	ID             string           `json:"id"`                        // UUID primary key for URL routing
	ExternalID     string           `json:"external_id"`               // External system's session ID
	Title          *string          `json:"title,omitempty"`           // Session title (may be nil)
	FirstSeen      time.Time        `json:"first_seen"`
	CWD            *string          `json:"cwd,omitempty"`             // Working directory
	TranscriptPath *string          `json:"transcript_path,omitempty"` // Original transcript path
	GitInfo        interface{}      `json:"git_info,omitempty"`        // Git metadata
	LastSyncAt     *time.Time       `json:"last_sync_at,omitempty"`    // Last sync timestamp
	Files          []SyncFileDetail `json:"files"`                     // Sync files
}

// SyncFileDetail represents a synced file
type SyncFileDetail struct {
	FileName       string    `json:"file_name"`
	FileType       string    `json:"file_type"`
	LastSyncedLine int       `json:"last_synced_line"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GetSessionDetail returns detailed information about a session by its UUID primary key
// Uses sync_files table for file information
func (db *DB) GetSessionDetail(ctx context.Context, sessionID string, userID int64) (*SessionDetail, error) {
	// Get the session with all metadata and verify ownership
	var session SessionDetail
	var gitInfoBytes []byte
	sessionQuery := `
		SELECT id, external_id, title, first_seen, cwd, transcript_path, git_info, last_sync_at
		FROM sessions
		WHERE id = $1 AND user_id = $2
	`
	err := db.conn.QueryRowContext(ctx, sessionQuery, sessionID, userID).Scan(
		&session.ID,
		&session.ExternalID,
		&session.Title,
		&session.FirstSeen,
		&session.CWD,
		&session.TranscriptPath,
		&gitInfoBytes,
		&session.LastSyncAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal git_info and load sync files
	if err := db.unmarshalSessionGitInfo(&session, gitInfoBytes); err != nil {
		return nil, err
	}
	if err := db.loadSessionSyncFiles(ctx, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// unmarshalSessionGitInfo unmarshals git_info JSONB if present
func (db *DB) unmarshalSessionGitInfo(session *SessionDetail, gitInfoBytes []byte) error {
	if len(gitInfoBytes) > 0 {
		if err := json.Unmarshal(gitInfoBytes, &session.GitInfo); err != nil {
			return fmt.Errorf("failed to unmarshal git_info: %w", err)
		}
	}
	return nil
}

// loadSessionSyncFiles loads sync files for a session
func (db *DB) loadSessionSyncFiles(ctx context.Context, session *SessionDetail) error {
	filesQuery := `
		SELECT file_name, file_type, last_synced_line, updated_at
		FROM sync_files
		WHERE session_id = $1
		ORDER BY file_type DESC, file_name ASC
	`

	rows, err := db.conn.QueryContext(ctx, filesQuery, session.ID)
	if err != nil {
		return fmt.Errorf("failed to query sync files: %w", err)
	}
	defer rows.Close()

	session.Files = make([]SyncFileDetail, 0)
	for rows.Next() {
		var file SyncFileDetail
		if err := rows.Scan(&file.FileName, &file.FileType, &file.LastSyncedLine, &file.UpdatedAt); err != nil {
			return fmt.Errorf("failed to scan sync file: %w", err)
		}
		session.Files = append(session.Files, file)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating sync files: %w", err)
	}

	return nil
}

// CheckSessionsExist checks which external IDs exist for a user
// Returns the list of external IDs that already exist in the database
func (db *DB) CheckSessionsExist(ctx context.Context, userID int64, externalIDs []string) ([]string, error) {
	if len(externalIDs) == 0 {
		return []string{}, nil
	}

	// Build query with placeholders
	placeholders := make([]string, len(externalIDs))
	args := make([]interface{}, len(externalIDs)+1)
	args[0] = userID
	for i, id := range externalIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf(`
		SELECT external_id FROM sessions
		WHERE user_id = $1 AND external_id IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to check sessions: %w", err)
	}
	defer rows.Close()

	var existing []string
	for rows.Next() {
		var externalID string
		if err := rows.Scan(&externalID); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		existing = append(existing, externalID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating existing sessions: %w", err)
	}

	return existing, nil
}

// SessionShare represents a share link
type SessionShare struct {
	ID             int64      `json:"id"`
	SessionID      string     `json:"session_id"`      // UUID references sessions.id
	ExternalID     string     `json:"external_id"`     // External system's session ID (for display)
	ShareToken     string     `json:"share_token"`
	Visibility     string     `json:"visibility"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	InvitedEmails  []string   `json:"invited_emails,omitempty"`
}

// CreateShare creates a new share link for a session (by UUID primary key)
func (db *DB) CreateShare(ctx context.Context, sessionID string, userID int64, shareToken, visibility string, expiresAt *time.Time, invitedEmails []string) (*SessionShare, error) {
	// Verify session exists for this user and get external_id for display
	var externalID string
	err := db.conn.QueryRowContext(ctx,
		`SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID).Scan(&externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify session: %w", err)
	}

	// Insert share
	query := `INSERT INTO session_shares (session_id, share_token, visibility, expires_at)
	          VALUES ($1, $2, $3, $4)
	          RETURNING id, created_at`

	var share SessionShare
	share.SessionID = sessionID
	share.ExternalID = externalID
	share.ShareToken = shareToken
	share.Visibility = visibility
	share.ExpiresAt = expiresAt

	err = db.conn.QueryRowContext(ctx, query, sessionID, shareToken, visibility, expiresAt).Scan(&share.ID, &share.CreatedAt)
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

			// Also record in invited_emails for login authorization
			// This persists even if the share is later revoked
			if err := db.RecordInvitedEmail(ctx, email); err != nil {
				// Log but don't fail - the share invite was already created
				// This is a secondary concern for login authorization
				// Note: In production, consider logging this error
			}
		}
		share.InvitedEmails = invitedEmails
	}

	return &share, nil
}

// ListShares returns all shares for a session (by UUID primary key)
func (db *DB) ListShares(ctx context.Context, sessionID string, userID int64) ([]SessionShare, error) {
	// Verify session exists for this user and get external_id for display
	var externalID string
	err := db.conn.QueryRowContext(ctx,
		`SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID).Scan(&externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to verify session: %w", err)
	}

	// Get shares
	query := `SELECT id, session_id, share_token, visibility, expires_at, created_at, last_accessed_at
	          FROM session_shares
	          WHERE session_id = $1
	          ORDER BY created_at DESC`

	rows, err := db.conn.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	shares := make([]SessionShare, 0)
	for rows.Next() {
		var share SessionShare
		err := rows.Scan(&share.ID, &share.SessionID, &share.ShareToken,
			&share.Visibility, &share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}
		share.ExternalID = externalID // Set from parent query

		// Get invited emails for private shares
		if share.Visibility == "private" {
			emails, err := db.loadShareInvitedEmails(ctx, share.ID)
			if err != nil {
				return nil, err
			}
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
			ss.id, ss.session_id, s.external_id, ss.share_token, ss.visibility,
			ss.expires_at, ss.created_at, ss.last_accessed_at,
			s.title
		FROM session_shares ss
		JOIN sessions s ON ss.session_id = s.id
		WHERE s.user_id = $1
		ORDER BY ss.created_at DESC
	`

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	shares := make([]ShareWithSessionInfo, 0)
	for rows.Next() {
		var share ShareWithSessionInfo
		err := rows.Scan(
			&share.ID, &share.SessionID, &share.ExternalID, &share.ShareToken,
			&share.Visibility, &share.ExpiresAt, &share.CreatedAt, &share.LastAccessedAt,
			&share.SessionTitle,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}

		// Get invited emails for private shares
		if share.Visibility == "private" {
			emails, err := db.loadShareInvitedEmails(ctx, share.ID)
			if err != nil {
				return nil, err
			}
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
	// Verify ownership via session and delete
	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM session_shares ss
		 USING sessions s
		 WHERE ss.session_id = s.id AND ss.share_token = $1 AND s.user_id = $2`,
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

// GetSharedSession returns session detail via share token (sessionID is the UUID primary key)
// Uses sync_files table for file information
func (db *DB) GetSharedSession(ctx context.Context, sessionID string, shareToken string, viewerEmail *string) (*SessionDetail, error) {
	// Get share and verify it belongs to this session
	var share SessionShare
	query := `SELECT ss.id, ss.session_id, ss.visibility, ss.expires_at, ss.last_accessed_at
	          FROM session_shares ss
	          WHERE ss.share_token = $1 AND ss.session_id = $2`

	err := db.conn.QueryRowContext(ctx, query, shareToken, sessionID).Scan(
		&share.ID, &share.SessionID, &share.Visibility,
		&share.ExpiresAt, &share.LastAccessedAt)
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

	// Get session detail with all metadata (no ownership check since share verified)
	var session SessionDetail
	var gitInfoBytes []byte
	sessionQuery := `
		SELECT id, external_id, title, first_seen, cwd, transcript_path, git_info, last_sync_at
		FROM sessions WHERE id = $1
	`
	err = db.conn.QueryRowContext(ctx, sessionQuery, sessionID).Scan(
		&session.ID,
		&session.ExternalID,
		&session.Title,
		&session.FirstSeen,
		&session.CWD,
		&session.TranscriptPath,
		&gitInfoBytes,
		&session.LastSyncAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal git_info and load sync files
	if err := db.unmarshalSessionGitInfo(&session, gitInfoBytes); err != nil {
		return nil, err
	}
	if err := db.loadSessionSyncFiles(ctx, &session); err != nil {
		return nil, err
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

// RecordInvitedEmail records that an email was invited to a private share
// This persists independently of the share lifecycle for login authorization
func (db *DB) RecordInvitedEmail(ctx context.Context, email string) error {
	query := `
		INSERT INTO invited_emails (email, first_invited_at, last_invited_at, invite_count)
		VALUES (LOWER($1), NOW(), NOW(), 1)
		ON CONFLICT (email) DO UPDATE SET
			last_invited_at = NOW(),
			invite_count = invited_emails.invite_count + 1
	`
	_, err := db.conn.ExecContext(ctx, query, email)
	if err != nil {
		return fmt.Errorf("failed to record invited email: %w", err)
	}
	return nil
}

// HasEmailBeenInvitedAfter checks if an email was invited on or after the given timestamp
// Used for login authorization via ALLOW_INVITED_EMAILS_AFTER_TS env var
func (db *DB) HasEmailBeenInvitedAfter(ctx context.Context, email string, afterTS time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM invited_emails WHERE email = LOWER($1) AND first_invited_at >= $2)`
	var exists bool
	err := db.conn.QueryRowContext(ctx, query, email, afterTS).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check invited email: %w", err)
	}
	return exists, nil
}

// NOTE: Legacy file-by-ID and run-based functions removed - using sync API instead

// DeleteSessionFromDB deletes an entire session and all its runs from the database
// S3 objects must be deleted BEFORE calling this function
func (db *DB) DeleteSessionFromDB(ctx context.Context, sessionID string, userID int64) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete the session (CASCADE will delete runs, files, shares, and share invites)
	deleteSessionQuery := `DELETE FROM sessions WHERE id = $1 AND user_id = $2`
	result, err := tx.ExecContext(ctx, deleteSessionQuery, sessionID, userID)
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

// NOTE: CountUserRunsInLastWeek and GetUserWeeklyUsage removed - legacy rate limiting
// Sync API uses different rate limiting strategy

// ============================================================================
// Device Code Flow (for CLI authentication)
// ============================================================================

// DeviceCode represents a pending device authorization
type DeviceCode struct {
	ID           int64      `json:"id"`
	DeviceCode   string     `json:"device_code"`
	UserCode     string     `json:"user_code"`
	KeyName      string     `json:"key_name"`
	UserID       *int64     `json:"user_id,omitempty"`
	ExpiresAt    time.Time  `json:"expires_at"`
	AuthorizedAt *time.Time `json:"authorized_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// CreateDeviceCode creates a new device code for CLI authentication
func (db *DB) CreateDeviceCode(ctx context.Context, deviceCode, userCode, keyName string, expiresAt time.Time) error {
	query := `INSERT INTO device_codes (device_code, user_code, key_name, expires_at) VALUES ($1, $2, $3, $4)`
	_, err := db.conn.ExecContext(ctx, query, deviceCode, userCode, keyName, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create device code: %w", err)
	}
	return nil
}

// GetDeviceCodeByUserCode retrieves a device code by user code (for web verification page)
func (db *DB) GetDeviceCodeByUserCode(ctx context.Context, userCode string) (*DeviceCode, error) {
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
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// GetDeviceCodeByDeviceCode retrieves a device code by device code (for CLI polling)
func (db *DB) GetDeviceCodeByDeviceCode(ctx context.Context, deviceCode string) (*DeviceCode, error) {
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
		return nil, fmt.Errorf("failed to get device code: %w", err)
	}
	return &dc, nil
}

// AuthorizeDeviceCode marks a device code as authorized by a user
func (db *DB) AuthorizeDeviceCode(ctx context.Context, userCode string, userID int64) error {
	query := `UPDATE device_codes SET user_id = $1, authorized_at = NOW()
	          WHERE user_code = $2 AND expires_at > NOW() AND authorized_at IS NULL`

	result, err := db.conn.ExecContext(ctx, query, userID, userCode)
	if err != nil {
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
	query := `DELETE FROM device_codes WHERE device_code = $1`
	_, err := db.conn.ExecContext(ctx, query, deviceCode)
	if err != nil {
		return fmt.Errorf("failed to delete device code: %w", err)
	}
	return nil
}

// CleanupExpiredDeviceCodes removes expired device codes
func (db *DB) CleanupExpiredDeviceCodes(ctx context.Context) (int64, error) {
	query := `DELETE FROM device_codes WHERE expires_at < NOW()`
	result, err := db.conn.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired device codes: %w", err)
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// ============================================================================
// Incremental Sync (for daemon-based session uploads)
// ============================================================================

// SyncFileState represents the sync state for a single file
type SyncFileState struct {
	FileName       string `json:"file_name"`
	FileType       string `json:"file_type"`
	LastSyncedLine int    `json:"last_synced_line"`
}

// SyncSessionParams contains parameters for creating/updating a sync session
type SyncSessionParams struct {
	ExternalID     string
	TranscriptPath string
	CWD            string
	GitInfo        json.RawMessage // Optional: JSONB for git metadata
}

// FindOrCreateSyncSession finds an existing session by external_id or creates a new one
// Returns the session UUID and current sync state for all files
// Uses catch-and-retry to handle race conditions on concurrent creates
// Also updates session metadata (cwd, transcript_path, git_info) on each call
func (db *DB) FindOrCreateSyncSession(ctx context.Context, userID int64, params SyncSessionParams) (sessionID string, files map[string]SyncFileState, err error) {
	selectQuery := `SELECT id FROM sessions WHERE user_id = $1 AND external_id = $2 AND session_type = 'Claude Code'`

	// Try to find existing session first
	err = db.conn.QueryRowContext(ctx, selectQuery, userID, params.ExternalID).Scan(&sessionID)
	if err == nil {
		// Session exists - update metadata and get sync state
		if err := db.updateSessionMetadata(ctx, sessionID, params); err != nil {
			return "", nil, fmt.Errorf("failed to update session metadata: %w", err)
		}
		return db.getSyncFilesForSession(ctx, sessionID)
	}
	if err != sql.ErrNoRows {
		return "", nil, fmt.Errorf("failed to find session: %w", err)
	}

	// Session not found - try to create it with metadata
	sessionID = uuid.New().String()
	insertQuery := `
		INSERT INTO sessions (id, user_id, external_id, first_seen, session_type, cwd, transcript_path, git_info, last_sync_at)
		VALUES ($1, $2, $3, NOW(), 'Claude Code', $4, $5, $6, NOW())
	`
	_, err = db.conn.ExecContext(ctx, insertQuery, sessionID, userID, params.ExternalID, params.CWD, params.TranscriptPath, params.GitInfo)
	if err == nil {
		// Successfully created - new session has no synced files
		return sessionID, make(map[string]SyncFileState), nil
	}

	// Check if it's a unique constraint violation (race condition - another request created it)
	if isUniqueViolation(err) {
		// Retry the SELECT - session was created by concurrent request
		err = db.conn.QueryRowContext(ctx, selectQuery, userID, params.ExternalID).Scan(&sessionID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to find session after conflict: %w", err)
		}
		// Update metadata for the existing session
		if err := db.updateSessionMetadata(ctx, sessionID, params); err != nil {
			return "", nil, fmt.Errorf("failed to update session metadata: %w", err)
		}
		return db.getSyncFilesForSession(ctx, sessionID)
	}

	return "", nil, fmt.Errorf("failed to create session: %w", err)
}

// updateSessionMetadata updates the metadata fields on an existing session
func (db *DB) updateSessionMetadata(ctx context.Context, sessionID string, params SyncSessionParams) error {
	query := `
		UPDATE sessions
		SET cwd = COALESCE($2, cwd),
		    transcript_path = COALESCE($3, transcript_path),
		    git_info = COALESCE($4, git_info),
		    last_sync_at = NOW()
		WHERE id = $1
	`
	_, err := db.conn.ExecContext(ctx, query, sessionID, params.CWD, params.TranscriptPath, params.GitInfo)
	return err
}

// getSyncFilesForSession retrieves sync state for all files in a session
func (db *DB) getSyncFilesForSession(ctx context.Context, sessionID string) (string, map[string]SyncFileState, error) {
	files := make(map[string]SyncFileState)
	filesQuery := `SELECT file_name, file_type, last_synced_line FROM sync_files WHERE session_id = $1`
	rows, err := db.conn.QueryContext(ctx, filesQuery, sessionID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to query sync files: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var state SyncFileState
		if err := rows.Scan(&state.FileName, &state.FileType, &state.LastSyncedLine); err != nil {
			return "", nil, fmt.Errorf("failed to scan sync file: %w", err)
		}
		files[state.FileName] = state
	}

	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("error iterating sync files: %w", err)
	}

	return sessionID, files, nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation
func isUniqueViolation(err error) bool {
	// PostgreSQL error code 23505 = unique_violation
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint")
}

// isInvalidUUIDError checks if the error is a PostgreSQL invalid UUID format error
func isInvalidUUIDError(err error) bool {
	return strings.Contains(err.Error(), "invalid input syntax for type uuid")
}

// loadShareInvitedEmails loads the invited emails for a private share
func (db *DB) loadShareInvitedEmails(ctx context.Context, shareID int64) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT email FROM session_share_invites WHERE share_id = $1 ORDER BY email`,
		shareID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invites: %w", err)
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		emails = append(emails, email)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating emails: %w", err)
	}

	return emails, nil
}

// VerifySessionOwnership checks if a session exists and is owned by the user
// Returns the external_id if found, or an error
func (db *DB) VerifySessionOwnership(ctx context.Context, sessionID string, userID int64) (externalID string, err error) {
	query := `SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`
	err = db.conn.QueryRowContext(ctx, query, sessionID, userID).Scan(&externalID)
	if err == sql.ErrNoRows {
		// Check if session exists at all (for 404 vs 403 distinction)
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := db.conn.QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			return "", fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			return "", ErrForbidden
		}
		return "", ErrSessionNotFound
	}
	if err != nil {
		if isInvalidUUIDError(err) {
			return "", ErrSessionNotFound
		}
		return "", fmt.Errorf("failed to verify session ownership: %w", err)
	}
	return externalID, nil
}

// UpdateSyncFileState updates the high-water mark for a file's sync state
// Creates the sync_file record if it doesn't exist (upsert)
// If lastMessageAt is provided and newer than current, updates session.last_message_at
// If title is provided and session has no title, sets the session title
func (db *DB) UpdateSyncFileState(ctx context.Context, sessionID, fileName, fileType string, lastSyncedLine int, lastMessageAt *time.Time, title string) error {
	// Update sync_files table
	syncQuery := `
		INSERT INTO sync_files (session_id, file_name, file_type, last_synced_line, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (session_id, file_name) DO UPDATE SET
			last_synced_line = $4,
			updated_at = NOW()
	`
	_, err := db.conn.ExecContext(ctx, syncQuery, sessionID, fileName, fileType, lastSyncedLine)
	if err != nil {
		return fmt.Errorf("failed to update sync file state: %w", err)
	}

	// Update session metadata (last_message_at, title, last_sync_at)
	if lastMessageAt != nil || title != "" {
		// Build dynamic update query based on what we have
		sessionQuery := `
			UPDATE sessions
			SET last_sync_at = NOW()
		`
		args := []interface{}{sessionID}
		argIdx := 2

		if lastMessageAt != nil {
			sessionQuery += fmt.Sprintf(", last_message_at = CASE WHEN last_message_at IS NULL OR last_message_at < $%d THEN $%d ELSE last_message_at END", argIdx, argIdx)
			args = append(args, lastMessageAt)
			argIdx++
		}

		if title != "" {
			sessionQuery += fmt.Sprintf(", title = COALESCE(title, $%d)", argIdx)
			args = append(args, title)
		}

		sessionQuery += " WHERE id = $1"
		_, err = db.conn.ExecContext(ctx, sessionQuery, args...)
		if err != nil {
			return fmt.Errorf("failed to update session metadata: %w", err)
		}
	} else {
		// Still update last_sync_at even without message timestamp or title
		sessionQuery := `UPDATE sessions SET last_sync_at = NOW() WHERE id = $1`
		_, err = db.conn.ExecContext(ctx, sessionQuery, sessionID)
		if err != nil {
			return fmt.Errorf("failed to update session last_sync_at: %w", err)
		}
	}

	return nil
}

// GetSyncFileState retrieves the sync state for a specific file
func (db *DB) GetSyncFileState(ctx context.Context, sessionID, fileName string) (*SyncFileState, error) {
	query := `SELECT file_name, file_type, last_synced_line FROM sync_files WHERE session_id = $1 AND file_name = $2`
	var state SyncFileState
	err := db.conn.QueryRowContext(ctx, query, sessionID, fileName).Scan(&state.FileName, &state.FileType, &state.LastSyncedLine)
	if err == sql.ErrNoRows {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sync file state: %w", err)
	}
	return &state, nil
}

// GetSyncChunkKeys returns all S3 keys for chunks of a session (for deletion)
// This queries the sync_files table to know which files have chunks, then builds the S3 prefix
func (db *DB) GetSyncFileNames(ctx context.Context, sessionID string) ([]string, error) {
	query := `SELECT file_name FROM sync_files WHERE session_id = $1`
	rows, err := db.conn.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query sync files: %w", err)
	}
	defer rows.Close()

	var fileNames []string
	for rows.Next() {
		var fileName string
		if err := rows.Scan(&fileName); err != nil {
			return nil, fmt.Errorf("failed to scan file name: %w", err)
		}
		fileNames = append(fileNames, fileName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating file names: %w", err)
	}

	return fileNames, nil
}

// GetSessionOwnerAndExternalID returns the user_id and external_id for a session
// Used for S3 path construction when accessing shared sessions
func (db *DB) GetSessionOwnerAndExternalID(ctx context.Context, sessionID string) (userID int64, externalID string, err error) {
	query := `SELECT user_id, external_id FROM sessions WHERE id = $1`
	err = db.conn.QueryRowContext(ctx, query, sessionID).Scan(&userID, &externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", ErrSessionNotFound
		}
		return 0, "", fmt.Errorf("failed to get session: %w", err)
	}
	return userID, externalID, nil
}

// SessionEventParams contains parameters for inserting a session event
type SessionEventParams struct {
	SessionID      string
	EventType      string
	EventTimestamp time.Time
	Payload        json.RawMessage
}

// InsertSessionEvent inserts a new event into the session_events table
func (db *DB) InsertSessionEvent(ctx context.Context, params SessionEventParams) error {
	query := `
		INSERT INTO session_events (session_id, event_type, event_timestamp, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, err := db.conn.ExecContext(ctx, query, params.SessionID, params.EventType, params.EventTimestamp, params.Payload)
	if err != nil {
		return fmt.Errorf("failed to insert session event: %w", err)
	}
	return nil
}
