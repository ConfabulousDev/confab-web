package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// Test user cap validation - security critical
func TestCanUserLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	// Save original env and restore after tests
	originalMaxUsers := os.Getenv("MAX_USERS")
	defer func() {
		if originalMaxUsers == "" {
			os.Unsetenv("MAX_USERS")
		} else {
			os.Setenv("MAX_USERS", originalMaxUsers)
		}
	}()

	t.Run("rejects empty email", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Unsetenv("MAX_USERS")

		allowed, err := auth.CanUserLogin(ctx, env.DB, "")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("empty email should not be allowed")
		}
	})

	t.Run("allows new user when under cap", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "10")

		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("new user should be allowed when under cap")
		}
	})

	t.Run("allows existing user even at cap", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		// Create a user first
		info := models.OAuthUserInfo{
			Provider:   models.ProviderGitHub,
			ProviderID: "github-existing-user",
			Email:      "existing@example.com",
			Name:       "Existing User",
			AvatarURL:  "https://example.com/avatar.png",
		}
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
		}

		// Set cap to 1 (we already have 1 user)
		os.Setenv("MAX_USERS", "1")

		// Existing user should still be allowed
		allowed, err := auth.CanUserLogin(ctx, env.DB, "existing@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("existing user should be allowed even at cap")
		}
	})

	t.Run("rejects new user when cap reached", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		// Create a user first
		info := models.OAuthUserInfo{
			Provider:   models.ProviderGitHub,
			ProviderID: "github-cap-user",
			Email:      "capuser@example.com",
			Name:       "Cap User",
			AvatarURL:  "https://example.com/avatar.png",
		}
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
		}

		// Set cap to 1 (we already have 1 user)
		os.Setenv("MAX_USERS", "1")

		// New user should be rejected
		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("new user should be rejected when cap is reached")
		}
	})

	t.Run("uses default cap of 50 when not configured", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Unsetenv("MAX_USERS")

		// With no users and default cap of 50, new user should be allowed
		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("new user should be allowed with default cap of 50")
		}
	})

	t.Run("uses default cap when MAX_USERS is invalid", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "not-a-number")

		// Invalid MAX_USERS should fall back to default (50)
		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("new user should be allowed when MAX_USERS is invalid (falls back to default)")
		}
	})

	t.Run("allows cap of zero to block all new users", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "0")

		// Cap of 0 should block all new users
		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("new user should be rejected when MAX_USERS is 0")
		}
	})

	t.Run("returns error when database is nil", func(t *testing.T) {
		ctx := context.Background()
		os.Setenv("MAX_USERS", "10")

		_, err := auth.CanUserLogin(ctx, nil, "test@example.com")
		if err == nil {
			t.Error("expected error when database is nil")
		}
	})

	t.Run("allows exactly cap number of users", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "3")

		// Create exactly 3 users (at the cap)
		for i := 1; i <= 3; i++ {
			email := fmt.Sprintf("user%d@example.com", i)

			// Check that user can login before creating
			allowed, err := auth.CanUserLogin(ctx, env.DB, email)
			if err != nil {
				t.Fatalf("CanUserLogin failed for user %d: %v", i, err)
			}
			if !allowed {
				t.Errorf("user %d should be allowed (under cap)", i)
			}

			// Create the user
			info := models.OAuthUserInfo{
				Provider:   models.ProviderGitHub,
				ProviderID: fmt.Sprintf("github-cap-%d", i),
				Email:      email,
				Name:       fmt.Sprintf("User %d", i),
			}
			_, err = env.DB.FindOrCreateUserByOAuth(ctx, info)
			if err != nil {
				t.Fatalf("FindOrCreateUserByOAuth failed for user %d: %v", i, err)
			}
		}

		// Now we're at the cap (3 users), new user should be rejected
		allowed, err := auth.CanUserLogin(ctx, env.DB, "user4@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("4th user should be rejected when cap is 3")
		}

		// But existing users should still be allowed
		for i := 1; i <= 3; i++ {
			email := fmt.Sprintf("user%d@example.com", i)
			allowed, err := auth.CanUserLogin(ctx, env.DB, email)
			if err != nil {
				t.Fatalf("CanUserLogin failed for existing user %d: %v", i, err)
			}
			if !allowed {
				t.Errorf("existing user %d should still be allowed at cap", i)
			}
		}
	})

	t.Run("handles negative MAX_USERS by blocking users", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "-5")

		// Negative values are technically parsed successfully by Atoi
		// With -5 as cap, currentUsers (0) >= -5 is TRUE, so user is blocked
		// This documents actual behavior - negative cap blocks all users
		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		// Note: 0 >= -5 is TRUE, so the cap is considered "reached"
		if allowed {
			t.Error("negative MAX_USERS should block users (0 >= -5 is true)")
		}
	})

	t.Run("rejects whitespace-only email", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "10")

		// Whitespace-only email should be rejected by validation
		allowed, err := auth.CanUserLogin(ctx, env.DB, "   ")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("whitespace-only email should be rejected")
		}
	})

	t.Run("rejects invalid email formats", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "10")

		invalidEmails := []string{
			"",
			"   ",
			"notanemail",
			"missing@domain",
			"@nodomain.com",
			"spaces in@email.com",
			"no@tld",
		}

		for _, email := range invalidEmails {
			allowed, err := auth.CanUserLogin(ctx, env.DB, email)
			if err != nil {
				t.Fatalf("CanUserLogin failed for %q: %v", email, err)
			}
			if allowed {
				t.Errorf("invalid email %q should be rejected", email)
			}
		}
	})

	t.Run("linked accounts count as single user for cap", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "1")

		// Create user with GitHub
		githubInfo := models.OAuthUserInfo{
			Provider:   models.ProviderGitHub,
			ProviderID: "github-linked",
			Email:      "linked@example.com",
			Name:       "Linked User",
		}
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth (GitHub) failed: %v", err)
		}

		// Link Google account (same email = same user, account linking)
		googleInfo := models.OAuthUserInfo{
			Provider:   models.ProviderGoogle,
			ProviderID: "google-linked",
			Email:      "linked@example.com",
			Name:       "Linked User",
		}
		_, err = env.DB.FindOrCreateUserByOAuth(ctx, googleInfo)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth (Google) failed: %v", err)
		}

		// User count should still be 1 (account linking)
		// So the existing user should be allowed
		allowed, err := auth.CanUserLogin(ctx, env.DB, "linked@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("linked account user should be allowed")
		}

		// New user should be rejected (cap of 1, we have 1 user)
		allowed, err = auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if allowed {
			t.Error("new user should be rejected when cap reached (linked accounts = 1 user)")
		}
	})

	t.Run("large cap value works correctly", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()
		os.Setenv("MAX_USERS", "1000000")

		allowed, err := auth.CanUserLogin(ctx, env.DB, "newuser@example.com")
		if err != nil {
			t.Fatalf("CanUserLogin failed: %v", err)
		}
		if !allowed {
			t.Error("new user should be allowed with large cap")
		}
	})
}

