package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func countAPIKeys(t *testing.T, env *testutil.TestEnvironment, userID int64) int {
	t.Helper()
	return countRows(t, env, `SELECT count(*) FROM api_keys WHERE user_id = $1`, userID)
}

// seedAPIKeysToLimit inserts MaxAPIKeysPerUser keys for userID in one statement.
func seedAPIKeysToLimit(t *testing.T, env *testutil.TestEnvironment, userID int64) {
	t.Helper()
	if _, err := env.DB.Conn().ExecContext(context.Background(),
		`INSERT INTO api_keys (user_id, key_hash, name, created_at)
		 SELECT $1, 'seed-hash-' || g, 'seed-' || g, NOW() FROM generate_series(1, $2::int) g`,
		userID, db.MaxAPIKeysPerUser); err != nil {
		t.Fatalf("seed api keys: %v", err)
	}
}

// serveCLIAuthorize sends GET /auth/cli/authorize?rawQuery, with a session
// cookie when sessionID is non-empty.
func serveCLIAuthorize(handler http.Handler, rawQuery, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/auth/cli/authorize?"+rawQuery, nil)
	if sessionID != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// assertCLIRedirectToLogin checks the "send to login selector, then come back
// confirmed" response shape.
func assertCLIRedirectToLogin(t *testing.T, rec *httptest.ResponseRecorder, rawQuery string) {
	t.Helper()
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want /login", loc)
	}
	c := findCookie(rec, "cli_redirect")
	if c == nil {
		t.Fatal("cli_redirect cookie not set")
	}
	if want := "/auth/cli/authorize?" + rawQuery + "&confirmed=1"; c.Value != want {
		t.Errorf("cli_redirect = %q, want %q", c.Value, want)
	}
	if !c.HttpOnly || c.MaxAge != 300 {
		t.Errorf("cli_redirect cookie flags: HttpOnly=%v MaxAge=%d", c.HttpOnly, c.MaxAge)
	}
}

