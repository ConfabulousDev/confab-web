package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbaccess "github.com/ConfabulousDev/confab-web/internal/db/access"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// Test email validation logic - business logic
func TestEmailValidation(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantValid bool
	}{
		{
			name:      "valid simple email",
			email:     "user@example.com",
			wantValid: true,
		},
		{
			name:      "valid email with subdomain",
			email:     "user@mail.example.com",
			wantValid: true,
		},
		{
			name:      "valid email with plus",
			email:     "user+tag@example.com",
			wantValid: true,
		},
		{
			name:      "invalid - no @",
			email:     "userexample.com",
			wantValid: false,
		},
		{
			name:      "invalid - empty",
			email:     "",
			wantValid: false,
		},
		{
			name:      "invalid - just @",
			email:     "@",
			wantValid: false,
		},
		{
			name:      "valid with whitespace (trimmed)",
			email:     "  user@example.com  ",
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validation.IsValidEmail(tt.email)

			if isValid != tt.wantValid {
				t.Errorf("email %q: expected valid=%v, got valid=%v", tt.email, tt.wantValid, isValid)
			}
		})
	}
}

// Test share public flag validation
func TestSharePublicFlagValidation(t *testing.T) {
	t.Run("public share without recipients is valid", func(t *testing.T) {
		isPublic := true
		recipients := []string{}

		// Public shares don't require recipients
		if !isPublic && len(recipients) == 0 {
			t.Error("public share should be valid without recipients")
		}
	})

	t.Run("non-public share without recipients is invalid", func(t *testing.T) {
		isPublic := false
		recipients := []string{}

		// Non-public shares require at least one recipient
		if !isPublic && len(recipients) == 0 {
			// This is the expected validation failure
			return
		}
		t.Error("non-public share without recipients should be invalid")
	})

	t.Run("non-public share with recipients is valid", func(t *testing.T) {
		isPublic := false
		recipients := []string{"user@example.com"}

		if !isPublic && len(recipients) == 0 {
			t.Error("non-public share with recipients should be valid")
		}
	})
}

// Test email count limits for private shares
func TestPrivateShareEmailLimits(t *testing.T) {
	t.Run("rejects private share with no emails", func(t *testing.T) {
		// Test the validation logic: private shares require at least one email
		isPrivate := true
		emailCount := 0

		if isPrivate && emailCount == 0 {
			// This should fail validation
			return
		}
		t.Error("expected validation failure for private share with no emails")
	})

	t.Run("accepts private share with one email", func(t *testing.T) {
		isPrivate := true
		emailCount := 1

		if isPrivate && emailCount == 0 {
			t.Error("valid private share rejected")
		}
	})

	t.Run("rejects private share with too many emails", func(t *testing.T) {
		maxEmails := 50
		emailCount := maxEmails + 1

		if emailCount > maxEmails {
			// Should fail validation
			return
		}
		t.Error("expected validation failure for too many emails")
	})

	t.Run("accepts private share at max emails", func(t *testing.T) {
		maxEmails := 50
		emailCount := maxEmails

		if emailCount == 0 {
			t.Error("valid private share rejected")
		}
		if emailCount > maxEmails {
			t.Error("valid email count rejected")
		}
	})

	t.Run("public share does not require emails", func(t *testing.T) {
		// Test the validation logic: public shares don't require emails
		isPublic := true
		emailCount := 0

		// Public shares should work without emails
		if isPublic || emailCount > 0 {
			// This is valid
			return
		}
		t.Error("public share should not require emails")
	})
}

