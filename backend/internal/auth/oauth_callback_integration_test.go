package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

const callbackFrontendURL = "http://frontend.test"

// fakeIdentity is the provider-neutral user a fake IdP returns.
type fakeIdentity struct {
	id       string
	email    string
	verified bool
	name     string
}

// fakeIdP controls what a provider's fake endpoints return.
type fakeIdP struct {
	tokenFails bool
	userFails  bool
	identity   fakeIdentity
}

// callbackProvider wires one OAuth provider's callback handler to fake
// endpoints. setup starts an httptest server serving idp, points the provider
// at it, and returns a config ready for the handler.
type callbackProvider struct {
	name      string // "github" | "google" | "oidc"
	errorCode string // provider-specific error code on exchange/user failure
	path      string
	handler   func(*auth.OAuthConfig, *db.DB) http.HandlerFunc
	setup     func(t *testing.T, idp *fakeIdP) *auth.OAuthConfig
	// supportsUnverified is false for GitHub, whose unverified emails are
	// rejected inside getGitHubUser (covered by the provider unit tests).
	supportsUnverified bool
}

func tokenHandler(idp *fakeIdP) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if idp.tokenFails {
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"fake-token"}`))
	}
}

// writeJSONOrFail writes v as JSON, or a 500 with an unparseable body when fail
// is set.
func writeJSONOrFail(w http.ResponseWriter, fail bool, v any) {
	w.Header().Set("Content-Type", "application/json")
	if fail {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`not-json`))
		return
	}
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

var callbackProviders = []callbackProvider{
	{
		name:      "github",
		errorCode: "github_error",
		path:      "/auth/github/callback",
		handler:   auth.HandleGitHubCallback,
		setup: func(t *testing.T, idp *fakeIdP) *auth.OAuthConfig {
			mux := http.NewServeMux()
			mux.HandleFunc("/token", tokenHandler(idp))
			mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
				writeJSONOrFail(w, idp.userFails, map[string]any{
					"id":    json.Number(idp.identity.id), // GitHub user IDs are numeric
					"login": "login-" + idp.identity.id,
					"name":  idp.identity.name,
				})
			})
			mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
				writeJSONOrFail(w, idp.userFails, []map[string]any{
					{"email": idp.identity.email, "primary": true, "verified": true},
				})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			auth.SetGitHubEndpointsForTest(t, srv.URL+"/token", srv.URL+"/user", srv.URL+"/user/emails")
			return &auth.OAuthConfig{GitHubClientID: "cid", GitHubClientSecret: "sec", GitHubRedirectURL: "http://localhost/cb"}
		},
	},
	{
		name:               "google",
		errorCode:          "google_error",
		path:               "/auth/google/callback",
		handler:            auth.HandleGoogleCallback,
		supportsUnverified: true,
		setup: func(t *testing.T, idp *fakeIdP) *auth.OAuthConfig {
			mux := http.NewServeMux()
			mux.HandleFunc("/token", tokenHandler(idp))
			mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
				writeJSONOrFail(w, idp.userFails, map[string]any{
					"id":             idp.identity.id,
					"email":          idp.identity.email,
					"verified_email": idp.identity.verified,
					"name":           idp.identity.name,
				})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			auth.SetGoogleEndpointsForTest(t, srv.URL+"/token", srv.URL+"/userinfo")
			return &auth.OAuthConfig{GoogleClientID: "cid", GoogleClientSecret: "sec", GoogleRedirectURL: "http://localhost/cb"}
		},
	},
	{
		name:               "oidc",
		errorCode:          "oidc_error",
		path:               "/auth/oidc/callback",
		handler:            auth.HandleOIDCCallback,
		supportsUnverified: true,
		setup: func(t *testing.T, idp *fakeIdP) *auth.OAuthConfig {
			var srv *httptest.Server
			mux := http.NewServeMux()
			mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
				writeJSONOrFail(w, false, map[string]any{
					"issuer":                 srv.URL,
					"authorization_endpoint": srv.URL + "/authorize",
					"token_endpoint":         srv.URL + "/token",
					"userinfo_endpoint":      srv.URL + "/userinfo",
				})
			})
			mux.HandleFunc("/token", tokenHandler(idp))
			mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
				writeJSONOrFail(w, idp.userFails, map[string]any{
					"sub":            idp.identity.id,
					"email":          idp.identity.email,
					"email_verified": idp.identity.verified,
					"name":           idp.identity.name,
				})
			})
			srv = httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			// Fresh config per test: getOIDCEndpoints caches discovery on the config.
			return &auth.OAuthConfig{OIDCClientID: "cid", OIDCClientSecret: "sec", OIDCRedirectURL: "http://localhost/cb", OIDCIssuerURL: srv.URL}
		},
	},
}

// callbackRequest builds a callback request carrying matching state + PKCE
// verifier cookies, plus any extra cookies.
func callbackRequest(path string, extra ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path+"?state=st&code=authcode", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "st"})
	req.AddCookie(&http.Cookie{Name: "oauth_verifier", Value: "ver"})
	for _, c := range extra {
		req.AddCookie(c)
	}
	return req
}

// assertRedirect asserts rec redirects with wantStatus to wantTarget (the
// Location with its query stripped) and returns the Location query.
func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantTarget string) url.Values {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, wantStatus, rec.Body.String())
	}
	raw := rec.Header().Get("Location")
	loc, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("bad Location %q: %v", raw, err)
	}
	target := *loc
	target.RawQuery = ""
	if got := target.String(); got != wantTarget {
		t.Fatalf("redirect = %q, want target %q", raw, wantTarget)
	}
	return loc.Query()
}

// parseLoginRedirect asserts a 307 to <frontend>/login and returns the query.
func parseLoginRedirect(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	return assertRedirect(t, rec, http.StatusTemporaryRedirect, callbackFrontendURL+"/login")
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// countRows runs a SELECT count(*) query and returns the count.
func countRows(t *testing.T, env *testutil.TestEnvironment, query string, args ...any) int {
	t.Helper()
	var n int
	if err := env.DB.Conn().QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func assertNoSessionMinted(t *testing.T, env *testutil.TestEnvironment, rec *httptest.ResponseRecorder) {
	t.Helper()
	if c := findCookie(rec, auth.SessionCookieName); c != nil && c.Value != "" {
		t.Errorf("session cookie set on rejection path: %q", c.Value)
	}
	if n := countRows(t, env, `SELECT count(*) FROM web_sessions`); n != 0 {
		t.Errorf("web_sessions rows = %d, want 0 on rejection path", n)
	}
}

func countUsersByEmail(t *testing.T, env *testutil.TestEnvironment, email string) int {
	t.Helper()
	return countRows(t, env, `SELECT count(*) FROM users WHERE email = $1`, email)
}

// callbackProviderNamed returns the callbackProviders entry with the given name.
func callbackProviderNamed(t *testing.T, name string) callbackProvider {
	t.Helper()
	for _, p := range callbackProviders {
		if p.name == name {
			return p
		}
	}
	t.Fatalf("no callback provider named %q", name)
	return callbackProvider{}
}

// TestOAuthCallbacks_Integration drives each provider's full callback through
// every post-validation branch against fake IdP endpoints and a real database.
func TestOAuthCallbacks_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	for _, p := range callbackProviders {
		t.Run(p.name, func(t *testing.T) {
			// prepare cleans the DB, sets the callback env, and starts a fake IdP.
			prepare := func(t *testing.T, idp *fakeIdP, maxUsers string) *auth.OAuthConfig {
				t.Helper()
				env.CleanDB(t)
				t.Setenv("FRONTEND_URL", callbackFrontendURL)
				t.Setenv("INSECURE_DEV_MODE", "")
				t.Setenv("MAX_USERS", maxUsers)
				return p.setup(t, idp)
			}
			serve := func(config *auth.OAuthConfig, cookies ...*http.Cookie) *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				p.handler(config, env.DB).ServeHTTP(rec, callbackRequest(p.path, cookies...))
				return rec
			}
			alice := func() *fakeIdP {
				return &fakeIdP{identity: fakeIdentity{id: "1001", email: "alice@example.com", verified: true, name: "Alice"}}
			}

			t.Run("token exchange failure redirects with provider error", func(t *testing.T) {
				idp := alice()
				idp.tokenFails = true
				rec := serve(prepare(t, idp, ""))
				if got := parseLoginRedirect(t, rec).Get("error"); got != p.errorCode {
					t.Errorf("error = %q, want %q", got, p.errorCode)
				}
				assertNoSessionMinted(t, env, rec)
			})

			t.Run("user fetch failure redirects with provider error", func(t *testing.T) {
				idp := alice()
				idp.userFails = true
				rec := serve(prepare(t, idp, ""))
				if got := parseLoginRedirect(t, rec).Get("error"); got != p.errorCode {
					t.Errorf("error = %q, want %q", got, p.errorCode)
				}
				assertNoSessionMinted(t, env, rec)
			})

			if p.supportsUnverified {
				t.Run("unverified email is rejected", func(t *testing.T) {
					idp := alice()
					idp.identity.verified = false
					rec := serve(prepare(t, idp, ""))
					if got := parseLoginRedirect(t, rec).Get("error"); got != "email_unverified" {
						t.Errorf("error = %q, want email_unverified", got)
					}
					assertNoSessionMinted(t, env, rec)
					if n := countUsersByEmail(t, env, "alice@example.com"); n != 0 {
						t.Errorf("unverified login created %d user rows", n)
					}
				})
			}

			t.Run("demo identity email is rejected", func(t *testing.T) {
				config := prepare(t, alice(), "")
				config.DemoIdentityEmail = "Alice@Example.com"
				rec := serve(config)
				if got := parseLoginRedirect(t, rec).Get("error"); got != "access_denied" {
					t.Errorf("error = %q, want access_denied", got)
				}
				assertNoSessionMinted(t, env, rec)
				if n := countUsersByEmail(t, env, "alice@example.com"); n != 0 {
					t.Errorf("demo-email login created %d user rows", n)
				}
			})

			t.Run("email domain not permitted is denied", func(t *testing.T) {
				config := prepare(t, alice(), "")
				config.AllowedEmailDomains = []string{"corp.test"}
				rec := serve(config)
				q := parseLoginRedirect(t, rec)
				if got := q.Get("error"); got != "access_denied" {
					t.Errorf("error = %q, want access_denied", got)
				}
				if desc := q.Get("error_description"); desc != "Your email domain is not permitted. Contact your administrator." {
					t.Errorf("error_description = %q", desc)
				}
				assertNoSessionMinted(t, env, rec)
			})

			t.Run("user cap reached denies a new user", func(t *testing.T) {
				config := prepare(t, alice(), "1")
				testutil.CreateTestUser(t, env, "existing@example.com", "Existing")
				rec := serve(config)
				q := parseLoginRedirect(t, rec)
				if got := q.Get("error"); got != "access_denied" {
					t.Errorf("error = %q, want access_denied", got)
				}
				if desc := q.Get("error_description"); desc != "This application has reached its user limit. Please contact the administrator." {
					t.Errorf("error_description = %q", desc)
				}
				if n := countUsersByEmail(t, env, "alice@example.com"); n != 0 {
					t.Errorf("over-cap login created %d user rows", n)
				}
				assertNoSessionMinted(t, env, rec)
			})

			t.Run("auto-link disabled refuses to link an existing account", func(t *testing.T) {
				idp := alice()
				idp.identity.id = "9999"
				idp.identity.name = "Attacker"
				config := prepare(t, idp, "")
				// An existing user with the same email but a different identity.
				existing := testutil.CreateTestUser(t, env, "alice@example.com", "Existing Alice")
				config.AutoLinkEmail = false
				rec := serve(config)
				if got := parseLoginRedirect(t, rec).Get("error"); got != "account_exists" {
					t.Errorf("error = %q, want account_exists", got)
				}
				assertNoSessionMinted(t, env, rec)
				if identities := countRows(t, env, `SELECT count(*) FROM user_identities WHERE user_id = $1`, existing.ID); identities != 1 {
					t.Errorf("identities for existing user = %d, want 1 (no link)", identities)
				}
			})

			t.Run("inactive user is rejected before a session is minted", func(t *testing.T) {
				config := prepare(t, alice(), "")
				// First login creates the user + identity.
				if first := serve(config); findCookie(first, auth.SessionCookieName) == nil {
					t.Fatalf("setup login did not mint a session; Location=%q", first.Header().Get("Location"))
				}
				var userID int64
				if err := env.DB.Conn().QueryRowContext(context.Background(),
					`SELECT id FROM users WHERE email = 'alice@example.com'`).Scan(&userID); err != nil {
					t.Fatalf("lookup user: %v", err)
				}
				if err := (&dbuser.Store{DB: env.DB}).UpdateUserStatus(context.Background(), userID, models.UserStatusInactive); err != nil {
					t.Fatalf("UpdateUserStatus: %v", err)
				}
				if _, err := env.DB.Conn().ExecContext(context.Background(), `DELETE FROM web_sessions`); err != nil {
					t.Fatalf("clear sessions: %v", err)
				}

				rec := serve(config)
				if got := parseLoginRedirect(t, rec).Get("error"); got != "account_inactive" {
					t.Errorf("error = %q, want account_inactive", got)
				}
				assertNoSessionMinted(t, env, rec)
			})

			t.Run("happy path mints a session and redirects to the frontend", func(t *testing.T) {
				rec := serve(prepare(t, alice(), ""))
				if rec.Code != http.StatusTemporaryRedirect {
					t.Fatalf("status = %d, want 307", rec.Code)
				}
				if loc := rec.Header().Get("Location"); loc != callbackFrontendURL {
					t.Errorf("Location = %q, want %q", loc, callbackFrontendURL)
				}
				cookie := findCookie(rec, auth.SessionCookieName)
				if cookie == nil || cookie.Value == "" {
					t.Fatal("session cookie not set")
				}
				if !cookie.HttpOnly {
					t.Error("session cookie must be HttpOnly")
				}
				if cookie.SameSite != http.SameSiteLaxMode {
					t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
				}
				if !cookie.Secure {
					t.Error("session cookie must be Secure when INSECURE_DEV_MODE is unset")
				}
				if !cookie.Expires.After(time.Now().Add(auth.SessionDuration - time.Hour)) {
					t.Errorf("cookie Expires = %v, want ~now+%v", cookie.Expires, auth.SessionDuration)
				}

				// The cookie must resolve to the newly created user.
				var sessionEmail string
				if err := env.DB.Conn().QueryRowContext(context.Background(),
					`SELECT u.email FROM web_sessions ws JOIN users u ON u.id = ws.user_id WHERE ws.id = $1`,
					db.HashToken(cookie.Value)).Scan(&sessionEmail); err != nil {
					t.Fatalf("session row for cookie: %v", err)
				}
				if sessionEmail != "alice@example.com" {
					t.Errorf("session user email = %q, want alice@example.com", sessionEmail)
				}
			})

			t.Run("email mismatch is reported on the post-login redirect", func(t *testing.T) {
				rec := serve(prepare(t, alice(), ""),
					&http.Cookie{Name: "expected_email", Value: "bob@example.com"},
					&http.Cookie{Name: "post_login_redirect", Value: "/sessions/42"},
				)
				q := assertRedirect(t, rec, http.StatusTemporaryRedirect, callbackFrontendURL+"/sessions/42")
				if q.Get("email_mismatch") != "1" || q.Get("expected") != "bob@example.com" || q.Get("actual") != "alice@example.com" {
					t.Errorf("mismatch params = %v", q)
				}
				if findCookie(rec, auth.SessionCookieName) == nil {
					t.Error("mismatch is a warning, not a rejection: session must still be minted")
				}
			})
		})
	}
}

// TestHandleOIDCCallback_DiscoveryFailure: an IdP whose discovery document is
// unavailable yields oidc_error, not a 500 or a session.
func TestHandleOIDCCallback_DiscoveryFailure(t *testing.T) {
	t.Setenv("FRONTEND_URL", callbackFrontendURL)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	config := &auth.OAuthConfig{OIDCClientID: "cid", OIDCIssuerURL: srv.URL}
	rec := httptest.NewRecorder()
	auth.HandleOIDCCallback(config, nil).ServeHTTP(rec, callbackRequest("/auth/oidc/callback"))

	if got := parseLoginRedirect(t, rec).Get("error"); got != "oidc_error" {
		t.Errorf("error = %q, want oidc_error", got)
	}
	if c := findCookie(rec, auth.SessionCookieName); c != nil {
		t.Errorf("session cookie set on discovery failure: %+v", c)
	}
}

// TestHandleOIDCCallback_InvalidEmailRejected: a verified but malformed email
// from the IdP is rejected before any DB access (nil DB proves it).
func TestHandleOIDCCallback_InvalidEmailRejected(t *testing.T) {
	t.Setenv("FRONTEND_URL", callbackFrontendURL)
	oidc := callbackProviderNamed(t, "oidc")
	config := oidc.setup(t, &fakeIdP{identity: fakeIdentity{id: "sub-1", email: "not-an-email", verified: true}})
	rec := httptest.NewRecorder()
	auth.HandleOIDCCallback(config, nil).ServeHTTP(rec, callbackRequest(oidc.path))

	q := parseLoginRedirect(t, rec)
	if got := q.Get("error"); got != "oidc_error" {
		t.Errorf("error = %q, want oidc_error", got)
	}
	if desc := q.Get("error_description"); desc != "Invalid email received from SSO provider." {
		t.Errorf("error_description = %q", desc)
	}
}
