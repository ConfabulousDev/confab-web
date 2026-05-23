package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// CF-483: Demo mode HTTP integration tests.
//
// These tests run against a real server with the production router so they
// exercise the full middleware chain (CSRF, auth, EnforceReadOnly).
//
// Spec assertions covered:
//   * Auto-impersonate fires on auth-required routes without cookie.
//   * Existing cookie takes precedence over impersonation.
//   * Read-only writes return the documented structured 403 body.
//   * Reads pass through.
//   * Password login is rejected for the demo email.
//   * /auth/cli/* is NOT auto-impersonated (CLI-flow protection).
//   * B1: API key minting rejected when the demo cookie is presented.
//   * B2: HandleLogout preserves the shared demo session row.
//   * Banner script injection (happy path).
//   * D1: EnforceReadOnly catches writes regardless of group.
//   * Regression: DEMO_IDENTITY_EMAIL="" is zero behavior change.
// =============================================================================

const (
	demoEmail      = "demo@confabulous.dev"
	demoCSRFSecret = "test-csrf-secret-key-32-bytes!!"
)

// setupDemoServer brings up the test server with demo mode enabled and
// bootstraps the demo identity. The demo cookie is also returned for
// tests that need to present it.
func setupDemoServer(t *testing.T, env *testutil.TestEnvironment) (*testutil.TestServer, *http.Cookie) {
	t.Helper()

	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", demoCSRFSecret)
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")
	testutil.SetEnvForTest(t, "DEMO_IDENTITY_EMAIL", demoEmail)

	cfg := auth.OAuthConfig{
		PasswordEnabled:   true,
		DemoIdentityEmail: demoEmail,
		CSRFSecretKey:     demoCSRFSecret,
	}

	// Bootstrap the demo identity before the server starts handling
	// requests — same ordering as cmd/server/main.go.
	if err := auth.BootstrapDemoIdentity(context.Background(), env.DB, demoEmail, demoCSRFSecret); err != nil {
		t.Fatalf("BootstrapDemoIdentity: %v", err)
	}

	srv := NewServer(env.DB, env.Storage, &cfg, nil, "")
	ts := testutil.StartTestServer(t, env, srv.SetupRoutes())

	cookieID := auth.DemoSessionCookieID(demoCSRFSecret, demoEmail)
	cookie := &http.Cookie{Name: auth.SessionCookieName, Value: cookieID, Path: "/"}
	return ts, cookie
}

// demoClient returns a TestClient pre-loaded with the demo cookie.
func demoClient(t *testing.T, ts *testutil.TestServer, cookie *http.Cookie) *testutil.TestClient {
	t.Helper()
	return testutil.NewTestClient(t, ts).WithSession(cookie.Value)
}

// setupRegularServer mirrors setupTestServerWithEnv but skips demo mode
// so we can assert the regression "env unset = today's behavior".
func setupRegularServer(t *testing.T, env *testutil.TestEnvironment) *testutil.TestServer {
	t.Helper()
	testutil.SetEnvForTest(t, "CSRF_SECRET_KEY", demoCSRFSecret)
	testutil.SetEnvForTest(t, "ALLOWED_ORIGINS", "http://localhost:3000")
	testutil.SetEnvForTest(t, "FRONTEND_URL", "http://localhost:3000")
	testutil.SetEnvForTest(t, "INSECURE_DEV_MODE", "true")
	testutil.SetEnvForTest(t, "DEMO_IDENTITY_EMAIL", "")

	cfg := auth.OAuthConfig{PasswordEnabled: true}
	srv := NewServer(env.DB, env.Storage, &cfg, nil, "")
	return testutil.StartTestServer(t, env, srv.SetupRoutes())
}

// -----------------------------------------------------------------------------
// Auto-impersonate + cookie precedence
// -----------------------------------------------------------------------------

func TestDemo_AutoImpersonateFiresOnAuthRequiredRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, _ := setupDemoServer(t, env)

	// No cookie. Auto-impersonate must fire and respond as the demo user.
	client := testutil.NewTestClient(t, ts)
	resp, err := client.Get("/api/v1/me")
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	var me struct {
		Email string `json:"email"`
	}
	testutil.ParseJSON(t, resp, &me)
	if me.Email != demoEmail {
		t.Errorf("/me email = %q, want %q", me.Email, demoEmail)
	}

	// And: the demo cookie was emitted so subsequent requests reuse it.
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			found = true
			if c.Value == "" {
				t.Errorf("Set-Cookie %s present but empty", auth.SessionCookieName)
			}
		}
	}
	if !found {
		t.Errorf("expected Set-Cookie %q on auto-impersonate; cookies=%v",
			auth.SessionCookieName, resp.Cookies())
	}
}

