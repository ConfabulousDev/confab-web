package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

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
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

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
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

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

// =============================================================================
// CreateShare Tests
// =============================================================================

// TestCreateShare_PublicSuccess tests creating a public share
func TestCreateShare_PublicSuccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "share-external-id")

	ctx := context.Background()
	shareToken := testutil.GenerateShareToken()

	share, err := env.DB.CreateShare(ctx, sessionID, owner.ID, shareToken, true, nil, nil)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	if share.ShareToken != shareToken {
		t.Errorf("ShareToken = %s, want %s", share.ShareToken, shareToken)
	}
	if !share.IsPublic {
		t.Error("share should be public")
	}
	if share.SessionID != sessionID {
		t.Errorf("SessionID = %s, want %s", share.SessionID, sessionID)
	}
}

// TestCreateShare_PrivateWithRecipients tests creating a private share with recipients
func TestCreateShare_PrivateWithRecipients(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient") // Create user so lookup works
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "share-external-id")

	ctx := context.Background()
	shareToken := testutil.GenerateShareToken()
	recipients := []string{"recipient@example.com"}

	share, err := env.DB.CreateShare(ctx, sessionID, owner.ID, shareToken, false, nil, recipients)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	if share.IsPublic {
		t.Error("share should not be public")
	}
	if len(share.Recipients) != 1 {
		t.Errorf("expected 1 recipient, got %d", len(share.Recipients))
	}
	if share.Recipients[0] != "recipient@example.com" {
		t.Errorf("recipient = %s, want recipient@example.com", share.Recipients[0])
	}
}

// TestCreateShare_SessionNotFound tests creating share for non-existent session
func TestCreateShare_SessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()
	shareToken := testutil.GenerateShareToken()

	_, err := env.DB.CreateShare(ctx, "00000000-0000-0000-0000-000000000000", owner.ID, shareToken, true, nil, nil)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestCreateShare_WrongOwner tests creating share for another user's session
func TestCreateShare_WrongOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	ctx := context.Background()
	shareToken := testutil.GenerateShareToken()

	// Attacker tries to create share for owner's session
	_, err := env.DB.CreateShare(ctx, sessionID, attacker.ID, shareToken, true, nil, nil)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound (security), got %v", err)
	}
}

// =============================================================================
// ListShares Tests
// =============================================================================

// TestListShares_Success tests listing shares for a session
func TestListShares_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "list-shares-session")

	// Create multiple shares
	token1 := testutil.GenerateShareToken()
	token2 := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, token1, true, nil, nil)
	testutil.CreateTestShare(t, env, sessionID, token2, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	shares, err := env.DB.ListShares(ctx, sessionID, owner.ID)
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}

	if len(shares) != 2 {
		t.Errorf("expected 2 shares, got %d", len(shares))
	}
}

// TestListShares_SessionNotFound tests listing shares for non-existent session
func TestListShares_SessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()

	_, err := env.DB.ListShares(ctx, "00000000-0000-0000-0000-000000000000", owner.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestListShares_WrongOwner tests listing shares for another user's session
func TestListShares_WrongOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	ctx := context.Background()

	// Attacker tries to list owner's shares
	_, err := env.DB.ListShares(ctx, sessionID, attacker.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound (security), got %v", err)
	}
}

// =============================================================================
// RevokeShare Tests
// =============================================================================

// TestRevokeShare_Success tests revoking a share
func TestRevokeShare_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "revoke-session")

	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	err := env.DB.RevokeShare(ctx, shareToken, owner.ID)
	if err != nil {
		t.Fatalf("RevokeShare failed: %v", err)
	}

	// Verify share is gone
	shares, err := env.DB.ListShares(ctx, sessionID, owner.ID)
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected 0 shares after revoke, got %d", len(shares))
	}
}

// TestRevokeShare_Unauthorized tests revoking another user's share
func TestRevokeShare_Unauthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	// Attacker tries to revoke owner's share
	err := env.DB.RevokeShare(ctx, shareToken, attacker.ID)
	if !errors.Is(err, db.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// Verify share still exists
	shares, err := env.DB.ListShares(ctx, sessionID, owner.ID)
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 1 {
		t.Error("share should still exist")
	}
}