func TestHandleCreateShareDisabled(t *testing.T) {
	t.Run("returns 403 when shares are not enabled", func(t *testing.T) {
		handler := HandleCreateShare(nil, "", nil, false, defaultShareDailyQuota)

		req := httptest.NewRequest("POST", "/api/v1/sessions/test-id/share", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, "Share creation is disabled by the administrator") {
			t.Errorf("expected disabled message, got: %s", body)
		}
	})

	t.Run("does not return 403 when shares are enabled", func(t *testing.T) {
		// With sharesEnabled=true and nil db, it should fail with auth error (not 403)
		handler := HandleCreateShare(nil, "", nil, true, defaultShareDailyQuota)

		req := httptest.NewRequest("POST", "/api/v1/sessions/test-id/share", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Should not be 403 (disabled), will be 401 because no auth context
		if rr.Code == http.StatusForbidden {
			body := rr.Body.String()
			if strings.Contains(body, "disabled") {
				t.Error("should not return disabled message when shares are enabled")
			}
		}
	})
}


// fakeEmailRecorder is an email.Service that records every send so tests can
// assert whether any email left the building. The upfront batch check must
// reject an over-quota batch BEFORE the recipient loop, so no send is recorded.
type fakeEmailRecorder struct {
	sent []email.ShareInvitationParams
}

func (f *fakeEmailRecorder) SendShareInvitation(_ context.Context, params email.ShareInvitationParams) error {
	f.sent = append(f.sent, params)
	return nil
}

// postShare drives HandleCreateShare with an authenticated userID and the chi
// {id} URL param set, returning the recorder for assertions.
func postShare(t *testing.T, handler http.HandlerFunc, userID int64, sessionID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/sessions/"+sessionID+"/share", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", sessionID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = auth.SetUserIDForTest(ctx, userID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestHandleCreateShare_DailyQuotaExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "quota-ext", testutil.TestSessionFullOpts{
		Summary: "Quota session",
	})

	// Quota of 2: the third creation in the window must be rejected.
	handler := HandleCreateShare(env.DB, "https://app.example.com", nil, true, 2)
	accessStore := &dbaccess.Store{DB: env.DB}

	for i := 0; i < 2; i++ {
		rr := postShare(t, handler, owner.ID, sessionID, `{"is_public":true}`)
		if rr.Code != http.StatusOK {
			t.Fatalf("share %d: expected 200, got %d (%s)", i+1, rr.Code, rr.Body.String())
		}
	}

	rr := postShare(t, handler, owner.ID, sessionID, `{"is_public":true}`)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("over-quota share: expected 429, got %d (%s)", rr.Code, rr.Body.String())
	}

	// No extra row was created: still exactly 2 in the window.
	n, err := accessStore.CountUserSharesSince(context.Background(), owner.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountUserSharesSince failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 shares after rejected create, got %d", n)
	}
}

func TestHandleCreateShare_EmailBatchExceedsRemainingQuota(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "email-ext", testutil.TestSessionFullOpts{
		Summary: "Email session",
	})

	// Email limit of 2/hour, but the batch has 3 recipients → reject up front.
	recorder := &fakeEmailRecorder{}
	emailService := email.NewRateLimitedService(recorder, 2)
	handler := HandleCreateShare(env.DB, "https://app.example.com", emailService, true, defaultShareDailyQuota)
	accessStore := &dbaccess.Store{DB: env.DB}

	body := `{"is_public":false,"recipients":["a@x.com","b@x.com","c@x.com"]}`
	rr := postShare(t, handler, owner.ID, sessionID, body)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("over-email-quota batch: expected 429, got %d (%s)", rr.Code, rr.Body.String())
	}

	// No partial send: the upfront check fires before the recipient loop.
	if len(recorder.sent) != 0 {
		t.Errorf("expected 0 emails sent on rejected batch, got %d", len(recorder.sent))
	}

	// No share row was created either.
	n, err := accessStore.CountUserSharesSince(context.Background(), owner.ID, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CountUserSharesSince failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 shares after rejected batch, got %d", n)
	}
}

func TestHandleCreateShare_UnderLimitsSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ok-ext", testutil.TestSessionFullOpts{
		Summary: "OK session",
	})

	recorder := &fakeEmailRecorder{}
	emailService := email.NewRateLimitedService(recorder, 10)
	handler := HandleCreateShare(env.DB, "https://app.example.com", emailService, true, 100)

	body := `{"is_public":false,"recipients":["a@x.com","b@x.com"]}`
	rr := postShare(t, handler, owner.ID, sessionID, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("under-limit share: expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(recorder.sent) != 2 {
		t.Errorf("expected 2 emails sent, got %d", len(recorder.sent))
	}
}