// TestCanUserLogin_DefaultConstant verifies the default max users constant
func TestCanUserLogin_DefaultConstant(t *testing.T) {
	if auth.DefaultMaxUsers != 50 {
		t.Errorf("DefaultMaxUsers = %d, want 50", auth.DefaultMaxUsers)
	}
}

// TestValidateAPIKey_ActiveUser tests that active users can authenticate with API keys
func TestValidateAPIKey_ActiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create an active user
	user := testutil.CreateTestUser(t, env, "active@example.com", "Active User")

	// Create an API key
	_, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	testutil.CreateTestAPIKey(t, env, user.ID, keyHash, "Test Key")

	// Validate the key - should succeed with active status
	userID, _, _, userStatus, err := env.DB.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if userID != user.ID {
		t.Errorf("userID = %d, want %d", userID, user.ID)
	}
	if userStatus != models.UserStatusActive {
		t.Errorf("userStatus = %s, want %s", userStatus, models.UserStatusActive)
	}
}

// TestValidateAPIKey_InactiveUser tests that inactive users get status returned for rejection
func TestValidateAPIKey_InactiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user and deactivate them
	user := testutil.CreateTestUser(t, env, "inactive@example.com", "Inactive User")
	err := env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	// Create an API key
	_, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	testutil.CreateTestAPIKey(t, env, user.ID, keyHash, "Test Key")

	// Validate the key - should return inactive status
	userID, _, _, userStatus, err := env.DB.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if userID != user.ID {
		t.Errorf("userID = %d, want %d", userID, user.ID)
	}
	if userStatus != models.UserStatusInactive {
		t.Errorf("userStatus = %s, want %s", userStatus, models.UserStatusInactive)
	}
}

