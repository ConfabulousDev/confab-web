package db_test

import (
	"context"
	"testing"

	"github.com/santaclaude2025/confab/backend/internal/models"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

func TestFindOrCreateUserByOAuth_NewUser(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a new user via GitHub OAuth
	info := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-123456",
		ProviderUsername: "testuser",
		Email:            "test@example.com",
		Name:             "Test User",
		AvatarURL:        "https://github.com/avatar.png",
	}

	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Verify user was created
	if user.ID == 0 {
		t.Error("Expected user ID to be set")
	}
	if user.Email != info.Email {
		t.Errorf("Expected email %q, got %q", info.Email, user.Email)
	}
	if user.Name == nil || *user.Name != info.Name {
		t.Errorf("Expected name %q, got %v", info.Name, user.Name)
	}
	if user.AvatarURL == nil || *user.AvatarURL != info.AvatarURL {
		t.Errorf("Expected avatar URL %q, got %v", info.AvatarURL, user.AvatarURL)
	}

	// Verify identity was created
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE user_id = $1 AND provider = $2 AND provider_id = $3`,
		user.ID, info.Provider, info.ProviderID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to query identity: %v", err)
	}
	if identityCount != 1 {
		t.Errorf("Expected 1 identity, got %d", identityCount)
	}

	// Verify provider_username was stored
	var providerUsername *string
	err = env.DB.QueryRow(ctx,
		`SELECT provider_username FROM user_identities WHERE user_id = $1 AND provider = $2`,
		user.ID, info.Provider).Scan(&providerUsername)
	if err != nil {
		t.Fatalf("Failed to query provider_username: %v", err)
	}
	if providerUsername == nil || *providerUsername != info.ProviderUsername {
		t.Errorf("Expected provider_username %q, got %v", info.ProviderUsername, providerUsername)
	}
}

func TestFindOrCreateUserByOAuth_ExistingIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create initial user
	info := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-existing",
		ProviderUsername: "existinguser",
		Email:            "existing@example.com",
		Name:             "Existing User",
		AvatarURL:        "https://github.com/avatar1.png",
	}

	user1, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("First FindOrCreateUserByOAuth failed: %v", err)
	}

	// Login again with updated profile info
	info.Name = "Updated Name"
	info.AvatarURL = "https://github.com/avatar2.png"
	info.ProviderUsername = "newusername"

	user2, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("Second FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be the same user
	if user1.ID != user2.ID {
		t.Errorf("Expected same user ID %d, got %d", user1.ID, user2.ID)
	}

	// Verify profile was updated
	updatedUser, err := env.DB.GetUserByID(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if updatedUser.Name == nil || *updatedUser.Name != "Updated Name" {
		t.Errorf("Expected name %q, got %v", "Updated Name", updatedUser.Name)
	}
	if updatedUser.AvatarURL == nil || *updatedUser.AvatarURL != "https://github.com/avatar2.png" {
		t.Errorf("Expected avatar URL to be updated")
	}

	// Verify provider_username was updated
	var providerUsername *string
	err = env.DB.QueryRow(ctx,
		`SELECT provider_username FROM user_identities WHERE user_id = $1 AND provider = $2`,
		user1.ID, info.Provider).Scan(&providerUsername)
	if err != nil {
		t.Fatalf("Failed to query provider_username: %v", err)
	}
	if providerUsername == nil || *providerUsername != "newusername" {
		t.Errorf("Expected provider_username %q, got %v", "newusername", providerUsername)
	}

	// Verify still only one identity
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE user_id = $1`,
		user1.ID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to count identities: %v", err)
	}
	if identityCount != 1 {
		t.Errorf("Expected 1 identity, got %d", identityCount)
	}
}

