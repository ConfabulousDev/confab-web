package db

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

// CreateGitHubLink creates or updates a GitHub link for a session (upsert).
// On conflict (same session, link_type, owner, repo, ref), it updates source and url.
// When overwriteTitle is true, the new title always wins.
// When overwriteTitle is false, the existing title is preserved if non-null (fill-only).
func (db *DB) CreateGitHubLink(ctx context.Context, link *models.GitHubLink, overwriteTitle bool) (*models.GitHubLink, error) {
	ctx, span := tracer.Start(ctx, "db.create_github_link",
		trace.WithAttributes(
			attribute.String("session.id", link.SessionID),
			attribute.String("link.type", string(link.LinkType)),
		))
	defer span.End()

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
	err := db.conn.QueryRowContext(ctx, query,
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to create github link: %w", err)
	}

	span.SetAttributes(attribute.Int64("link.id", link.ID))
	return link, nil
}

// GetGitHubLinksForSession returns all GitHub links for a session.
func (db *DB) GetGitHubLinksForSession(ctx context.Context, sessionID string) ([]models.GitHubLink, error) {
	ctx, span := tracer.Start(ctx, "db.get_github_links_for_session",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `
		SELECT id, session_id, link_type, url, owner, repo, ref, title, source, created_at
		FROM session_github_links
		WHERE session_id = $1
		ORDER BY created_at DESC
	`
	rows, err := db.conn.QueryContext(ctx, query, sessionID)
	if err != nil {
		if isInvalidUUIDError(err) {
			return nil, ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
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
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan github link: %w", err)
		}
		links = append(links, link)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating github links: %w", err)
	}

	span.SetAttributes(attribute.Int("links.count", len(links)))
	return links, nil
}

// DeleteGitHubLink deletes a GitHub link by ID.
// Returns ErrGitHubLinkNotFound if link doesn't exist.
func (db *DB) DeleteGitHubLink(ctx context.Context, linkID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_github_link",
		trace.WithAttributes(attribute.Int64("link.id", linkID)))
	defer span.End()

	result, err := db.conn.ExecContext(ctx,
		`DELETE FROM session_github_links WHERE id = $1`,
		linkID,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete github link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrGitHubLinkNotFound
	}

	return nil
}

// GetGitHubLinkByID returns a GitHub link by ID.
// Returns ErrGitHubLinkNotFound if link doesn't exist.
func (db *DB) GetGitHubLinkByID(ctx context.Context, linkID int64) (*models.GitHubLink, error) {
	ctx, span := tracer.Start(ctx, "db.get_github_link_by_id",
		trace.WithAttributes(attribute.Int64("link.id", linkID)))
	defer span.End()

	query := `
		SELECT id, session_id, link_type, url, owner, repo, ref, title, source, created_at
		FROM session_github_links
		WHERE id = $1
	`
	var link models.GitHubLink
	err := db.conn.QueryRowContext(ctx, query, linkID).Scan(
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
			return nil, ErrGitHubLinkNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get github link: %w", err)
	}

	return &link, nil
}