// TestGetWebSession_ActiveUser tests that active users can authenticate with web sessions
func TestGetWebSession_ActiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create an active user
	user := testutil.CreateTestUser(t, env, "active-web@example.com", "Active Web User")

	// Create a web session
	sessionID := "test-session-active-123"
	expiresAt := time.Now().Add(24 * time.Hour)
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Get the session - should succeed with active status
	session, err := env.DB.GetWebSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}
	if session.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", session.UserID, user.ID)
	}
	if session.UserStatus != models.UserStatusActive {
		t.Errorf("UserStatus = %s, want %s", session.UserStatus, models.UserStatusActive)
	}
}

// TestGetWebSession_InactiveUser tests that inactive users get status returned for rejection
func TestGetWebSession_InactiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user and deactivate them
	user := testutil.CreateTestUser(t, env, "inactive-web@example.com", "Inactive Web User")
	err := env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	// Create a web session
	sessionID := "test-session-inactive-456"
	expiresAt := time.Now().Add(24 * time.Hour)
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Get the session - should return inactive status
	session, err := env.DB.GetWebSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}
	if session.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", session.UserID, user.ID)
	}
	if session.UserStatus != models.UserStatusInactive {
		t.Errorf("UserStatus = %s, want %s", session.UserStatus, models.UserStatusInactive)
	}
}

// TestUserReactivation_FullLifecycle tests that a user can be deactivated and then reactivated
func TestUserReactivation_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create an active user with API key and web session
	user := testutil.CreateTestUser(t, env, "lifecycle@example.com", "Lifecycle User")

	_, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	testutil.CreateTestAPIKey(t, env, user.ID, keyHash, "Test Key")

	sessionID := "test-session-lifecycle-789"
	expiresAt := time.Now().Add(24 * time.Hour)
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Step 1: Verify user starts as active
	_, _, _, apiStatus, err := env.DB.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey failed: %v", err)
	}
	if apiStatus != models.UserStatusActive {
		t.Errorf("initial API key status = %s, want %s", apiStatus, models.UserStatusActive)
	}

	webSession, err := env.DB.GetWebSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}
	if webSession.UserStatus != models.UserStatusActive {
		t.Errorf("initial web session status = %s, want %s", webSession.UserStatus, models.UserStatusActive)
	}

	// Step 2: Deactivate user
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus (deactivate) failed: %v", err)
	}

	// Verify both auth methods return inactive
	_, _, _, apiStatus, err = env.DB.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey after deactivation failed: %v", err)
	}
	if apiStatus != models.UserStatusInactive {
		t.Errorf("deactivated API key status = %s, want %s", apiStatus, models.UserStatusInactive)
	}

	webSession, err = env.DB.GetWebSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetWebSession after deactivation failed: %v", err)
	}
	if webSession.UserStatus != models.UserStatusInactive {
		t.Errorf("deactivated web session status = %s, want %s", webSession.UserStatus, models.UserStatusInactive)
	}

	// Step 3: Reactivate user
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusActive)
	if err != nil {
		t.Fatalf("UpdateUserStatus (reactivate) failed: %v", err)
	}

	// Verify both auth methods return active again
	_, _, _, apiStatus, err = env.DB.ValidateAPIKey(ctx, keyHash)
	if err != nil {
		t.Fatalf("ValidateAPIKey after reactivation failed: %v", err)
	}
	if apiStatus != models.UserStatusActive {
		t.Errorf("reactivated API key status = %s, want %s", apiStatus, models.UserStatusActive)
	}

	webSession, err = env.DB.GetWebSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetWebSession after reactivation failed: %v", err)
	}
	if webSession.UserStatus != models.UserStatusActive {
		t.Errorf("reactivated web session status = %s, want %s", webSession.UserStatus, models.UserStatusActive)
	}
}

