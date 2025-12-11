package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestGetSharedSession_ActiveOwner tests accessing a shared session with active owner
func TestGetSharedSession_ActiveOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create an active user with a session
	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Session Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-external-id")

	// Create a public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, "public", nil, nil)

	// Access the shared session (should succeed)
	session, err := env.DB.GetSharedSession(context.Background(), sessionID, shareToken, nil)
	if err != nil {
		t.Fatalf("GetSharedSession failed for active owner: %v", err)
	}
	if session == nil {
		t.Error("expected session to be returned")
	}
}

// TestGetSharedSession_InactiveOwner tests that shares are blocked when owner is inactive
func TestGetSharedSession_InactiveOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user and deactivate them
	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Session Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-external-id")

	// Create a public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, "public", nil, nil)

	// Deactivate the owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	// Try to access the shared session (should fail with ErrOwnerInactive)
	_, err = env.DB.GetSharedSession(context.Background(), sessionID, shareToken, nil)
	if err == nil {
		t.Error("expected error when accessing share of inactive owner")
	}
	if !errors.Is(err, db.ErrOwnerInactive) {
		t.Errorf("expected ErrOwnerInactive, got: %v", err)
	}
}

// TestGetSharedSession_ReactivatedOwner tests that shares work again after owner is reactivated
func TestGetSharedSession_ReactivatedOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create a user
	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Session Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-external-id")

	// Create a public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, "public", nil, nil)

	// Deactivate the owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, models.UserStatusInactive)
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	// Verify share is blocked
	_, err = env.DB.GetSharedSession(context.Background(), sessionID, shareToken, nil)
	if !errors.Is(err, db.ErrOwnerInactive) {
		t.Errorf("expected ErrOwnerInactive while deactivated, got: %v", err)
	}

	// Reactivate the owner
	err = env.DB.UpdateUserStatus(context.Background(), owner.ID, models.UserStatusActive)
	if err != nil {
		t.Fatalf("failed to reactivate owner: %v", err)
	}

	// Now share should work again
	session, err := env.DB.GetSharedSession(context.Background(), sessionID, shareToken, nil)
	if err != nil {
		t.Fatalf("GetSharedSession failed after reactivation: %v", err)
	}
	if session == nil {
		t.Error("expected session to be returned after reactivation")
	}
}
