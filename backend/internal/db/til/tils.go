package til

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// ListParams contains parameters for listing TILs.
type ListParams struct {
	Query       string     // full-text search on title || summary
	Owners      []string   // filter by session owner email(s)
	Repos       []string   // filter by session git_repo
	Branches    []string   // filter by session git_branch
	Cursor      string     // compound cursor (created_at|id)
	PageSize    int        // default 50, max 100 (or MaxPageSize if set)
	MaxPageSize int        // override max page size (0 = use default of 100)
	From        *time.Time // inclusive lower bound on created_at
	To          *time.Time // exclusive upper bound on created_at
}

// TILWithSession extends TIL with session context for list display.
type TILWithSession struct {
	models.TIL
	SessionTitle *string `json:"session_title,omitempty"`
	GitRepo      *string `json:"git_repo,omitempty"`
	GitBranch    *string `json:"git_branch,omitempty"`
	OwnerEmail   string  `json:"owner_email"`
	IsOwner      bool    `json:"is_owner"`
	AccessType   string  `json:"access_type"`
}

// FilterOptions contains available filter values computed from TILs' sessions.
type FilterOptions struct {
	Repos    []string `json:"repos"`
	Branches []string `json:"branches"`
	Owners   []string `json:"owners"`
}

// ListResult is the paginated result of listing TILs.
type ListResult struct {
	TILs          []TILWithSession `json:"tils"`
	HasMore       bool             `json:"has_more"`
	NextCursor    string           `json:"next_cursor"`
	PageSize      int              `json:"page_size"`
	FilterOptions FilterOptions    `json:"filter_options"`
}

// Create inserts a new TIL and returns it with the generated id and created_at.
func (s *Store) Create(ctx context.Context, til *models.TIL) (*models.TIL, error) {
	ctx, span := tracer.Start(ctx, "db.create_til",
		trace.WithAttributes(
			attribute.String("session.id", til.SessionID),
			attribute.Int64("owner.id", til.OwnerID),
		))
	defer span.End()

	query := `
		INSERT INTO tils (title, summary, session_id, message_uuid, owner_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := s.conn().QueryRowContext(ctx, query,
		til.Title,
		til.Summary,
		til.SessionID,
		til.MessageUUID,
		til.OwnerID,
	).Scan(&til.ID, &til.CreatedAt)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if db.IsInvalidUUIDError(err) {
			return nil, db.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to create TIL: %w", err)
	}

	span.SetAttributes(attribute.Int64("til.id", til.ID))
	return til, nil
}

// GetByID returns a TIL by its ID.
func (s *Store) GetByID(ctx context.Context, tilID int64) (*models.TIL, error) {
	ctx, span := tracer.Start(ctx, "db.get_til_by_id",
		trace.WithAttributes(attribute.Int64("til.id", tilID)))
	defer span.End()

	query := `
		SELECT id, title, summary, session_id, message_uuid, owner_id, created_at
		FROM tils
		WHERE id = $1
	`
	var til models.TIL
	err := s.conn().QueryRowContext(ctx, query, tilID).Scan(
		&til.ID, &til.Title, &til.Summary, &til.SessionID,
		&til.MessageUUID, &til.OwnerID, &til.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrTILNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get TIL: %w", err)
	}

	return &til, nil
}

// Delete removes a TIL by ID. Returns ErrTILNotFound if it doesn't exist.
func (s *Store) Delete(ctx context.Context, tilID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_til",
		trace.WithAttributes(attribute.Int64("til.id", tilID)))
	defer span.End()

	result, err := s.conn().ExecContext(ctx,
		`DELETE FROM tils WHERE id = $1`, tilID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete TIL: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return db.ErrTILNotFound
	}

	return nil
}

// ListForSession returns all TILs for a given session. No pagination or access check.
func (s *Store) ListForSession(ctx context.Context, sessionID string) ([]models.TIL, error) {
	ctx, span := tracer.Start(ctx, "db.list_tils_for_session",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `
		SELECT id, title, summary, session_id, message_uuid, owner_id, created_at
		FROM tils
		WHERE session_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.conn().QueryContext(ctx, query, sessionID)
	if err != nil {
		if db.IsInvalidUUIDError(err) {
			return nil, db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to list TILs for session: %w", err)
	}
	defer rows.Close()

	var tils []models.TIL
	for rows.Next() {
		var til models.TIL
		if err := rows.Scan(
			&til.ID, &til.Title, &til.Summary, &til.SessionID,
			&til.MessageUUID, &til.OwnerID, &til.CreatedAt,
		); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan TIL: %w", err)
		}
		tils = append(tils, til)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating TILs: %w", err)
	}

	span.SetAttributes(attribute.Int("tils.count", len(tils)))
	return tils, nil
}

