package db

import "fmt"

// CF-491 — centralized SQL fragments for repo extraction + fork-to-root
// resolution. These exist so the 7+ call sites (Sessions filter list/match,
// TILs filter list/match, org_repos, org_analytics, trends) share one source
// of truth for the regex and the COALESCE-through-session_repos pattern.
//
// The fragments are pure strings; no escape work is needed because every
// caller passes them through database/sql with parameter placeholders.

// repoExtractExpr is the SQL fragment that extracts `owner/repo` from a
// session row's git_info->>'repo_url'. The alias is the SQL alias of the
// sessions table in the surrounding query (e.g. "s" or "v"). Internal to
// this file: external callers should use RepoRootExpr (SELECT projection)
// or RepoMatchExpr (WHERE clause).
func repoExtractExpr(alias string) string {
	return fmt.Sprintf(
		`regexp_replace(regexp_replace(%s.git_info->>'repo_url', '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1')`,
		alias)
}

// RepoRootExpr wraps the extraction in a COALESCE that resolves the
// extracted fork repo through session_repos.root_name. Repos with no
// observed upstream (root_name IS NULL) fall through unchanged.
//
// Shape:
//
//	COALESCE(
//	    (SELECT sr.root_name FROM session_repos sr
//	     WHERE sr.repo_name = <extracted> AND sr.root_name IS NOT NULL),
//	    <extracted>
//	)
func RepoRootExpr(alias string) string {
	extracted := repoExtractExpr(alias)
	return fmt.Sprintf(
		`COALESCE((SELECT sr.root_name FROM session_repos sr WHERE sr.repo_name = %s AND sr.root_name IS NOT NULL), %s)`,
		extracted, extracted)
}

// RepoMatchExpr returns a WHERE-clause fragment that compares the
// resolved root repo to a parameter array. paramPlaceholder is the full
// placeholder expression (e.g. "$4" or "$4::text[]").
func RepoMatchExpr(alias, paramPlaceholder string) string {
	return RepoRootExpr(alias) + " = ANY(" + paramPlaceholder + ")"
}