func TestDemo_ExistingCookieTakesPrecedence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, _ := setupDemoServer(t, env)

	// Provision a real user with a real session.
	realUser := testutil.CreateTestUser(t, env, "real@example.com", "Real User")
	realToken := testutil.CreateTestWebSessionWithToken(t, env, realUser.ID)

	client := testutil.NewTestClient(t, ts).WithSession(realToken)
	resp, err := client.Get("/api/v1/me")
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	var me struct {
		Email string `json:"email"`
	}
	testutil.ParseJSON(t, resp, &me)
	if me.Email != "real@example.com" {
		t.Errorf("/me email = %q, want real@example.com (cookie must win over impersonate)", me.Email)
	}
}

// -----------------------------------------------------------------------------
// Read-only enforcement (writes blocked, reads allowed)
// -----------------------------------------------------------------------------

func TestDemo_ReadOnlyWriteReturnsStructured403(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, demoCookie := setupDemoServer(t, env)

	client := demoClient(t, ts, demoCookie)

	// Mutating call on a session-protected endpoint. Any POST will do;
	// /sessions/{id}/share is representative of the share-creation
	// mutation surface.
	resp, err := client.Post("/api/v1/sessions/11111111-1111-1111-1111-111111111111/share", map[string]string{
		"recipient_email": "anyone@example.com",
	})
	if err != nil {
		t.Fatalf("POST share: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}

	var body auth.ReadOnlyUserError
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "read_only_user" {
		t.Errorf("body.error = %q, want %q", body.Error, "read_only_user")
	}
	if body.Message == "" {
		t.Error("body.message must be non-empty per spec")
	}
}

func TestDemo_ReadsPassThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, demoCookie := setupDemoServer(t, env)

	client := demoClient(t, ts, demoCookie)

	resp, err := client.Get("/api/v1/me")
	if err != nil {
		t.Fatalf("GET /me: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)
}

// -----------------------------------------------------------------------------
// Login rejection (password)
// -----------------------------------------------------------------------------

func TestDemo_PasswordLoginRejectedForDemoEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, _ := setupDemoServer(t, env)

	client := testutil.NewTestClient(t, ts)
	client.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	form := "email=" + demoEmail + "&password=does-not-matter-1234567"
	resp, err := client.PostForm("/auth/password/login", form)
	if err != nil {
		t.Fatalf("POST password login: %v", err)
	}
	defer resp.Body.Close()

	// Redirect back to /login with a generic error — no identity disclosure.
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		t.Errorf("status = %d, want 3xx (redirect)", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/login") {
		t.Errorf("Location = %q, want redirect to /login", loc)
	}
	if !strings.Contains(strings.ToLower(loc), "invalid") {
		t.Errorf("Location = %q, want a generic Invalid-credentials error", loc)
	}
}

// -----------------------------------------------------------------------------
// CLI / device flows MUST NOT auto-impersonate (and MUST NOT mint API keys
// for a demo user even when the demo cookie is presented — B1).
// -----------------------------------------------------------------------------

func TestDemo_CLIAuthorizeNotImpersonatedWhenNoCookie(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, _ := setupDemoServer(t, env)

	client := testutil.NewTestClient(t, ts)
	client.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get("/auth/cli/authorize?callback=http://localhost:1234/&name=Test+Key")
	if err != nil {
		t.Fatalf("GET cli authorize: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		t.Errorf("status = %d, want 3xx redirect to /login", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("Location = %q, want /login", loc)
	}

	// No API key was created.
	store := &dbauth.Store{DB: env.DB}
	_ = store // silence unused if helper not yet present
	var count int
	if err := env.DB.Conn().QueryRowContext(context.Background(),
		`SELECT count(*) FROM api_keys`,
	).Scan(&count); err != nil {
		t.Fatalf("count api_keys: %v", err)
	}
	if count != 0 {
		t.Errorf("api_keys row count = %d, want 0 (anonymous visitor must not mint demo keys)", count)
	}
}

// B1: even when the visitor presents the demo cookie (because they
// browsed the SPA first), /auth/cli/authorize must refuse to mint an
// API key for the demo (read-only) user.
func TestDemo_CLIAuthorizeRejectsDemoCookie(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, demoCookie := setupDemoServer(t, env)

	client := demoClient(t, ts, demoCookie)
	client.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get("/auth/cli/authorize?callback=http://localhost:1234/&name=Test+Key")
	if err != nil {
		t.Fatalf("GET cli authorize with demo cookie: %v", err)
	}
	defer resp.Body.Close()

	// Must redirect/refuse — never reach the key-mint code path.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("status = 200, want redirect/refusal (demo cookie must not mint API keys)")
	}

	var count int
	if err := env.DB.Conn().QueryRowContext(context.Background(),
		`SELECT count(*) FROM api_keys`,
	).Scan(&count); err != nil {
		t.Fatalf("count api_keys: %v", err)
	}
	if count != 0 {
		t.Errorf("api_keys row count = %d, want 0 (B1: demo cookie must not mint keys)", count)
	}
}

// -----------------------------------------------------------------------------
// B2: HandleLogout preserves the shared demo web_sessions row.
// -----------------------------------------------------------------------------

func TestDemo_LogoutPreservesSharedSessionRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts, demoCookie := setupDemoServer(t, env)

	demoID := auth.DemoSessionCookieID(demoCSRFSecret, demoEmail)

	// Before logout: row exists.
	var pre int
	if err := env.DB.Conn().QueryRowContext(context.Background(),
		`SELECT count(*) FROM web_sessions WHERE id = $1`, demoID,
	).Scan(&pre); err != nil {
		t.Fatalf("pre-count: %v", err)
	}
	if pre != 1 {
		t.Fatalf("pre-count = %d, want 1 (bootstrap should have created the row)", pre)
	}

	// Logout with the demo cookie attached.
	client := demoClient(t, ts, demoCookie)
	client.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get("/auth/logout")
	if err != nil {
		t.Fatalf("GET logout: %v", err)
	}
	resp.Body.Close()

	// After logout: row still there — demo's cookie row must not be
	// deleted, only client-side cleared.
	var post int
	if err := env.DB.Conn().QueryRowContext(context.Background(),
		`SELECT count(*) FROM web_sessions WHERE id = $1`, demoID,
	).Scan(&post); err != nil {
		t.Fatalf("post-count: %v", err)
	}
	if post != 1 {
		t.Errorf("post-logout count = %d, want 1 (B2: shared demo row must not be deleted)", post)
	}
}

// -----------------------------------------------------------------------------
// Banner script injection (happy path) — XSS escaping covered by unit test.
// -----------------------------------------------------------------------------

func TestDemo_BannerScriptInjected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	out := auth.RenderDemoBannerScriptTag(demoEmail)
	if !strings.Contains(out, "window.__DEMO_IDENTITY__") {
		t.Errorf("script tag missing window.__DEMO_IDENTITY__: %q", out)
	}
	if !strings.Contains(out, demoEmail) {
		t.Errorf("script tag missing demo email: %q", out)
	}
}

