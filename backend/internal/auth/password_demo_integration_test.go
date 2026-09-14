package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func postPasswordLogin(handler http.HandlerFunc, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/password/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// loginErrorRedirect asserts a 303 to /login and returns the query.
func loginErrorRedirect(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	return assertRedirect(t, rec, http.StatusSeeOther, "/login")
}

// TestHandlePasswordLogin_RemainingBranches covers the password-login branches
// not exercised by TestHandlePasswordLogin: demo email, domain restriction,
// oversize password, locked account, inactive user, redirect preservation, and
// the post-login redirect sequence.
func TestHandlePasswordLogin_RemainingBranches(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	ctx := context.Background()
	t.Setenv("FRONTEND_URL", callbackFrontendURL)

	const password = "correct-horse-battery"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	authStore := &dbauth.Store{DB: env.DB}
	newPasswordUser := func(t *testing.T, email string) *models.User {
		t.Helper()
		env.CleanDB(t)
		u, err := authStore.CreatePasswordUser(ctx, email, hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser: %v", err)
		}
		return u
	}
	creds := func(email, pw string) url.Values {
		return url.Values{"email": {email}, "password": {pw}}
	}
	plain := auth.HandlePasswordLogin(env.DB, &auth.OAuthConfig{})

	t.Run("demo identity email gets the generic invalid-credentials copy", func(t *testing.T) {
		newPasswordUser(t, "demo@example.com")
		handler := auth.HandlePasswordLogin(env.DB, &auth.OAuthConfig{DemoIdentityEmail: "demo@example.com"})
		rec := postPasswordLogin(handler, creds("DEMO@example.com", password))
		if got := loginErrorRedirect(t, rec).Get("error"); got != "Invalid email or password" {
			t.Errorf("error = %q, want generic invalid-credentials copy", got)
		}
		assertNoSessionMinted(t, env, rec)
	})

	t.Run("email domain not permitted is rejected before authentication", func(t *testing.T) {
		newPasswordUser(t, "user@example.com")
		handler := auth.HandlePasswordLogin(env.DB, &auth.OAuthConfig{AllowedEmailDomains: []string{"corp.test"}})
		rec := postPasswordLogin(handler, creds("user@example.com", password))
		if got := loginErrorRedirect(t, rec).Get("error"); got != "Your email domain is not permitted. Contact your administrator." {
			t.Errorf("error = %q, want domain rejection", got)
		}
		assertNoSessionMinted(t, env, rec)
	})

	t.Run("password over 1024 bytes is rejected", func(t *testing.T) {
		q := loginErrorRedirect(t, postPasswordLogin(plain, creds("user@example.com", strings.Repeat("x", 1025))))
		if got := q.Get("error"); got != "Password is too long" {
			t.Errorf("error = %q, want Password is too long", got)
		}
	})

	t.Run("validation errors preserve the redirect parameter", func(t *testing.T) {
		form := creds("user@example.com", "")
		form.Set("redirect", "/sessions/7")
		q := loginErrorRedirect(t, postPasswordLogin(plain, form))
		if got := q.Get("error"); got != "Password is required" {
			t.Errorf("error = %q, want Password is required", got)
		}
		if got := q.Get("redirect"); got != "/sessions/7" {
			t.Errorf("redirect = %q, want /sessions/7", got)
		}
	})

	t.Run("locked account reports temporary lock and mints no session", func(t *testing.T) {
		newPasswordUser(t, "locked@example.com")
		if _, err := env.DB.Conn().ExecContext(ctx,
			`UPDATE identity_passwords SET locked_until = NOW() + interval '1 hour'
			 WHERE identity_id IN (
			   SELECT i.id FROM user_identities i JOIN users u ON u.id = i.user_id
			   WHERE u.email = 'locked@example.com' AND i.provider = 'password')`); err != nil {
			t.Fatalf("lock account: %v", err)
		}
		rec := postPasswordLogin(plain, creds("locked@example.com", password))
		if got := loginErrorRedirect(t, rec).Get("error"); got != "Account is temporarily locked. Please try again later." {
			t.Errorf("error = %q, want temporary lock message", got)
		}
		assertNoSessionMinted(t, env, rec)
	})

	t.Run("inactive user cannot log in and gets the generic copy", func(t *testing.T) {
		u := newPasswordUser(t, "inactive@example.com")
		if err := (&dbuser.Store{DB: env.DB}).UpdateUserStatus(ctx, u.ID, models.UserStatusInactive); err != nil {
			t.Fatalf("UpdateUserStatus: %v", err)
		}
		rec := postPasswordLogin(plain, creds("inactive@example.com", password))
		if got := loginErrorRedirect(t, rec).Get("error"); got != "Invalid email or password" {
			t.Errorf("error = %q, want generic copy (must not reveal inactive state)", got)
		}
		assertNoSessionMinted(t, env, rec)
	})

	t.Run("post_login_redirect to a frontend path is honored", func(t *testing.T) {
		newPasswordUser(t, "redir@example.com")
		rec := postPasswordLogin(plain, creds("redir@example.com", password),
			&http.Cookie{Name: "post_login_redirect", Value: "/sessions/9"})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != callbackFrontendURL+"/sessions/9" {
			t.Errorf("Location = %q, want %s/sessions/9", loc, callbackFrontendURL)
		}
		if c := findCookie(rec, "post_login_redirect"); c == nil || c.MaxAge != -1 {
			t.Error("post_login_redirect cookie must be cleared")
		}
	})

	t.Run("post_login_redirect to a backend device path is not prefixed", func(t *testing.T) {
		newPasswordUser(t, "device-redir@example.com")
		rec := postPasswordLogin(plain, creds("device-redir@example.com", password),
			&http.Cookie{Name: "post_login_redirect", Value: "/auth/device?code=ABCD-2345"})
		if loc := rec.Header().Get("Location"); loc != "/auth/device?code=ABCD-2345" {
			t.Errorf("Location = %q, want /auth/device?code=ABCD-2345", loc)
		}
	})

	t.Run("protocol-relative post_login_redirect is ignored (no open redirect)", func(t *testing.T) {
		newPasswordUser(t, "open-redir@example.com")
		rec := postPasswordLogin(plain, creds("open-redir@example.com", password),
			&http.Cookie{Name: "post_login_redirect", Value: "//evil.com/phish"})
		if loc := rec.Header().Get("Location"); loc != callbackFrontendURL {
			t.Errorf("Location = %q, want frontend root %q", loc, callbackFrontendURL)
		}
	})

	t.Run("cli_redirect cookie resumes the CLI flow", func(t *testing.T) {
		newPasswordUser(t, "cli@example.com")
		rec := postPasswordLogin(plain, creds("cli@example.com", password),
			&http.Cookie{Name: "cli_redirect", Value: "/auth/cli/authorize?callback=x&confirmed=1"})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != "/auth/cli/authorize?callback=x&confirmed=1" {
			t.Errorf("Location = %q, want the cli_redirect target", loc)
		}
	})

	t.Run("malformed form body is a 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/password/login", strings.NewReader("%zz"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		plain.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
	})
}

// TestAutoImpersonateIfDemo_Integration covers the demo fallback against a
// real database: lazy self-healing when the shared row is missing, cookie
// handling for matching and foreign cookies, and provisioning failure.
func TestAutoImpersonateIfDemo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	ctx := context.Background()
	const demoEmail = "demo@confab.test"
	sharedID := auth.DemoSessionCookieID(demoCSRFSecret, demoEmail)

	t.Run("missing shared row is re-provisioned and the cookie set", func(t *testing.T) {
		env.CleanDB(t)
		rec := httptest.NewRecorder()
		res := auth.AutoImpersonateIfDemo(rec, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil), env.DB, demoEmail, demoCSRFSecret)
		userID, email, readOnly, ok := auth.SessionAuthResultForTest(res)
		if !ok {
			t.Fatal("expected demo auth result")
		}
		if email != demoEmail || !readOnly {
			t.Errorf("result email=%q readOnly=%v, want read-only demo user", email, readOnly)
		}
		c := findCookie(rec, auth.SessionCookieName)
		if c == nil || c.Value != sharedID || !c.HttpOnly {
			t.Fatalf("shared demo cookie not set correctly: %+v", c)
		}
		// Self-healing: the shared row now exists and resolves to the demo user.
		session, err := (&dbauth.Store{DB: env.DB}).GetWebSession(ctx, sharedID, 0)
		if err != nil {
			t.Fatalf("shared session row not provisioned: %v", err)
		}
		if session.UserID != userID || !session.ReadOnly {
			t.Errorf("shared session = %+v, want read-only row for user %d", session, userID)
		}
	})

	t.Run("existing shared row with matching cookie does not re-set the cookie", func(t *testing.T) {
		env.CleanDB(t)
		if err := auth.BootstrapDemoIdentity(ctx, env.DB, demoEmail, demoCSRFSecret); err != nil {
			t.Fatalf("BootstrapDemoIdentity: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sharedID})
		rec := httptest.NewRecorder()
		_, _, readOnly, ok := auth.SessionAuthResultForTest(auth.AutoImpersonateIfDemo(rec, req, env.DB, demoEmail, demoCSRFSecret))
		if !ok || !readOnly {
			t.Fatalf("expected read-only demo result, got ok=%v readOnly=%v", ok, readOnly)
		}
		if c := findCookie(rec, auth.SessionCookieName); c != nil {
			t.Errorf("cookie re-set although request already carried it: %+v", c)
		}
	})

	t.Run("existing shared row with a foreign cookie swaps in the demo cookie", func(t *testing.T) {
		env.CleanDB(t)
		if err := auth.BootstrapDemoIdentity(ctx, env.DB, demoEmail, demoCSRFSecret); err != nil {
			t.Fatalf("BootstrapDemoIdentity: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "expired-real-session"})
		rec := httptest.NewRecorder()
		if _, _, _, ok := auth.SessionAuthResultForTest(auth.AutoImpersonateIfDemo(rec, req, env.DB, demoEmail, demoCSRFSecret)); !ok {
			t.Fatal("expected demo auth result")
		}
		if c := findCookie(rec, auth.SessionCookieName); c == nil || c.Value != sharedID {
			t.Errorf("demo cookie not set over a foreign cookie: %+v", c)
		}
	})

	t.Run("provisioning failure returns nil and sets no cookie", func(t *testing.T) {
		env.CleanDB(t)
		// A canceled request context makes every DB call fail, so neither the
		// lookup nor the lazy re-provisioning can succeed.
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil).WithContext(cctx)
		rec := httptest.NewRecorder()
		if _, _, _, ok := auth.SessionAuthResultForTest(auth.AutoImpersonateIfDemo(rec, req, env.DB, demoEmail, demoCSRFSecret)); ok {
			t.Error("expected nil when the demo identity cannot be provisioned")
		}
		if c := findCookie(rec, auth.SessionCookieName); c != nil {
			t.Errorf("cookie set despite provisioning failure: %+v", c)
		}
	})
}
