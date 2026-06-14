package db

import (
	"strings"
)

// SanitizeGitInfoForSharing returns a redacted copy of an unmarshaled git_info
// value safe to serve to ANY non-owner viewer — recipient, system, and
// anonymous public alike (d29s/D2: identical treatment; none are entitled to
// remote URLs or host metadata).
//
// It keeps only:
//   - branch — copied verbatim.
//   - repo_url — reduced to a bare "owner/repo" display string with scheme,
//     credentials, and host stripped.
//
// Every other key — the full repo_url, remotes (each carrying fetch_url/
// push_url), tracking_remote, author, commit_message, commit_sha, is_dirty,
// and any unrecognized key — is dropped.
//
// The input is the map[string]interface{} produced by json-unmarshaling the
// git_info JSONB blob; it is never mutated. A nil or non-map value (git_info
// absent or of an unexpected shape) returns nil, as does a blob with no
// whitelisted keys — fail-safe in every case.
func SanitizeGitInfoForSharing(raw interface{}) interface{} {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}

	out := make(map[string]interface{}, 2)

	if branch, ok := m["branch"].(string); ok && branch != "" {
		out["branch"] = branch
	}
	if repoURL, ok := m["repo_url"].(string); ok && repoURL != "" {
		if display := repoDisplayName(repoURL); display != "" {
			out["repo_url"] = display
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// repoDisplayName reduces a git remote URL to a bare "owner/repo" string,
// explicitly dropping scheme, any embedded credentials, AND the host — so no
// credential or host can survive into the display name in any URL shape
// (including single-path-segment remotes, where a trailing-segment regex would
// otherwise capture "user:token@host/repo"). Returns "" when no owner/repo pair
// remains after stripping — never the original URL, so it fails safe.
//
// Handles both URL-style (`scheme://[user[:pass]@]host[:port]/owner/repo`) and
// scp-style (`[user@]host:owner/repo`) remotes.
//
// NOTE: this deliberately does NOT reuse db.ExtractRepoName (helpers.go) or the
// repo_filter.go SQL extraction. Those are display-oriented for the OWNER's own
// data and leak in edge cases — they fall back to returning the ORIGINAL URL on
// an unrecognized shape, and don't strip the host for single-segment paths. For
// non-owner redaction that fallback is a credential/host leak, so this helper
// keeps a stricter, drop-by-default contract. Do not consolidate them without
// preserving fail-safe-drop here.
func repoDisplayName(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	s = strings.TrimRight(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")

	// Drop the scheme (https://, ssh://, git://, …).
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	// Drop credentials: userinfo (`user` or `user:token`) only appears before
	// the first '/', terminated by '@'. Remove everything up to and including it.
	firstSlash := strings.IndexByte(s, '/')
	authority := s
	if firstSlash >= 0 {
		authority = s[:firstSlash]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		s = s[at+1:]
	}

	// Split the remainder (host[:port]/owner/repo or host:owner/repo) on both
	// '/' and ':'. The first non-empty segment is the host — drop it — and the
	// repo display is the LAST TWO of what's left (matching the frontend's
	// formatRepoName, which keeps the trailing two path segments).
	var segs []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == ':' }) {
		if p != "" {
			segs = append(segs, p)
		}
	}
	if len(segs) < 3 {
		// Need host + owner + repo; fewer means no clean owner/repo remains.
		return ""
	}
	return segs[len(segs)-2] + "/" + segs[len(segs)-1]
}
