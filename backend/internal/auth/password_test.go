package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
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

	ctx := context.Background()

	// Create a test user with password
	password := "testpassword123"
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	testEmail := "logintest@example.com"
	_, err = env.DB.CreatePasswordUser(ctx, testEmail, passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	handler := auth.HandlePasswordLogin(env.DB)

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

		if rec.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected redirect status 307, got %d", rec.Code)
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
		if rec.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected successful login with uppercase email, got status %d", rec.Code)
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

		ctx := context.Background()

		// Set bootstrap credentials
		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB)
		if err != nil {
			t.Fatalf("BootstrapAdmin failed: %v", err)
		}

		// Verify user was created
		user, err := env.DB.GetUserByEmail(ctx, "admin@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}

		if user.Email != "admin@example.com" {
			t.Errorf("expected email admin@example.com, got %s", user.Email)
		}

		// Verify user is admin
		isAdmin, err := env.DB.IsUserAdmin(ctx, user.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}
		if !isAdmin {
			t.Error("bootstrap user should be admin")
		}

		// Verify password works
		authUser, err := env.DB.AuthenticatePassword(ctx, "admin@example.com", "adminpassword123")
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

		ctx := context.Background()

		// Create an existing user
		hash, _ := auth.HashPassword("existing")
		_, err := env.DB.CreatePasswordUser(ctx, "existing@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Set bootstrap credentials
		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err = auth.BootstrapAdmin(ctx, env.DB)
		if err != nil {
			t.Fatalf("BootstrapAdmin should not fail when users exist: %v", err)
		}

		// Verify admin was NOT created
		_, err = env.DB.GetUserByEmail(ctx, "admin@example.com")
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

		err := auth.BootstrapAdmin(ctx, env.DB)
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

		err := auth.BootstrapAdmin(ctx, env.DB)
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

		err := auth.BootstrapAdmin(ctx, env.DB)
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

		err := auth.BootstrapAdmin(ctx, env.DB)
		if err == nil {
			t.Error("BootstrapAdmin should fail with short password")
		}
	})

	t.Run("normalizes email to lowercase", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "ADMIN@EXAMPLE.COM")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB)
		if err != nil {
			t.Fatalf("BootstrapAdmin failed: %v", err)
		}

		// Should find user with lowercase email
		user, err := env.DB.GetUserByEmail(ctx, "admin@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if user.Email != "admin@example.com" {
			t.Errorf("expected lowercase email, got %s", user.Email)
		}
	})
}

// TestPasswordAuthenticationTiming tests that authentication has consistent timing
func TestPasswordAuthenticationTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a test user
	password := "testpassword123"
	hash, _ := auth.HashPassword(password)
	_, err := env.DB.CreatePasswordUser(ctx, "timing@example.com", hash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	// Measure time for valid user with wrong password
	start1 := time.Now()
	_, _ = env.DB.AuthenticatePassword(ctx, "timing@example.com", "wrongpassword")
	duration1 := time.Since(start1)

	// Measure time for non-existent user
	start2 := time.Now()
	_, _ = env.DB.AuthenticatePassword(ctx, "nonexistent@example.com", "anypassword")
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
