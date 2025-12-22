package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GetSessionDetail Tests
// =============================================================================

// TestGetSessionDetail_Success tests successful session detail retrieval
func TestGetSessionDetail_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "detail@test.com", "Detail User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "detail-external-id")

	// Add some sync files
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	testutil.CreateTestSyncFile(t, env, sessionID, "input.txt", "input", 50)

	ctx := context.Background()

	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}

	if detail.ID != sessionID {
		t.Errorf("ID = %s, want %s", detail.ID, sessionID)
	}
	if detail.ExternalID != "detail-external-id" {
		t.Errorf("ExternalID = %s, want detail-external-id", detail.ExternalID)
	}
	if len(detail.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(detail.Files))
	}
}

// TestGetSessionDetail_NotFound tests non-existent session
func TestGetSessionDetail_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "NotFound User")

	ctx := context.Background()

	_, err := env.DB.GetSessionDetail(ctx, "00000000-0000-0000-0000-000000000000", user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestGetSessionDetail_InvalidUUID tests invalid UUID format
func TestGetSessionDetail_InvalidUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "invaliduuid@test.com", "InvalidUUID User")

	ctx := context.Background()

	_, err := env.DB.GetSessionDetail(ctx, "not-a-valid-uuid", user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for invalid UUID, got %v", err)
	}
}

// TestGetSessionDetail_WrongUser tests accessing another user's session
func TestGetSessionDetail_WrongUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	user2 := testutil.CreateTestUser(t, env, "other@test.com", "Other")
	sessionID := testutil.CreateTestSession(t, env, user1.ID, "owner-session")

	ctx := context.Background()

	// User2 tries to access User1's session
	_, err := env.DB.GetSessionDetail(ctx, sessionID, user2.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound when accessing another user's session, got %v", err)
	}
}

// TestGetSessionDetail_ExcludesTodoFiles tests that todo files are excluded
func TestGetSessionDetail_ExcludesTodoFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "todo@test.com", "Todo User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "todo-external-id")

	// Add various file types including todo
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	testutil.CreateTestSyncFile(t, env, sessionID, "todo.jsonl", "todo", 50)
	testutil.CreateTestSyncFile(t, env, sessionID, "input.txt", "input", 25)

	ctx := context.Background()

	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}

	// Should only have 2 files (todo excluded)
	if len(detail.Files) != 2 {
		t.Errorf("expected 2 files (todo excluded), got %d", len(detail.Files))
	}

	// Verify no todo files
	for _, f := range detail.Files {
		if f.FileType == "todo" {
			t.Error("todo files should be excluded")
		}
	}
}

// =============================================================================
// DeleteSessionFromDB Tests
// =============================================================================

// TestDeleteSessionFromDB_Success tests successful session deletion
func TestDeleteSessionFromDB_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "delete@test.com", "Delete User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "delete-external-id")

	// Add some files
	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)

	ctx := context.Background()

	err := env.DB.DeleteSessionFromDB(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("DeleteSessionFromDB failed: %v", err)
	}

	// Verify session is gone
	_, err = env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after deletion, got %v", err)
	}
}

// TestDeleteSessionFromDB_NotFound tests deleting non-existent session
func TestDeleteSessionFromDB_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "NotFound User")

	ctx := context.Background()

	err := env.DB.DeleteSessionFromDB(ctx, "00000000-0000-0000-0000-000000000000", user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestDeleteSessionFromDB_WrongUser tests that users can't delete others' sessions
func TestDeleteSessionFromDB_WrongUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	user2 := testutil.CreateTestUser(t, env, "attacker@test.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, user1.ID, "owner-session")

	ctx := context.Background()

	// User2 tries to delete User1's session
	err := env.DB.DeleteSessionFromDB(ctx, sessionID, user2.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound when deleting another user's session, got %v", err)
	}

	// Verify session still exists
	_, err = env.DB.GetSessionDetail(ctx, sessionID, user1.ID)
	if err != nil {
		t.Errorf("session should still exist: %v", err)
	}
}

// =============================================================================
// VerifySessionOwnership Tests
// =============================================================================

// TestVerifySessionOwnership_Success tests successful ownership verification
func TestVerifySessionOwnership_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "owner-external-id")

	ctx := context.Background()

	externalID, err := env.DB.VerifySessionOwnership(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("VerifySessionOwnership failed: %v", err)
	}
	if externalID != "owner-external-id" {
		t.Errorf("externalID = %s, want owner-external-id", externalID)
	}
}