func TestHandleCLIAuthorize_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	t.Setenv("FRONTEND_URL", callbackFrontendURL)

	handler := auth.HandleCLIAuthorize(env.DB)
	const query = "callback=http%3A%2F%2Flocalhost%3A9876%2Fcb&name=laptop"

	newUserSession := func(t *testing.T, email string) (int64, string) {
		t.Helper()
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, email, "CLI User")
		sid := "cli-session-" + email
		// UTC: web_sessions.expires_at is a timezone-naive TIMESTAMP compared to NOW().
		testutil.CreateTestWebSession(t, env, sid, user.ID, time.Now().UTC().Add(time.Hour))
		return user.ID, sid
	}

	t.Run("no session cookie redirects to login with cli_redirect", func(t *testing.T) {
		rec := serveCLIAuthorize(handler, query, "")
		assertCLIRedirectToLogin(t, rec, query)
	})

	t.Run("invalid session clears the stale cookie and redirects to login", func(t *testing.T) {
		env.CleanDB(t)
		rec := serveCLIAuthorize(handler, query, "no-such-session")
		assertCLIRedirectToLogin(t, rec, query)
		if c := findCookie(rec, auth.SessionCookieName); c == nil || c.MaxAge != -1 {
			t.Errorf("stale session cookie must be cleared (MaxAge=-1), got %+v", c)
		}
	})

	t.Run("read-only demo session is denied and no key is minted", func(t *testing.T) {
		env.CleanDB(t)
		const demoEmail = "demo@confab.test"
		if err := auth.BootstrapDemoIdentity(context.Background(), env.DB, demoEmail, demoCSRFSecret); err != nil {
			t.Fatalf("BootstrapDemoIdentity: %v", err)
		}
		var demoID int64
		if err := env.DB.Conn().QueryRowContext(context.Background(),
			`SELECT id FROM users WHERE email = $1`, demoEmail).Scan(&demoID); err != nil {
			t.Fatalf("lookup demo user: %v", err)
		}

		rec := serveCLIAuthorize(handler, query+"&confirmed=1", auth.DemoSessionCookieID(demoCSRFSecret, demoEmail))

		if got := parseLoginRedirect(t, rec).Get("error"); got != "access_denied" {
			t.Errorf("error = %q, want access_denied", got)
		}
		if c := findCookie(rec, auth.SessionCookieName); c == nil || c.MaxAge != -1 {
			t.Errorf("demo session cookie must be cleared, got %+v", c)
		}
		if n := countAPIKeys(t, env, demoID); n != 0 {
			t.Errorf("read-only user got %d API keys, want 0", n)
		}
	})

	t.Run("valid session without confirmation shows the provider selector", func(t *testing.T) {
		userID, sid := newUserSession(t, "unconfirmed@example.com")
		rec := serveCLIAuthorize(handler, query, sid)
		assertCLIRedirectToLogin(t, rec, query)
		if n := countAPIKeys(t, env, userID); n != 0 {
			t.Errorf("unconfirmed request minted %d keys", n)
		}
	})

	t.Run("confirmed request mints a key and redirects to the localhost callback", func(t *testing.T) {
		userID, sid := newUserSession(t, "confirmed@example.com")
		rec := serveCLIAuthorize(handler, query+"&confirmed=1", sid)

		key := assertRedirect(t, rec, http.StatusTemporaryRedirect, "http://localhost:9876/cb").Get("key")
		if !strings.HasPrefix(key, "cfb_") {
			t.Fatalf("callback key = %q, want cfb_ prefix", key)
		}
		var name string
		if err := env.DB.Conn().QueryRowContext(context.Background(),
			`SELECT name FROM api_keys WHERE user_id = $1 AND key_hash = $2`, userID, auth.HashAPIKey(key)).Scan(&name); err != nil {
			t.Fatalf("minted key not stored under its hash: %v", err)
		}
		if name != "laptop" {
			t.Errorf("key name = %q, want laptop", name)
		}
	})

	t.Run("confirmed request without a name uses the default key name", func(t *testing.T) {
		userID, sid := newUserSession(t, "noname@example.com")
		rec := serveCLIAuthorize(handler, "callback=http%3A%2F%2F127.0.0.1%3A5000%2F&confirmed=1", sid)
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want 307", rec.Code)
		}
		var name string
		if err := env.DB.Conn().QueryRowContext(context.Background(),
			`SELECT name FROM api_keys WHERE user_id = $1`, userID).Scan(&name); err != nil {
			t.Fatalf("lookup key: %v", err)
		}
		if name != "CLI Key" {
			t.Errorf("key name = %q, want CLI Key", name)
		}
	})

	t.Run("missing or non-localhost callback is rejected without minting", func(t *testing.T) {
		for _, tc := range []struct {
			query, wantBody string
		}{
			{"confirmed=1", "Missing callback parameter"},
			{"callback=http%3A%2F%2Fevil.com%2Fsteal&confirmed=1", "Callback must be localhost"},
			{"callback=http%3A%2F%2Flocalhost%40evil.com%2F&confirmed=1", "Callback must be localhost"},
		} {
			userID, sid := newUserSession(t, "badcb@example.com")
			rec := serveCLIAuthorize(handler, tc.query, sid)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want 400", tc.query, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Errorf("%s: body = %q, want %q", tc.query, rec.Body.String(), tc.wantBody)
			}
			if n := countAPIKeys(t, env, userID); n != 0 {
				t.Errorf("%s: minted %d keys on rejected callback", tc.query, n)
			}
		}
	})

	t.Run("API key limit returns 409 page pointing back to the CLI", func(t *testing.T) {
		userID, sid := newUserSession(t, "full@example.com")
		seedAPIKeysToLimit(t, env, userID)

		rec := serveCLIAuthorize(handler, "callback=http%3A%2F%2Flocalhost%3A9876%2Fcb&name=new-machine&confirmed=1", sid)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "http://localhost:9876/cb?error=api_key_limit_exceeded") {
			t.Errorf("limit page must redirect CLI with error param; body=%q", body)
		}
		if !strings.Contains(body, callbackFrontendURL+"/settings/api-keys") {
			t.Errorf("limit page must link to key management; body=%q", body)
		}
		if n := countAPIKeys(t, env, userID); n != db.MaxAPIKeysPerUser {
			t.Errorf("key count = %d, want unchanged %d", n, db.MaxAPIKeysPerUser)
		}
	})
}

func TestHandleDevicePage_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	handler := auth.HandleDevicePage(env.DB)

	t.Run("logged out redirects to login preserving the escaped code", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/device?code=ABCD-2345", nil))
		if rec.Code != http.StatusTemporaryRedirect {
			t.Fatalf("status = %d, want 307", rec.Code)
		}
		want := "/login?redirect=" + url.QueryEscape("/auth/device?code=ABCD-2345")
		if loc := rec.Header().Get("Location"); loc != want {
			t.Errorf("Location = %q, want %q", loc, want)
		}
	})

	t.Run("logged out without code redirects back to the bare device page", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/device", nil))
		want := "/login?redirect=" + url.QueryEscape("/auth/device")
		if loc := rec.Header().Get("Location"); loc != want {
			t.Errorf("Location = %q, want %q", loc, want)
		}
	})

	t.Run("invalid session is treated as logged out", func(t *testing.T) {
		env.CleanDB(t)
		req := httptest.NewRequest(http.MethodGet, "/auth/device?code=ABCD-2345", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "bogus"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		want := "/login?redirect=" + url.QueryEscape("/auth/device?code=ABCD-2345")
		if rec.Code != http.StatusTemporaryRedirect || rec.Header().Get("Location") != want {
			t.Errorf("status=%d Location=%q, want 307 to %q", rec.Code, rec.Header().Get("Location"), want)
		}
	})

	t.Run("logged in renders the form with the code HTML-escaped", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "device-page@example.com", "Device")
		testutil.CreateTestWebSession(t, env, "device-page-session", user.ID, time.Now().UTC().Add(time.Hour))

		req := httptest.NewRequest(http.MethodGet, "/auth/device?code="+url.QueryEscape(`"><script>alert(1)</script>`), nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "device-page-session"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", ct)
		}
		body := rec.Body.String()
		if strings.Contains(body, "<script>alert(1)</script>") {
			t.Error("device page reflected the code unescaped (XSS)")
		}
		if !strings.Contains(body, `value="&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;"`) {
			t.Errorf("escaped code not pre-filled in the form; body=%q", body)
		}
	})
}

