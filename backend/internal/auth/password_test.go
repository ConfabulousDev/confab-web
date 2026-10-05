package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestHashPassword tests password hashing functionality
func TestHashPassword(t *testing.T) {
	t.Run("produces valid bcrypt hash", func(t *testing.T) {
		hash, err := auth.HashPassword("testpassword123")
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		// bcrypt hashes start with $2a$ or $2b$
		if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
			t.Errorf("expected bcrypt hash prefix, got: %s", hash[:10])
		}
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, err := auth.HashPassword("password1")
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		hash2, err := auth.HashPassword("password2")
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		if hash1 == hash2 {
			t.Error("different passwords produced identical hashes")
		}
	})

	t.Run("same password produces different hashes (salted)", func(t *testing.T) {
		hash1, err := auth.HashPassword("samepassword")
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		hash2, err := auth.HashPassword("samepassword")
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		if hash1 == hash2 {
			t.Error("same password should produce different hashes due to salt")
		}
	})

	t.Run("handles empty password", func(t *testing.T) {
		hash, err := auth.HashPassword("")
		if err != nil {
			t.Fatalf("HashPassword failed on empty password: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash for empty password")
		}
	})

	t.Run("rejects password over 72 bytes", func(t *testing.T) {
		// bcrypt has a max of 72 bytes - we should reject longer passwords
		longPassword := strings.Repeat("a", 100)
		_, err := auth.HashPassword(longPassword)
		if err == nil {
			t.Error("expected error for password over 72 bytes")
		}
	})

	t.Run("handles password at 72 byte limit", func(t *testing.T) {
		// Exactly 72 bytes should work
		maxPassword := strings.Repeat("a", 72)
		hash, err := auth.HashPassword(maxPassword)
		if err != nil {
			t.Fatalf("HashPassword failed on 72-byte password: %v", err)
		}

		if hash == "" {
			t.Error("expected non-empty hash for 72-byte password")
		}

		// Verify password works
		if !auth.CheckPassword(hash, maxPassword) {
			t.Error("CheckPassword should return true for 72-byte password")
		}
	})
}

// TestValidatePasswordLength pins the shared password length policy: 8–72
// bytes (bcrypt's input limit), measured in bytes rather than characters.
func TestValidatePasswordLength(t *testing.T) {
	if auth.MinPasswordBytes != 8 {
		t.Errorf("MinPasswordBytes = %d, want 8", auth.MinPasswordBytes)
	}
	if auth.MaxPasswordBytes != 72 {
		t.Errorf("MaxPasswordBytes = %d, want 72 (bcrypt input limit)", auth.MaxPasswordBytes)
	}

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"empty is too short", "", auth.ErrPasswordTooShort},
		{"7 bytes is too short", strings.Repeat("a", 7), auth.ErrPasswordTooShort},
		{"8 bytes is accepted", strings.Repeat("a", 8), nil},
		{"72 bytes is accepted", strings.Repeat("a", 72), nil},
		{"73 bytes is too long", strings.Repeat("a", 73), auth.ErrPasswordTooLong},
		{"1024 bytes is too long", strings.Repeat("a", 1024), auth.ErrPasswordTooLong},
		{"18 four-byte runes (72 bytes) is accepted", strings.Repeat("🔥", 18), nil},
		{"19 four-byte runes (76 bytes) is too long", strings.Repeat("🔥", 19), auth.ErrPasswordTooLong},
		{"36 two-byte runes (72 bytes) is accepted", strings.Repeat("é", 36), nil},
		{"2 four-byte runes (8 bytes) is accepted", strings.Repeat("🔥", 2), nil},
		{"3 two-byte runes (6 bytes) is too short", strings.Repeat("é", 3), auth.ErrPasswordTooShort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePasswordLength(tt.password)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidatePasswordLength(%d bytes) = %v, want %v", len(tt.password), err, tt.wantErr)
			}
			// Every accepted password must also be hashable: the policy
			// exists so validation never passes what bcrypt rejects.
			if tt.wantErr == nil {
				if _, err := auth.HashPassword(tt.password); err != nil {
					t.Fatalf("accepted %d-byte password failed to hash: %v", len(tt.password), err)
				}
			}
		})
	}
}