// TestRevokeShare_NotFound tests revoking non-existent share
func TestRevokeShare_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()

	err := env.DB.RevokeShare(ctx, "nonexistent-token", owner.ID)
	if !errors.Is(err, db.ErrUnauthorized) {
		// Returns unauthorized for security (doesn't reveal if token exists)
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// =============================================================================
// GetSharedSession Tests (Extended)
// =============================================================================

// TestGetSharedSession_ShareNotFound tests invalid share token
func TestGetSharedSession_ShareNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "share-session")

	ctx := context.Background()

	_, err := env.DB.GetSharedSession(ctx, sessionID, "invalid-token", nil)
	if !errors.Is(err, db.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound, got %v", err)
	}
}

// TestGetSharedSession_ShareExpired tests accessing expired share
func TestGetSharedSession_ShareExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "expired-share-session")

	shareToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, &expiredTime, nil)

	ctx := context.Background()

	_, err := env.DB.GetSharedSession(ctx, sessionID, shareToken, nil)
	if !errors.Is(err, db.ErrShareExpired) {
		t.Errorf("expected ErrShareExpired, got %v", err)
	}
}

// TestGetSharedSession_PrivateNoAuth tests accessing private share without auth
func TestGetSharedSession_PrivateNoAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "private-share-session")

	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Access without auth (viewerUserID = nil)
	_, err := env.DB.GetSharedSession(ctx, sessionID, shareToken, nil)
	if !errors.Is(err, db.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// TestGetSharedSession_PrivateForbidden tests accessing private share as non-recipient
func TestGetSharedSession_PrivateForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	nonRecipient := testutil.CreateTestUser(t, env, "nonrecipient@example.com", "Non-Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "private-share-session")

	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Non-recipient tries to access
	_, err := env.DB.GetSharedSession(ctx, sessionID, shareToken, &nonRecipient.ID)
	if !errors.Is(err, db.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}

	// Actual recipient should have access
	session, err := env.DB.GetSharedSession(ctx, sessionID, shareToken, &recipient.ID)
	if err != nil {
		t.Fatalf("recipient should have access: %v", err)
	}
	if session == nil {
		t.Error("expected session to be returned for recipient")
	}
}

// TestGetSharedSession_TokenSessionMismatch tests wrong session ID for share token
func TestGetSharedSession_TokenSessionMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	session1 := testutil.CreateTestSession(t, env, owner.ID, "session-1")
	session2 := testutil.CreateTestSession(t, env, owner.ID, "session-2")

	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, session1, shareToken, true, nil, nil) // Share for session1

	ctx := context.Background()

	// Try to use token with session2
	_, err := env.DB.GetSharedSession(ctx, session2, shareToken, nil)
	if !errors.Is(err, db.ErrShareNotFound) {
		t.Errorf("expected ErrShareNotFound for mismatched session, got %v", err)
	}
}

// =============================================================================
// ListAllUserShares Tests
// =============================================================================

// TestListAllUserShares_Success tests listing all shares across sessions
func TestListAllUserShares_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	session1 := testutil.CreateTestSession(t, env, owner.ID, "session-1")
	session2 := testutil.CreateTestSession(t, env, owner.ID, "session-2")

	// Create shares for different sessions
	token1 := testutil.GenerateShareToken()
	token2 := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, session1, token1, true, nil, nil)
	testutil.CreateTestShare(t, env, session2, token2, true, nil, nil)

	ctx := context.Background()

	shares, err := env.DB.ListAllUserShares(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListAllUserShares failed: %v", err)
	}

	if len(shares) != 2 {
		t.Errorf("expected 2 shares across sessions, got %d", len(shares))
	}
}

// TestListAllUserShares_Empty tests listing when user has no shares
func TestListAllUserShares_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestSession(t, env, owner.ID, "no-shares-session")

	ctx := context.Background()

	shares, err := env.DB.ListAllUserShares(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListAllUserShares failed: %v", err)
	}

	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}
