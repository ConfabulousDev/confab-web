package session

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// paramBuilder tracks $N indices for dynamic SQL parameter construction.
type paramBuilder struct {
	args    []interface{}
	nextIdx int
}

// newParamBuilder creates a paramBuilder with $1 = userID.
func newParamBuilder(userID int64) *paramBuilder {
	return &paramBuilder{
		args:    []interface{}{userID},
		nextIdx: 2,
	}
}

// add appends a value and returns its $N placeholder.
func (pb *paramBuilder) add(val interface{}) string {
	placeholder := fmt.Sprintf("$%d", pb.nextIdx)
	pb.args = append(pb.args, val)
	pb.nextIdx++
	return placeholder
}

// addArray appends a string slice as pq.Array and returns its $N placeholder.
func (pb *paramBuilder) addArray(vals []string) string {
	return pb.add(pq.Array(vals))
}

func nonNilSlice(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// lowercaseSlice returns a new slice with all strings lowercased.
func lowercaseSlice(ss []string) []string {
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = strings.ToLower(s)
	}
	return result
}

// ListUserSessions returns all sessions visible to a user (owned + shared) with deduplication.
func (s *Store) ListUserSessions(ctx context.Context, userID int64) ([]db.SessionListItem, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_sessions",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := s.buildSharedWithMeQuery()

	rows, err := s.conn().QueryContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessionListItems(rows)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(attribute.Int("sessions.count", len(sessions)))
	return sessions, nil
}