// TestVerifySessionOwnership_NotFound tests non-existent session
func TestVerifySessionOwnership_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")

	ctx := context.Background()

	_, err := env.DB.VerifySessionOwnership(ctx, "00000000-0000-0000-0000-000000000000", user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestVerifySessionOwnership_Forbidden tests accessing another user's session
func TestVerifySessionOwnership_Forbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	user2 := testutil.CreateTestUser(t, env, "other@test.com", "Other")
	sessionID := testutil.CreateTestSession(t, env, user1.ID, "owner-session")

	ctx := context.Background()

	// User2 tries to verify ownership of User1's session
	_, err := env.DB.VerifySessionOwnership(ctx, sessionID, user2.ID)
	if !errors.Is(err, db.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestVerifySessionOwnership_InvalidUUID tests invalid UUID format
func TestVerifySessionOwnership_InvalidUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")

	ctx := context.Background()

	_, err := env.DB.VerifySessionOwnership(ctx, "not-a-valid-uuid", user.ID)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for invalid UUID, got %v", err)
	}
}

// =============================================================================
// UpdateSessionCustomTitle Tests
// =============================================================================

// TestUpdateSessionCustomTitle_SetTitle tests setting a custom title
func TestUpdateSessionCustomTitle_SetTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "title@test.com", "Title User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "title-external-id")

	ctx := context.Background()

	customTitle := "My Custom Title"
	err := env.DB.UpdateSessionCustomTitle(ctx, sessionID, user.ID, &customTitle)
	if err != nil {
		t.Fatalf("UpdateSessionCustomTitle failed: %v", err)
	}

	// Verify title was set
	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.CustomTitle == nil || *detail.CustomTitle != customTitle {
		t.Errorf("CustomTitle = %v, want %s", detail.CustomTitle, customTitle)
	}
}

// TestUpdateSessionCustomTitle_ClearTitle tests clearing a custom title
func TestUpdateSessionCustomTitle_ClearTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cleartitle@test.com", "ClearTitle User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "clear-title-external-id")

	ctx := context.Background()

	// Set a title first
	customTitle := "Initial Title"
	err := env.DB.UpdateSessionCustomTitle(ctx, sessionID, user.ID, &customTitle)
	if err != nil {
		t.Fatalf("UpdateSessionCustomTitle (set) failed: %v", err)
	}

	// Clear the title
	err = env.DB.UpdateSessionCustomTitle(ctx, sessionID, user.ID, nil)
	if err != nil {
		t.Fatalf("UpdateSessionCustomTitle (clear) failed: %v", err)
	}

	// Verify title was cleared
	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.CustomTitle != nil {
		t.Errorf("CustomTitle = %v, want nil", detail.CustomTitle)
	}
}

// TestUpdateSessionCustomTitle_NotFound tests updating non-existent session
func TestUpdateSessionCustomTitle_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "NotFound User")

	ctx := context.Background()

	customTitle := "Title"
	err := env.DB.UpdateSessionCustomTitle(ctx, "00000000-0000-0000-0000-000000000000", user.ID, &customTitle)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestUpdateSessionCustomTitle_Forbidden tests updating another user's session
func TestUpdateSessionCustomTitle_Forbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	user2 := testutil.CreateTestUser(t, env, "attacker@test.com", "Attacker")
	sessionID := testutil.CreateTestSession(t, env, user1.ID, "owner-session")

	ctx := context.Background()

	customTitle := "Attacker's Title"
	err := env.DB.UpdateSessionCustomTitle(ctx, sessionID, user2.ID, &customTitle)
	if !errors.Is(err, db.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// TestUpdateSessionCustomTitle_InvalidUUID tests invalid UUID format
func TestUpdateSessionCustomTitle_InvalidUUID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "invaliduuid@test.com", "InvalidUUID User")

	ctx := context.Background()

	customTitle := "Title"
	err := env.DB.UpdateSessionCustomTitle(ctx, "not-a-valid-uuid", user.ID, &customTitle)
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound for invalid UUID, got %v", err)
	}
}

// =============================================================================
// UpdateSessionSummary Tests
// =============================================================================

// TestUpdateSessionSummary_Success tests setting a summary
func TestUpdateSessionSummary_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "summary@test.com", "Summary User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "summary-external-id")

	ctx := context.Background()

	summary := "This is a test summary"
	err := env.DB.UpdateSessionSummary(ctx, "summary-external-id", user.ID, summary)
	if err != nil {
		t.Fatalf("UpdateSessionSummary failed: %v", err)
	}

	// Verify summary was set
	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.Summary == nil || *detail.Summary != summary {
		t.Errorf("Summary = %v, want %s", detail.Summary, summary)
	}
}