// List returns a paginated list of TILs visible to the given user, with session context.
// Visibility follows the session access model: owned sessions, private shares, system shares.
func (s *Store) List(ctx context.Context, userID int64, params ListParams) (*ListResult, error) {
	ctx, span := tracer.Start(ctx, "db.list_tils",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	maxPageSize := params.MaxPageSize
	if maxPageSize <= 0 {
		maxPageSize = 100
	}
	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if params.PageSize > maxPageSize {
		params.PageSize = maxPageSize
	}

	filterOpts, err := s.queryFilterOptions(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to query TIL filter options: %w", err)
	}

	tils, hasMore, nextCursor, err := s.queryPaginatedTILs(ctx, userID, params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to query paginated TILs: %w", err)
	}

	span.SetAttributes(attribute.Int("tils.count", len(tils)))
	return &ListResult{
		TILs:          tils,
		HasMore:       hasMore,
		NextCursor:    nextCursor,
		PageSize:      params.PageSize,
		FilterOptions: filterOpts,
	}, nil
}

// paramBuilder tracks $N indices for dynamic SQL parameter construction.
type paramBuilder struct {
	args    []interface{}
	nextIdx int
}

func newParamBuilder(userID int64) *paramBuilder {
	return &paramBuilder{
		args:    []interface{}{userID},
		nextIdx: 2,
	}
}

func (pb *paramBuilder) add(val interface{}) string {
	placeholder := fmt.Sprintf("$%d", pb.nextIdx)
	pb.args = append(pb.args, val)
	pb.nextIdx++
	return placeholder
}

func (pb *paramBuilder) addArray(vals []string) string {
	return pb.add(pq.Array(vals))
}

func lowercaseSlice(ss []string) []string {
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = strings.ToLower(s)
	}
	return result
}

func encodeCursor(t time.Time, id int64) string {
	raw := t.Format(time.RFC3339Nano) + "|" + fmt.Sprintf("%d", id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor encoding: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor time: %w", err)
	}
	var id int64
	if _, err := fmt.Sscanf(parts[1], "%d", &id); err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor id: %w", err)
	}
	return t, id, nil
}

// Session title fallback: COALESCE(custom_title, suggested_session_title, summary, first_user_message)
const sessionTitleExpr = `COALESCE(s.custom_title, s.suggested_session_title, s.summary, s.first_user_message)`

// Repo extraction from git_info JSONB (matches session list query)
const repoExtractExpr = `regexp_replace(regexp_replace(s.git_info->>'repo_url', '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1')`

// tilSelectCols are the columns selected in each CTE.
const tilSelectCols = `
				t.id, t.title, t.summary, t.session_id, t.message_uuid, t.owner_id, t.created_at,
				` + sessionTitleExpr + ` as session_title,
				s.git_info->>'repo_url' as git_repo_url,
				s.git_info->>'branch' as git_branch,
				u.email as owner_email`

func buildTILFilters(pb *paramBuilder, params ListParams) (commonFilters, ownedOwnerFilter, sharedOwnerFilter string) {
	if len(params.Repos) > 0 {
		p := pb.addArray(params.Repos)
		commonFilters += "\n\t\t\t\tAND " + repoExtractExpr + " = ANY(" + p + ")"
	}
	if len(params.Branches) > 0 {
		p := pb.addArray(params.Branches)
		commonFilters += "\n\t\t\t\tAND s.git_info->>'branch' = ANY(" + p + ")"
	}
	if len(params.Owners) > 0 {
		p := pb.addArray(lowercaseSlice(params.Owners))
		ownedOwnerFilter = "\n\t\t\t\tAND LOWER((SELECT email FROM users WHERE id = $1)) = ANY(" + p + ")"
		sharedOwnerFilter = "\n\t\t\t\tAND LOWER(u.email) = ANY(" + p + ")"
	}
	if params.Query != "" {
		tsquery := dbsession.BuildPrefixTsquery(params.Query)
		if tsquery != "" {
			tsqueryParam := pb.add(tsquery)
			commonFilters += "\n\t\t\t\tAND to_tsvector('english', t.title || ' ' || t.summary) @@ to_tsquery('english', " + tsqueryParam + ")"
		}
	}
	if params.From != nil {
		p := pb.add(*params.From)
		commonFilters += "\n\t\t\t\tAND t.created_at >= " + p
	}
	if params.To != nil {
		p := pb.add(*params.To)
		commonFilters += "\n\t\t\t\tAND t.created_at < " + p
	}
	return
}

