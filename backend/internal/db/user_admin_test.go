package db_test

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestListAllUsers_EmptyDatabase tests listing users in an empty database
func TestListAllUsers_EmptyDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	users, err := env.DB.ListAllUsers(ctx)
	if err != nil {
		t.Fatalf("ListAllUsers failed: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("expected 0 users in empty database, got %d", len(users))
	}
}

// TestListAllUsers_WithUsers tests listing multiple users
func TestListAllUsers_WithUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create 3 users
	oauthUsers := []models.OAuthUserInfo{
		{Provider: models.ProviderGitHub, ProviderID: "gh-list-1", Email: "list1@example.com", Name: "List User 1"},
		{Provider: models.ProviderGitHub, ProviderID: "gh-list-2", Email: "list2@example.com", Name: "List User 2"},
		{Provider: models.ProviderGoogle, ProviderID: "google-list-1", Email: "list3@example.com", Name: "List User 3"},
	}

	for _, info := range oauthUsers {
		_, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
		if err != nil {
			t.Fatalf("FindOrCreateUserByOAuth failed for %s: %v", info.Email, err)
		}
	}

	users, err := env.DB.ListAllUsers(ctx)
	if err != nil {
		t.Fatalf("ListAllUsers failed: %v", err)
	}

	if len(users) != 3 {
		t.Errorf("expected 3 users, got %d", len(users))
	}

	// Verify all users have active status
	for _, u := range users {
		if u.Status != models.UserStatusActive {
			t.Errorf("expected user %s to have active status, got %s", u.Email, u.Status)
		}
	}
}

// TestListAllUsers_IncludesInactiveUsers tests that inactive users are included
func TestListAllUsers_IncludesInactiveUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-inactive-list",
		Email:      "inactive-list@example.com",
		Name:       "Inactive List User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Deactivate the user
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	// List should still include inactive user
	users, err := env.DB.ListAllUsers(ctx)
	if err != nil {
		t.Fatalf("ListAllUsers failed: %v", err)
	}

	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}

	if users[0].Status != models.UserStatusInactive {
		t.Errorf("expected user to have inactive status, got %s", users[0].Status)
	}
}

// TestUpdateUserStatus_Deactivate tests deactivating a user
func TestUpdateUserStatus_Deactivate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-deactivate",
		Email:      "deactivate@example.com",
		Name:       "Deactivate User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Verify initial status is active
	if user.Status != models.UserStatusActive {
		t.Fatalf("expected initial status to be active, got %s", user.Status)
	}

	// Deactivate the user
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus failed: %v", err)
	}

	// Verify status changed
	updatedUser, err := env.DB.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if updatedUser.Status != models.UserStatusInactive {
		t.Errorf("expected status to be inactive, got %s", updatedUser.Status)
	}
}

// TestUpdateUserStatus_Reactivate tests reactivating a deactivated user
func TestUpdateUserStatus_Reactivate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-reactivate",
		Email:      "reactivate@example.com",
		Name:       "Reactivate User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Deactivate first
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("UpdateUserStatus (deactivate) failed: %v", err)
	}

	// Reactivate
	err = env.DB.UpdateUserStatus(ctx, user.ID, models.UserStatusActive)
	if err != nil {
		t.Fatalf("UpdateUserStatus (reactivate) failed: %v", err)
	}

	// Verify status changed back
	updatedUser, err := env.DB.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if updatedUser.Status != models.UserStatusActive {
		t.Errorf("expected status to be active, got %s", updatedUser.Status)
	}
}

// TestUpdateUserStatus_NonExistentUser tests updating status of non-existent user
func TestUpdateUserStatus_NonExistentUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	err := env.DB.UpdateUserStatus(ctx, 99999, models.UserStatusInactive)
	if err != db.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// TestDeleteUser_Success tests successfully deleting a user
func TestDeleteUser_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-delete",
		Email:      "delete@example.com",
		Name:       "Delete User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	// Delete the user
	err = env.DB.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Verify user no longer exists
	_, err = env.DB.GetUserByID(ctx, user.ID)
	if err != db.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after deletion, got %v", err)
	}
}

// TestDeleteUser_CascadesAPIKeys tests that deleting user cascades to API keys
func TestDeleteUser_CascadesAPIKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user with an API key
	user := testutil.CreateTestUser(t, env, "cascade-keys@example.com", "Cascade Keys User")
	testutil.CreateTestAPIKey(t, env, user.ID, "testhash123", "test-key")

	// Verify API key exists
	keys, err := env.DB.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 API key, got %d", len(keys))
	}

	// Delete the user
	err = env.DB.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// API keys should be gone (cascade delete)
	keys, err = env.DB.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 API keys after user deletion, got %d", len(keys))
	}
}

// TestDeleteUser_NonExistentUser tests deleting a non-existent user
func TestDeleteUser_NonExistentUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	err := env.DB.DeleteUser(ctx, 99999)
	if err != db.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// TestGetUserSessionIDs_NoSessions tests getting sessions for user with no sessions
func TestGetUserSessionIDs_NoSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user with no sessions
	info := models.OAuthUserInfo{
		Provider:   models.ProviderGitHub,
		ProviderID: "gh-no-sessions",
		Email:      "no-sessions@example.com",
		Name:       "No Sessions User",
	}
	user, err := env.DB.FindOrCreateUserByOAuth(ctx, info)
	if err != nil {
		t.Fatalf("FindOrCreateUserByOAuth failed: %v", err)
	}

	sessionIDs, err := env.DB.GetUserSessionIDs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessionIDs failed: %v", err)
	}

	if len(sessionIDs) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessionIDs))
	}
}

// TestGetUserSessionIDs_WithSessions tests getting sessions for user with sessions
func TestGetUserSessionIDs_WithSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user with sessions
	user := testutil.CreateTestUser(t, env, "with-sessions@example.com", "With Sessions User")

	// Create sessions for the user
	sessionID1 := testutil.CreateTestSession(t, env, user.ID, "external-1")
	sessionID2 := testutil.CreateTestSession(t, env, user.ID, "external-2")

	sessionIDs, err := env.DB.GetUserSessionIDs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessionIDs failed: %v", err)
	}

	if len(sessionIDs) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessionIDs))
	}

	// Verify the session IDs match
	expectedIDs := map[string]bool{sessionID1: true, sessionID2: true}
	for _, id := range sessionIDs {
		if !expectedIDs[id] {
			t.Errorf("unexpected session ID: %s", id)
		}
	}
}

// TestDeleteUser_CascadesSessions tests that deleting user cascades to sessions
func TestDeleteUser_CascadesSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create a user with a session
	user := testutil.CreateTestUser(t, env, "cascade-sessions@example.com", "Cascade Sessions User")
	testutil.CreateTestSession(t, env, user.ID, "external-cascade")

	// Verify session exists
	sessionIDs, err := env.DB.GetUserSessionIDs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessionIDs failed: %v", err)
	}
	if len(sessionIDs) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessionIDs))
	}

	// Delete the user
	err = env.DB.DeleteUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Sessions should be gone (cascade delete)
	sessionIDs, err = env.DB.GetUserSessionIDs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserSessionIDs failed: %v", err)
	}
	if len(sessionIDs) != 0 {
		t.Errorf("expected 0 sessions after user deletion, got %d", len(sessionIDs))
	}
}
