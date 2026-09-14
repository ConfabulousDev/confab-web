package auth

import "testing"

// Test-only hooks for the external auth_test package. Integration tests must
// live in auth_test because internal/testutil imports this package; these
// helpers expose just enough internals without adding exported production API.
// Endpoint overrides are package-global, so callers must not use t.Parallel().

// SetGitHubEndpointsForTest points the GitHub token, user, and emails endpoints
// at test servers for the duration of t.
func SetGitHubEndpointsForTest(t *testing.T, tokenURL, userURL, emailsURL string) {
	t.Helper()
	overrideURL(t, &githubTokenURL, tokenURL)
	overrideURL(t, &githubUserURL, userURL)
	overrideURL(t, &githubEmailsURL, emailsURL)
}

// SetGoogleEndpointsForTest points the Google token and userinfo endpoints at
// test servers for the duration of t.
func SetGoogleEndpointsForTest(t *testing.T, tokenURL, userinfoURL string) {
	t.Helper()
	overrideURL(t, &googleTokenURL, tokenURL)
	overrideURL(t, &googleUserinfoURL, userinfoURL)
}

// SessionAuthResultForTest unpacks the unexported auth result returned by
// AutoImpersonateIfDemo. ok is false when r is nil.
func SessionAuthResultForTest(r *sessionAuthResult) (userID int64, email string, readOnly bool, ok bool) {
	if r == nil {
		return 0, "", false, false
	}
	return r.userID, r.userEmail, r.userReadOnly, true
}
