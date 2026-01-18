package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// extractRepoName extracts the org/repo from a git URL
// Examples:
//   - "https://github.com/ConfabulousDev/confab-web.git" -> "ConfabulousDev/confab"
//   - "git@github.com:ConfabulousDev/confab.git" -> "ConfabulousDev/confab"
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

// ListUserSessions returns sessions for a user based on the specified view.
// Uses sync_files table for file counts and sync state.
//
// NOTE: The SharedWithMe query is intentionally complex (6 CTEs, ~140 lines). While this
// could be simplified with a database view, keeping the SQL inline in Go code provides
// better tooling (IDE support, refactoring, grep, version control diffs) and makes the
// query logic explicit and self-contained. The duplication across CTEs is acceptable.
func (db *DB) ListUserSessions(ctx context.Context, userID int64, view SessionListView) ([]SessionListItem, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_sessions",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.view", string(view)),
		))
	defer span.End()

	var query string
	switch view {
	case SessionListViewOwned:
		// Only owned sessions - using sync_files for file counts
		query = `
			SELECT
				s.id,
				s.external_id,
				s.first_seen,
				COALESCE(sf_stats.file_count, 0) as file_count,
				s.last_message_at,
				s.custom_title,
				s.suggested_session_title,
				s.summary,
				s.first_user_message,
				s.session_type,
				COALESCE(sf_stats.total_lines, 0) as total_lines,
				s.git_info->>'repo_url' as git_repo_url,
				s.git_info->>'branch' as git_branch,
				COALESCE((
					SELECT array_agg(ref ORDER BY created_at)
					FROM session_github_links
					WHERE session_id = s.id AND link_type = 'pull_request'
				), ARRAY[]::text[]) as github_prs,
				COALESCE((
					SELECT array_agg(ref ORDER BY created_at DESC)
					FROM session_github_links
					WHERE session_id = s.id AND link_type = 'commit'
				), ARRAY[]::text[]) as github_commits,
				true as is_owner,
				'owner' as access_type,
				NULL::text as shared_by_email,
				s.hostname,
				s.username
			FROM sessions s
			LEFT JOIN (
				SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
				FROM sync_files
				GROUP BY session_id
			) sf_stats ON s.id = sf_stats.session_id
			WHERE s.user_id = $1
			ORDER BY COALESCE(s.last_message_at, s.first_seen) DESC
		`
	case SessionListViewSharedWithMe:
		// Shared sessions (private shares + system shares), includes owned for deduplication
		query = `
			WITH
			-- GitHub PRs for each session (pre-aggregated to avoid correlated subquery in DISTINCT ON)
			github_pr_refs AS (
				SELECT session_id, array_agg(ref ORDER BY created_at) as prs
				FROM session_github_links
				WHERE link_type = 'pull_request'
				GROUP BY session_id
			),
			-- GitHub commits for each session (latest first)
			github_commit_refs AS (
				SELECT session_id, array_agg(ref ORDER BY created_at DESC) as commits
				FROM session_github_links
				WHERE link_type = 'commit'
				GROUP BY session_id
			),
			-- User's own sessions
			owned_sessions AS (
				SELECT
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.custom_title,
					s.suggested_session_title,
					s.summary,
					s.first_user_message,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					true as is_owner,
					'owner' as access_type,
					NULL::text as shared_by_email,
					s.hostname,
					s.username
				FROM sessions s
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				WHERE s.user_id = $1
			),
			-- Sessions shared with user (via session_share_recipients by user_id)
			-- NOTE: hostname/username are NULL for privacy - only visible to session owner
			shared_sessions AS (
				SELECT DISTINCT ON (s.id)
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.custom_title,
					s.suggested_session_title,
					s.summary,
					s.first_user_message,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					false as is_owner,
					'private_share' as access_type,
					u.email as shared_by_email,
					NULL::text as hostname,
					NULL::text as username
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_recipients sr ON sh.id = sr.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				WHERE sr.user_id = $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				ORDER BY s.id, sh.created_at DESC  -- Pick most recent share per session
			),
			-- System-wide shares (visible to all authenticated users)
			-- NOTE: hostname/username are NULL for privacy - only visible to session owner
			system_shared_sessions AS (
				SELECT DISTINCT ON (s.id)
					s.id,
					s.external_id,
					s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at,
					s.custom_title,
					s.suggested_session_title,
					s.summary,
					s.first_user_message,
					s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					false as is_owner,
					'system_share' as access_type,
					u.email as shared_by_email,
					NULL::text as hostname,
					NULL::text as username
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_system sss ON sh.id = sss.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				ORDER BY s.id, sh.created_at DESC  -- Pick most recent share per session
			)
			-- Dedupe: prefer owner > private_share > system_share, then sort by time
			SELECT * FROM (
				SELECT DISTINCT ON (id) * FROM (
					SELECT * FROM owned_sessions
					UNION ALL
					SELECT * FROM shared_sessions
					UNION ALL
					SELECT * FROM system_shared_sessions
				) combined
				ORDER BY id, CASE access_type
					WHEN 'owner' THEN 1
					WHEN 'private_share' THEN 2
					WHEN 'system_share' THEN 3
					ELSE 4
				END
			) deduped
			ORDER BY COALESCE(last_message_at, first_seen) DESC
		`
	default:
		err := fmt.Errorf("invalid session list view: %s", view)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	rows, err := db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]SessionListItem, 0)
	for rows.Next() {
		var session SessionListItem
		var gitRepoURL *string           // Full URL from git_info JSONB
		var githubPRs pq.StringArray     // GitHub PR refs as PostgreSQL text array
		var githubCommits pq.StringArray // GitHub commit SHAs as PostgreSQL text array
		if err := rows.Scan(
			&session.ID,
			&session.ExternalID,
			&session.FirstSeen,
			&session.FileCount,
			&session.LastSyncTime,
			&session.CustomTitle,
			&session.SuggestedSessionTitle,
			&session.Summary,
			&session.FirstUserMessage,
			&session.SessionType,
			&session.TotalLines,
			&gitRepoURL,
			&session.GitBranch,
			&githubPRs,
			&githubCommits,
			&session.IsOwner,
			&session.AccessType,
			&session.SharedByEmail,
			&session.Hostname,
			&session.Username,
		); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		// Extract org/repo from full git URL (e.g., "https://github.com/org/repo.git" -> "org/repo")
		if gitRepoURL != nil && *gitRepoURL != "" {
			session.GitRepo = extractRepoName(*gitRepoURL)
			session.GitRepoURL = gitRepoURL
		}

		// Convert pq.StringArray to []string (only if non-empty)
		if len(githubPRs) > 0 {
			session.GitHubPRs = []string(githubPRs)
		}
		if len(githubCommits) > 0 {
			session.GitHubCommits = []string(githubCommits)
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	span.SetAttributes(attribute.Int("sessions.count", len(sessions)))
	return sessions, nil
}

// GetSessionsLastModified returns the most recent last_sync_at timestamp for polling ETag.
//
// TODO: If this becomes a bottleneck at scale, consider maintaining a separate
// user_sessions_metadata table with a denormalized last_sync_at column, updated on each sync.
func (db *DB) GetSessionsLastModified(ctx context.Context, userID int64, view SessionListView) (time.Time, error) {
	ctx, span := tracer.Start(ctx, "db.get_sessions_last_modified",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.view", string(view)),
		))
	defer span.End()

	var query string
	switch view {
	case SessionListViewOwned:
		query = `
			SELECT COALESCE(MAX(last_sync_at), '1970-01-01'::timestamp)
			FROM sessions
			WHERE user_id = $1
		`
	case SessionListViewSharedWithMe:
		query = `
			SELECT COALESCE(MAX(last_sync_at), '1970-01-01'::timestamp)
			FROM (
				-- Private shares
				SELECT s.last_sync_at
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_recipients sr ON sh.id = sr.share_id
				WHERE sr.user_id = $1
				  AND s.user_id != $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				UNION ALL
				-- System shares
				SELECT s.last_sync_at
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_system sss ON sh.id = sss.share_id
				WHERE s.user_id != $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			) combined
		`
	default:
		err := fmt.Errorf("invalid session list view: %s", view)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return time.Time{}, err
	}

	var lastModified time.Time
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(&lastModified)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return time.Time{}, fmt.Errorf("failed to get sessions last modified: %w", err)
	}

	return lastModified, nil
}

