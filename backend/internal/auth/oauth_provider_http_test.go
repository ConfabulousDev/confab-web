package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// These tests exercise the GitHub and Google provider calls against httptest
// fake servers by pointing the package-level endpoint vars at them. Because the
// vars are package-global, none of these tests may call t.Parallel().

// overrideURL points *target at value for the duration of the test.
func overrideURL(t *testing.T, target *string, value string) {
	t.Helper()
	orig := *target
	*target = value
	t.Cleanup(func() { *target = orig })
}

// closedServerURL returns the URL of a server that has already been shut down,
// so any request to it fails at the transport layer.
func closedServerURL(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	u := srv.URL
	srv.Close()
	return u
}

// serveStatic points *target at a test server that answers every request with
// status and body.
func serveStatic(t *testing.T, target *string, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	overrideURL(t, target, srv.URL)
}

func githubTestConfig() *OAuthConfig {
	return &OAuthConfig{
		GitHubClientID:     "gh-client",
		GitHubClientSecret: "gh-secret",
		GitHubRedirectURL:  "http://localhost:8080/auth/github/callback",
	}
}

func googleTestConfig() *OAuthConfig {
	return &OAuthConfig{
		GoogleClientID:     "g-client",
		GoogleClientSecret: "g-secret",
		GoogleRedirectURL:  "http://localhost:8080/auth/google/callback",
	}
}

// ---------------------------------------------------------------------------
// GitHub token exchange
// ---------------------------------------------------------------------------