// TestCanUserLogin_EmailNormalization verifies that email matching works correctly
// when emails have been normalized to lowercase at the OAuth entry points
func TestCanUserLogin_EmailNormalization(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Save and restore MAX_USERS
	originalMaxUsers := os.Getenv("MAX_USERS")
	defer func() {
		if originalMaxUsers == "" {
			os.Unsetenv("MAX_USERS")
		} else {
			os.Setenv("MAX_USERS", originalMaxUsers)
		}
	}()
	os.Setenv("MAX_USERS", "1")

	// Create a user with lowercase email (as OAuth providers would after normalization)
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "github-normalize-test",
		Email:      "user@example.com", // lowercase
		Name:       "Test User",
	}
	_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Now we're at cap (1 user). The same user with lowercase email should be allowed
	allowed, err := auth.CanUserLogin(ctx, env.DB, "user@example.com")
	if err != nil {
		t.Fatalf("CanUserLogin failed: %v", err)
	}
	if !allowed {
		t.Error("existing user with lowercase email should be allowed")
	}

	// A different email should be rejected (cap reached)
	allowed, err = auth.CanUserLogin(ctx, env.DB, "other@example.com")
	if err != nil {
		t.Fatalf("CanUserLogin failed: %v", err)
	}
	if allowed {
		t.Error("new user should be rejected when cap is reached")
	}
}

// TestAPIKeyMiddleware_Integration tests the full API key middleware flow with a real database
func TestAPIKeyMiddleware_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("valid API key grants access", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		// Create a user
		user := testutil.CreateTestUser(t, env, "apiuser@example.com", "API User")

		// Create an API key for the user
		rawKey, keyHash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		_, _, err = env.DB.CreateAPIKeyWithReturn(ctx, user.ID, keyHash, "test-key")
		if err != nil {
			t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
		}

		// Create handler wrapped with middleware
		var capturedUserID int64
		handler := auth.RequireAPIKey(env.DB, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.GetUserID(r.Context())
			if !ok {
				t.Error("expected user ID in context")
			}
			capturedUserID = userID
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if capturedUserID != user.ID {
			t.Errorf("captured userID = %d, want %d", capturedUserID, user.ID)
		}
	})

	t.Run("invalid API key returns 401", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		handler := auth.RequireAPIKey(env.DB, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for invalid key")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer cfb_invalid_key_that_does_not_exist")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("inactive user returns 403", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		// Create a user and then deactivate them
		user := testutil.CreateTestUser(t, env, "inactive@example.com", "Inactive User")

		// Create an API key
		rawKey, keyHash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}

		_, _, err = env.DB.CreateAPIKeyWithReturn(ctx, user.ID, keyHash, "test-key")
		if err != nil {
			t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
		}

		// Deactivate the user
		err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
		if err != nil {
			t.Fatalf("SetUserStatus failed: %v", err)
		}

		handler := auth.RequireAPIKey(env.DB, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for inactive user")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if body := rec.Body.String(); body != "Account deactivated\n" {
			t.Errorf("body = %q, want %q", body, "Account deactivated\n")
		}
	})
}