func scanSessionListItems(rows *sql.Rows) ([]db.SessionListItem, error) {
	sessions := make([]db.SessionListItem, 0)
	for rows.Next() {
		var session db.SessionListItem
		var gitRepoURL *string
		var githubPRs pq.StringArray
		var githubCommits pq.StringArray
		if err := rows.Scan(
			&session.ID, &session.ExternalID, &session.FirstSeen,
			&session.FileCount, &session.LastSyncTime, &session.CustomTitle,
			&session.SuggestedSessionTitle, &session.Summary, &session.FirstUserMessage,
			&session.Provider, &session.TotalLines, &gitRepoURL, &session.GitBranch,
			&githubPRs, &githubCommits, &session.EstimatedCostUSD,
			&session.IsOwner, &session.AccessType, &session.SharedByEmail, &session.OwnerEmail,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		session.Provider = models.NormalizeProvider(session.Provider)
		if gitRepoURL != nil && *gitRepoURL != "" {
			session.GitRepo = db.ExtractRepoName(*gitRepoURL)
			session.GitRepoURL = gitRepoURL
		}
		if len(githubPRs) > 0 {
			session.GitHubPRs = []string(githubPRs)
		}
		if len(githubCommits) > 0 {
			session.GitHubCommits = []string(githubCommits)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}
	return sessions, nil
}

// buildSharedWithMeQuery returns the SQL for the SharedWithMe view.
// CF-495: routes through db.VisibleSessionsCTE + the shared dedup pass so
// the visibility predicate has a single source of truth.
func (s *Store) buildSharedWithMeQuery() string {
	return `
		WITH` + githubRefCTEs + `,
		` + db.VisibleSessionsCTE(s.DB.ShareAllSessions) + `,` + dedupedVisibleCTE + `
		SELECT` + sessionSelectCols + `,
				(d.access_type = 'owner') as is_owner,
				d.access_type,
				d.shared_by_email,
				d.owner_email
			FROM deduped_visible d
			JOIN sessions s ON d.id = s.id` + sessionStatsJoins + `
			ORDER BY COALESCE(s.last_message_at, s.first_seen) DESC
	`
}

// ListUserSessionsPaginated returns filtered, cursor-paginated sessions with pre-materialized filter options.
func (s *Store) ListUserSessionsPaginated(ctx context.Context, userID int64, params db.SessionListParams) (*db.SessionListResult, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_sessions_paginated",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	if params.PageSize == 0 {
		params.PageSize = db.DefaultPageSize
	}

	filterOptions, err := s.queryFilterOptions(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	sessions, hasMore, nextCursor, err := s.queryPaginatedSessions(ctx, userID, params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("sessions.count", len(sessions)),
		attribute.Bool("sessions.has_more", hasMore),
	)

	return &db.SessionListResult{
		Sessions:      sessions,
		HasMore:       hasMore,
		NextCursor:    nextCursor,
		PageSize:      params.PageSize,
		FilterOptions: filterOptions,
	}, nil
}

// queryFilterOptions derives the repo/branch/owner filter dropdowns live from
// the viewer's visible sessions. CF-495: routes through db.VisibleSessionsCTE
// so the visibility predicate has a single source of truth (and, with
// ShareAllSessions, the share-all variant returns every session — no separate
// global path or lookup tables). The wrapper SELECT DISTINCT collapses the
// UNION-ALL duplicates emitted by the helper (e.g. a recipient who also has a
// system share to the same session).
func (s *Store) queryFilterOptions(ctx context.Context, userID int64) (db.SessionFilterOptions, error) {
	query := `
		WITH ` + db.VisibleSessionsCTE(s.DB.ShareAllSessions) + `,
		visible AS (
			SELECT DISTINCT vs.id, vs.user_id, vs.owner_email FROM visible_sessions vs
		)
		SELECT
			COALESCE(r.repos, ARRAY[]::text[]) as repos,
			COALESCE(b.branches, ARRAY[]::text[]) as branches,
			COALESCE(o.owners, ARRAY[]::text[]) as owners
		FROM
			(SELECT array_agg(DISTINCT ` + db.RepoRootExpr("s") + ` ORDER BY ` + db.RepoRootExpr("s") + `) as repos
			 FROM visible v JOIN sessions s ON v.id = s.id
			 WHERE s.git_info->>'repo_url' IS NOT NULL) r,
			(SELECT array_agg(DISTINCT s.git_info->>'branch' ORDER BY s.git_info->>'branch') as branches
			 FROM visible v JOIN sessions s ON v.id = s.id
			 WHERE s.git_info->>'branch' IS NOT NULL) b,
			(SELECT array_agg(DISTINCT LOWER(owner_email) ORDER BY LOWER(owner_email)) as owners
			 FROM visible) o
	`

	var repos, branches, owners []string
	err := s.conn().QueryRowContext(ctx, query, userID).Scan(pq.Array(&repos), pq.Array(&branches), pq.Array(&owners))
	if err != nil {
		return db.SessionFilterOptions{}, fmt.Errorf("failed to query scoped filter options: %w", err)
	}
	return db.SessionFilterOptions{
		Repos:     nonNilSlice(repos),
		Branches:  nonNilSlice(branches),
		Owners:    nonNilSlice(owners),
		Providers: models.CanonicalProviders,
	}, nil
}

func encodeCursor(t time.Time, id string) string {
	raw := t.Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(cursor string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor encoding: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor format")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid cursor time: %w", err)
	}
	return t, parts[1], nil
}

// buildPushdownFilters returns SQL fragments appended to the outer query.
// CF-495: owner filter now applies uniformly to d.owner_email (post-dedup
// from db.VisibleSessionsCTE), so the historical owned/shared split is gone.
func buildPushdownFilters(pb *paramBuilder, params db.SessionListParams) (commonFilters, ownerFilter, searchJoin string) {
	commonFilters = "\n\t\t\t\tAND COALESCE(sf_stats.total_lines, 0) > 0" +
		"\n\t\t\t\tAND (s.summary IS NOT NULL OR s.first_user_message IS NOT NULL)"

	if len(params.Repos) > 0 {
		p := pb.addArray(params.Repos)
		commonFilters += "\n\t\t\t\tAND " + db.RepoMatchExpr("s", p)
	}
	if len(params.Branches) > 0 {
		p := pb.addArray(params.Branches)
		commonFilters += "\n\t\t\t\tAND s.git_info->>'branch' = ANY(" + p + ")"
	}
	if len(params.Providers) > 0 {
		p := pb.addArray(models.ExpandWithAliases(params.Providers))
		commonFilters += "\n\t\t\t\tAND s.session_type = ANY(" + p + ")"
	}
	if len(params.Owners) > 0 {
		p := pb.addArray(lowercaseSlice(params.Owners))
		ownerFilter = "\n\t\t\t\tAND LOWER(d.owner_email) = ANY(" + p + ")"
	}
	if len(params.PRs) > 0 {
		p := pb.addArray(params.PRs)
		commonFilters += "\n\t\t\t\tAND EXISTS (SELECT 1 FROM session_github_links sgl WHERE sgl.session_id = s.id AND sgl.link_type = 'pull_request' AND sgl.ref = ANY(" + p + "))"
	}
	if params.Query != nil && *params.Query != "" {
		tsquery := BuildPrefixTsquery(*params.Query)
		if tsquery != "" {
			tsqueryParam := pb.add(tsquery)
			rawQueryParam := pb.add(*params.Query)
			searchJoin = "\n\t\t\tLEFT JOIN session_search_index ssi ON s.id = ssi.session_id"
			commonFilters += "\n\t\t\t\tAND (ssi.search_vector @@ to_tsquery('english', " + tsqueryParam + ")" +
				" OR EXISTS (SELECT 1 FROM session_github_links sgl WHERE sgl.session_id = s.id AND sgl.link_type = 'commit' AND LOWER(sgl.ref) LIKE LOWER(" + rawQueryParam + ")||'%'))"
		}
	}
	return
}

var tsquerySpecialChars = regexp.MustCompile(`[&|!<>():'\\]`)

// BuildPrefixTsquery builds a tsquery string with prefix matching from a search query.
func BuildPrefixTsquery(query string) string {
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}
	terms := make([]string, 0, len(words))
	for _, w := range words {
		escaped := tsquerySpecialChars.ReplaceAllString(w, "")
		if escaped == "" {
			continue
		}
		terms = append(terms, escaped+":*")
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " & ")
}