func TestExchangeGitHubCode_SendsCredentialsAndPKCEVerifier(t *testing.T) {
	var gotMethod, gotAccept string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAccept = r.Header.Get("Accept")
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"access_token":"gho_token"}`))
	}))
	t.Cleanup(srv.Close)
	overrideURL(t, &githubTokenURL, srv.URL)

	token, err := exchangeGitHubCode("the-code", "the-verifier", githubTestConfig())
	if err != nil {
		t.Fatalf("exchangeGitHubCode: %v", err)
	}
	if token != "gho_token" {
		t.Errorf("token = %q, want gho_token", token)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	want := map[string]string{
		"client_id":     "gh-client",
		"client_secret": "gh-secret",
		"code":          "the-code",
		"redirect_uri":  "http://localhost:8080/auth/github/callback",
		"code_verifier": "the-verifier",
	}
	for k, v := range want {
		if got := gotQuery.Get(k); got != v {
			t.Errorf("token request %s = %q, want %q", k, got, v)
		}
	}
}

func TestExchangeGitHubCode_Errors(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantErrText string
	}{
		{name: "non-JSON body", body: `<html>rate limited</html>`},
		{name: "missing access_token", body: `{"error":"bad_verification_code"}`, wantErrText: "no access token"},
		{name: "empty access_token", body: `{"access_token":""}`, wantErrText: "no access token"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serveStatic(t, &githubTokenURL, http.StatusOK, c.body)

			token, err := exchangeGitHubCode("code", "verifier", githubTestConfig())
			if err == nil {
				t.Fatalf("expected error, got token %q", token)
			}
			if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
				t.Errorf("error = %v, want it to mention %q", err, c.wantErrText)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		overrideURL(t, &githubTokenURL, closedServerURL(t))
		if _, err := exchangeGitHubCode("code", "verifier", githubTestConfig()); err == nil {
			t.Fatal("expected transport error")
		}
	})
}

// ---------------------------------------------------------------------------
// GitHub user + verified email
// ---------------------------------------------------------------------------

// newGitHubAPIFake serves /user and /user/emails with canned bodies and points
// the package URL vars at it. It records the Authorization header seen on each
// path.
func newGitHubAPIFake(t *testing.T, userBody, emailsBody string) map[string]string {
	t.Helper()
	auths := map[string]string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		auths["/user"] = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(userBody))
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		auths["/user/emails"] = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(emailsBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	overrideURL(t, &githubUserURL, srv.URL+"/user")
	overrideURL(t, &githubEmailsURL, srv.URL+"/user/emails")
	return auths
}

func TestGetGitHubUser_UsesVerifiedPrimaryEmailLowercased(t *testing.T) {
	auths := newGitHubAPIFake(t,
		`{"id":42,"login":"octo","email":"public-unverified@evil.com","name":"Octo Cat","avatar_url":"https://a/x.png"}`,
		`[{"email":"other@example.com","primary":false,"verified":true},{"email":"Octo@Example.COM","primary":true,"verified":true}]`,
	)

	user, err := getGitHubUser("tok123")
	if err != nil {
		t.Fatalf("getGitHubUser: %v", err)
	}
	if user.ID != 42 || user.Login != "octo" || user.Name != "Octo Cat" {
		t.Errorf("unexpected profile: %+v", user)
	}
	// SECURITY: the public /user email must be ignored in favor of the
	// primary+verified address from /user/emails, normalized to lowercase.
	if user.Email != "octo@example.com" {
		t.Errorf("email = %q, want octo@example.com", user.Email)
	}
	for _, path := range []string{"/user", "/user/emails"} {
		if auths[path] != "Bearer tok123" {
			t.Errorf("%s Authorization = %q, want Bearer tok123", path, auths[path])
		}
	}
}

func TestGetGitHubUser_Errors(t *testing.T) {
	cases := []struct {
		name        string
		userBody    string
		emailsBody  string
		wantErrText string
	}{
		{
			name:       "malformed user JSON",
			userBody:   `not-json`,
			emailsBody: `[{"email":"a@example.com","primary":true,"verified":true}]`,
		},
		{
			name:        "emails endpoint failure propagates",
			userBody:    `{"id":1,"login":"a"}`,
			emailsBody:  `not-json`,
			wantErrText: "failed to get verified email",
		},
		{
			name:        "invalid email format from GitHub rejected",
			userBody:    `{"id":1,"login":"a"}`,
			emailsBody:  `[{"email":"not an email","primary":true,"verified":true}]`,
			wantErrText: "invalid email format",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newGitHubAPIFake(t, c.userBody, c.emailsBody)
			user, err := getGitHubUser("tok")
			if err == nil {
				t.Fatalf("expected error, got user %+v", user)
			}
			if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
				t.Errorf("error = %v, want it to mention %q", err, c.wantErrText)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		overrideURL(t, &githubUserURL, closedServerURL(t))
		if _, err := getGitHubUser("tok"); err == nil {
			t.Fatal("expected transport error")
		}
	})
}

// TestGetGitHubPrimaryEmail_RequiresPrimaryAndVerified is the SECURITY contract:
// only an address that is BOTH primary and verified is trusted.
func TestGetGitHubPrimaryEmail_RequiresPrimaryAndVerified(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantEmail string
		wantErr   string
	}{
		{
			name:      "picks the primary verified address",
			body:      `[{"email":"a@example.com","primary":false,"verified":true},{"email":"b@example.com","primary":true,"verified":true}]`,
			wantEmail: "b@example.com",
		},
		{
			name:    "primary unverified plus verified non-primary is rejected",
			body:    `[{"email":"attacker@victim.com","primary":true,"verified":false},{"email":"real@example.com","primary":false,"verified":true}]`,
			wantErr: "no verified email",
		},
		{
			name:    "empty list is rejected",
			body:    `[]`,
			wantErr: "no verified email",
		},
		{
			name: "malformed body is rejected",
			body: `{"message":"Bad credentials"}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			newGitHubAPIFake(t, `{}`, c.body)
			email, err := getGitHubPrimaryEmail("tok")
			if c.wantEmail != "" {
				if err != nil {
					t.Fatalf("getGitHubPrimaryEmail: %v", err)
				}
				if email != c.wantEmail {
					t.Errorf("email = %q, want %q", email, c.wantEmail)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error, got email %q", email)
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %v, want it to mention %q", err, c.wantErr)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		overrideURL(t, &githubEmailsURL, closedServerURL(t))
		if _, err := getGitHubPrimaryEmail("tok"); err == nil {
			t.Fatal("expected transport error")
		}
	})
}