// GetSessionDetail returns detailed information about a session by its UUID primary key
// Uses sync_files table for file information
func (db *DB) GetSessionDetail(ctx context.Context, sessionID string, userID int64) (*SessionDetail, error) {
	ctx, span := tracer.Start(ctx, "db.get_session_detail",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	// Get the session with all metadata and verify ownership
	var session SessionDetail
	var gitInfoBytes []byte
	sessionQuery := `
		SELECT id, external_id, custom_title, suggested_session_title, summary, first_user_message, first_seen, cwd, transcript_path, git_info, last_sync_at, hostname, username
		FROM sessions
		WHERE id = $1 AND user_id = $2
	`
	err := db.conn.QueryRowContext(ctx, sessionQuery, sessionID, userID).Scan(
		&session.ID,
		&session.ExternalID,
		&session.CustomTitle,
		&session.SuggestedSessionTitle,
		&session.Summary,
		&session.FirstUserMessage,
		&session.FirstSeen,
		&session.CWD,
		&session.TranscriptPath,
		&gitInfoBytes,
		&session.LastSyncAt,
		&session.Hostname,
		&session.Username,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal git_info and load sync files
	if err := db.unmarshalSessionGitInfo(&session, gitInfoBytes); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if err := db.loadSessionSyncFiles(ctx, &session); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
// Excludes todo files - they are transient state not useful for transcript history
func (db *DB) loadSessionSyncFiles(ctx context.Context, session *SessionDetail) error {
	filesQuery := `
		SELECT file_name, file_type, last_synced_line, updated_at
		FROM sync_files
		WHERE session_id = $1 AND file_type != 'todo'
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

// DeleteSessionFromDB deletes an entire session and all its runs from the database
// S3 objects must be deleted BEFORE calling this function
func (db *DB) DeleteSessionFromDB(ctx context.Context, sessionID string, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_session",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete the session (CASCADE will delete runs, files, shares, and share invites)
	deleteSessionQuery := `DELETE FROM sessions WHERE id = $1 AND user_id = $2`
	result, err := tx.ExecContext(ctx, deleteSessionQuery, sessionID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrSessionNotFound
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// VerifySessionOwnership checks if a session exists and is owned by the user
// Returns the external_id if found, or an error
func (db *DB) VerifySessionOwnership(ctx context.Context, sessionID string, userID int64) (externalID string, err error) {
	ctx, span := tracer.Start(ctx, "db.verify_session_ownership",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`
	err = db.conn.QueryRowContext(ctx, query, sessionID, userID).Scan(&externalID)
	if err == sql.ErrNoRows {
		// Check if session exists at all (for 404 vs 403 distinction)
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := db.conn.QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			span.RecordError(checkErr)
			span.SetStatus(codes.Error, checkErr.Error())
			return "", fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			span.SetAttributes(attribute.String("result", "forbidden"))
			return "", ErrForbidden
		}
		span.SetAttributes(attribute.String("result", "not_found"))
		return "", ErrSessionNotFound
	}
	if err != nil {
		if isInvalidUUIDError(err) {
			span.SetAttributes(attribute.String("result", "not_found"))
			return "", ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to verify session ownership: %w", err)
	}
	span.SetAttributes(attribute.String("result", "owner"))
	return externalID, nil
}

// UpdateSessionSummary updates the summary field for a session identified by external_id
// Returns ErrSessionNotFound if session doesn't exist, ErrForbidden if user doesn't own it
func (db *DB) UpdateSessionSummary(ctx context.Context, externalID string, userID int64, summary string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_summary",
		trace.WithAttributes(
			attribute.String("session.external_id", externalID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `
		UPDATE sessions
		SET summary = $1
		WHERE external_id = $2 AND user_id = $3
	`
	result, err := db.conn.ExecContext(ctx, query, summary, externalID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update session summary: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Check if session exists but belongs to another user
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE external_id = $1)`
		if checkErr := db.conn.QueryRowContext(ctx, checkQuery, externalID).Scan(&exists); checkErr != nil {
			return fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			return ErrForbidden
		}
		return ErrSessionNotFound
	}

	return nil
}

// UpdateSessionCustomTitle updates the custom_title field for a session identified by UUID
// Pass nil to clear the custom title (revert to auto-derived title)
// Returns ErrSessionNotFound if session doesn't exist, ErrForbidden if user doesn't own it
func (db *DB) UpdateSessionCustomTitle(ctx context.Context, sessionID string, userID int64, customTitle *string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_custom_title",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `
		UPDATE sessions
		SET custom_title = $1
		WHERE id = $2 AND user_id = $3
	`
	result, err := db.conn.ExecContext(ctx, query, customTitle, sessionID, userID)
	if err != nil {
		if isInvalidUUIDError(err) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("failed to update session custom title: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Check if session exists but belongs to another user
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := db.conn.QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			if isInvalidUUIDError(checkErr) {
				return ErrSessionNotFound
			}
			return fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			return ErrForbidden
		}
		return ErrSessionNotFound
	}

	return nil
}

// UpdateSessionSuggestedTitle updates the suggested_session_title field for a session.
// This is called when the Smart Recap LLM generates a title suggestion.
// Returns nil if suggestedTitle is empty (no update needed).
func (db *DB) UpdateSessionSuggestedTitle(ctx context.Context, sessionID string, suggestedTitle string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_suggested_title",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		))
	defer span.End()

	if suggestedTitle == "" {
		return nil // Don't update with empty value
	}

	query := `UPDATE sessions SET suggested_session_title = $1 WHERE id = $2`
	_, err := db.conn.ExecContext(ctx, query, suggestedTitle, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update suggested session title: %w", err)
	}

	return nil
}

// GetSessionOwnerAndExternalID returns the user_id and external_id for a session
// Used for S3 path construction when accessing shared sessions
func (db *DB) GetSessionOwnerAndExternalID(ctx context.Context, sessionID string) (userID int64, externalID string, err error) {
	ctx, span := tracer.Start(ctx, "db.get_session_owner_and_external_id",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `SELECT user_id, external_id FROM sessions WHERE id = $1`
	err = db.conn.QueryRowContext(ctx, query, sessionID).Scan(&userID, &externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, "", fmt.Errorf("failed to get session: %w", err)
	}
	span.SetAttributes(attribute.Int64("user.id", userID))
	return userID, externalID, nil
}

// GetSessionIDByExternalID looks up the internal session ID by external_id for a specific user.
// Returns the internal UUID, or ErrSessionNotFound if not found or not owned by user.
func (db *DB) GetSessionIDByExternalID(ctx context.Context, externalID string, userID int64) (sessionID string, err error) {
	ctx, span := tracer.Start(ctx, "db.get_session_id_by_external_id",
		trace.WithAttributes(
			attribute.String("session.external_id", externalID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `SELECT id FROM sessions WHERE external_id = $1 AND user_id = $2`
	err = db.conn.QueryRowContext(ctx, query, externalID, userID).Scan(&sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	return sessionID, nil
}