const sessionSelectCols = `
				s.id, s.external_id, s.first_seen,
				COALESCE(sf_stats.file_count, 0) as file_count,
				s.last_message_at, s.custom_title, s.suggested_session_title,
				s.summary, s.first_user_message, s.session_type,
				COALESCE(sf_stats.total_lines, 0) as total_lines,
				s.git_info->>'repo_url' as git_repo_url,
				s.git_info->>'branch' as git_branch,
				COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
				COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
				sct.estimated_cost_usd`

const sessionStatsJoins = `
			LEFT JOIN (
				SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
				FROM sync_files GROUP BY session_id
			) sf_stats ON s.id = sf_stats.session_id
			LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
			LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
			LEFT JOIN session_card_tokens sct ON s.id = sct.session_id`

const githubRefCTEs = `
		github_pr_refs AS (
			SELECT session_id, array_agg(url ORDER BY created_at) as prs
			FROM session_github_links WHERE link_type = 'pull_request' GROUP BY session_id
		),
		github_commit_refs AS (
			SELECT session_id, array_agg(ref ORDER BY created_at DESC) as commits
			FROM session_github_links WHERE link_type = 'commit' GROUP BY session_id
		)`

// dedupedVisibleCTE wraps db.VisibleSessionsCTE with a DISTINCT ON (id) pass
// that picks the highest-priority access_type per session: owner > private_share > system_share.
// The aliased columns become d.id, d.access_type, d.shared_by_email, d.owner_email
// for the outer query. Used by every paginated/list query.
const dedupedVisibleCTE = `
		deduped_visible AS (
			SELECT DISTINCT ON (vs.id)
				vs.id, vs.owner_email, vs.access_type, vs.shared_by_email
			FROM visible_sessions vs
			ORDER BY vs.id, CASE vs.access_type
				WHEN 'owner' THEN 1
				WHEN 'private_share' THEN 2
				WHEN 'system_share' THEN 3
				ELSE 4
			END
		)`

// CF-495: single SQL shape for paginated session listing — routes visibility
// through db.VisibleSessionsCTE so both default and share-all modes share the
// same column projection. Owner filter applied uniformly on the deduped
// visible CTE (d.owner_email). access_type / shared_by_email come from the
// helper rather than per-branch CASE expressions.
func (s *Store) buildFilteredSessionsQuery(userID int64, params db.SessionListParams) (string, []interface{}) {
	pb := newParamBuilder(userID)
	commonFilters, ownerFilter, searchJoin := buildPushdownFilters(pb, params)
	limitP := pb.add(params.PageSize + 1)

	query := `
		WITH` + githubRefCTEs + `,
		` + db.VisibleSessionsCTE(s.DB.ShareAllSessions) + `,` + dedupedVisibleCTE + `
		SELECT` + sessionSelectCols + `,
				(d.access_type = 'owner') as is_owner,
				d.access_type,
				d.shared_by_email,
				d.owner_email
			FROM deduped_visible d
			JOIN sessions s ON d.id = s.id` + sessionStatsJoins + searchJoin + `
			WHERE 1=1` + commonFilters + ownerFilter

	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(params.Cursor)
		if err == nil {
			cursorTimeP := pb.add(cursorTime)
			cursorIDP := pb.add(cursorID)
			query += `
				AND (COALESCE(s.last_message_at, s.first_seen), s.id) < (` + cursorTimeP + `, ` + cursorIDP + `)`
		}
	}

	query += `
			ORDER BY COALESCE(s.last_message_at, s.first_seen) DESC, s.id DESC
			LIMIT ` + limitP

	return query, pb.args
}