func TestFindOrCreateUserByOAuth_AccountLinking_GitHubFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	sharedEmail := "shared@example.com"

	// User signs in with GitHub first
	githubInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-link-test",
		ProviderUsername: "githubuser",
		Email:            sharedEmail,
		Name:             "GitHub User",
		AvatarURL:        "https://github.com/avatar.png",
	}

	githubUser, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
	if err != nil {
		t.Fatalf("GitHub FindOrCreateUserByOAuth failed: %v", err)
	}

	// User signs in with Google (same email)
	googleInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGoogle,
		ProviderID:       "google-link-test",
		ProviderUsername: "",
		Email:            sharedEmail,
		Name:             "Google User",
		AvatarURL:        "https://google.com/avatar.png",
	}

	googleUser, err := env.DB.FindOrCreateUserByOAuth(ctx, googleInfo)
	if err != nil {
		t.Fatalf("Google FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be the SAME user (account linking)
	if githubUser.ID != googleUser.ID {
		t.Errorf("Expected accounts to be linked. GitHub user ID: %d, Google user ID: %d", githubUser.ID, googleUser.ID)
	}

	// Verify user has both identities
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE user_id = $1`,
		githubUser.ID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to count identities: %v", err)
	}
	if identityCount != 2 {
		t.Errorf("Expected 2 identities (GitHub + Google), got %d", identityCount)
	}

	// Verify both provider identities exist
	var githubIdentityExists, googleIdentityExists bool
	err = env.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_identities WHERE user_id = $1 AND provider = 'github')`,
		githubUser.ID).Scan(&githubIdentityExists)
	if err != nil || !githubIdentityExists {
		t.Error("GitHub identity not found")
	}

	err = env.DB.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_identities WHERE user_id = $1 AND provider = 'google')`,
		githubUser.ID).Scan(&googleIdentityExists)
	if err != nil || !googleIdentityExists {
		t.Error("Google identity not found")
	}
}

func TestFindOrCreateUserByOAuth_AccountLinking_GoogleFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	sharedEmail := "google-first@example.com"

	// User signs in with Google first
	googleInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGoogle,
		ProviderID:       "google-first-test",
		ProviderUsername: "",
		Email:            sharedEmail,
		Name:             "Google User",
		AvatarURL:        "https://google.com/avatar.png",
	}

	googleUser, err := env.DB.FindOrCreateUserByOAuth(ctx, googleInfo)
	if err != nil {
		t.Fatalf("Google FindOrCreateUserByOAuth failed: %v", err)
	}

	// User signs in with GitHub (same email)
	githubInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-first-test",
		ProviderUsername: "githubuser2",
		Email:            sharedEmail,
		Name:             "GitHub User",
		AvatarURL:        "https://github.com/avatar.png",
	}

	githubUser, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
	if err != nil {
		t.Fatalf("GitHub FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be the SAME user (account linking)
	if googleUser.ID != githubUser.ID {
		t.Errorf("Expected accounts to be linked. Google user ID: %d, GitHub user ID: %d", googleUser.ID, githubUser.ID)
	}

	// Verify user has both identities
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE user_id = $1`,
		googleUser.ID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to count identities: %v", err)
	}
	if identityCount != 2 {
		t.Errorf("Expected 2 identities (GitHub + Google), got %d", identityCount)
	}
}

