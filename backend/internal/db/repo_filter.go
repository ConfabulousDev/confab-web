package db

import "fmt"

// CF-510 — centralized SQL fragments for repo extraction + fork→upstream
// resolution. These exist so the call sites (Sessions filter list/match,
// org_repos, org_analytics, trends) share one source of truth for the regex
// and the resolution logic.
//
// Resolution is computed live from each session's own git_info — there is no
// shared lookup table. A session's upstream is whatever its tracking_remote
// points at; nothing one session observes affects another. (This replaced the
// global session_repos.root_name dictionary, which collapsed forks across
// sessions/tenants from a single observation.)
//
// The fragments are pure strings; no escape work is needed because every
// caller passes them through database/sql with parameter placeholders.

// repoExtractFromURL extracts `owner/repo` from a git URL SQL expression.
// Strips trailing slashes (CF-509) and a `.git` suffix, then takes the final
// `owner/repo` after the last `/` or `:`. Returns the input unchanged when it
// doesn't match (e.g. a bare string), mirroring Go's ExtractRepoName.
func repoExtractFromURL(urlExpr string) string {
	return fmt.Sprintf(
		`regexp_replace(regexp_replace(regexp_replace(%s, '/+$', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1')`,
		urlExpr)
}

// repoExtractExpr extracts `owner/repo` from a session row's
// git_info->>'repo_url'. The alias is the SQL alias of the sessions table in
// the surrounding query (e.g. "s"). This is the "fork" — the repo the session
// was actually working in, before any upstream resolution.
func repoExtractExpr(alias string) string {
	return repoExtractFromURL(alias + ".git_info->>'repo_url'")
}

// RepoRootExpr resolves a session's fork→upstream root live from its own
// git_info, falling back to the extracted fork when there is no upstream
// signal. Mirrors the retired Go resolver (ResolveForkFromRemotes) exactly:
//
//   - No tracking_remote (or empty) → the fork.
//   - tracking_remote names a remote (case-sensitive, first match wins) whose
//     fetch_url (else push_url) extracts to an owner/repo that differs from the
//     fork case-insensitively → that upstream.
//   - Otherwise (unknown remote, unextractable URL, self-loop) → the fork.
//
// Emitted as a correlated scalar subquery over git_info->'remotes'; valid in
// both SELECT projections and WHERE clauses. The jsonb_typeof guard keeps
// jsonb_array_elements from erroring on a missing or non-array `remotes`.
func RepoRootExpr(alias string) string {
	fork := repoExtractExpr(alias)
	root := repoExtractFromURL(`COALESCE(NULLIF(r->>'fetch_url', ''), r->>'push_url')`)
	return fmt.Sprintf(`CASE
		WHEN NULLIF(%[1]s.git_info->>'tracking_remote', '') IS NULL THEN %[2]s
		WHEN %[2]s IS NULL OR %[2]s = '' THEN %[2]s
		ELSE COALESCE(
			(SELECT CASE
					WHEN %[3]s IS NULL OR %[3]s = '' OR lower(%[3]s) = lower(%[2]s) THEN NULL
					ELSE %[3]s
				END
			 FROM jsonb_array_elements(
					CASE WHEN jsonb_typeof(%[1]s.git_info->'remotes') = 'array'
						 THEN %[1]s.git_info->'remotes' ELSE '[]'::jsonb END) AS r
			 WHERE r->>'name' = %[1]s.git_info->>'tracking_remote'
			 LIMIT 1),
			%[2]s)
	END`, alias, fork, root)
}

// RepoMatchExpr returns a WHERE-clause fragment that compares the resolved
// root repo to a parameter array. paramPlaceholder is the full placeholder
// expression (e.g. "$4" or "$4::text[]").
func RepoMatchExpr(alias, paramPlaceholder string) string {
	return RepoRootExpr(alias) + " = ANY(" + paramPlaceholder + ")"
}

// ListableSessionPredicate is the single source of truth for whether a session
// is "listable" — i.e. eligible to appear in the paginated session list. A
// session qualifies only when it has synced lines (> 0) AND a summary or a
// first_user_message. 0407: both the list query and the filter-option queries
// (session-list + Trends) apply this same fragment so an offered filter option
// can never orphan to an empty list.
//
// The EXISTS(... last_synced_line > 0) form is equivalent to the list query's
// SUM(last_synced_line) > 0 (line counts are non-negative, so any positive line
// implies a positive sum) and needs no aggregate join in the option queries.
//
// alias is the SQL alias of the sessions table in the surrounding query (e.g.
// "s"). The fragment is a pure string with no user input, so no escaping is
// needed.
func ListableSessionPredicate(alias string) string {
	return "EXISTS (SELECT 1 FROM sync_files sf WHERE sf.session_id = " + alias +
		".id AND sf.last_synced_line > 0)" +
		" AND (" + alias + ".summary IS NOT NULL OR " + alias + ".first_user_message IS NOT NULL)"
}