func postDeviceToken(handler http.HandlerFunc, body string) (*httptest.ResponseRecorder, auth.DeviceTokenResponse) {
	req := httptest.NewRequest(http.MethodPost, "/auth/device/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var resp auth.DeviceTokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec, resp
}

// TestHandleDeviceToken_RemainingBranches covers the device-token branches the
// HTTP integration suite in internal/api/auth does not: malformed JSON, the
// email-domain restriction, and the API key limit.
func TestHandleDeviceToken_RemainingBranches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	authorizedCode := func(t *testing.T, email string) (int64, string) {
		t.Helper()
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, email, "Device User")
		deviceCode := "device-code-" + email
		testutil.CreateTestDeviceCode(t, env, deviceCode, "WXYZ-2345", "cli", time.Now().UTC().Add(time.Minute))
		testutil.AuthorizeTestDeviceCode(t, env, "WXYZ-2345", user.ID)
		return user.ID, deviceCode
	}
	deviceCodeExists := func(t *testing.T, deviceCode string) bool {
		t.Helper()
		return countRows(t, env, `SELECT count(*) FROM device_codes WHERE device_code = $1`, db.HashToken(deviceCode)) > 0
	}
	body := func(code string) string { return fmt.Sprintf(`{"device_code":%q}`, code) }

	t.Run("malformed JSON is invalid_request", func(t *testing.T) {
		rec, resp := postDeviceToken(auth.HandleDeviceToken(env.DB, nil), `{not json`)
		if rec.Code != http.StatusBadRequest || resp.Error != "invalid_request" {
			t.Errorf("status=%d error=%q, want 400 invalid_request", rec.Code, resp.Error)
		}
	})

	t.Run("authorized user outside allowed domains is denied and the code consumed", func(t *testing.T) {
		userID, code := authorizedCode(t, "outsider@example.com")
		rec, resp := postDeviceToken(auth.HandleDeviceToken(env.DB, []string{"corp.test"}), body(code))
		if rec.Code != http.StatusForbidden || resp.Error != "access_denied" {
			t.Errorf("status=%d error=%q, want 403 access_denied", rec.Code, resp.Error)
		}
		if resp.AccessToken != "" {
			t.Error("access token issued to a disallowed domain")
		}
		if n := countAPIKeys(t, env, userID); n != 0 {
			t.Errorf("minted %d keys for disallowed domain", n)
		}
		if deviceCodeExists(t, code) {
			t.Error("denied device code must be deleted so it cannot be retried")
		}
	})

	t.Run("authorized user inside allowed domains gets a token", func(t *testing.T) {
		userID, code := authorizedCode(t, "insider@corp.test")
		rec, resp := postDeviceToken(auth.HandleDeviceToken(env.DB, []string{"corp.test"}), body(code))
		if rec.Code != http.StatusOK || !strings.HasPrefix(resp.AccessToken, "cfb_") || resp.TokenType != "Bearer" {
			t.Fatalf("status=%d resp=%+v, want 200 with cfb_ Bearer token", rec.Code, resp)
		}
		if n := countAPIKeys(t, env, userID); n != 1 {
			t.Errorf("key count = %d, want 1", n)
		}
		if deviceCodeExists(t, code) {
			t.Error("device code must be single-use (deleted after token exchange)")
		}
	})

	t.Run("API key limit returns 409 api_key_limit_exceeded", func(t *testing.T) {
		userID, code := authorizedCode(t, "full-device@example.com")
		seedAPIKeysToLimit(t, env, userID)
		rec, resp := postDeviceToken(auth.HandleDeviceToken(env.DB, nil), body(code))
		if rec.Code != http.StatusConflict || resp.Error != "api_key_limit_exceeded" {
			t.Errorf("status=%d error=%q, want 409 api_key_limit_exceeded", rec.Code, resp.Error)
		}
		if resp.AccessToken != "" {
			t.Error("access token issued despite key limit")
		}
	})
}