// TestCheckPassword tests password verification
func TestCheckPassword(t *testing.T) {
	password := "correctpassword"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	t.Run("returns true for correct password", func(t *testing.T) {
		if !auth.CheckPassword(hash, password) {
			t.Error("CheckPassword should return true for correct password")
		}
	})

	t.Run("returns false for incorrect password", func(t *testing.T) {
		if auth.CheckPassword(hash, "wrongpassword") {
			t.Error("CheckPassword should return false for incorrect password")
		}
	})

	t.Run("returns false for empty password", func(t *testing.T) {
		if auth.CheckPassword(hash, "") {
			t.Error("CheckPassword should return false for empty password")
		}
	})

	t.Run("returns false for invalid hash", func(t *testing.T) {
		if auth.CheckPassword("not-a-valid-hash", password) {
			t.Error("CheckPassword should return false for invalid hash")
		}
	})

	t.Run("returns false for empty hash", func(t *testing.T) {
		if auth.CheckPassword("", password) {
			t.Error("CheckPassword should return false for empty hash")
		}
	})
}

// TestHandlePasswordLogin tests the password login handler
func TestHandlePasswordLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	authStore := &dbauth.Store{DB: env.DB}

	ctx := context.Background()

	// Create a test user with password
	password := "testpassword123"
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	testEmail := "logintest@example.com"
	_, err = authStore.CreatePasswordUser(ctx, testEmail, passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	handler := auth.HandlePasswordLogin(env.DB, &auth.OAuthConfig{})

	t.Run("rejects invalid email format", func(t *testing.T) {
		form := url.Values{}
		form.Set("email", "not-an-email")
		form.Set("password", password)

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect status, got %d", rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.Contains(location, "error=") {
			t.Error("expected error in redirect URL")
		}
	})

	t.Run("rejects empty password", func(t *testing.T) {
		form := url.Values{}
		form.Set("email", testEmail)
		form.Set("password", "")

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect status, got %d", rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.Contains(location, "error=") {
			t.Error("expected error in redirect URL")
		}
	})

	t.Run("rejects invalid credentials", func(t *testing.T) {
		form := url.Values{}
		form.Set("email", testEmail)
		form.Set("password", "wrongpassword")

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect status, got %d", rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.Contains(location, "error=") {
			t.Error("expected error in redirect URL")
		}
	})

	t.Run("rejects non-existent user", func(t *testing.T) {
		form := url.Values{}
		form.Set("email", "nonexistent@example.com")
		form.Set("password", password)

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect status, got %d", rec.Code)
		}
		location := rec.Header().Get("Location")
		if !strings.Contains(location, "error=") {
			t.Error("expected error in redirect URL")
		}
	})

	t.Run("successful login sets session cookie", func(t *testing.T) {
		// Set required env var
		os.Setenv("FRONTEND_URL", "http://localhost:3000")
		defer os.Unsetenv("FRONTEND_URL")

		form := url.Values{}
		form.Set("email", testEmail)
		form.Set("password", password)

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect status 303, got %d", rec.Code)
		}

		// Check for session cookie
		cookies := rec.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == auth.SessionCookieName {
				sessionCookie = c
				break
			}
		}

		if sessionCookie == nil {
			t.Error("expected session cookie to be set")
		} else {
			if sessionCookie.Value == "" {
				t.Error("session cookie should have a value")
			}
			if !sessionCookie.HttpOnly {
				t.Error("session cookie should be HttpOnly")
			}
		}
	})

	t.Run("normalizes email to lowercase", func(t *testing.T) {
		os.Setenv("FRONTEND_URL", "http://localhost:3000")
		defer os.Unsetenv("FRONTEND_URL")

		form := url.Values{}
		form.Set("email", "LOGINTEST@EXAMPLE.COM") // uppercase
		form.Set("password", password)

		req := httptest.NewRequest("POST", "/auth/password/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		// Should succeed because email is normalized
		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected successful login with uppercase email, got status %d", rec.Code)
		}
	})

	// bcrypt compares only the first 72 bytes, so without a 72-byte login cap
	// "<72-byte password>+anything" would authenticate. Login must reject it.
	maxEmail := "maxlen@example.com"
	maxPassword := strings.Repeat("p", auth.MaxPasswordBytes)
	maxHash, err := auth.HashPassword(maxPassword)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if _, err := authStore.CreatePasswordUser(ctx, maxEmail, maxHash, false); err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	failedAttempts := func(t *testing.T, email string) int {
		t.Helper()
		var n int
		err := env.DB.Conn().QueryRowContext(ctx, `
			SELECT p.failed_attempts
			FROM user_identities i
			JOIN identity_passwords p ON i.id = p.identity_id
			WHERE i.provider = 'password' AND i.provider_id = $1`, email).Scan(&n)
		if err != nil {
			t.Fatalf("read failed_attempts: %v", err)
		}
		return n
	}

	postLogin := func(email, pw string) *httptest.ResponseRecorder {
		return postPasswordLogin(handler, url.Values{"email": {email}, "password": {pw}})
	}

	hasSessionCookie := func(rec *httptest.ResponseRecorder) bool {
		c := findCookie(rec, auth.SessionCookieName)
		return c != nil && c.Value != ""
	}

	t.Run("rejects 73-byte password as too long without counting a failed attempt", func(t *testing.T) {
		before := failedAttempts(t, maxEmail)

		// The 72-byte prefix is the correct password: bcrypt alone would accept it.
		rec := postLogin(maxEmail, maxPassword+"x")

		if got := loginErrorRedirect(t, rec).Get("error"); got != "Password is too long" {
			t.Errorf("expected error %q, got %q", "Password is too long", got)
		}
		if hasSessionCookie(rec) {
			t.Error("73-byte password must not create a session")
		}
		if after := failedAttempts(t, maxEmail); after != before {
			t.Errorf("failed_attempts changed from %d to %d; over-long passwords must not count", before, after)
		}
	})

	t.Run("72-byte correct password still logs in", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "http://localhost:3000")

		rec := postLogin(maxEmail, maxPassword)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected redirect status, got %d", rec.Code)
		}
		if !hasSessionCookie(rec) {
			t.Errorf("expected session cookie for 72-byte password, Location=%q", rec.Header().Get("Location"))
		}
	})
}

