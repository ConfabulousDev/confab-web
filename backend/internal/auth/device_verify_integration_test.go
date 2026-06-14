package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// postDeviceVerify submits a user_code to a HandleDeviceVerify handler as the
// given logged-in web session and returns the recorder.
func postDeviceVerify(t *testing.T, handler http.HandlerFunc, sessionID, code string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"code": {code}}
	req := httptest.NewRequest("POST", "/auth/device/verify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

// TestHandleDeviceVerify_LocksOutAfterRepeatedFailures asserts the per-verifier
// throttle (8epk): a logged-in session that submits many wrong user_codes is
// locked out of /auth/device/verify before it can keep brute-forcing.
func TestHandleDeviceVerify_LocksOutAfterRepeatedFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "verifier@example.com", "Verifier")
	sessionID := "web-session-lockout-test"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().Add(24*time.Hour))

	handler := auth.HandleDeviceVerify(env.DB, nil)

	// The first run of wrong attempts returns the ordinary invalid-code page.
	// deviceVerifyMaxFailures is 5; submit that many, all rejected as invalid.
	const maxFailures = 5
	for i := 0; i < maxFailures; i++ {
		rec := postDeviceVerify(t, handler, sessionID, "WXYZ-2345")
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d locked out too early (before %d failures)", i+1, maxFailures)
		}
		if !strings.Contains(rec.Body.String(), "Invalid or expired") {
			t.Fatalf("attempt %d: expected invalid-code page, got status %d body %q", i+1, rec.Code, rec.Body.String())
		}
	}

	// The next attempt must be locked out (429), not merely "invalid".
	rec := postDeviceVerify(t, handler, sessionID, "WXYZ-2345")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 lockout after %d failures, got status %d body %q", maxFailures, rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "too many") {
		t.Errorf("expected a 'too many attempts' message on lockout, got %q", rec.Body.String())
	}
}

// TestHandleDeviceVerify_SuccessAuthorizesAndIsNotThrottled asserts a verifier
// that submits the correct code (after a couple of fumbles) succeeds and is not
// locked out — the throttle targets brute-force, not legitimate use (8epk).
func TestHandleDeviceVerify_SuccessAuthorizesAndIsNotThrottled(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	ctx := context.Background()

	user := testutil.CreateTestUser(t, env, "verifier2@example.com", "Verifier2")
	authStore := &dbauth.Store{DB: env.DB}
	sessionID := "web-session-success-test"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().Add(24*time.Hour))

	const userCode = "ABCD-2345"
	if err := authStore.CreateDeviceCode(ctx, "device-code-xyz", userCode, "test-cli", time.Now().UTC().Add(5*time.Minute)); err != nil {
		t.Fatalf("CreateDeviceCode: %v", err)
	}

	handler := auth.HandleDeviceVerify(env.DB, nil)

	// Two fumbles, then the correct code.
	for i := 0; i < 2; i++ {
		_ = postDeviceVerify(t, handler, sessionID, "WXYZ-2345")
	}
	rec := postDeviceVerify(t, handler, sessionID, userCode)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on correct code, got status %d body %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "authorized") {
		t.Errorf("expected authorized success page, got %q", rec.Body.String())
	}
}