// TestUpdateSessionSummary_NotFound tests updating non-existent session
func TestUpdateSessionSummary_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "notfound@test.com", "NotFound User")

	ctx := context.Background()

	err := env.DB.UpdateSessionSummary(ctx, "nonexistent-external-id", user.ID, "summary")
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

// TestUpdateSessionSummary_Forbidden tests updating another user's session
func TestUpdateSessionSummary_Forbidden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	user2 := testutil.CreateTestUser(t, env, "attacker@test.com", "Attacker")
	testutil.CreateTestSession(t, env, user1.ID, "owner-external-id")

	ctx := context.Background()

	err := env.DB.UpdateSessionSummary(ctx, "owner-external-id", user2.ID, "attacker summary")
	if !errors.Is(err, db.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

// =============================================================================
// ListUserSessions Tests
// =============================================================================

// TestListUserSessions_OwnedOnly tests listing only owned sessions
func TestListUserSessions_OwnedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "list@test.com", "List User")

	// Create multiple sessions
	testutil.CreateTestSession(t, env, user.ID, "session-1")
	testutil.CreateTestSession(t, env, user.ID, "session-2")
	testutil.CreateTestSession(t, env, user.ID, "session-3")

	ctx := context.Background()

	sessions, err := env.DB.ListUserSessions(ctx, user.ID, db.SessionListViewOwned)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}

	for _, s := range sessions {
		if !s.IsOwner {
			t.Error("all sessions should be owned by user")
		}
		if s.AccessType != "owner" {
			t.Errorf("AccessType = %s, want owner", s.AccessType)
		}
	}
}

// TestListUserSessions_Empty tests listing when user has no sessions
func TestListUserSessions_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty User")

	ctx := context.Background()

	sessions, err := env.DB.ListUserSessions(ctx, user.ID, db.SessionListViewOwned)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// TestListUserSessions_WithShared tests including shared sessions
func TestListUserSessions_WithShared(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	// Create owner's session
	ownerSession := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	// Create recipient's own session
	testutil.CreateTestSession(t, env, recipient.ID, "recipient-session")

	// Share owner's session with recipient
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, ownerSession, shareToken, false, nil, []string{"recipient@test.com"})

	ctx := context.Background()

	// List shared sessions view (includes owned for deduplication)
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions (1 owned + 1 shared), got %d", len(sessions))
	}

	// Verify we have both owned and shared
	var hasOwned, hasShared bool
	for _, s := range sessions {
		if s.IsOwner {
			hasOwned = true
		} else {
			hasShared = true
			if s.AccessType != "private_share" {
				t.Errorf("shared session AccessType = %s, want private_share", s.AccessType)
			}
		}
	}
	if !hasOwned {
		t.Error("should have owned session")
	}
	if !hasShared {
		t.Error("should have shared session")
	}
}

// TestListUserSessions_ExcludesSharedByDefault tests that shared sessions are excluded by default
func TestListUserSessions_ExcludesSharedByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	// Create owner's session
	ownerSession := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	// Share with recipient
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, ownerSession, shareToken, false, nil, []string{"recipient@test.com"})

	ctx := context.Background()

	// List owned sessions only (recipient has none)
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID, db.SessionListViewOwned)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (recipient owns none), got %d", len(sessions))
	}
}

// TestListUserSessions_ExpiredShareExcluded tests that expired shares are excluded
func TestListUserSessions_ExpiredShareExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	// Create owner's session
	ownerSession := testutil.CreateTestSession(t, env, owner.ID, "owner-session")

	// Create expired share
	shareToken := testutil.GenerateShareToken()
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, ownerSession, shareToken, false, &expiredTime, []string{"recipient@test.com"})

	ctx := context.Background()

	// List shared sessions - expired should be excluded
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (expired share excluded), got %d", len(sessions))
	}
}

// =============================================================================
// GetUserByID Tests
// =============================================================================

// TestGetUserByID_Success tests successful user retrieval
func TestGetUserByID_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "getuser@test.com", "Get User")

	ctx := context.Background()

	retrieved, err := env.DB.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("ID = %d, want %d", retrieved.ID, user.ID)
	}
	if retrieved.Email != "getuser@test.com" {
		t.Errorf("Email = %s, want getuser@test.com", retrieved.Email)
	}
	if retrieved.Name == nil || *retrieved.Name != "Get User" {
		t.Errorf("Name = %v, want Get User", retrieved.Name)
	}
}

