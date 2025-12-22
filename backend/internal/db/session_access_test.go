package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GetSessionAccessType Tests (CF-132: Canonical Session URLs)
// =============================================================================

// TestGetSessionAccessType_Owner tests that session owner has owner access
func TestGetSessionAccessType_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	ctx := context.Background()

	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &owner.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessOwner {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessOwner, accessInfo.AccessType)
	}
	if accessInfo.ShareID != nil {
		t.Error("expected ShareID = nil for owner access")
	}
}

// TestGetSessionAccessType_PublicShare_Unauthenticated tests public share access without auth
func TestGetSessionAccessType_PublicShare_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	shareID := testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	// Unauthenticated access (nil viewerUserID)
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessPublic {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessPublic, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != shareID {
		t.Errorf("expected ShareID = %d, got %v", shareID, accessInfo.ShareID)
	}
}

// TestGetSessionAccessType_PublicShare_Authenticated tests public share access with auth (non-owner)
func TestGetSessionAccessType_PublicShare_Authenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	shareID := testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	// Authenticated non-owner should get public access (not owner access)
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &viewer.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessPublic {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessPublic, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != shareID {
		t.Errorf("expected ShareID = %d, got %v", shareID, accessInfo.ShareID)
	}
}

// TestGetSessionAccessType_SystemShare_Authenticated tests system share access for authenticated user
func TestGetSessionAccessType_SystemShare_Authenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create system share
	shareToken := testutil.GenerateShareToken()
	shareID := testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	ctx := context.Background()

	// Any authenticated user should get system access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &viewer.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessSystem {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessSystem, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != shareID {
		t.Errorf("expected ShareID = %d, got %v", shareID, accessInfo.ShareID)
	}
}

// TestGetSessionAccessType_SystemShare_Unauthenticated tests that system share requires auth
func TestGetSessionAccessType_SystemShare_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create system share (no public share)
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, nil)

	ctx := context.Background()

	// Unauthenticated should get no access (system shares require auth)
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_RecipientShare_Authorized tests recipient share access for authorized user
func TestGetSessionAccessType_RecipientShare_Authorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share with recipient
	shareToken := testutil.GenerateShareToken()
	shareID := testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Recipient should get recipient access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &recipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessRecipient {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessRecipient, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != shareID {
		t.Errorf("expected ShareID = %d, got %v", shareID, accessInfo.ShareID)
	}
}

