package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"golang.org/x/crypto/bcrypt"
)

// Helper to hash a password for tests
func hashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(hash)
}

// TestCreatePasswordUser tests creating users with password authentication
func TestCreatePasswordUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	t.Run("creates user with password identity", func(t *testing.T) {
		passwordHash := hashPassword(t, "testpassword")

		user, err := env.DB.CreatePasswordUser(ctx, "test@example.com", passwordHash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		if user.ID == 0 {
			t.Error("expected non-zero user ID")
		}
		if user.Email != "test@example.com" {
			t.Errorf("expected email test@example.com, got %s", user.Email)
		}
		if user.Status != "active" {
			t.Errorf("expected status active, got %s", user.Status)
		}
	})

	t.Run("sets name from email prefix", func(t *testing.T) {
		passwordHash := hashPassword(t, "testpassword")

		user, err := env.DB.CreatePasswordUser(ctx, "john.doe@example.com", passwordHash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		if user.Name == nil || *user.Name != "john.doe" {
			t.Errorf("expected name john.doe, got %v", user.Name)
		}
	})

	t.Run("creates admin user when isAdmin is true", func(t *testing.T) {
		passwordHash := hashPassword(t, "testpassword")

		user, err := env.DB.CreatePasswordUser(ctx, "admin@example.com", passwordHash, true)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		isAdmin, err := env.DB.IsUserAdmin(ctx, user.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}

		if !isAdmin {
			t.Error("expected user to be admin")
		}
	})

	t.Run("creates non-admin user when isAdmin is false", func(t *testing.T) {
		passwordHash := hashPassword(t, "testpassword")

		user, err := env.DB.CreatePasswordUser(ctx, "regular@example.com", passwordHash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		isAdmin, err := env.DB.IsUserAdmin(ctx, user.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}

		if isAdmin {
			t.Error("expected user to not be admin")
		}
	})

	t.Run("fails for duplicate email", func(t *testing.T) {
		passwordHash := hashPassword(t, "testpassword")

		_, err := env.DB.CreatePasswordUser(ctx, "duplicate@example.com", passwordHash, false)
		if err != nil {
			t.Fatalf("first CreatePasswordUser failed: %v", err)
		}

		_, err = env.DB.CreatePasswordUser(ctx, "duplicate@example.com", passwordHash, false)
		if err == nil {
			t.Error("expected error for duplicate email")
		}
	})

	t.Run("resolves pending share recipients", func(t *testing.T) {
		// This test verifies the share recipient resolution code path
		// The actual share recipient logic is tested in share_test.go
		passwordHash := hashPassword(t, "testpassword")

		user, err := env.DB.CreatePasswordUser(ctx, "sharerecipient@example.com", passwordHash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Verify user was created (share resolution runs but may not have any pending shares)
		if user.ID == 0 {
			t.Error("expected non-zero user ID")
		}
	})
}

// TestAuthenticatePassword tests password authentication
func TestAuthenticatePassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create test user
	password := "correctpassword"
	passwordHash := hashPassword(t, password)
	createdUser, err := env.DB.CreatePasswordUser(ctx, "auth@example.com", passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	t.Run("succeeds with correct password", func(t *testing.T) {
		user, err := env.DB.AuthenticatePassword(ctx, "auth@example.com", password)
		if err != nil {
			t.Fatalf("AuthenticatePassword failed: %v", err)
		}

		if user.ID != createdUser.ID {
			t.Errorf("expected user ID %d, got %d", createdUser.ID, user.ID)
		}
		if user.Email != "auth@example.com" {
			t.Errorf("expected email auth@example.com, got %s", user.Email)
		}
	})

	t.Run("fails with incorrect password", func(t *testing.T) {
		_, err := env.DB.AuthenticatePassword(ctx, "auth@example.com", "wrongpassword")
		if err != db.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("fails for non-existent user", func(t *testing.T) {
		_, err := env.DB.AuthenticatePassword(ctx, "nonexistent@example.com", password)
		if err != db.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("fails with empty password", func(t *testing.T) {
		_, err := env.DB.AuthenticatePassword(ctx, "auth@example.com", "")
		if err != db.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("resets failed attempts on success", func(t *testing.T) {
		// Create a fresh user for this test
		hash := hashPassword(t, "resettest")
		_, err := env.DB.CreatePasswordUser(ctx, "reset@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Fail a few times
		for i := 0; i < 3; i++ {
			env.DB.AuthenticatePassword(ctx, "reset@example.com", "wrongpassword")
		}

		// Succeed
		_, err = env.DB.AuthenticatePassword(ctx, "reset@example.com", "resettest")
		if err != nil {
			t.Fatalf("AuthenticatePassword should succeed: %v", err)
		}

		// Fail again - should not immediately lock (attempts were reset)
		_, err = env.DB.AuthenticatePassword(ctx, "reset@example.com", "wrongpassword")
		if err == db.ErrAccountLocked {
			t.Error("account should not be locked after successful login reset")
		}
	})
}

// TestAuthenticatePasswordLockout tests account lockout after failed attempts
func TestAuthenticatePasswordLockout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create test user
	password := "correctpassword"
	passwordHash := hashPassword(t, password)
	_, err := env.DB.CreatePasswordUser(ctx, "lockout@example.com", passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	t.Run("locks account after max failed attempts", func(t *testing.T) {
		// Fail MaxFailedAttempts times
		for i := 0; i < db.MaxFailedAttempts; i++ {
			_, err := env.DB.AuthenticatePassword(ctx, "lockout@example.com", "wrongpassword")
			if i < db.MaxFailedAttempts-1 {
				if err != db.ErrInvalidCredentials {
					t.Errorf("attempt %d: expected ErrInvalidCredentials, got %v", i+1, err)
				}
			} else {
				// Last attempt should trigger lockout
				if err != db.ErrAccountLocked {
					t.Errorf("attempt %d: expected ErrAccountLocked, got %v", i+1, err)
				}
			}
		}
	})

	t.Run("rejects login attempts when locked", func(t *testing.T) {
		// Create another user for this test
		hash := hashPassword(t, "lockeduser")
		_, err := env.DB.CreatePasswordUser(ctx, "locked@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Lock the account
		for i := 0; i < db.MaxFailedAttempts; i++ {
			env.DB.AuthenticatePassword(ctx, "locked@example.com", "wrongpassword")
		}

		// Try with correct password - should still be locked
		_, err = env.DB.AuthenticatePassword(ctx, "locked@example.com", "lockeduser")
		if err != db.ErrAccountLocked {
			t.Errorf("expected ErrAccountLocked even with correct password, got %v", err)
		}
	})

	t.Run("increments failed attempts counter", func(t *testing.T) {
		// Create user for this test
		hash := hashPassword(t, "counter")
		_, err := env.DB.CreatePasswordUser(ctx, "counter@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Fail 2 times
		for i := 0; i < 2; i++ {
			_, err := env.DB.AuthenticatePassword(ctx, "counter@example.com", "wrongpassword")
			if err != db.ErrInvalidCredentials {
				t.Errorf("expected ErrInvalidCredentials, got %v", err)
			}
		}

		// Should still be able to login (not locked yet)
		_, err = env.DB.AuthenticatePassword(ctx, "counter@example.com", "counter")
		if err != nil {
			t.Errorf("should still be able to login after 2 failures: %v", err)
		}
	})
}

// TestAuthenticatePasswordInactiveUser tests that inactive users cannot login
func TestAuthenticatePasswordInactiveUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create and deactivate user
	password := "testpassword"
	passwordHash := hashPassword(t, password)
	user, err := env.DB.CreatePasswordUser(ctx, "inactive@example.com", passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	// Deactivate the user
	err = env.DB.UpdateUserStatus(ctx, user.ID, "inactive")
	if err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	// Try to login
	_, err = env.DB.AuthenticatePassword(ctx, "inactive@example.com", password)
	if err != db.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials for inactive user, got %v", err)
	}
}

// TestUpdateUserPassword tests password update functionality
func TestUpdateUserPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create test user
	oldPassword := "oldpassword"
	oldHash := hashPassword(t, oldPassword)
	user, err := env.DB.CreatePasswordUser(ctx, "update@example.com", oldHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	t.Run("updates password successfully", func(t *testing.T) {
		newPassword := "newpassword123"
		newHash := hashPassword(t, newPassword)

		err := env.DB.UpdateUserPassword(ctx, user.ID, newHash)
		if err != nil {
			t.Fatalf("UpdateUserPassword failed: %v", err)
		}

		// Old password should fail
		_, err = env.DB.AuthenticatePassword(ctx, "update@example.com", oldPassword)
		if err != db.ErrInvalidCredentials {
			t.Error("old password should no longer work")
		}

		// New password should work
		_, err = env.DB.AuthenticatePassword(ctx, "update@example.com", newPassword)
		if err != nil {
			t.Errorf("new password should work: %v", err)
		}
	})

	t.Run("resets failed attempts and lockout", func(t *testing.T) {
		// Create a locked user
		hash := hashPassword(t, "lockedpwd")
		lockedUser, err := env.DB.CreatePasswordUser(ctx, "lockedupdate@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		// Lock the account
		for i := 0; i < db.MaxFailedAttempts; i++ {
			env.DB.AuthenticatePassword(ctx, "lockedupdate@example.com", "wrongpassword")
		}

		// Update password
		newHash := hashPassword(t, "newpwd123")
		err = env.DB.UpdateUserPassword(ctx, lockedUser.ID, newHash)
		if err != nil {
			t.Fatalf("UpdateUserPassword failed: %v", err)
		}

		// Should be able to login now (lockout cleared)
		_, err = env.DB.AuthenticatePassword(ctx, "lockedupdate@example.com", "newpwd123")
		if err != nil {
			t.Errorf("should be able to login after password reset: %v", err)
		}
	})

	t.Run("fails for non-existent user", func(t *testing.T) {
		newHash := hashPassword(t, "whatever")
		err := env.DB.UpdateUserPassword(ctx, 99999, newHash)
		if err == nil {
			t.Error("expected error for non-existent user")
		}
	})

	t.Run("fails for user without password identity", func(t *testing.T) {
		// Create an OAuth user (no password identity)
		oauthUser, err := env.DB.FindOrCreateUserByOAuth(ctx, testutil.TestGitHubUser("oauth-only"))
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
		}

		newHash := hashPassword(t, "whatever")
		err = env.DB.UpdateUserPassword(ctx, oauthUser.ID, newHash)
		if err == nil {
			t.Error("expected error for user without password identity")
		}
	})
}

// TestGetUserByEmail tests email lookup
func TestGetUserByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create test user
	passwordHash := hashPassword(t, "testpassword")
	createdUser, err := env.DB.CreatePasswordUser(ctx, "lookup@example.com", passwordHash, false)
	if err != nil {
		t.Fatalf("CreatePasswordUser failed: %v", err)
	}

	t.Run("finds existing user", func(t *testing.T) {
		user, err := env.DB.GetUserByEmail(ctx, "lookup@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail failed: %v", err)
		}

		if user.ID != createdUser.ID {
			t.Errorf("expected user ID %d, got %d", createdUser.ID, user.ID)
		}
		if user.Email != "lookup@example.com" {
			t.Errorf("expected email lookup@example.com, got %s", user.Email)
		}
	})

	t.Run("returns ErrUserNotFound for non-existent user", func(t *testing.T) {
		_, err := env.DB.GetUserByEmail(ctx, "nonexistent@example.com")
		if err != db.ErrUserNotFound {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("email lookup is case sensitive", func(t *testing.T) {
		// By design, emails are stored lowercase during creation
		// but lookup should be exact match
		_, err := env.DB.GetUserByEmail(ctx, "LOOKUP@example.com")
		if err != db.ErrUserNotFound {
			t.Error("email lookup should be case sensitive")
		}
	})
}

// TestIsUserAdmin tests admin status checking
func TestIsUserAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	t.Run("returns true for admin user", func(t *testing.T) {
		hash := hashPassword(t, "adminpwd")
		adminUser, err := env.DB.CreatePasswordUser(ctx, "isadmin@example.com", hash, true)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		isAdmin, err := env.DB.IsUserAdmin(ctx, adminUser.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}

		if !isAdmin {
			t.Error("expected true for admin user")
		}
	})

	t.Run("returns false for non-admin user", func(t *testing.T) {
		hash := hashPassword(t, "regularpwd")
		regularUser, err := env.DB.CreatePasswordUser(ctx, "notadmin@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		isAdmin, err := env.DB.IsUserAdmin(ctx, regularUser.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}

		if isAdmin {
			t.Error("expected false for non-admin user")
		}
	})

	t.Run("returns ErrUserNotFound for non-existent user", func(t *testing.T) {
		_, err := env.DB.IsUserAdmin(ctx, 99999)
		if err != db.ErrUserNotFound {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("returns false for OAuth user (default)", func(t *testing.T) {
		oauthUser, err := env.DB.FindOrCreateUserByOAuth(ctx, testutil.TestGitHubUser("oauth-admin-check"))
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
		}

		isAdmin, err := env.DB.IsUserAdmin(ctx, oauthUser.ID)
		if err != nil {
			t.Fatalf("IsUserAdmin failed: %v", err)
		}

		if isAdmin {
			t.Error("OAuth users should default to non-admin")
		}
	})
}

// TestPasswordCredentialsTimestamps tests that timestamps are properly set
func TestPasswordCredentialsTimestamps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	t.Run("user creation sets timestamps", func(t *testing.T) {
		before := time.Now().Add(-time.Second)

		hash := hashPassword(t, "timestamps")
		user, err := env.DB.CreatePasswordUser(ctx, "timestamps@example.com", hash, false)
		if err != nil {
			t.Fatalf("CreatePasswordUser failed: %v", err)
		}

		after := time.Now().Add(time.Second)

		if user.CreatedAt.Before(before) || user.CreatedAt.After(after) {
			t.Errorf("created_at outside expected range: %v", user.CreatedAt)
		}
		if user.UpdatedAt.Before(before) || user.UpdatedAt.After(after) {
			t.Errorf("updated_at outside expected range: %v", user.UpdatedAt)
		}
	})
}