func TestFindOrCreateUserByOAuth_MultipleIdentitiesLogin(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	sharedEmail := "multi-login@example.com"

	// Create user with GitHub
	githubInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-multi",
		ProviderUsername: "multiuser",
		Email:            sharedEmail,
		Name:             "Multi User",
		AvatarURL:        "https://github.com/avatar.png",
	}

	user1, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
	if err != nil {
		t.Fatalf("GitHub FindOrCreateUserByOAuth failed: %v", err)
	}

	// Link Google
	googleInfo := models.OAuthUserInfo{
		Provider:         models.ProviderGoogle,
		ProviderID:       "google-multi",
		ProviderUsername: "",
		Email:            sharedEmail,
		Name:             "Multi User",
		AvatarURL:        "https://google.com/avatar.png",
	}

	user2, err := env.DB.FindOrCreateUserByOAuth(ctx, googleInfo)
	if err != nil {
		t.Fatalf("Google FindOrCreateUserByOAuth failed: %v", err)
	}

	// Both should be same user
	if user1.ID != user2.ID {
		t.Fatalf("Expected same user after linking")
	}

	// Now login again with GitHub - should find via identity, not email
	user3, err := env.DB.FindOrCreateUserByOAuth(ctx, githubInfo)
	if err != nil {
		t.Fatalf("Second GitHub login failed: %v", err)
	}
	if user3.ID != user1.ID {
		t.Errorf("Login with GitHub should return same user. Expected %d, got %d", user1.ID, user3.ID)
	}

	// Login with Google - should find via identity, not email
	user4, err := env.DB.FindOrCreateUserByOAuth(ctx, googleInfo)
	if err != nil {
		t.Fatalf("Second Google login failed: %v", err)
	}
	if user4.ID != user1.ID {
		t.Errorf("Login with Google should return same user. Expected %d, got %d", user1.ID, user4.ID)
	}

	// Still only 2 identities (not duplicated)
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE user_id = $1`,
		user1.ID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to count identities: %v", err)
	}
	if identityCount != 2 {
		t.Errorf("Expected 2 identities, got %d", identityCount)
	}
}

func TestFindOrCreateUserByOAuth_DifferentEmailsDifferentUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// User 1 with GitHub
	user1Info := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-user1",
		ProviderUsername: "user1",
		Email:            "user1@example.com",
		Name:             "User One",
		AvatarURL:        "https://github.com/user1.png",
	}

	user1, err := env.DB.FindOrCreateUserByOAuth(ctx, user1Info)
	if err != nil {
		t.Fatalf("User1 FindOrCreateUserByOAuth failed: %v", err)
	}

	// User 2 with GitHub (different email)
	user2Info := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-user2",
		ProviderUsername: "user2",
		Email:            "user2@example.com",
		Name:             "User Two",
		AvatarURL:        "https://github.com/user2.png",
	}

	user2, err := env.DB.FindOrCreateUserByOAuth(ctx, user2Info)
	if err != nil {
		t.Fatalf("User2 FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be DIFFERENT users
	if user1.ID == user2.ID {
		t.Errorf("Expected different users, but got same ID: %d", user1.ID)
	}

	// Each should have their own identity
	var user1IdentityCount, user2IdentityCount int
	env.DB.QueryRow(ctx, `SELECT COUNT(*) FROM user_identities WHERE user_id = $1`, user1.ID).Scan(&user1IdentityCount)
	env.DB.QueryRow(ctx, `SELECT COUNT(*) FROM user_identities WHERE user_id = $1`, user2.ID).Scan(&user2IdentityCount)

	if user1IdentityCount != 1 || user2IdentityCount != 1 {
		t.Errorf("Expected 1 identity each, got user1=%d, user2=%d", user1IdentityCount, user2IdentityCount)
	}
}

func TestFindOrCreateUserByOAuth_SameProviderDifferentIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Two GitHub accounts with different provider IDs and different emails
	// These should NOT be linked even if they try to use the same provider
	account1 := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-account-1",
		ProviderUsername: "account1",
		Email:            "account1@example.com",
		Name:             "Account One",
		AvatarURL:        "https://github.com/account1.png",
	}

	user1, err := env.DB.FindOrCreateUserByOAuth(ctx, account1)
	if err != nil {
		t.Fatalf("Account1 FindOrCreateUserByOAuth failed: %v", err)
	}

	account2 := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       "github-account-2",
		ProviderUsername: "account2",
		Email:            "account2@example.com",
		Name:             "Account Two",
		AvatarURL:        "https://github.com/account2.png",
	}

	user2, err := env.DB.FindOrCreateUserByOAuth(ctx, account2)
	if err != nil {
		t.Fatalf("Account2 FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be different users
	if user1.ID == user2.ID {
		t.Errorf("Different GitHub accounts should create different users")
	}
}

func TestFindOrCreateUserByOAuth_ProviderIDUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping database test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Same provider ID can exist for different providers
	// (e.g., GitHub user "123" and Google user "123" are different people)
	providerID := "shared-provider-id-123"

	githubUser := models.OAuthUserInfo{
		Provider:         models.ProviderGitHub,
		ProviderID:       providerID,
		ProviderUsername: "githubuser",
		Email:            "github-unique@example.com",
		Name:             "GitHub Unique",
		AvatarURL:        "https://github.com/unique.png",
	}

	user1, err := env.DB.FindOrCreateUserByOAuth(ctx, githubUser)
	if err != nil {
		t.Fatalf("GitHub FindOrCreateUserByOAuth failed: %v", err)
	}

	googleUser := models.OAuthUserInfo{
		Provider:         models.ProviderGoogle,
		ProviderID:       providerID,
		ProviderUsername: "",
		Email:            "google-unique@example.com",
		Name:             "Google Unique",
		AvatarURL:        "https://google.com/unique.png",
	}

	user2, err := env.DB.FindOrCreateUserByOAuth(ctx, googleUser)
	if err != nil {
		t.Fatalf("Google FindOrCreateUserByOAuth failed: %v", err)
	}

	// Should be different users because emails are different
	// (same provider_id but different providers and different emails)
	if user1.ID == user2.ID {
		t.Errorf("Same provider_id across different providers with different emails should create different users")
	}

	// Verify the UNIQUE(provider, provider_id) constraint allows this
	var identityCount int
	err = env.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_identities WHERE provider_id = $1`,
		providerID).Scan(&identityCount)
	if err != nil {
		t.Fatalf("Failed to count identities: %v", err)
	}
	if identityCount != 2 {
		t.Errorf("Expected 2 identities with same provider_id (different providers), got %d", identityCount)
	}
}
