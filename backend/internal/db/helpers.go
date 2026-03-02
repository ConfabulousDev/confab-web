package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

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