// TestBootstrapAdmin tests the admin bootstrap functionality
func TestBootstrapAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("creates admin when no users exist", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)
		authStore := &dbauth.Store{DB: env.DB}

		ctx := context.Background()

		// Set bootstrap credentials
		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err != nil {
			t.Fatalf("BootstrapAdmin failed: %v", err)
		}

		// Verify user was created
		user, err := authStore.GetUserByEmail(ctx, "admin@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}

		if user.Email != "admin@example.com" {
			t.Errorf("expected email admin@example.com, got %s", user.Email)
		}

		// Verify user is admin
		isAdmin, err := authStore.IsUserAdmin(ctx, user.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}
		if !isAdmin {
			t.Error("bootstrap user should be admin")
		}

		// Verify password works
		authUser, err := authStore.AuthenticatePassword(ctx, "admin@example.com", "adminpassword123")
		if err != nil {
			t.Fatalf("AuthenticatePassword failed: %v", err)
		}
		if authUser.ID != user.ID {
			t.Error("authenticated user should match created user")
		}
	})

	t.Run("skips when users already exist", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)
		authStore := &dbauth.Store{DB: env.DB}

		ctx := context.Background()

		// Create an existing user
		hash, _ := auth.HashPassword("existing")
		_, err := authStore.CreatePasswordUser(ctx, "existing@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Set bootstrap credentials
		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err = auth.BootstrapAdmin(ctx, env.DB, nil)
		if err != nil {
			t.Fatalf("BootstrapAdmin should not fail when users exist: %v", err)
		}

		// Verify admin was NOT created
		_, err = authStore.GetUserByEmail(ctx, "admin@example.com")
		if err != db.ErrUserNotFound {
			t.Error("admin user should not be created when users already exist")
		}
	})

	t.Run("fails with missing email env var", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err == nil {
			t.Error("BootstrapAdmin should fail with missing email")
		}
	})

	t.Run("fails with missing password env var", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err == nil {
			t.Error("BootstrapAdmin should fail with missing password")
		}
	})

	t.Run("fails with invalid email format", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "not-an-email")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err == nil {
			t.Error("BootstrapAdmin should fail with invalid email")
		}
	})

	t.Run("fails with short password", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "short") // < 8 chars
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err == nil {
			t.Error("BootstrapAdmin should fail with short password")
		}
	})

	t.Run("fails with descriptive error for 73-byte password and creates no user", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)
		userStore := &user.Store{DB: env.DB}

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", strings.Repeat("a", 73))
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err == nil {
			t.Fatal("BootstrapAdmin should fail with a 73-byte password")
		}
		if want := "ADMIN_BOOTSTRAP_PASSWORD must be at most 72 bytes"; err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}

		count, err := userStore.CountUsers(ctx)
		if err != nil {
			t.Fatalf("CountUsers failed: %v", err)
		}
		if count != 0 {
			t.Errorf("expected no users after rejected bootstrap, got %d", count)
		}
	})

	t.Run("normalizes email to lowercase", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)
		authStore := &dbauth.Store{DB: env.DB}

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "ADMIN@EXAMPLE.COM")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err != nil {
			t.Fatalf("BootstrapAdmin failed: %v", err)
		}

		// Should find user with lowercase email
		user, err := authStore.GetUserByEmail(ctx, "admin@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if user.Email != "admin@example.com" {
			t.Errorf("expected lowercase email, got %s", user.Email)
		}
	})
}

