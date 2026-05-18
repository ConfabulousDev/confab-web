package access_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/access"
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "share-external-id")

	ctx := context.Background()

	share, err := store.CreateShare(ctx, sessionID, owner.ID, true, nil, nil)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient") // Create user so lookup works
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "share-external-id")

	ctx := context.Background()
	recipients := []string{"recipient@example.com"}

	share, err := store.CreateShare(ctx, sessionID, owner.ID, false, nil, recipients)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()

	_, err := store.CreateShare(ctx, "00000000-0000-0000-0000-000000000000", owner.ID, true, nil, nil)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	ctx := context.Background()

	// Attacker tries to create share for owner's session
	_, err := store.CreateShare(ctx, sessionID, attacker.ID, true, nil, nil)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "list-shares-session")

	// Create multiple shares
	testutil.CreateTestShare(t, env, sessionID, true, nil, nil)
	testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	shares, err := store.ListShares(ctx, sessionID, owner.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()

	_, err := store.ListShares(ctx, "00000000-0000-0000-0000-000000000000", owner.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	ctx := context.Background()

	// Attacker tries to list owner's shares
	_, err := store.ListShares(ctx, sessionID, attacker.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "revoke-session")

	shareID := testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

	ctx := context.Background()

	err := store.RevokeShare(ctx, shareID, owner.ID)
	if err != nil {
		t.Fatalf("RevokeShare failed: %v", err)
	}

	// Verify share is gone
	shares, err := store.ListShares(ctx, sessionID, owner.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	attacker := testutil.CreateTestUser(t, env, "attacker@example.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	shareID := testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

	ctx := context.Background()

	// Attacker tries to revoke owner's share
	err := store.RevokeShare(ctx, shareID, attacker.ID)
	if !errors.Is(err, db.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}

	// Verify share still exists
	shares, err := store.ListShares(ctx, sessionID, owner.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	ctx := context.Background()

	err := store.RevokeShare(ctx, 99999, owner.ID)
	if !errors.Is(err, db.ErrUnauthorized) {
		// Returns unauthorized for security (doesn't reveal if share exists)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	session1 := testutil.CreateTestSession(t, env, owner.ID, "session-1")
	session2 := testutil.CreateTestSession(t, env, owner.ID, "session-2")

	// Create shares for different sessions
	testutil.CreateTestShare(t, env, session1, true, nil, nil)
	testutil.CreateTestShare(t, env, session2, true, nil, nil)

	ctx := context.Background()

	shares, err := store.ListAllUserShares(ctx, owner.ID)
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
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestSession(t, env, owner.ID, "no-shares-session")

	ctx := context.Background()

	shares, err := store.ListAllUserShares(ctx, owner.ID)
	if err != nil {
		t.Fatalf("ListAllUserShares failed: %v", err)
	}

	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

// =============================================================================
// Provider canonicalization on share reads (CF-370)
// =============================================================================

// TestListSystemShares_PopulatesCanonicalProvider verifies that the admin
// system-shares list returns the canonical provider for each row, including
// for legacy "Claude Code" session_type rows that older binaries may have
// written. The admin UI needs this to render the provider chip without
// re-implementing NormalizeProvider in the frontend.
func TestListSystemShares_PopulatesCanonicalProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	claudeSessionID := testutil.CreateTestSessionWithProvider(t, env, owner.ID, "ext-claude", "claude-code")
	codexSessionID := testutil.CreateTestSessionWithProvider(t, env, owner.ID, "ext-codex", "codex")
	legacySessionID := testutil.CreateTestSessionLegacyClaudeCode(t, env, owner.ID, "ext-legacy")

	testutil.CreateTestSystemShare(t, env, claudeSessionID, nil)
	testutil.CreateTestSystemShare(t, env, codexSessionID, nil)
	testutil.CreateTestSystemShare(t, env, legacySessionID, nil)

	ctx := context.Background()
	shares, err := store.ListSystemShares(ctx)
	if err != nil {
		t.Fatalf("ListSystemShares failed: %v", err)
	}
	if len(shares) != 3 {
		t.Fatalf("expected 3 system shares, got %d", len(shares))
	}

	wantProvider := map[string]string{
		claudeSessionID: "claude-code",
		codexSessionID:  "codex",
		legacySessionID: "claude-code", // legacy "Claude Code" normalizes to canonical
	}
	for _, share := range shares {
		want, ok := wantProvider[share.SessionID]
		if !ok {
			t.Errorf("unexpected share for session %s", share.SessionID)
			continue
		}
		if share.Provider != want {
			t.Errorf("session %s: Provider = %q, want %q", share.SessionID, share.Provider, want)
		}
	}
}

// TestListShares_PopulatesCanonicalProvider mirrors the system-shares test for
// the owner-side ListShares path. Provider is exposed in the JSON response so
// any consumer (current or future) gets a canonical value regardless of what
// the legacy session_type column stored.
func TestListShares_PopulatesCanonicalProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSessionLegacyClaudeCode(t, env, owner.ID, "legacy-share")
	testutil.CreateTestShare(t, env, sessionID, true, nil, nil)

	ctx := context.Background()
	shares, err := store.ListShares(ctx, sessionID, owner.ID)
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 1 {
		t.Fatalf("expected 1 share, got %d", len(shares))
	}
	if got := shares[0].Provider; got != "claude-code" {
		t.Errorf("legacy session: Provider = %q, want %q", got, "claude-code")
	}
}

// TestCreateSystemShare_ReturnsCanonicalProvider verifies that the system
// share constructor populates the canonical Provider on the returned struct so
// callers (like the admin audit log) can record it without an extra round-trip.
func TestCreateSystemShare_ReturnsCanonicalProvider(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &access.Store{DB: env.DB}

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	codexSessionID := testutil.CreateTestSessionWithProvider(t, env, owner.ID, "ext-codex", "codex")
	legacySessionID := testutil.CreateTestSessionLegacyClaudeCode(t, env, owner.ID, "ext-legacy")

	ctx := context.Background()

	codexShare, err := store.CreateSystemShare(ctx, codexSessionID, nil)
	if err != nil {
		t.Fatalf("CreateSystemShare (codex) failed: %v", err)
	}
	if codexShare.Provider != "codex" {
		t.Errorf("codex share: Provider = %q, want %q", codexShare.Provider, "codex")
	}

	legacyShare, err := store.CreateSystemShare(ctx, legacySessionID, nil)
	if err != nil {
		t.Fatalf("CreateSystemShare (legacy) failed: %v", err)
	}
	if legacyShare.Provider != "claude-code" {
		t.Errorf("legacy share: Provider = %q, want %q", legacyShare.Provider, "claude-code")
	}
}