func (s *Store) queryPaginatedSessions(ctx context.Context, userID int64, params db.SessionListParams) ([]db.SessionListItem, bool, string, error) {
	query, args := s.buildFilteredSessionsQuery(userID, params)

	rows, err := s.conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to query paginated sessions: %w", err)
	}
	defer rows.Close()

	sessions, err := scanSessionListItems(rows)
	if err != nil {
		return nil, false, "", err
	}

	hasMore := len(sessions) > params.PageSize
	if hasMore {
		sessions = sessions[:params.PageSize]
	}

	var nextCursor string
	if hasMore && len(sessions) > 0 {
		last := sessions[len(sessions)-1]
		cursorTime := last.FirstSeen
		if last.LastSyncTime != nil {
			cursorTime = *last.LastSyncTime
		}
		nextCursor = encodeCursor(cursorTime, last.ID)
	}

	return sessions, hasMore, nextCursor, nil
}

// GetSessionDetail returns detailed information about a session by its UUID primary key
func (s *Store) GetSessionDetail(ctx context.Context, sessionID string, userID int64) (*db.SessionDetail, error) {
	ctx, span := tracer.Start(ctx, "db.get_session_detail",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	var session db.SessionDetail
	var gitInfoBytes []byte
	// Column list and Scan targets live in db/session_detail.go so this
	// reader stays in lockstep with access.GetSessionDetailWithAccess —
	// see SessionDetailColumns for why.
	sessionQuery := `
		SELECT ` + db.SessionDetailColumns + `
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1 AND s.user_id = $2
	`
	err := s.conn().QueryRowContext(ctx, sessionQuery, sessionID, userID).Scan(
		db.SessionDetailScanTargets(&session, &gitInfoBytes)...,
	)
	if err != nil {
		if err == sql.ErrNoRows || db.IsInvalidUUIDError(err) {
			return nil, db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	session.Provider = models.NormalizeProvider(session.Provider)

	if err := db.UnmarshalSessionGitInfo(&session, gitInfoBytes); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if err := db.LoadSessionSyncFiles(ctx, s.DB, &session); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &session, nil
}

// DeleteSessionFromDB deletes an entire session and all its runs from the database
func (s *Store) DeleteSessionFromDB(ctx context.Context, sessionID string, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.delete_session",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	result, err := s.conn().ExecContext(ctx, `DELETE FROM sessions WHERE id = $1 AND user_id = $2`, sessionID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return db.ErrSessionNotFound
	}

	return nil
}

// VerifySessionOwnership checks if a session exists and is owned by the user.
// It returns the session's external_id and the canonical provider value
// (legacy 'Claude Code' rows are normalized to 'claude-code' here).
func (s *Store) VerifySessionOwnership(ctx context.Context, sessionID string, userID int64) (externalID string, provider string, err error) {
	ctx, span := tracer.Start(ctx, "db.verify_session_ownership",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `SELECT external_id, session_type FROM sessions WHERE id = $1 AND user_id = $2`
	err = s.conn().QueryRowContext(ctx, query, sessionID, userID).Scan(&externalID, &provider)
	if err == sql.ErrNoRows {
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := s.conn().QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			span.RecordError(checkErr)
			span.SetStatus(codes.Error, checkErr.Error())
			return "", "", fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			span.SetAttributes(attribute.String("result", "forbidden"))
			return "", "", db.ErrForbidden
		}
		span.SetAttributes(attribute.String("result", "not_found"))
		return "", "", db.ErrSessionNotFound
	}
	if err != nil {
		if db.IsInvalidUUIDError(err) {
			span.SetAttributes(attribute.String("result", "not_found"))
			return "", "", db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", "", fmt.Errorf("failed to verify session ownership: %w", err)
	}
	provider = models.NormalizeProvider(provider)
	span.SetAttributes(
		attribute.String("result", "owner"),
		attribute.String("session.provider", provider),
	)
	return externalID, provider, nil
}

// UpdateSessionSummary updates the summary field for a session identified by external_id
func (s *Store) UpdateSessionSummary(ctx context.Context, externalID string, userID int64, summary string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_summary",
		trace.WithAttributes(
			attribute.String("session.external_id", externalID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `UPDATE sessions SET summary = $1 WHERE external_id = $2 AND user_id = $3`
	result, err := s.conn().ExecContext(ctx, query, summary, externalID, userID)
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
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE external_id = $1)`
		if checkErr := s.conn().QueryRowContext(ctx, checkQuery, externalID).Scan(&exists); checkErr != nil {
			return fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			return db.ErrForbidden
		}
		return db.ErrSessionNotFound
	}
	return nil
}

// UpdateSessionCustomTitle updates the custom_title field for a session identified by UUID
func (s *Store) UpdateSessionCustomTitle(ctx context.Context, sessionID string, userID int64, customTitle *string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_custom_title",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `UPDATE sessions SET custom_title = $1 WHERE id = $2 AND user_id = $3`
	result, err := s.conn().ExecContext(ctx, query, customTitle, sessionID, userID)
	if err != nil {
		if db.IsInvalidUUIDError(err) {
			return db.ErrSessionNotFound
		}
		return fmt.Errorf("failed to update session custom title: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := s.conn().QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			if db.IsInvalidUUIDError(checkErr) {
				return db.ErrSessionNotFound
			}
			return fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			return db.ErrForbidden
		}
		return db.ErrSessionNotFound
	}
	return nil
}

// UpdateSessionSuggestedTitle updates the suggested_session_title field for a session.
func (s *Store) UpdateSessionSuggestedTitle(ctx context.Context, sessionID string, suggestedTitle string) error {
	ctx, span := tracer.Start(ctx, "db.update_session_suggested_title",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	if suggestedTitle == "" {
		return nil
	}

	query := `UPDATE sessions SET suggested_session_title = $1 WHERE id = $2`
	_, err := s.conn().ExecContext(ctx, query, suggestedTitle, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to update suggested session title: %w", err)
	}
	return nil
}

// GetSessionOwnerExternalIDAndProvider returns the user_id, external_id, and
// canonical provider for a session. Legacy 'Claude Code' rows are normalized
// to models.ProviderClaudeCode via models.NormalizeProvider so callers can
// pass the returned provider straight into the chunk-storage methods without
// further massaging. Used by canonical-access read paths (analytics, sync
// file read, transcript download) that don't go through the owner-only
// VerifySessionOwnership route.
func (s *Store) GetSessionOwnerExternalIDAndProvider(ctx context.Context, sessionID string) (userID int64, externalID string, provider string, err error) {
	ctx, span := tracer.Start(ctx, "db.get_session_owner_external_id_and_provider",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `SELECT user_id, external_id, session_type FROM sessions WHERE id = $1`
	err = s.conn().QueryRowContext(ctx, query, sessionID).Scan(&userID, &externalID, &provider)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", "", db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, "", "", fmt.Errorf("failed to get session: %w", err)
	}
	provider = models.NormalizeProvider(provider)
	span.SetAttributes(
		attribute.Int64("user.id", userID),
		attribute.String("session.provider", provider),
	)
	return userID, externalID, provider, nil
}

// GetSessionIDByExternalID looks up the internal session ID by external_id for a specific user.
func (s *Store) GetSessionIDByExternalID(ctx context.Context, externalID string, userID int64) (sessionID string, err error) {
	ctx, span := tracer.Start(ctx, "db.get_session_id_by_external_id",
		trace.WithAttributes(
			attribute.String("session.external_id", externalID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `SELECT id FROM sessions WHERE external_id = $1 AND user_id = $2`
	err = s.conn().QueryRowContext(ctx, query, externalID, userID).Scan(&sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	return sessionID, nil
}
