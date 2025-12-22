package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

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
