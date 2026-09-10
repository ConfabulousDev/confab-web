package github

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// githubLinkColumns is the shared SELECT column list for session_github_links,
// in the order the row scanners expect. Kept in one place so the read queries
// cannot drift — a parallel column list is a ghost-field bug waiting to happen.
const githubLinkColumns = "id, session_id, link_type, url, owner, repo, ref, title, source, created_at"

// CreateGitHubLink creates or updates a GitHub link for a session (upsert).
// On conflict (same session, link_type, owner, repo, ref), it updates source and url.
// When overwriteTitle is true, the new title always wins.
// When overwriteTitle is false, the existing title is preserved if non-null (fill-only).
func (s *Store) CreateGitHubLink(ctx context.Context, link *models.GitHubLink, overwriteTitle bool) (*models.GitHubLink, error) {
	query := `
		INSERT INTO session_github_links (session_id, link_type, url, owner, repo, ref, title, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id, link_type, owner, repo, ref)
		DO UPDATE SET
			source = EXCLUDED.source,
			url = EXCLUDED.url,
			title = CASE WHEN $9 THEN EXCLUDED.title ELSE COALESCE(session_github_links.title, EXCLUDED.title) END
		RETURNING id, created_at
	`
	err := s.conn().QueryRowContext(ctx, query,
		link.SessionID,
		link.LinkType,
		link.URL,
		link.Owner,
		link.Repo,
		link.Ref,
		link.Title,
		link.Source,
		overwriteTitle,
	).Scan(&link.ID, &link.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create github link: %w", err)
	}

	return link, nil
}

// GetGitHubLinksForSession returns all GitHub links for a session.
func (s *Store) GetGitHubLinksForSession(ctx context.Context, sessionID string) ([]models.GitHubLink, error) {
	query := `SELECT ` + githubLinkColumns + `
		FROM session_github_links
		WHERE session_id = $1
		ORDER BY created_at DESC`
	rows, err := s.conn().QueryContext(ctx, query, sessionID)
	if err != nil {
		if db.IsInvalidUUIDError(err) {
			return nil, db.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to get github links: %w", err)
	}
	defer rows.Close()

	var links []models.GitHubLink
	for rows.Next() {
		var link models.GitHubLink
		err := rows.Scan(
			&link.ID,
			&link.SessionID,
			&link.LinkType,
			&link.URL,
			&link.Owner,
			&link.Repo,
			&link.Ref,
			&link.Title,
			&link.Source,
			&link.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan github link: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating github links: %w", err)
	}

	return links, nil
}

// DeleteGitHubLink deletes a GitHub link by ID.
// Returns db.ErrGitHubLinkNotFound if link doesn't exist.
func (s *Store) DeleteGitHubLink(ctx context.Context, linkID int64) error {
	result, err := s.conn().ExecContext(ctx,
		`DELETE FROM session_github_links WHERE id = $1`,
		linkID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete github link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return db.ErrGitHubLinkNotFound
	}

	return nil
}

// GetGitHubLinkByID returns a GitHub link by ID.
// Returns db.ErrGitHubLinkNotFound if link doesn't exist.
func (s *Store) GetGitHubLinkByID(ctx context.Context, linkID int64) (*models.GitHubLink, error) {
	query := `SELECT ` + githubLinkColumns + `
		FROM session_github_links
		WHERE id = $1`
	var link models.GitHubLink
	err := s.conn().QueryRowContext(ctx, query, linkID).Scan(
		&link.ID,
		&link.SessionID,
		&link.LinkType,
		&link.URL,
		&link.Owner,
		&link.Repo,
		&link.Ref,
		&link.Title,
		&link.Source,
		&link.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrGitHubLinkNotFound
		}
		return nil, fmt.Errorf("failed to get github link: %w", err)
	}

	return &link, nil
}
