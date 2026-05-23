package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Querier is anything that can execute SQL — *sql.DB or *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// RecordRepoRoot stamps a fork→root mapping onto session_repos. First-write-
// wins via the `root_name IS NULL` guard; if a mapping already exists for the
// fork, this call no-ops. Callers should treat errors as non-fatal (log and
// continue): the next sync chunk on the same session retries.
//
// CF-491: the canonical observation comes from comparing a session's
// extracted git_repo_url to a PR link's owner/repo. The resolver in
// api/sync.go::HandleSyncChunk invokes this after recording PR links.
func RecordRepoRoot(ctx context.Context, q Querier, fork, root, source string) error {
	_, err := q.ExecContext(ctx,
		`UPDATE session_repos
		   SET root_name = $2, root_source = $3
		   WHERE repo_name = $1 AND root_name IS NULL`,
		fork, root, source)
	return err
}

// IsInvalidUUIDError checks if the error is a PostgreSQL invalid UUID format error.
// Exported for use by sub-packages.
func IsInvalidUUIDError(err error) bool {
	return strings.Contains(err.Error(), "invalid input syntax for type uuid")
}

// IsUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
// Exported for use by sub-packages.
func IsUniqueViolation(err error) bool {
	// PostgreSQL error code 23505 = unique_violation
	return strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "unique constraint")
}

// ExtractRepoFromGitInfo unmarshals a sessions.git_info JSON blob and returns
// the owner/repo extracted from its `repo_url` field. Returns "" if gitInfo is
// empty, malformed, or has no repo_url — callers should treat empty as "no
// repo extractable" and skip downstream work. Mirrors the regex baked into
// repoExtractExpr (db/repo_filter.go) on the SQL side.
func ExtractRepoFromGitInfo(gitInfo []byte) string {
	if len(gitInfo) == 0 {
		return ""
	}
	var gi struct {
		RepoURL string `json:"repo_url"`
	}
	if err := json.Unmarshal(gitInfo, &gi); err != nil || gi.RepoURL == "" {
		return ""
	}
	repo := ExtractRepoName(gi.RepoURL)
	if repo == nil {
		return ""
	}
	return *repo
}

// ExtractRepoName extracts the org/repo from a git URL.
// Examples:
//   - "https://github.com/ConfabulousDev/confab-web.git" -> "ConfabulousDev/confab"
//   - "git@github.com:ConfabulousDev/confab.git" -> "ConfabulousDev/confab"
func ExtractRepoName(repoURL string) *string {
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

// UnmarshalSessionGitInfo unmarshals git_info JSONB bytes into the session's GitInfo field.
// Exported for use by sub-packages (session, access).
func UnmarshalSessionGitInfo(session *SessionDetail, gitInfoBytes []byte) error {
	if len(gitInfoBytes) > 0 {
		if err := json.Unmarshal(gitInfoBytes, &session.GitInfo); err != nil {
			return fmt.Errorf("failed to unmarshal git_info: %w", err)
		}
	}
	return nil
}

// LoadSessionSyncFiles loads sync files for a session from the database.
// Excludes todo files - they are transient state not useful for transcript history.
// Exported for use by sub-packages (session, access).
func LoadSessionSyncFiles(ctx context.Context, d *DB, session *SessionDetail) error {
	filesQuery := `
		SELECT file_name, file_type, last_synced_line, updated_at
		FROM sync_files
		WHERE session_id = $1 AND file_type != 'todo'
		ORDER BY file_type DESC, file_name ASC
	`

	rows, err := d.conn.QueryContext(ctx, filesQuery, session.ID)
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
