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
			&session.SessionType, &session.TotalLines, &gitRepoURL, &session.GitBranch,
			&githubPRs, &githubCommits, &session.EstimatedCostUSD,
			&session.IsOwner, &session.AccessType, &session.SharedByEmail, &session.OwnerEmail,
		); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	span.SetAttributes(attribute.Int("sessions.count", len(sessions)))
	return sessions, nil
}

// buildSharedWithMeQuery returns the SQL for the SharedWithMe view.
func (s *Store) buildSharedWithMeQuery() string {
	systemSharedCTE := `
			system_shared_sessions AS (
				SELECT DISTINCT ON (s.id)
					s.id, s.external_id, s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at, s.custom_title, s.suggested_session_title,
					s.summary, s.first_user_message, s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					sct.estimated_cost_usd,
					false as is_owner, 'system_share' as access_type,
					u.email as shared_by_email, u.email as owner_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_system sss ON sh.id = sss.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1
				ORDER BY s.id, sh.created_at DESC
			)`

	if s.DB.ShareAllSessions {
		systemSharedCTE = `
			system_shared_sessions AS (
				SELECT DISTINCT ON (s.id)
					s.id, s.external_id, s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at, s.custom_title, s.suggested_session_title,
					s.summary, s.first_user_message, s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					sct.estimated_cost_usd,
					false as is_owner, 'system_share' as access_type,
					u.email as shared_by_email, u.email as owner_email
				FROM sessions s
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files GROUP BY session_id
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
			github_pr_refs AS (
				SELECT session_id, array_agg(ref ORDER BY created_at) as prs
				FROM session_github_links WHERE link_type = 'pull_request' GROUP BY session_id
			),
			github_commit_refs AS (
				SELECT session_id, array_agg(ref ORDER BY created_at DESC) as commits
				FROM session_github_links WHERE link_type = 'commit' GROUP BY session_id
			),
			owned_sessions AS (
				SELECT
					s.id, s.external_id, s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at, s.custom_title, s.suggested_session_title,
					s.summary, s.first_user_message, s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					sct.estimated_cost_usd,
					true as is_owner, 'owner' as access_type,
					NULL::text as shared_by_email, u.email as owner_email
				FROM sessions s
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE s.user_id = $1
			),
			shared_sessions AS (
				SELECT DISTINCT ON (s.id)
					s.id, s.external_id, s.first_seen,
					COALESCE(sf_stats.file_count, 0) as file_count,
					s.last_message_at, s.custom_title, s.suggested_session_title,
					s.summary, s.first_user_message, s.session_type,
					COALESCE(sf_stats.total_lines, 0) as total_lines,
					s.git_info->>'repo_url' as git_repo_url,
					s.git_info->>'branch' as git_branch,
					COALESCE(gpr.prs, ARRAY[]::text[]) as github_prs,
					COALESCE(gcr.commits, ARRAY[]::text[]) as github_commits,
					sct.estimated_cost_usd,
					false as is_owner, 'private_share' as access_type,
					u.email as shared_by_email, u.email as owner_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_recipients sr ON sh.id = sr.share_id
				JOIN users u ON s.user_id = u.id
				LEFT JOIN (
					SELECT session_id, COUNT(*) as file_count, SUM(last_synced_line) as total_lines
					FROM sync_files GROUP BY session_id
				) sf_stats ON s.id = sf_stats.session_id
				LEFT JOIN github_pr_refs gpr ON s.id = gpr.session_id
				LEFT JOIN github_commit_refs gcr ON s.id = gcr.session_id
				LEFT JOIN session_card_tokens sct ON s.id = sct.session_id
				WHERE sr.user_id = $1
				  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1
				ORDER BY s.id, sh.created_at DESC
			),` + systemSharedCTE + `
			SELECT * FROM (
				SELECT DISTINCT ON (id) * FROM (
					SELECT * FROM owned_sessions
					UNION ALL SELECT * FROM shared_sessions
					UNION ALL SELECT * FROM system_shared_sessions
				) combined
				ORDER BY id, CASE access_type
					WHEN 'owner' THEN 1 WHEN 'private_share' THEN 2
					WHEN 'system_share' THEN 3 ELSE 4
				END
			) deduped
			ORDER BY COALESCE(last_message_at, first_seen) DESC
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

func (s *Store) queryFilterOptions(ctx context.Context, userID int64) (db.SessionFilterOptions, error) {
	if s.DB.ShareAllSessions {
		return s.queryFilterOptionsGlobal(ctx)
	}
	return s.queryFilterOptionsScoped(ctx, userID)
}

func (s *Store) queryFilterOptionsGlobal(ctx context.Context) (db.SessionFilterOptions, error) {
	opts := db.SessionFilterOptions{Repos: make([]string, 0), Branches: make([]string, 0), Owners: make([]string, 0)}

	rows, err := s.conn().QueryContext(ctx, "SELECT repo_name FROM session_repos ORDER BY repo_name")
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

	rows, err = s.conn().QueryContext(ctx, "SELECT branch_name FROM session_branches ORDER BY branch_name")
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

	rows, err = s.conn().QueryContext(ctx, "SELECT LOWER(email) FROM users ORDER BY email")
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

func (s *Store) queryFilterOptionsScoped(ctx context.Context, userID int64) (db.SessionFilterOptions, error) {
	opts := db.SessionFilterOptions{Repos: make([]string, 0), Branches: make([]string, 0), Owners: make([]string, 0)}

	query := `
		WITH visible AS (
			SELECT id, user_id, git_info FROM sessions WHERE user_id = $1
			UNION
			SELECT s.id, s.user_id, s.git_info FROM sessions s
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients ssr ON sh.id = ssr.share_id
			WHERE ssr.user_id = $1 AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			UNION
			SELECT s.id, s.user_id, s.git_info FROM sessions s
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
				FROM visible v WHERE v.git_info->>'repo_url' IS NOT NULL) r2) r,
			(SELECT array_agg(DISTINCT branch ORDER BY branch) as branches
			 FROM (SELECT v.git_info->>'branch' as branch FROM visible v WHERE v.git_info->>'branch' IS NOT NULL) b2) b,
			(SELECT array_agg(DISTINCT LOWER(u.email) ORDER BY LOWER(u.email)) as owners
			 FROM visible v JOIN users u ON v.user_id = u.id) o
	`

	var repos, branches, owners []string
	err := s.conn().QueryRowContext(ctx, query, userID).Scan(pq.Array(&repos), pq.Array(&branches), pq.Array(&owners))
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

func buildPushdownFilters(pb *paramBuilder, params db.SessionListParams) (commonFilters, ownedOwnerFilter, sharedOwnerFilter, searchJoin string) {
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
			SELECT session_id, array_agg(ref ORDER BY created_at) as prs
			FROM session_github_links WHERE link_type = 'pull_request' GROUP BY session_id
		),
		github_commit_refs AS (
			SELECT session_id, array_agg(ref ORDER BY created_at DESC) as commits
			FROM session_github_links WHERE link_type = 'commit' GROUP BY session_id
		)`

func (s *Store) buildShareAllQuery(userID int64, params db.SessionListParams) (string, []interface{}) {
	pb := newParamBuilder(userID)
	commonFilters, _, sharedOwnerFilter, searchJoin := buildPushdownFilters(pb, params)
	limitP := pb.add(params.PageSize + 1)

	query := `
		WITH` + githubRefCTEs + `
		SELECT` + sessionSelectCols + `,
				(s.user_id = $1) as is_owner,
				CASE WHEN s.user_id = $1 THEN 'owner' ELSE 'system_share' END as access_type,
				CASE WHEN s.user_id = $1 THEN NULL ELSE u.email END as shared_by_email,
				u.email as owner_email
			FROM sessions s
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + searchJoin + `
			WHERE 1=1` + commonFilters + sharedOwnerFilter

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

func (s *Store) buildFilteredSessionsQuery(userID int64, params db.SessionListParams) (string, []interface{}) {
	if s.DB.ShareAllSessions {
		return s.buildShareAllQuery(userID, params)
	}

	pb := newParamBuilder(userID)
	commonFilters, ownedOwnerFilter, sharedOwnerFilter, searchJoin := buildPushdownFilters(pb, params)
	limitP := pb.add(params.PageSize + 1)

	systemSharedCTE := `
			system_shared_sessions AS (
				SELECT DISTINCT ON (s.id)` + sessionSelectCols + `,
					false as is_owner, 'system_share' as access_type,
					u.email as shared_by_email, u.email as owner_email
				FROM sessions s
				JOIN session_shares sh ON s.id = sh.session_id
				JOIN session_share_system sss ON sh.id = sss.share_id
				JOIN users u ON s.user_id = u.id` + sessionStatsJoins + searchJoin + `
				WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
				  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
				ORDER BY s.id, sh.created_at DESC
			)`

	query := `
		WITH` + githubRefCTEs + `,
		owned_sessions AS (
			SELECT` + sessionSelectCols + `,
				true as is_owner, 'owner' as access_type,
				NULL::text as shared_by_email, u.email as owner_email
			FROM sessions s
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + searchJoin + `
			WHERE s.user_id = $1` + commonFilters + ownedOwnerFilter + `
		),
		shared_sessions AS (
			SELECT DISTINCT ON (s.id)` + sessionSelectCols + `,
				false as is_owner, 'private_share' as access_type,
				u.email as shared_by_email, u.email as owner_email
			FROM sessions s
			JOIN session_shares sh ON s.id = sh.session_id
			JOIN session_share_recipients sr ON sh.id = sr.share_id
			JOIN users u ON s.user_id = u.id` + sessionStatsJoins + searchJoin + `
			WHERE sr.user_id = $1
			  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
			  AND s.user_id != $1` + commonFilters + sharedOwnerFilter + `
			ORDER BY s.id, sh.created_at DESC
		),` + systemSharedCTE + `
		SELECT * FROM (
			SELECT DISTINCT ON (id) * FROM (
				SELECT * FROM owned_sessions
				UNION ALL SELECT * FROM shared_sessions
				UNION ALL SELECT * FROM system_shared_sessions
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
		WHERE (COALESCE(last_message_at, first_seen), id) < (` + cursorTimeP + `, ` + cursorIDP + `)`
		}
	}

	query += `
		ORDER BY COALESCE(last_message_at, first_seen) DESC, id DESC
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
			&session.SessionType, &session.TotalLines, &gitRepoURL, &session.GitBranch,
			&githubPRs, &githubCommits, &session.EstimatedCostUSD,
			&session.IsOwner, &session.AccessType, &session.SharedByEmail, &session.OwnerEmail,
		); err != nil {
			return nil, false, "", fmt.Errorf("failed to scan session: %w", err)
		}
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
		return nil, false, "", fmt.Errorf("error iterating sessions: %w", err)
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
	sessionQuery := `
		SELECT s.id, s.external_id, s.custom_title, s.suggested_session_title, s.summary, s.first_user_message, s.first_seen, s.cwd, s.transcript_path, s.git_info, s.last_sync_at, s.hostname, s.username, u.email
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.id = $1 AND s.user_id = $2
	`
	err := s.conn().QueryRowContext(ctx, sessionQuery, sessionID, userID).Scan(
		&session.ID, &session.ExternalID, &session.CustomTitle,
		&session.SuggestedSessionTitle, &session.Summary, &session.FirstUserMessage,
		&session.FirstSeen, &session.CWD, &session.TranscriptPath, &gitInfoBytes,
		&session.LastSyncAt, &session.Hostname, &session.Username, &session.OwnerEmail,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, db.ErrSessionNotFound
		}
		if db.IsInvalidUUIDError(err) {
			return nil, db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

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

	tx, err := s.conn().BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	deleteSessionQuery := `DELETE FROM sessions WHERE id = $1 AND user_id = $2`
	result, err := tx.ExecContext(ctx, deleteSessionQuery, sessionID, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return db.ErrSessionNotFound
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// VerifySessionOwnership checks if a session exists and is owned by the user
func (s *Store) VerifySessionOwnership(ctx context.Context, sessionID string, userID int64) (externalID string, err error) {
	ctx, span := tracer.Start(ctx, "db.verify_session_ownership",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
			attribute.Int64("user.id", userID),
		))
	defer span.End()

	query := `SELECT external_id FROM sessions WHERE id = $1 AND user_id = $2`
	err = s.conn().QueryRowContext(ctx, query, sessionID, userID).Scan(&externalID)
	if err == sql.ErrNoRows {
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM sessions WHERE id = $1)`
		if checkErr := s.conn().QueryRowContext(ctx, checkQuery, sessionID).Scan(&exists); checkErr != nil {
			span.RecordError(checkErr)
			span.SetStatus(codes.Error, checkErr.Error())
			return "", fmt.Errorf("failed to check session existence: %w", checkErr)
		}
		if exists {
			span.SetAttributes(attribute.String("result", "forbidden"))
			return "", db.ErrForbidden
		}
		span.SetAttributes(attribute.String("result", "not_found"))
		return "", db.ErrSessionNotFound
	}
	if err != nil {
		if db.IsInvalidUUIDError(err) {
			span.SetAttributes(attribute.String("result", "not_found"))
			return "", db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to verify session ownership: %w", err)
	}
	span.SetAttributes(attribute.String("result", "owner"))
	return externalID, nil
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

// GetSessionOwnerAndExternalID returns the user_id and external_id for a session
func (s *Store) GetSessionOwnerAndExternalID(ctx context.Context, sessionID string) (userID int64, externalID string, err error) {
	ctx, span := tracer.Start(ctx, "db.get_session_owner_and_external_id",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `SELECT user_id, external_id FROM sessions WHERE id = $1`
	err = s.conn().QueryRowContext(ctx, query, sessionID).Scan(&userID, &externalID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, "", db.ErrSessionNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return 0, "", fmt.Errorf("failed to get session: %w", err)
	}
	span.SetAttributes(attribute.Int64("user.id", userID))
	return userID, externalID, nil
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