// ---------------------------------------------------------------------------
// Google token exchange + userinfo
// ---------------------------------------------------------------------------

func TestExchangeGoogleCode_SendsFormWithPKCEVerifier(t *testing.T) {
	var gotMethod string
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`{"access_token":"ya29.token"}`))
	}))
	t.Cleanup(srv.Close)
	overrideURL(t, &googleTokenURL, srv.URL)

	token, err := exchangeGoogleCode("g-code", "g-verifier", googleTestConfig())
	if err != nil {
		t.Fatalf("exchangeGoogleCode: %v", err)
	}
	if token != "ya29.token" {
		t.Errorf("token = %q, want ya29.token", token)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	want := map[string]string{
		"client_id":     "g-client",
		"client_secret": "g-secret",
		"code":          "g-code",
		"redirect_uri":  "http://localhost:8080/auth/google/callback",
		"grant_type":    "authorization_code",
		"code_verifier": "g-verifier",
	}
	for k, v := range want {
		if got := gotForm.Get(k); got != v {
			t.Errorf("form %s = %q, want %q", k, got, v)
		}
	}
}

func TestExchangeGoogleCode_Errors(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		wantErrText string
	}{
		{name: "error response", status: http.StatusBadRequest, body: `{"error":"invalid_grant","error_description":"Bad Request"}`, wantErrText: "invalid_grant"},
		{name: "missing access_token", status: http.StatusOK, body: `{}`, wantErrText: "no access token"},
		{name: "non-JSON body", status: http.StatusBadGateway, body: `<html>oops</html>`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serveStatic(t, &googleTokenURL, c.status, c.body)

			token, err := exchangeGoogleCode("code", "verifier", googleTestConfig())
			if err == nil {
				t.Fatalf("expected error, got token %q", token)
			}
			if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
				t.Errorf("error = %v, want it to mention %q", err, c.wantErrText)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		overrideURL(t, &googleTokenURL, closedServerURL(t))
		if _, err := exchangeGoogleCode("code", "verifier", googleTestConfig()); err == nil {
			t.Fatal("expected transport error")
		}
	})
}

func TestGetGoogleUser(t *testing.T) {
	t.Run("success lowercases email and sends bearer token", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id":"g-1","email":"Alice@Example.com","verified_email":true,"name":"Alice","picture":"https://p/x.png"}`))
		}))
		t.Cleanup(srv.Close)
		overrideURL(t, &googleUserinfoURL, srv.URL)

		user, err := getGoogleUser("ya29")
		if err != nil {
			t.Fatalf("getGoogleUser: %v", err)
		}
		if gotAuth != "Bearer ya29" {
			t.Errorf("Authorization = %q, want Bearer ya29", gotAuth)
		}
		if user.ID != "g-1" || user.Email != "alice@example.com" || !user.VerifiedEmail {
			t.Errorf("unexpected user: %+v", user)
		}
	})

	cases := []struct {
		name        string
		status      int
		body        string
		wantErrText string
	}{
		{name: "malformed JSON", status: http.StatusOK, body: `not-json`},
		// An error status from Google carries a JSON error object with no email,
		// which must be rejected rather than treated as an (empty) user.
		{name: "error status without email", status: http.StatusUnauthorized, body: `{"error":{"code":401}}`, wantErrText: "invalid email format"},
		{name: "invalid email format", status: http.StatusOK, body: `{"id":"g-1","email":"nope","verified_email":true}`, wantErrText: "invalid email format"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			serveStatic(t, &googleUserinfoURL, c.status, c.body)

			user, err := getGoogleUser("tok")
			if err == nil {
				t.Fatalf("expected error, got user %+v", user)
			}
			if c.wantErrText != "" && !strings.Contains(err.Error(), c.wantErrText) {
				t.Errorf("error = %v, want it to mention %q", err, c.wantErrText)
			}
		})
	}

	t.Run("transport error", func(t *testing.T) {
		overrideURL(t, &googleUserinfoURL, closedServerURL(t))
		if _, err := getGoogleUser("tok"); err == nil {
			t.Fatal("expected transport error")
		}
	})
}