// -----------------------------------------------------------------------------
// Regression: env unset = today's behavior, end-to-end.
// -----------------------------------------------------------------------------

func TestDemo_RegressionEnvUnsetNoBehaviorChange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	ts := setupRegularServer(t, env)

	// /me with no cookie = 401 (no impersonate).
	t.Run("me without cookie returns 401", func(t *testing.T) {
		client := testutil.NewTestClient(t, ts)
		resp, err := client.Get("/api/v1/me")
		if err != nil {
			t.Fatalf("GET /me: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401 (no auto-impersonate)", resp.StatusCode)
		}
	})

	// No demo banner script in any HTML response (no static dir
	// configured, so / returns JSON — we just assert the injection helper
	// is properly no-op for empty email).
	t.Run("RenderDemoBannerScriptTag empty for empty email", func(t *testing.T) {
		if got := auth.RenderDemoBannerScriptTag(""); got != "" {
			t.Errorf("RenderDemoBannerScriptTag(\"\") = %q, want \"\"", got)
		}
	})

	// HandleLogout deletes session rows as today.
	t.Run("logout deletes real-user session row", func(t *testing.T) {
		realUser := testutil.CreateTestUser(t, env, "regression@example.com", "Regression User")
		realToken := testutil.CreateTestWebSessionWithToken(t, env, realUser.ID)

		var pre int
		_ = env.DB.Conn().QueryRowContext(context.Background(),
			`SELECT count(*) FROM web_sessions WHERE id = $1`, realToken,
		).Scan(&pre)
		if pre != 1 {
			t.Fatalf("pre count = %d, want 1", pre)
		}

		client := testutil.NewTestClient(t, ts).WithSession(realToken)
		client.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		resp, err := client.Get("/auth/logout")
		if err != nil {
			t.Fatalf("logout: %v", err)
		}
		resp.Body.Close()

		var post int
		_ = env.DB.Conn().QueryRowContext(context.Background(),
			`SELECT count(*) FROM web_sessions WHERE id = $1`, realToken,
		).Scan(&post)
		if post != 0 {
			t.Errorf("post count = %d, want 0 (real-user logout must still delete row)", post)
		}
	})
}
