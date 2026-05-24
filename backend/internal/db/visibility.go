package db

// CF-495 — centralized SQL fragment for the session-visibility predicate
// shared by analytics callers, session-list pagination, and filter-options
// paths. Single source of truth: every consumer that needs to know "which
// sessions can this user see" routes through this helper.
//
// Output columns (UNION ALL — NOT deduplicated by id):
//
//	visible_sessions(id uuid, user_id bigint, owner_email text,
//	                 access_type text, shared_by_email text)
//
// access_type ∈ {'owner', 'private_share', 'system_share'}. shared_by_email
// is NULL when access_type='owner' and equals the session owner's email
// otherwise. owner_email is the session owner's email in original case;
// callers wanting case-insensitive comparison apply LOWER() at the filter
// site (mirrors ListUserSessionsPaginated's owner-filter semantics).
//
// Callers that don't care about access_type / shared_by_email (analytics,
// filter-options) wrap with:
//
//	SELECT DISTINCT id, user_id, owner_email FROM visible_sessions
//
// Pagination consumes the full shape and dedupes via
// DISTINCT ON (id) ORDER BY id, <access_type priority>.
//
// Expects $1 = userID. The share-all variant ignores $1 in row selection
// but $1 must still bind to the query's parameter list, so callers can
// keep userID as $1 regardless of mode.
//
// Mirrors the cross-cutting pattern of db.RepoRootExpr / db.RepoMatchExpr.
func VisibleSessionsCTE(shareAllSessions bool) string {
	if shareAllSessions {
		return visibleSessionsCTEShareAll
	}
	return visibleSessionsCTEDefault
}

// Share-all mode: every session is visible. The owner-or-system branch
// covers everyone; the extra private-share branch ensures a private share
// is still surfaced so the priority dedup (owner > private_share > system_share)
// can prefer it when both apply. Without the private-share branch, a session
// the viewer received as a private share would only appear as system_share.
const visibleSessionsCTEShareAll = `visible_sessions AS (
	SELECT s.id, s.user_id, u.email AS owner_email,
	       CASE WHEN s.user_id = $1 THEN 'owner' ELSE 'system_share' END AS access_type,
	       CASE WHEN s.user_id = $1 THEN NULL ELSE u.email END AS shared_by_email
	FROM sessions s
	JOIN users u ON s.user_id = u.id
	UNION ALL
	SELECT s.id, s.user_id, u.email,
	       'private_share' AS access_type, u.email AS shared_by_email
	FROM sessions s
	JOIN session_shares sh ON s.id = sh.session_id
	JOIN session_share_recipients ssr ON sh.id = ssr.share_id
	JOIN users u ON s.user_id = u.id
	WHERE ssr.user_id = $1
	  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
	  AND s.user_id != $1
)`

// Default mode: UNION ALL of owned ∪ private-share ∪ system-share. Each
// branch stamps its access_type so downstream callers can priority-dedup
// (owner > private_share > system_share). NOT deduplicated by id —
// analytics callers wrap with `SELECT DISTINCT id, user_id, owner_email`.
const visibleSessionsCTEDefault = `visible_sessions AS (
	SELECT s.id, s.user_id, u.email AS owner_email,
	       'owner' AS access_type, NULL::text AS shared_by_email
	FROM sessions s
	JOIN users u ON s.user_id = u.id
	WHERE s.user_id = $1
	UNION ALL
	SELECT s.id, s.user_id, u.email,
	       'private_share' AS access_type, u.email AS shared_by_email
	FROM sessions s
	JOIN session_shares sh ON s.id = sh.session_id
	JOIN session_share_recipients ssr ON sh.id = ssr.share_id
	JOIN users u ON s.user_id = u.id
	WHERE ssr.user_id = $1
	  AND (sh.expires_at IS NULL OR sh.expires_at > NOW())
	  AND s.user_id != $1
	UNION ALL
	SELECT s.id, s.user_id, u.email,
	       'system_share' AS access_type, u.email AS shared_by_email
	FROM sessions s
	JOIN session_shares sh ON s.id = sh.session_id
	JOIN session_share_system sss ON sh.id = sss.share_id
	JOIN users u ON s.user_id = u.id
	WHERE (sh.expires_at IS NULL OR sh.expires_at > NOW())
	  AND s.user_id != $1
)`