// TestEmailDomainRestriction_APIKey tests domain restrictions on API key middleware
func TestEmailDomainRestriction_APIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("API key with matching domain succeeds", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		user := testutil.CreateTestUser(t, env, "user@company.com", "Company User")
		rawKey, keyHash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}
		_, _, err = env.DB.CreateAPIKeyWithReturn(ctx, user.ID, keyHash, "test-key")
		if err != nil {
			t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
		}

		handler := auth.RequireAPIKey(env.DB, []string{"company.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("API key with non-matching domain returns 403", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		user := testutil.CreateTestUser(t, env, "user@other.com", "Other User")
		rawKey, keyHash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}
		_, _, err = env.DB.CreateAPIKeyWithReturn(ctx, user.ID, keyHash, "test-key")
		if err != nil {
			t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
		}

		handler := auth.RequireAPIKey(env.DB, []string{"company.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for non-matching domain")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if body := rec.Body.String(); body != "Email domain not permitted\n" {
			t.Errorf("body = %q, want %q", body, "Email domain not permitted\n")
		}
	})

	t.Run("empty allowed domains permits all", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		user := testutil.CreateTestUser(t, env, "user@anything.com", "Any User")
		rawKey, keyHash, err := auth.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey failed: %v", err)
		}
		_, _, err = env.DB.CreateAPIKeyWithReturn(ctx, user.ID, keyHash, "test-key")
		if err != nil {
			t.Fatalf("CreateAPIKeyWithReturn failed: %v", err)
		}

		handler := auth.RequireAPIKey(env.DB, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/test", nil)
		req.Header.Set("Authorization", "Bearer "+rawKey)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

// TestEmailDomainRestriction_Session tests domain restrictions on session middleware
func TestEmailDomainRestriction_Session(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("session with non-matching domain returns 403", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		user := testutil.CreateTestUser(t, env, "user@other.com", "Other User")

		sessionID := "test-session-domain-check"
		expiresAt := time.Now().Add(24 * time.Hour)
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

		handler := auth.RequireSession(env.DB, []string{"company.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called for non-matching domain")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
		if body := rec.Body.String(); body != "Email domain not permitted\n" {
			t.Errorf("body = %q, want %q", body, "Email domain not permitted\n")
		}
	})

	t.Run("session with matching domain succeeds", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		user := testutil.CreateTestUser(t, env, "user@company.com", "Company User")

		sessionID := "test-session-domain-ok"
		expiresAt := time.Now().Add(24 * time.Hour)
		testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

		handler := auth.RequireSession(env.DB, []string{"company.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

// TestEmailDomainRestriction_Bootstrap tests domain restrictions on admin bootstrap
func TestEmailDomainRestriction_Bootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	t.Run("bootstrap fails when domain not in allowed list", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@other.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, []string{"company.com"})
		if err == nil {
			t.Error("BootstrapAdmin should fail when email domain not in allowed list")
		}
		if err != nil && !strings.Contains(err.Error(), "not in ALLOWED_EMAIL_DOMAINS") {
			t.Errorf("expected domain mismatch error, got: %v", err)
		}
	})

	t.Run("bootstrap succeeds when domain matches", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@company.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, []string{"company.com"})
		if err != nil {
			t.Fatalf("BootstrapAdmin should succeed when domain matches: %v", err)
		}

		// Verify user was created
		user, err := env.DB.GetUserByEmail(ctx, "admin@company.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}
		if user.Email != "admin@company.com" {
			t.Errorf("expected email admin@company.com, got %s", user.Email)
		}
	})

	t.Run("bootstrap succeeds when no domain restrictions", func(t *testing.T) {
		env := testutil.SetupTestEnvironment(t)
		defer env.Cleanup(t)

		ctx := context.Background()

		os.Setenv("ADMIN_BOOTSTRAP_EMAIL", "admin@anything.com")
		os.Setenv("ADMIN_BOOTSTRAP_PASSWORD", "adminpassword123")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_EMAIL")
		defer os.Unsetenv("ADMIN_BOOTSTRAP_PASSWORD")

		err := auth.BootstrapAdmin(ctx, env.DB, nil)
		if err != nil {
			t.Fatalf("BootstrapAdmin should succeed with no domain restrictions: %v", err)
		}
	})
}