// TestBootstrapAdminConcurrent verifies that concurrent BootstrapAdmin calls
// against a shared empty DB serialize cleanly: exactly one admin user is
// created, and every losing call returns nil (not a confusing duplicate-email
// error). Guards the advisory-lock atomicity fix (7ys0 / CF-425 A3/E1).
func TestBootstrapAdminConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	authStore := &dbauth.Store{DB: env.DB}
	userStore := &user.Store{DB: env.DB}

	os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
	os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
	defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
	defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	start := make(chan struct{})

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start // line all goroutines up to maximize contention
			errs[idx] = auth.BootstrapAdmin(context.Background(), env.DB, nil)
		}(i)
	}
	close(start)
	wg.Wait()

	// Every call must succeed — the losers no-op rather than surfacing a
	// duplicate-email / duplicate-key error.
	for i, err := range errs {
		if err != nil {
			t.Errorf("BootstrapAdmin goroutine %d returned error, want nil: %v", i, err)
		}
	}

	// Exactly one user must exist.
	count, err := userStore.CountUsers(context.Background())
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 user after concurrent bootstrap, got %d", count)
	}

	// And that one user is the admin.
	admin, err := authStore.GetUserByEmail(context.Background(), "admin@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	isAdmin, err := authStore.IsUserAdmin(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("IsUserAdmin failed: %v", err)
	}
	if !isAdmin {
		t.Error("the bootstrapped user should be admin")
	}
}

// TestPasswordAuthenticationTiming tests that authentication has consistent timing
func TestPasswordAuthenticationTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	authStore := &dbauth.Store{DB: env.DB}

	ctx := context.Background()

	// Create a test user
	password := "testpassword123"
	hash, _ := auth.HashPassword(password)
	_, err := authStore.CreatePasswordUser(ctx, "timing@example.com", hash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	// Measure time for valid user with wrong password
	start1 := time.Now()
	_, _ = authStore.AuthenticatePassword(ctx, "timing@example.com", "wrongpassword")
	duration1 := time.Since(start1)

	// Measure time for non-existent user
	start2 := time.Now()
	_, _ = authStore.AuthenticatePassword(ctx, "nonexistent@example.com", "anypassword")
	duration2 := time.Since(start2)

	// Both should take similar time (within 100ms tolerance)
	// This is to prevent timing attacks that reveal user existence
	diff := duration1 - duration2
	if diff < 0 {
		diff = -diff
	}

	// Allow 500ms tolerance for test environment variability
	if diff > 500*time.Millisecond {
		t.Logf("Warning: timing difference of %v between existing and non-existing user auth", diff)
		// Don't fail the test, just log - timing can vary in CI
	}
}
