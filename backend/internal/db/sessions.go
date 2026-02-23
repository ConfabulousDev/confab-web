package db

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// DefaultPageSize is the number of sessions per page in paginated results.
const DefaultPageSize = 50

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

// lowercaseSlice returns a new slice with all strings lowercased.
func lowercaseSlice(ss []string) []string {
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = strings.ToLower(s)
	}
	return result
}

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

// ListUserSessions returns all sessions visible to a user (owned + shared) with deduplication.
// Uses sync_files table for file counts and sync state.
//
// NOTE: The unified query is intentionally complex (6 CTEs, ~140 lines). While this
// could be simplified with a database view, keeping the SQL inline in Go code provides
// better tooling (IDE support, refactoring, grep, version control diffs) and makes the
// query logic explicit and self-contained. The duplication across CTEs is acceptable.
func (db *DB) ListUserSessions(ctx context.Context, userID int64) ([]SessionListItem, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_sessions",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := db.buildSharedWithMeQuery()

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
			&session.EstimatedCostUSD,
			&session.IsOwner,
			&session.AccessType,
			&session.SharedByEmail,
			&session.OwnerEmail,
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

// buildSharedWithMeQuery returns the SQL for the SharedWithMe view.
// When ShareAllSessions is enabled, the system_shared_sessions CTE selects ALL
// non-owned sessions instead of requiring session_share_system rows.
func (db *DB) buildSharedWithMeQuery() string {
	systemSharedCTE := `
			-- System-wide shares (visible to all authenticated users)
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
					sct.estimated_cost_usd,
					false as is_owner,
					'system_share' as access_type,
					u.email as shared_by_email,
					u.email as owner_email
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
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				ORDER BY s.id, sh.created_at DESC  -- Pick most recent share per session
			)`

	if db.ShareAllSessions {
		// When ShareAllSessions is enabled, ALL non-owned sessions are visible
		// as system shares — no session_share rows needed.
		systemSharedCTE = `
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
					sct.estimated_cost_usd,
					false as is_owner,
					'system_share' as access_type,
					u.email as shared_by_email,
					u.email as owner_email
				FROM sessions s
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE s.user_id != $1
				ORDER BY s.id
			)`
	}

	return `
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
					sct.estimated_cost_usd,
					true as is_owner,
					'owner' as access_type,
					NULL::text as shared_by_email,
					u.email as owner_email
				FROM sessions s
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files
					GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE s.user_id = $1
			),
			-- Sessions shared with user (via session_share_recipients by user_id)
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
					sct.estimated_cost_usd,
					false as is_owner,
					'private_share' as access_type,
					u.email as shared_by_email,
					u.email as owner_email
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
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE sr.user_id = $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1  -- Don't duplicate owned sessions
				ORDER BY s.id, sh.created_at DESC  -- Pick most recent share per session
			),` + systemSharedCTE + `
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
}

// ListUserSessionsPaginated returns filtered, cursor-paginated sessions with pre-materialized filter options.
func (db *DB) ListUserSessionsPaginated(ctx context.Context, userID int64, params SessionListParams) (*SessionListResult, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_sessions_paginated",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	if params.PageSize == 0 {
		params.PageSize = DefaultPageSize
	}

	// Query 1: Filter options
	// Share-all mode: read from global lookup tables (O(distinct values))
	// Non-share-all mode: derive from user's visible sessions (O(V), but V is small)
	filterOptions, err := db.queryFilterOptions(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Query 2: Cursor-paginated results with filter pushdown
	sessions, hasMore, nextCursor, err := db.queryPaginatedSessions(ctx, userID, params)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("sessions.count", len(sessions)),
		attribute.Bool("sessions.has_more", hasMore),
	)

	return &SessionListResult{
		Sessions:      sessions,
		HasMore:       hasMore,
		NextCursor:    nextCursor,
		PageSize:      params.PageSize,
		FilterOptions: filterOptions,
	}, nil
}

// queryFilterOptions returns the distinct repo, branch, and owner values available
// to the given user.
//
// Share-all mode: reads from global lookup tables (O(distinct values)).
// Non-share-all mode: derives from the user's visible sessions (O(V), V is small).
func (db *DB) queryFilterOptions(ctx context.Context, userID int64) (SessionFilterOptions, error) {
	if db.ShareAllSessions {
		return db.queryFilterOptionsGlobal(ctx)
	}
	return db.queryFilterOptionsScoped(ctx, userID)
}

// queryFilterOptionsGlobal reads pre-materialized filter values from global lookup tables.
func (db *DB) queryFilterOptionsGlobal(ctx context.Context) (SessionFilterOptions, error) {
	opts := SessionFilterOptions{
		Repos:    make([]string, 0),
		Branches: make([]string, 0),
		Owners:   make([]string, 0),
	}

	// Repos from lookup table
	rows, err := db.conn.QueryContext(ctx, "SELECT repo_name FROM session_repos ORDER BY repo_name")
	if err != nil {
		return opts, fmt.Errorf("failed to query session_repos: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return opts, fmt.Errorf("failed to scan repo: %w", err)
		}
		opts.Repos = append(opts.Repos, name)
	}
	if err := rows.Err(); err != nil {
		return opts, fmt.Errorf("error iterating repos: %w", err)
	}

	// Branches from lookup table
	rows, err = db.conn.QueryContext(ctx, "SELECT branch_name FROM session_branches ORDER BY branch_name")
	if err != nil {
		return opts, fmt.Errorf("failed to query session_branches: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return opts, fmt.Errorf("failed to scan branch: %w", err)
		}
		opts.Branches = append(opts.Branches, name)
	}
	if err := rows.Err(); err != nil {
		return opts, fmt.Errorf("error iterating branches: %w", err)
	}

	// Owners: all users (simple, O(users))
	rows, err = db.conn.QueryContext(ctx, "SELECT LOWER(email) FROM users ORDER BY email")
	if err != nil {
		return opts, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return opts, fmt.Errorf("failed to scan user email: %w", err)
		}
		opts.Owners = append(opts.Owners, email)
	}
	if err := rows.Err(); err != nil {
		return opts, fmt.Errorf("error iterating users: %w", err)
	}

	return opts, nil
}

// queryFilterOptionsScoped derives filter options from only the sessions visible
// to the given user (owned + privately shared + system shared).
func (db *DB) queryFilterOptionsScoped(ctx context.Context, userID int64) (SessionFilterOptions, error) {
	opts := SessionFilterOptions{
		Repos:    make([]string, 0),
		Branches: make([]string, 0),
		Owners:   make([]string, 0),
	}

	query := `
		WITH visible AS (
			-- Owned sessions
			SELECT id, user_id, git_info FROM sessions WHERE user_id = $1
			UNION
			-- Privately shared with me
			SELECT s.id, s.user_id, s.git_info FROM sessions s
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients ssr ON sh.id = ssr.share_id
			WHERE ssr.user_id = $1
			  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			UNION
			-- System shared
			SELECT s.id, s.user_id, s.git_info FROM sessions s
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_system sss ON sh.id = sss.share_id
			WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
			  AND s.user_id != $1
		)
		SELECT
			COALESCE(r.repos, ARRAY[]::text[]) as repos,
			COALESCE(b.branches, ARRAY[]::text[]) as branches,
			COALESCE(o.owners, ARRAY[]::text[]) as owners
		FROM
			(SELECT array_agg(DISTINCT repo ORDER BY repo) as repos
			 FROM (
				SELECT regexp_replace(regexp_replace(v.git_info->>'repo_url', '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') as repo
				FROM visible v
				WHERE v.git_info->>'repo_url' IS NOT NULL
			 ) r2
			) r,
			(SELECT array_agg(DISTINCT branch ORDER BY branch) as branches
			 FROM (
				SELECT v.git_info->>'branch' as branch
				FROM visible v
				WHERE v.git_info->>'branch' IS NOT NULL
			 ) b2
			) b,
			(SELECT array_agg(DISTINCT LOWER(u.email) ORDER BY LOWER(u.email)) as owners
			 FROM visible v
			 JOIN users u ON v.user_id = u.id
			) o
	`

	var repos, branches, owners []string
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		pq.Array(&repos),
		pq.Array(&branches),
		pq.Array(&owners),
	)
	if err != nil {
		return opts, fmt.Errorf("failed to query scoped filter options: %w", err)
	}

	if repos != nil {
		opts.Repos = repos
	}
	if branches != nil {
		opts.Branches = branches
	}
	if owners != nil {
		opts.Owners = owners
	}

	return opts, nil
}

// encodeCursor encodes a (time, id) pair into an opaque base64 cursor string.
func encodeCursor(t time.Time, id string) string {
	raw := t.Format(time.RFC3339Nano) + "|" + id
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes an opaque cursor string back into (time, id).
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

// buildPushdownFilters builds WHERE clause fragments for the pushdown query (Query 2).
// Returns commonFilters (applied to all CTEs on the 's' table alias),
// ownedOwnerFilter (for owned_sessions CTE), and sharedOwnerFilter (for shared/system CTEs).
func buildPushdownFilters(pb *paramBuilder, params SessionListParams) (commonFilters, ownedOwnerFilter, sharedOwnerFilter string) {
	// Visibility (always applied)
	commonFilters = "\n\t\t\t\tAND COALESCE(sf_stats.total_lines, 0) > 0" +
		"\n\t\t\t\tAND (s.summary IS NOT NULL OR s.first_user_message IS NOT NULL)"

	if len(params.Repos) > 0 {
		p := pb.addArray(params.Repos)
		commonFilters += "\n\t\t\t\tAND regexp_replace(regexp_replace(s.git_info->>'repo_url', '\\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\\1') = ANY(" + p + ")"
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
	if len(params.PRs) > 0 {
		p := pb.addArray(params.PRs)
		commonFilters += "\n\t\t\t\tAND EXISTS (SELECT 1 FROM session_github_links sgl WHERE sgl.session_id = s.id AND sgl.link_type = 'pull_request' AND sgl.ref = ANY(" + p + "))"
	}
	if params.Query != nil && *params.Query != "" {
		p := pb.add(*params.Query)
		commonFilters += "\n\t\t\t\tAND (s.custom_title ILIKE '%'||" + p + "||'%'" +
			" OR s.suggested_session_title ILIKE '%'||" + p + "||'%'" +
			" OR s.summary ILIKE '%'||" + p + "||'%'" +
			" OR s.first_user_message ILIKE '%'||" + p + "||'%'" +
			" OR EXISTS (SELECT 1 FROM session_github_links sgl WHERE sgl.session_id = s.id AND sgl.link_type = 'commit' AND LOWER(sgl.ref) LIKE LOWER(" + p + ")||'%'))"
	}
	return
}

// Column list shared across session list query builders (minus the last 3 CTE-specific columns).
const sessionSelectCols = `
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
				sct.estimated_cost_usd`

// JOIN clause shared across session list query builders.
const sessionStatsJoins = `
			LEFT JOIN (
				SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
				FROM sync_files
				GROUP BY session_id
			) sf_stats ON s.id = sf_stats.session_id
			LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
			LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
			LEFT JOIN session_card_tokens sct ON s.id = sct.session_id`

// GitHub link CTE definitions shared across session list query builders.
const githubRefCTEs = `
		github_pr_refs AS (
			SELECT session_id, array_agg(ref ORDER BY created_at) as prs
			FROM session_github_links
			WHERE link_type = 'pull_request'
			GROUP BY session_id
		),
		github_commit_refs AS (
			SELECT session_id, array_agg(ref ORDER BY created_at DESC) as commits
			FROM session_github_links
			WHERE link_type = 'commit'
			GROUP BY session_id
		)`

// buildShareAllQuery builds a flat session list query for share-all mode.
// Skips the UNION ALL + dedup since all sessions are visible to all users.
// Returns the same 18-column output as buildFilteredSessionsQuery.
func (db *DB) buildShareAllQuery(userID int64, params SessionListParams) (string, []interface{}) {
	pb := newParamBuilder(userID)
	commonFilters, _, sharedOwnerFilter := buildPushdownFilters(pb, params)
	limitP := pb.add(params.PageSize + 1) // N+1 trick

	query := `
		WITH` + githubRefCTEs + `
		SELECT` + sessionSelectCols + `,
				(s.user_id = $1) as is_owner,
				CASE WHEN s.user_id = $1 THEN 'owner' ELSE 'system_share' END as access_type,
				CASE WHEN s.user_id = $1 THEN NULL ELSE u.email END as shared_by_email,
				u.email as owner_email
			FROM sessions s
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + `
			WHERE 1=1` + commonFilters + sharedOwnerFilter

	// Add cursor WHERE clause if cursor is provided
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

// buildFilteredSessionsQuery builds the filtered+cursor-paginated session list query.
// In share-all mode, delegates to buildShareAllQuery (flat scan, no UNION ALL).
// Otherwise, uses a UNION ALL of owned + shared + system CTEs with dedup.
// Fetches pageSize+1 rows (N+1 trick) for has_more detection.
func (db *DB) buildFilteredSessionsQuery(userID int64, params SessionListParams) (string, []interface{}) {
	// Fast path: share-all mode skips UNION ALL + dedup entirely
	if db.ShareAllSessions {
		return db.buildShareAllQuery(userID, params)
	}

	pb := newParamBuilder(userID)
	commonFilters, ownedOwnerFilter, sharedOwnerFilter := buildPushdownFilters(pb, params)
	limitP := pb.add(params.PageSize + 1) // N+1 trick

	// Build system_shared_sessions CTE (requires session_share_system rows)
	systemSharedCTE := `
			system_shared_sessions AS (
				SELECT DISTINCT ON (s.id)` + sessionSelectCols + `,
					false as is_owner,
					'system_share' as access_type,
					u.email as shared_by_email,
					u.email as owner_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_system sss ON sh.id = sss.share_id
				JOIN users u ON s.user_id = u.id` + sessionStatsJoins + `
				WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
				ORDER BY s.id, sh.created_at DESC
			)`

	query := `
		WITH` + githubRefCTEs + `,
		owned_sessions AS (
			SELECT` + sessionSelectCols + `,
				true as is_owner,
				'owner' as access_type,
				NULL::text as shared_by_email,
				u.email as owner_email
			FROM sessions s
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + `
			WHERE s.user_id = $1` + commonFilters + ownedOwnerFilter + `
		),
		shared_sessions AS (
			SELECT DISTINCT ON (s.id)` + sessionSelectCols + `,
				false as is_owner,
				'private_share' as access_type,
				u.email as shared_by_email,
				u.email as owner_email
			FROM sessions s
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients sr ON sh.id = sr.share_id
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + `
			WHERE sr.user_id = $1
			  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
			ORDER BY s.id, sh.created_at DESC
		),` + systemSharedCTE + `
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
		) deduped`

	// Add cursor WHERE clause if cursor is provided
	if params.Cursor != "" {
		cursorTime, cursorID, err := decodeCursor(params.Cursor)
		if err == nil {
			cursorTimeP := pb.add(cursorTime)
			cursorIDP := pb.add(cursorID)
			query += `
		WHERE (COALESCE(last_message_at, first_seen), id) < (` + cursorTimeP + `, ` + cursorIDP + `)`
		}
	}

	query += `
		ORDER BY COALESCE(last_message_at, first_seen) DESC, id DESC
		LIMIT ` + limitP

	return query, pb.args
}

// queryPaginatedSessions runs the filtered cursor-paginated query.
// Uses the N+1 trick: fetches pageSize+1 rows, trims to pageSize, and sets hasMore.
// Returns (sessions, hasMore, nextCursor, error).
func (db *DB) queryPaginatedSessions(ctx context.Context, userID int64, params SessionListParams) ([]SessionListItem, bool, string, error) {
	query, args := db.buildFilteredSessionsQuery(userID, params)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to query paginated sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]SessionListItem, 0)
	for rows.Next() {
		var session SessionListItem
		var gitRepoURL *string
		var githubPRs pq.StringArray
		var githubCommits pq.StringArray
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
			&session.EstimatedCostUSD,
			&session.IsOwner,
			&session.AccessType,
			&session.SharedByEmail,
			&session.OwnerEmail,
		); err != nil {
			return nil, false, "", fmt.Errorf("failed to scan session: %w", err)
		}

		if gitRepoURL != nil && *gitRepoURL != "" {
			session.GitRepo = extractRepoName(*gitRepoURL)
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
		return nil, false, "", fmt.Errorf("error iterating sessions: %w", err)
	}

	// N+1 trick: if we got more than pageSize rows, there are more results
	hasMore := len(sessions) > params.PageSize
	if hasMore {
		sessions = sessions[:params.PageSize]
	}

	// Build cursor from the last returned row
	var nextCursor string
	if hasMore && len(sessions) > 0 {
		last := sessions[len(sessions)-1]
		// Use LastSyncTime (which maps to last_message_at) with FirstSeen as fallback
		cursorTime := last.FirstSeen
		if last.LastSyncTime != nil {
			cursorTime = *last.LastSyncTime
		}
		nextCursor = encodeCursor(cursorTime, last.ID)
	}

	return sessions, hasMore, nextCursor, nil
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
		SELECT s.id, s.external_id, s.custom_title, s.suggested_session_title, s.summary, s.first_user_message, s.first_seen, s.cwd, s.transcript_path, s.git_info, s.last_sync_at, s.hostname, s.username, u.email
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1 AND s.user_id = $2
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
		&session.OwnerEmail,
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