// TestGetSessionAccessType_RecipientShare_NotAuthorized tests that non-recipients can't access private shares
func TestGetSessionAccessType_RecipientShare_NotAuthorized(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	nonRecipient := testutil.CreateTestUser(t, env, "nonrecipient@example.com", "Non-Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share with specific recipient
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Non-recipient should get no access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &nonRecipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}

	// Verify actual recipient still has access
	accessInfo, err = env.DB.GetSessionAccessType(ctx, sessionID, &recipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}
	if accessInfo.AccessType != db.SessionAccessRecipient {
		t.Errorf("expected recipient AccessType = %s, got %s", db.SessionAccessRecipient, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_RecipientShare_Unauthenticated tests that private shares require auth
func TestGetSessionAccessType_RecipientShare_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create private share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Unauthenticated should get no access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_NoAccess tests that users without any access get none
func TestGetSessionAccessType_NoAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	stranger := testutil.CreateTestUser(t, env, "stranger@example.com", "Stranger")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")
	// No shares created

	ctx := context.Background()

	// Stranger should get no access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &stranger.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_SessionNotFound tests error handling for non-existent session
func TestGetSessionAccessType_SessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	ctx := context.Background()

	// Non-existent session should return ErrSessionNotFound
	_, err := env.DB.GetSessionAccessType(ctx, "00000000-0000-0000-0000-000000000000", &viewer.ID)
	if err != db.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestGetSessionAccessType_InvalidUUID tests error handling for invalid session ID
func TestGetSessionAccessType_InvalidUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	ctx := context.Background()

	// Invalid UUID should return ErrSessionNotFound
	_, err := env.DB.GetSessionAccessType(ctx, "not-a-uuid", &viewer.ID)
	if err != db.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestGetSessionAccessType_ExpiredPublicShare tests that expired public shares don't grant access
func TestGetSessionAccessType_ExpiredPublicShare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create expired public share
	shareToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, &expiredTime, nil)

	ctx := context.Background()

	// Expired share should not grant access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s for expired share, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_ExpiredSystemShare tests that expired system shares don't grant access
func TestGetSessionAccessType_ExpiredSystemShare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create expired system share
	shareToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestSystemShare(t, env, sessionID, shareToken, &expiredTime)

	ctx := context.Background()

	// Expired share should not grant access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &viewer.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s for expired share, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_ExpiredRecipientShare tests that expired private shares don't grant access
func TestGetSessionAccessType_ExpiredRecipientShare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create expired private share
	shareToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, &expiredTime, []string{"recipient@example.com"})

	ctx := context.Background()

	// Expired share should not grant access even to recipient
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &recipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessNone {
		t.Errorf("expected AccessType = %s for expired share, got %s", db.SessionAccessNone, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_AccessPrecedence tests that owner access takes precedence over shares
func TestGetSessionAccessType_AccessPrecedence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	// Owner should get owner access, not public access
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &owner.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessOwner {
		t.Errorf("expected AccessType = %s (owner should take precedence), got %s", db.SessionAccessOwner, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_MultipleShares tests access through any valid share
func TestGetSessionAccessType_MultipleShares(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create expired public share
	expiredToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, sessionID, expiredToken, true, &expiredTime, nil)

	// Create valid private share for recipient
	validToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, validToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Recipient should get access through the valid private share, not the expired public one
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &recipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessRecipient {
		t.Errorf("expected AccessType = %s (through valid share), got %s", db.SessionAccessRecipient, accessInfo.AccessType)
	}
}

// TestGetSessionAccessType_SystemTakesPrecedenceOverPublic tests that system share is returned before public share
// (more specific access types take precedence)
func TestGetSessionAccessType_SystemTakesPrecedenceOverPublic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create both public and system shares
	publicToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, publicToken, true, nil, nil)

	systemToken := testutil.GenerateShareToken()
	systemShareID := testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	ctx := context.Background()

	// Should get system access (more specific than public)
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &viewer.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessSystem {
		t.Errorf("expected AccessType = %s (system takes precedence over public), got %s", db.SessionAccessSystem, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != systemShareID {
		t.Errorf("expected ShareID = %d (system share), got %v", systemShareID, accessInfo.ShareID)
	}
}

// TestGetSessionAccessType_RecipientTakesPrecedenceOverSystem tests that recipient share is returned before system share
func TestGetSessionAccessType_RecipientTakesPrecedenceOverSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@example.com", "Recipient")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create both system and recipient shares
	systemToken := testutil.GenerateShareToken()
	testutil.CreateTestSystemShare(t, env, sessionID, systemToken, nil)

	recipientToken := testutil.GenerateShareToken()
	recipientShareID := testutil.CreateTestShare(t, env, sessionID, recipientToken, false, nil, []string{"recipient@example.com"})

	ctx := context.Background()

	// Should get recipient access (more specific than system)
	accessInfo, err := env.DB.GetSessionAccessType(ctx, sessionID, &recipient.ID)
	if err != nil {
		t.Fatalf("GetSessionAccessType failed: %v", err)
	}

	if accessInfo.AccessType != db.SessionAccessRecipient {
		t.Errorf("expected AccessType = %s (recipient takes precedence over system), got %s", db.SessionAccessRecipient, accessInfo.AccessType)
	}
	if accessInfo.ShareID == nil || *accessInfo.ShareID != recipientShareID {
		t.Errorf("expected ShareID = %d (recipient share), got %v", recipientShareID, accessInfo.ShareID)
	}
}

// =============================================================================
// GetSessionDetailWithAccess Tests
// =============================================================================

// TestGetSessionDetailWithAccess_Owner tests owner access returns full details
func TestGetSessionDetailWithAccess_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	ctx := context.Background()

	accessInfo := &db.SessionAccessInfo{AccessType: db.SessionAccessOwner}
	session, err := env.DB.GetSessionDetailWithAccess(ctx, sessionID, &owner.ID, accessInfo)
	if err != nil {
		t.Fatalf("GetSessionDetailWithAccess failed: %v", err)
	}

	if session.ID != sessionID {
		t.Errorf("expected session ID = %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || !*session.IsOwner {
		t.Error("expected IsOwner = true for owner access")
	}
	// Owner should have access to hostname/username (even if nil in test data)
}

// TestGetSessionDetailWithAccess_SharedAccess tests shared access hides sensitive fields
func TestGetSessionDetailWithAccess_SharedAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Add hostname/username to the session
	_, err := env.DB.Exec(env.Ctx,
		"UPDATE sessions SET hostname = 'test-host', username = 'test-user' WHERE id = $1",
		sessionID)
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	ctx := context.Background()

	shareID := int64(1)
	accessInfo := &db.SessionAccessInfo{AccessType: db.SessionAccessPublic, ShareID: &shareID}
	session, err := env.DB.GetSessionDetailWithAccess(ctx, sessionID, &viewer.ID, accessInfo)
	if err != nil {
		t.Fatalf("GetSessionDetailWithAccess failed: %v", err)
	}

	if session.ID != sessionID {
		t.Errorf("expected session ID = %s, got %s", sessionID, session.ID)
	}
	if session.IsOwner == nil || *session.IsOwner {
		t.Error("expected IsOwner = false for shared access")
	}
	// Shared access should NOT have hostname/username
	if session.Hostname != nil {
		t.Error("expected Hostname = nil for shared access")
	}
	if session.Username != nil {
		t.Error("expected Username = nil for shared access")
	}
}

// TestGetSessionDetailWithAccess_InactiveOwner tests that inactive owner blocks access
func TestGetSessionDetailWithAccess_InactiveOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Deactivate owner
	err := env.DB.UpdateUserStatus(context.Background(), owner.ID, "inactive")
	if err != nil {
		t.Fatalf("failed to deactivate owner: %v", err)
	}

	ctx := context.Background()

	shareID := int64(1)
	accessInfo := &db.SessionAccessInfo{AccessType: db.SessionAccessPublic, ShareID: &shareID}
	_, err = env.DB.GetSessionDetailWithAccess(ctx, sessionID, &viewer.ID, accessInfo)
	if err != db.ErrOwnerInactive {
		t.Errorf("expected ErrOwnerInactive, got %v", err)
	}
}

// TestGetSessionDetailWithAccess_UpdatesLastAccessedAt tests that share's last_accessed_at is updated
func TestGetSessionDetailWithAccess_UpdatesLastAccessedAt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "test-session")

	// Create public share
	shareToken := testutil.GenerateShareToken()
	shareID := testutil.CreateTestShare(t, env, sessionID, shareToken, true, nil, nil)

	ctx := context.Background()

	// Get initial last_accessed_at (should be NULL)
	var lastAccessedBefore *time.Time
	row := env.DB.QueryRow(env.Ctx, "SELECT last_accessed_at FROM session_shares WHERE id = $1", shareID)
	if err := row.Scan(&lastAccessedBefore); err != nil {
		t.Fatalf("failed to query share: %v", err)
	}
	if lastAccessedBefore != nil {
		t.Error("expected last_accessed_at to be NULL initially")
	}

	// Access the session
	accessInfo := &db.SessionAccessInfo{AccessType: db.SessionAccessPublic, ShareID: &shareID}
	_, err := env.DB.GetSessionDetailWithAccess(ctx, sessionID, &viewer.ID, accessInfo)
	if err != nil {
		t.Fatalf("GetSessionDetailWithAccess failed: %v", err)
	}

	// Check that last_accessed_at was updated
	var lastAccessedAfter *time.Time
	row = env.DB.QueryRow(env.Ctx, "SELECT last_accessed_at FROM session_shares WHERE id = $1", shareID)
	if err := row.Scan(&lastAccessedAfter); err != nil {
		t.Fatalf("failed to query share: %v", err)
	}
	if lastAccessedAfter == nil {
		t.Error("expected last_accessed_at to be set after access")
	}
}

// TestGetSessionDetailWithAccess_SessionNotFound tests error for non-existent session
func TestGetSessionDetailWithAccess_SessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	ctx := context.Background()

	accessInfo := &db.SessionAccessInfo{AccessType: db.SessionAccessPublic}
	_, err := env.DB.GetSessionDetailWithAccess(ctx, "00000000-0000-0000-0000-000000000000", &viewer.ID, accessInfo)
	if err != db.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