// TestGetUserByID_NotFound tests non-existent user
func TestGetUserByID_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	ctx := context.Background()

	_, err := env.DB.GetUserByID(ctx, 99999)
	if !errors.Is(err, db.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

// =============================================================================
// GetSessionsLastModified Tests
// =============================================================================

// TestGetSessionsLastModified_IncludesSystemShares tests that system shares
// are included in the ETag calculation for the shared-with-me view.
// This is a regression test for CF-133.
func TestGetSessionsLastModified_IncludesSystemShares(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Create session owner
	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	// Create another user who will view the shared session
	viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")

	ctx := context.Background()

	// Initially, viewer has no shared sessions
	lastModified1, err := env.DB.GetSessionsLastModified(ctx, viewer.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("GetSessionsLastModified failed: %v", err)
	}

	// Create owner's session
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "system-shared-session")

	// Update the session's last_sync_at to a known time
	knownTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	_, err = env.DB.Exec(ctx, "UPDATE sessions SET last_sync_at = $1 WHERE id = $2", knownTime, sessionID)
	if err != nil {
		t.Fatalf("failed to update session last_sync_at: %v", err)
	}

	// Create a system share (accessible to all authenticated users)
	shareToken := testutil.GenerateShareToken()
	_, err = env.DB.CreateSystemShare(ctx, sessionID, shareToken, nil)
	if err != nil {
		t.Fatalf("failed to create system share: %v", err)
	}

	// Now viewer should see the system share reflected in lastModified
	lastModified2, err := env.DB.GetSessionsLastModified(ctx, viewer.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("GetSessionsLastModified after system share failed: %v", err)
	}

	// The lastModified should have changed to reflect the system share
	if !lastModified2.After(lastModified1) {
		t.Errorf("lastModified did not increase after system share: before=%v, after=%v",
			lastModified1, lastModified2)
	}

	// The lastModified should match the session's last_sync_at
	if !lastModified2.Equal(knownTime) {
		t.Errorf("lastModified = %v, want %v", lastModified2, knownTime)
	}
}

// TestGetSessionsLastModified_PrivateShares tests that private shares are
// still included in the ETag calculation (baseline test).
func TestGetSessionsLastModified_PrivateShares(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	ctx := context.Background()

	// Initially, recipient has no shared sessions
	lastModified1, err := env.DB.GetSessionsLastModified(ctx, recipient.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("GetSessionsLastModified failed: %v", err)
	}

	// Create owner's session
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "private-shared-session")

	// Update the session's last_sync_at to a known time
	knownTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	_, err = env.DB.Exec(ctx, "UPDATE sessions SET last_sync_at = $1 WHERE id = $2", knownTime, sessionID)
	if err != nil {
		t.Fatalf("failed to update session last_sync_at: %v", err)
	}

	// Share with recipient (private share)
	shareToken := testutil.GenerateShareToken()
	testutil.CreateTestShare(t, env, sessionID, shareToken, false, nil, []string{"recipient@test.com"})

	// Now recipient should see the private share reflected in lastModified
	lastModified2, err := env.DB.GetSessionsLastModified(ctx, recipient.ID, db.SessionListViewSharedWithMe)
	if err != nil {
		t.Fatalf("GetSessionsLastModified after private share failed: %v", err)
	}

	// The lastModified should have changed to reflect the share
	if !lastModified2.After(lastModified1) {
		t.Errorf("lastModified did not increase after private share: before=%v, after=%v",
			lastModified1, lastModified2)
	}

	// The lastModified should match the session's last_sync_at
	if !lastModified2.Equal(knownTime) {
		t.Errorf("lastModified = %v, want %v", lastModified2, knownTime)
	}
}

// TestGetSessionsLastModified_OwnedView tests the owned view returns correct lastModified.
func TestGetSessionsLastModified_OwnedView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "user@test.com", "User")

	ctx := context.Background()

	// Create a session
	sessionID := testutil.CreateTestSession(t, env, user.ID, "owned-session")

	// Update the session's last_sync_at to a known time
	knownTime := time.Date(2024, 7, 20, 14, 45, 0, 0, time.UTC)
	_, err := env.DB.Exec(ctx, "UPDATE sessions SET last_sync_at = $1 WHERE id = $2", knownTime, sessionID)
	if err != nil {
		t.Fatalf("failed to update session last_sync_at: %v", err)
	}

	lastModified, err := env.DB.GetSessionsLastModified(ctx, user.ID, db.SessionListViewOwned)
	if err != nil {
		t.Fatalf("GetSessionsLastModified failed: %v", err)
	}

	if !lastModified.Equal(knownTime) {
		t.Errorf("lastModified = %v, want %v", lastModified, knownTime)
	}
}
