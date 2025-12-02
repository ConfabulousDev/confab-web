package db_test

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestCountUsers_EmptyDatabase tests counting users in an empty database
func TestCountUsers_EmptyDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	count, err := env.DB.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 users in empty database, got %d", count)
	}
}

// TestCountUsers_WithUsers tests counting users after creating some
func TestCountUsers_WithUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create 3 users
	users := []models.OAuthUserInfo{
		{Provider: models.ProviderGitHub, ProviderID: "gh-1", Email: "user1@example.com", Name: "User 1"},
		{Provider: models.ProviderGitHub, ProviderID: "gh-2", Email: "user2@example.com", Name: "User 2"},
		{Provider: models.ProviderGoogle, ProviderID: "google-1", Email: "user3@example.com", Name: "User 3"},
	}

	for _, info := range users {
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed for %s: %v", info.Email, err)
		}
	}

	count, err := env.DB.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 users, got %d", count)
	}
}

// TestCountUsers_AccountLinkingDoesNotDoubleCount tests that linked accounts count as one user
func TestCountUsers_AccountLinkingDoesNotDoubleCount(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create user with GitHub
	githubInfo := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-linked",
		Email:      "linked@example.com",
		Name:       "Linked User",
	}
	_, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth (GitHub) failed: %v", err)
	}

	// Link Google account with same email
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

	count, err := env.DB.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers failed: %v", err)
	}

	// Should still be 1 user (account linking)
	if count != 1 {
		t.Errorf("expected 1 user after account linking, got %d", count)
	}
}

// TestUserExistsByEmail_NonExistent tests checking for a non-existent user
func TestUserExistsByEmail_NonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	exists, err := env.DB.UserExistsByEmail(ctx, "nonexistent@example.com")
	if err != nil {
		t.Fatalf("UserExistsByEmail failed: %v", err)
	}

	if exists {
		t.Error("expected user to not exist")
	}
}

// TestUserExistsByEmail_Exists tests checking for an existing user
func TestUserExistsByEmail_Exists(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-exists",
		Email:      "exists@example.com",
		Name:       "Existing User",
	}
	_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	exists, err := env.DB.UserExistsByEmail(ctx, "exists@example.com")
	if err != nil {
		t.Fatalf("UserExistsByEmail failed: %v", err)
	}

	if !exists {
		t.Error("expected user to exist")
	}
}

// TestUserExistsByEmail_EmailsStoredLowercase tests that emails are stored and matched in lowercase
// Note: Email normalization to lowercase happens at OAuth entry points (getGitHubUser, getGoogleUser)
// so by the time emails reach the database, they should already be lowercase.
// This test verifies the database layer works correctly with lowercase emails.
func TestUserExistsByEmail_EmailsStoredLowercase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user with lowercase email (as it would come from OAuth after normalization)
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-case",
		Email:      "testuser@example.com",
		Name:       "Case Test User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Verify the email was stored as provided (lowercase)
	if user.Email != "testuser@example.com" {
		t.Errorf("expected email to be stored as 'testuser@example.com', got %q", user.Email)
	}

	// Check with exact case - should find the user
	exists, err := env.DB.UserExistsByEmail(ctx, "testuser@example.com")
	if err != nil {
		t.Fatalf("UserExistsByEmail failed: %v", err)
	}
	if !exists {
		t.Error("expected user to exist with lowercase email")
	}
}

// TestUserExistsByEmail_EmptyEmail tests checking with empty email
func TestUserExistsByEmail_EmptyEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	exists, err := env.DB.UserExistsByEmail(ctx, "")
	if err != nil {
		t.Fatalf("UserExistsByEmail failed: %v", err)
	}

	if exists {
		t.Error("empty email should not match any user")
	}
}

// TestUserExistsByEmail_MultipleUsers tests with multiple users in database
func TestUserExistsByEmail_MultipleUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create multiple users
	users := []models.OAuthUserInfo{
		{Provider: models.ProviderGitHub, ProviderID: "gh-multi-1", Email: "multi1@example.com", Name: "Multi 1"},
		{Provider: models.ProviderGitHub, ProviderID: "gh-multi-2", Email: "multi2@example.com", Name: "Multi 2"},
		{Provider: models.ProviderGitHub, ProviderID: "gh-multi-3", Email: "multi3@example.com", Name: "Multi 3"},
	}

	for _, info := range users {
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
		}
	}

	// Check each user exists
	for _, info := range users {
		exists, err := env.DB.UserExistsByEmail(ctx, info.Email)
		if err != nil {
			t.Fatalf("UserExistsByEmail failed for %s: %v", info.Email, err)
		}
		if !exists {
			t.Errorf("expected user %s to exist", info.Email)
		}
	}

	// Check non-existent user
	exists, err := env.DB.UserExistsByEmail(ctx, "notinlist@example.com")
	if err != nil {
		t.Fatalf("UserExistsByEmail failed: %v", err)
	}
	if exists {
		t.Error("expected user notinlist@example.com to not exist")
	}
}
