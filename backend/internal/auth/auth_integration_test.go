package auth_test

import (
	"context"
	"fmt"
	"os"
	"testing"

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