func (s *Store) queryPaginatedTILs(ctx context.Context, userID int64, params ListParams) ([]TILWithSession, bool, string, error) {
	pb := newParamBuilder(userID)
	commonFilters, ownedOwnerFilter, sharedOwnerFilter := buildTILFilters(pb, params)
	limitP := pb.add(params.PageSize + 1)

	query := `
		WITH
		owned_tils AS (
			SELECT` + tilSelectCols + `,
				true as is_owner, 'owner' as access_type
			FROM tils t
			JOIN sessions s ON t.session_id = s.id
			JOIN users u ON s.user_id = u.id
			WHERE s.user_id = $1` + commonFilters + ownedOwnerFilter + `
		),
		shared_tils AS (
			SELECT DISTINCT ON (t.id)` + tilSelectCols + `,
				false as is_owner, 'private_share' as access_type
			FROM tils t
			JOIN sessions s ON t.session_id = s.id
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients sr ON sh.id = sr.share_id
			JOIN users u ON s.user_id = u.id
			WHERE sr.user_id = $1
			  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
			ORDER BY t.id, sh.created_at DESC
		),
		system_shared_tils AS (
			SELECT DISTINCT ON (t.id)` + tilSelectCols + `,
				false as is_owner, 'system_share' as access_type
			FROM tils t
			JOIN sessions s ON t.session_id = s.id
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_system sss ON sh.id = sss.share_id
			JOIN users u ON s.user_id = u.id
			WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
			  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
			ORDER BY t.id, sh.created_at DESC
		)
		SELECT * FROM (
			SELECT DISTINCT ON (id) * FROM (
				SELECT * FROM owned_tils
				UNION ALL SELECT * FROM shared_tils
				UNION ALL SELECT * FROM system_shared_tils
			) combined
			ORDER BY id, CASE access_type
				WHEN 'owner' THEN 1 WHEN 'private_share' THEN 2
				WHEN 'system_share' THEN 3 ELSE 4
			END
		) deduped`

	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(params.Cursor)
		if err == nil {
			cursorTimeP := pb.add(cursorTime)
			cursorIDP := pb.add(cursorID)
			query += `
		WHERE (created_at, id) < (` + cursorTimeP + `, ` + cursorIDP + `)`
		}
	}

	query += `
		ORDER BY created_at DESC, id DESC
		LIMIT ` + limitP

	rows, err := s.conn().QueryContext(ctx, query, pb.args...)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to query paginated TILs: %w", err)
	}
	defer rows.Close()

	tils := make([]TILWithSession, 0)
	for rows.Next() {
		var t TILWithSession
		var gitRepoURL *string
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Summary, &t.SessionID,
			&t.MessageUUID, &t.OwnerID, &t.CreatedAt,
			&t.SessionTitle, &gitRepoURL, &t.GitBranch,
			&t.OwnerEmail, &t.IsOwner, &t.AccessType,
		); err != nil {
			return nil, false, "", fmt.Errorf("failed to scan TIL: %w", err)
		}
		if gitRepoURL != nil && *gitRepoURL != "" {
			t.GitRepo = db.ExtractRepoName(*gitRepoURL)
		}
		tils = append(tils, t)
	}

	if err := rows.Err(); err != nil {
		return nil, false, "", fmt.Errorf("error iterating TILs: %w", err)
	}

	hasMore := len(tils) > params.PageSize
	var nextCursor string
	if hasMore {
		tils = tils[:params.PageSize]
		last := tils[len(tils)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}

	return tils, hasMore, nextCursor, nil
}

func (s *Store) queryFilterOptions(ctx context.Context, userID int64) (FilterOptions, error) {
	opts := FilterOptions{Repos: []string{}, Branches: []string{}, Owners: []string{}}

	query := `
		WITH visible_sessions AS (
			SELECT DISTINCT s.id, s.user_id, s.git_info FROM tils t
			JOIN sessions s ON t.session_id = s.id
			WHERE s.user_id = $1
			UNION
			SELECT DISTINCT s.id, s.user_id, s.git_info FROM tils t
			JOIN sessions s ON t.session_id = s.id
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients ssr ON sh.id = ssr.share_id
			WHERE ssr.user_id = $1 AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			UNION
			SELECT DISTINCT s.id, s.user_id, s.git_info FROM tils t
			JOIN sessions s ON t.session_id = s.id
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_system sss ON sh.id = sss.share_id
			WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW()) AND s.user_id != $1
		)
		SELECT
			COALESCE(r.repos, ARRAY[]::text[]) as repos,
			COALESCE(b.branches, ARRAY[]::text[]) as branches,
			COALESCE(o.owners, ARRAY[]::text[]) as owners
		FROM
			(SELECT array_agg(DISTINCT repo ORDER BY repo) as repos
			 FROM (SELECT regexp_replace(regexp_replace(v.git_info->>'repo_url', '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') as repo
				FROM visible_sessions v WHERE v.git_info->>'repo_url' IS NOT NULL) r2) r,
			(SELECT array_agg(DISTINCT branch ORDER BY branch) as branches
			 FROM (SELECT v.git_info->>'branch' as branch FROM visible_sessions v WHERE v.git_info->>'branch' IS NOT NULL) b2) b,
			(SELECT array_agg(DISTINCT LOWER(u.email) ORDER BY LOWER(u.email)) as owners
			 FROM visible_sessions v JOIN users u ON v.user_id = u.id) o
	`

	err := s.conn().QueryRowContext(ctx, query, userID).Scan(
		pq.Array(&opts.Repos), pq.Array(&opts.Branches), pq.Array(&opts.Owners),
	)
	if err != nil {
		return opts, fmt.Errorf("failed to query TIL filter options: %w", err)
	}
	return opts, nil
}
