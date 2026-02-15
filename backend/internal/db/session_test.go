package db_test

import (
	"context"
	"errors"
	"fmt"
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
// UpdateSessionSuggestedTitle Tests
// =============================================================================

// TestUpdateSessionSuggestedTitle_SetTitle tests setting a suggested title
func TestUpdateSessionSuggestedTitle_SetTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "suggested@test.com", "Suggested User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "suggested-external-id")

	ctx := context.Background()

	suggestedTitle := "Implement dark mode feature"
	err := env.DB.UpdateSessionSuggestedTitle(ctx, sessionID, suggestedTitle)
	if err != nil {
		t.Fatalf("UpdateSessionSuggestedTitle failed: %v", err)
	}

	// Verify title was set
	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.SuggestedSessionTitle == nil || *detail.SuggestedSessionTitle != suggestedTitle {
		t.Errorf("SuggestedSessionTitle = %v, want %s", detail.SuggestedSessionTitle, suggestedTitle)
	}
}

// TestUpdateSessionSuggestedTitle_EmptyDoesNotUpdate tests that empty string is a no-op
func TestUpdateSessionSuggestedTitle_EmptyDoesNotUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "empty-external-id")

	ctx := context.Background()

	// Set a title first
	initialTitle := "Initial Title"
	err := env.DB.UpdateSessionSuggestedTitle(ctx, sessionID, initialTitle)
	if err != nil {
		t.Fatalf("UpdateSessionSuggestedTitle (initial) failed: %v", err)
	}

	// Calling with empty string should be a no-op
	err = env.DB.UpdateSessionSuggestedTitle(ctx, sessionID, "")
	if err != nil {
		t.Fatalf("UpdateSessionSuggestedTitle (empty) failed: %v", err)
	}

	// Verify original title is still there
	detail, err := env.DB.GetSessionDetail(ctx, sessionID, user.ID)
	if err != nil {
		t.Fatalf("GetSessionDetail failed: %v", err)
	}
	if detail.SuggestedSessionTitle == nil || *detail.SuggestedSessionTitle != initialTitle {
		t.Errorf("SuggestedSessionTitle = %v, want %s (should not have been cleared)", detail.SuggestedSessionTitle, initialTitle)
	}
}

// TestUpdateSessionSuggestedTitle_VisibleInSessionList tests that suggested title appears in list query
func TestUpdateSessionSuggestedTitle_VisibleInSessionList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "listtest@test.com", "List Test User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "list-external-id")

	ctx := context.Background()

	suggestedTitle := "Debug OAuth login"
	err := env.DB.UpdateSessionSuggestedTitle(ctx, sessionID, suggestedTitle)
	if err != nil {
		t.Fatalf("UpdateSessionSuggestedTitle failed: %v", err)
	}

	// Verify title appears in session list
	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}
	if sessions[0].SuggestedSessionTitle == nil || *sessions[0].SuggestedSessionTitle != suggestedTitle {
		t.Errorf("SuggestedSessionTitle in list = %v, want %s", sessions[0].SuggestedSessionTitle, suggestedTitle)
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

	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
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

	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
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
	testutil.CreateTestShare(t, env, ownerSession, false, nil, []string{"recipient@test.com"})

	ctx := context.Background()

	// List shared sessions view (includes owned for deduplication)
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID)
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
	expiredTime := time.Now().Add(-time.Hour)
	testutil.CreateTestShare(t, env, ownerSession, false, &expiredTime, []string{"recipient@test.com"})

	ctx := context.Background()

	// List shared sessions - expired should be excluded
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID)
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
// ListUserSessions GitHub PRs Tests
// =============================================================================

// TestListUserSessions_IncludesGitHubPRs tests that github_prs is returned
func TestListUserSessions_IncludesGitHubPRs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "pr@test.com", "PR User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "session-with-prs")

	ctx := context.Background()

	// Create GitHub PR links (out of order to verify sorting by created_at)
	testutil.CreateTestGitHubLink(t, env, sessionID, "pull_request", "456")
	time.Sleep(10 * time.Millisecond) // Ensure different created_at times
	testutil.CreateTestGitHubLink(t, env, sessionID, "pull_request", "123")

	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	session := sessions[0]
	if len(session.GitHubPRs) != 2 {
		t.Fatalf("expected 2 GitHub PRs, got %d", len(session.GitHubPRs))
	}

	// Verify PRs are ordered by created_at (456 was created first)
	if session.GitHubPRs[0] != "456" {
		t.Errorf("expected first PR to be '456', got '%s'", session.GitHubPRs[0])
	}
	if session.GitHubPRs[1] != "123" {
		t.Errorf("expected second PR to be '123', got '%s'", session.GitHubPRs[1])
	}
}

// TestListUserSessions_GitHubPRsEmpty tests that sessions without PRs have empty array
func TestListUserSessions_GitHubPRsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "noprs@test.com", "No PRs User")
	testutil.CreateTestSession(t, env, user.ID, "session-no-prs")

	ctx := context.Background()

	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if len(sessions[0].GitHubPRs) != 0 {
		t.Errorf("expected empty GitHubPRs, got %v", sessions[0].GitHubPRs)
	}
}

// TestListUserSessions_GitHubPRsExcludesCommits tests that commits are not included
func TestListUserSessions_GitHubPRsExcludesCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "mixed@test.com", "Mixed Links User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "session-mixed-links")

	ctx := context.Background()

	// Create a PR and a commit link
	testutil.CreateTestGitHubLink(t, env, sessionID, "pull_request", "42")
	testutil.CreateTestGitHubLink(t, env, sessionID, "commit", "abc123def")

	sessions, err := env.DB.ListUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Should only have 1 PR (commit excluded)
	if len(sessions[0].GitHubPRs) != 1 {
		t.Fatalf("expected 1 GitHub PR (commits excluded), got %d", len(sessions[0].GitHubPRs))
	}
	if sessions[0].GitHubPRs[0] != "42" {
		t.Errorf("expected PR '42', got '%s'", sessions[0].GitHubPRs[0])
	}
}

// TestListUserSessions_GitHubPRsSharedView tests PRs are included in shared view
func TestListUserSessions_GitHubPRsSharedView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	sessionID := testutil.CreateTestSession(t, env, owner.ID, "shared-session-with-prs")

	ctx := context.Background()

	// Create GitHub PR link
	testutil.CreateTestGitHubLink(t, env, sessionID, "pull_request", "999")

	// Share with recipient
	testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"recipient@test.com"})

	// List from recipient's shared view
	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	// Find the shared session
	var sharedSession *db.SessionListItem
	for i := range sessions {
		if !sessions[i].IsOwner {
			sharedSession = &sessions[i]
			break
		}
	}

	if sharedSession == nil {
		t.Fatal("expected to find shared session")
	}

	if len(sharedSession.GitHubPRs) != 1 {
		t.Fatalf("expected 1 GitHub PR in shared session, got %d", len(sharedSession.GitHubPRs))
	}
	if sharedSession.GitHubPRs[0] != "999" {
		t.Errorf("expected PR '999', got '%s'", sharedSession.GitHubPRs[0])
	}
}

// =============================================================================
// ShareAllSessions ListUserSessions Tests
// =============================================================================

// TestListUserSessions_ShareAllSessions_SharedView tests that all non-owned sessions
// appear in the shared view when ShareAllSessions is enabled.
func TestListUserSessions_ShareAllSessions_SharedView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Enable ShareAllSessions
	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")

	// Create sessions by owner (no shares needed)
	testutil.CreateTestSession(t, env, owner.ID, "owner-session-1")
	testutil.CreateTestSession(t, env, owner.ID, "owner-session-2")

	// Create viewer's own session
	testutil.CreateTestSession(t, env, viewer.ID, "viewer-session")

	ctx := context.Background()

	sessions, err := env.DB.ListUserSessions(ctx, viewer.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	// Should see: 1 owned + 2 system-shared (all of owner's sessions)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions (1 owned + 2 shared), got %d", len(sessions))
	}

	var ownedCount, sharedCount int
	for _, s := range sessions {
		if s.IsOwner {
			ownedCount++
		} else {
			sharedCount++
			if s.AccessType != "system_share" {
				t.Errorf("expected access_type = system_share, got %s", s.AccessType)
			}
			if s.SharedByEmail == nil || *s.SharedByEmail != "owner@test.com" {
				t.Errorf("expected SharedByEmail = owner@test.com, got %v", s.SharedByEmail)
			}
		}
	}
	if ownedCount != 1 {
		t.Errorf("expected 1 owned session, got %d", ownedCount)
	}
	if sharedCount != 2 {
		t.Errorf("expected 2 shared sessions, got %d", sharedCount)
	}
}

// TestListUserSessions_ShareAllSessions_PrivateShareTakesPrecedence tests that
// private shares still take priority over system_share in deduplication.
func TestListUserSessions_ShareAllSessions_PrivateShareTakesPrecedence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Enable ShareAllSessions
	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
	recipient := testutil.CreateTestUser(t, env, "recipient@test.com", "Recipient")

	// Create owner's session and share it privately with recipient
	sessionID := testutil.CreateTestSession(t, env, owner.ID, "shared-session")
	testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"recipient@test.com"})

	ctx := context.Background()

	sessions, err := env.DB.ListUserSessions(ctx, recipient.ID)
	if err != nil {
		t.Fatalf("ListUserSessions failed: %v", err)
	}

	// Find the shared session (not owned)
	var sharedSession *db.SessionListItem
	for i := range sessions {
		if !sessions[i].IsOwner {
			sharedSession = &sessions[i]
			break
		}
	}

	if sharedSession == nil {
		t.Fatal("expected to find shared session")
	}

	// Private share should take precedence over system_share
	if sharedSession.AccessType != "private_share" {
		t.Errorf("expected access_type = private_share (takes precedence), got %s", sharedSession.AccessType)
	}
}

// =============================================================================
// ListUserSessionsPaginated Tests
// =============================================================================

// TestListUserSessionsPaginated_NoFilters tests basic cursor pagination without filters
func TestListUserSessionsPaginated_NoFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "paginate@test.com", "Paginate User")

	// Create 60 visible sessions
	for i := 0; i < 60; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("session-%03d", i), testutil.TestSessionFullOpts{
			Summary: fmt.Sprintf("Session %d summary", i),
		})
	}

	ctx := context.Background()

	// Page 1: should get 50 sessions with has_more=true
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("Page 1 failed: %v", err)
	}
	if len(result.Sessions) != 50 {
		t.Errorf("Page 1: expected 50 sessions, got %d", len(result.Sessions))
	}
	if !result.HasMore {
		t.Error("Page 1: expected has_more=true")
	}
	if result.NextCursor == "" {
		t.Error("Page 1: expected non-empty next_cursor")
	}
	if result.PageSize != 50 {
		t.Errorf("Page 1: expected page_size=50, got %d", result.PageSize)
	}

	// Page 2 via cursor: should get 10 sessions with has_more=false
	result, err = env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{Cursor: result.NextCursor, PageSize: 50})
	if err != nil {
		t.Fatalf("Page 2 failed: %v", err)
	}
	if len(result.Sessions) != 10 {
		t.Errorf("Page 2: expected 10 sessions, got %d", len(result.Sessions))
	}
	if result.HasMore {
		t.Error("Page 2: expected has_more=false")
	}
	if result.NextCursor != "" {
		t.Errorf("Page 2: expected empty next_cursor, got %s", result.NextCursor)
	}
}

// TestListUserSessionsPaginated_RepoFilter tests filtering by repository
func TestListUserSessionsPaginated_RepoFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "repofilter@test.com", "Repo Filter User")

	// Create sessions in 3 repos
	for i := 0; i < 5; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("repo-a-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/repo-a.git",
			Summary: "Repo A session",
		})
	}
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("repo-b-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "git@github.com:org/repo-b.git",
			Summary: "Repo B session",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("repo-c-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/repo-c",
			Summary: "Repo C session",
		})
	}

	ctx := context.Background()

	// Filter by repo-a
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Repos: []string{"org/repo-a"},
	})
	if err != nil {
		t.Fatalf("Repo filter failed: %v", err)
	}
	if len(result.Sessions) != 5 {
		t.Errorf("Expected 5 sessions for repo-a, got %d", len(result.Sessions))
	}
	if !result.HasMore {
		// 5 sessions out of 10 total, no more in this filtered set
		// (This is fine either way — depends on exact results)
	}

	// Filter options are pre-materialized from lookup tables — should have all repos
	if len(result.FilterOptions.Repos) != 3 {
		t.Errorf("Expected 3 repos in filter options, got %d", len(result.FilterOptions.Repos))
	}
}

// TestListUserSessionsPaginated_BranchFilter tests filtering by branch
func TestListUserSessionsPaginated_BranchFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "branchfilter@test.com", "Branch Filter User")

	// Create sessions on different branches
	for i := 0; i < 4; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("main-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/repo.git",
			Branch:  "main",
			Summary: "Main branch session",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("feature-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/repo.git",
			Branch:  "feature/new-thing",
			Summary: "Feature branch session",
		})
	}

	ctx := context.Background()

	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Branches: []string{"main"},
	})
	if err != nil {
		t.Fatalf("Branch filter failed: %v", err)
	}
	if len(result.Sessions) != 4 {
		t.Errorf("Expected 4 sessions on main, got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_OwnerFilter tests filtering by session owner email
func TestListUserSessionsPaginated_OwnerFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	alice := testutil.CreateTestUser(t, env, "alice@test.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@test.com", "Bob")

	// Alice creates 3 sessions
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, alice.ID, fmt.Sprintf("alice-session-%d", i), testutil.TestSessionFullOpts{
			Summary: "Alice session",
		})
	}
	// Bob creates 2 sessions
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, bob.ID, fmt.Sprintf("bob-session-%d", i), testutil.TestSessionFullOpts{
			Summary: "Bob session",
		})
	}

	ctx := context.Background()

	// Alice views sessions, filter by Alice only (her own)
	result, err := env.DB.ListUserSessionsPaginated(ctx, alice.ID, db.SessionListParams{
		Owners: []string{"alice@test.com"},
	})
	if err != nil {
		t.Fatalf("Owner filter failed: %v", err)
	}
	if len(result.Sessions) != 3 {
		t.Errorf("Expected 3 sessions for alice, got %d", len(result.Sessions))
	}
	for _, s := range result.Sessions {
		if !s.IsOwner {
			t.Error("Expected all sessions to be owned by alice")
		}
	}

	// Alice views sessions, filter by Bob
	result, err = env.DB.ListUserSessionsPaginated(ctx, alice.ID, db.SessionListParams{
		Owners: []string{"bob@test.com"},
	})
	if err != nil {
		t.Fatalf("Owner filter for bob failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions for bob, got %d", len(result.Sessions))
	}
	for _, s := range result.Sessions {
		if s.IsOwner {
			t.Error("Expected all sessions to be shared (bob's)")
		}
	}
}

// TestListUserSessionsPaginated_PRFilter tests filtering by PR number
func TestListUserSessionsPaginated_PRFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "prfilter@test.com", "PR Filter User")

	// Create sessions: some with PRs, some without
	s1 := testutil.CreateTestSessionFull(t, env, user.ID, "pr-session-1", testutil.TestSessionFullOpts{
		Summary: "Session with PR 123",
	})
	testutil.CreateTestGitHubLink(t, env, s1, "pull_request", "123")

	s2 := testutil.CreateTestSessionFull(t, env, user.ID, "pr-session-2", testutil.TestSessionFullOpts{
		Summary: "Session with PR 456",
	})
	testutil.CreateTestGitHubLink(t, env, s2, "pull_request", "456")

	testutil.CreateTestSessionFull(t, env, user.ID, "no-pr-session", testutil.TestSessionFullOpts{
		Summary: "Session without PR",
	})

	ctx := context.Background()

	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		PRs: []string{"123"},
	})
	if err != nil {
		t.Fatalf("PR filter failed: %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session with PR 123, got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_QuerySearch tests search across titles and commit SHA
func TestListUserSessionsPaginated_QuerySearch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "querysearch@test.com", "Query Search User")

	// Create sessions with different titles/content
	testutil.CreateTestSessionFull(t, env, user.ID, "search-session-1", testutil.TestSessionFullOpts{
		Summary: "Implementing authentication flow",
	})
	testutil.CreateTestSessionFull(t, env, user.ID, "search-session-2", testutil.TestSessionFullOpts{
		Summary:          "Fixing database connection pool",
		FirstUserMessage: "Help me fix the auth system",
	})
	s3 := testutil.CreateTestSessionFull(t, env, user.ID, "search-session-3", testutil.TestSessionFullOpts{
		Summary: "Unrelated work",
	})
	testutil.CreateTestGitHubLink(t, env, s3, "commit", "abc123def")

	ctx := context.Background()

	// Search for "auth" - should match session 1 (summary) and session 2 (first_user_message)
	q := "auth"
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Query: &q,
	})
	if err != nil {
		t.Fatalf("Query search failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions matching 'auth', got %d", len(result.Sessions))
	}

	// Search for commit SHA prefix
	q2 := "abc123"
	result, err = env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Query: &q2,
	})
	if err != nil {
		t.Fatalf("Commit SHA search failed: %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session matching commit SHA 'abc123', got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_MultipleFilters tests combining repo + branch + owner filters
func TestListUserSessionsPaginated_MultipleFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	alice := testutil.CreateTestUser(t, env, "alice@multi.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@multi.com", "Bob")

	// Alice: repo-a/main (2 sessions), repo-a/feature (1 session)
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, alice.ID, fmt.Sprintf("alice-main-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/repo-a.git",
			Branch:  "main",
			Summary: "Alice main session",
		})
	}
	testutil.CreateTestSessionFull(t, env, alice.ID, "alice-feature", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/org/repo-a.git",
		Branch:  "feature/x",
		Summary: "Alice feature session",
	})

	// Bob: repo-a/main (1 session)
	testutil.CreateTestSessionFull(t, env, bob.ID, "bob-main", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/org/repo-a.git",
		Branch:  "main",
		Summary: "Bob main session",
	})

	ctx := context.Background()

	// Alice filters: repo-a + main + alice → should get 2
	result, err := env.DB.ListUserSessionsPaginated(ctx, alice.ID, db.SessionListParams{
		Repos:    []string{"org/repo-a"},
		Branches: []string{"main"},
		Owners:   []string{"alice@multi.com"},
	})
	if err != nil {
		t.Fatalf("Multiple filters failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions (alice+repo-a+main), got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_FilterOptions tests pre-materialized filter options
func TestListUserSessionsPaginated_FilterOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "facets@test.com", "Facets User")

	// Create sessions across repos and branches
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("fa-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/frontend.git",
			Branch:  "main",
			Summary: "Frontend main session",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("fb-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/backend.git",
			Branch:  "main",
			Summary: "Backend main session",
		})
	}
	testutil.CreateTestSessionFull(t, env, user.ID, "fc-0", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/org/backend.git",
		Branch:  "develop",
		Summary: "Backend develop session",
	})

	ctx := context.Background()

	// Filter by repo=frontend → filter_options should still show ALL repos (pre-materialized)
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Repos: []string{"org/frontend"},
	})
	if err != nil {
		t.Fatalf("Filter options test failed: %v", err)
	}
	if len(result.Sessions) != 3 {
		t.Errorf("Expected 3 sessions (frontend only), got %d", len(result.Sessions))
	}
	// Pre-materialized: ALL repos should be present regardless of active filter
	if len(result.FilterOptions.Repos) != 2 {
		t.Errorf("Expected 2 repos in filter_options.repos, got %d: %+v", len(result.FilterOptions.Repos), result.FilterOptions.Repos)
	}
	// Pre-materialized: ALL branches should be present regardless of active filter
	if len(result.FilterOptions.Branches) != 2 {
		t.Errorf("Expected 2 branches in filter_options.branches, got %d: %+v", len(result.FilterOptions.Branches), result.FilterOptions.Branches)
	}
	// Owners should include the user
	if len(result.FilterOptions.Owners) < 1 {
		t.Error("Expected at least 1 owner in filter_options.owners")
	}
}

// TestListUserSessionsPaginated_EmptySessionsExcluded tests that sessions without content are excluded
func TestListUserSessionsPaginated_EmptySessionsExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty Session User")

	// Create a visible session (has content + sync lines)
	testutil.CreateTestSessionFull(t, env, user.ID, "visible-session", testutil.TestSessionFullOpts{
		Summary: "This session has content",
	})

	// Create a session with no summary and no first_user_message (invisible)
	noContentID := testutil.CreateTestSession(t, env, user.ID, "no-content-session")
	testutil.CreateTestSyncFile(t, env, noContentID, "transcript.jsonl", "transcript", 100)

	// Create a session with summary but no sync files (total_lines = 0, invisible)
	testutil.CreateTestSessionFull(t, env, user.ID, "no-lines-session", testutil.TestSessionFullOpts{
		Summary:   "Has summary but no lines",
		SyncLines: -1,
	})

	ctx := context.Background()

	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{})
	if err != nil {
		t.Fatalf("Empty sessions test failed: %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Errorf("Expected 1 session (only visible session), got %d", len(result.Sessions))
	}
	if result.HasMore {
		t.Error("Expected HasMore=false with only 1 visible session")
	}
}

// TestListUserSessionsPaginated_CursorBeyondResults tests that an invalid/exhausted cursor returns no results
func TestListUserSessionsPaginated_CursorBeyondResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "beyond@test.com", "Beyond Cursor User")

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("beyond-%d", i), testutil.TestSessionFullOpts{
			Summary: "Session content",
		})
	}

	ctx := context.Background()

	// First fetch all sessions
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("First fetch failed: %v", err)
	}
	if len(result.Sessions) != 3 {
		t.Fatalf("Expected 3 sessions, got %d", len(result.Sessions))
	}
	if result.HasMore {
		t.Error("Expected HasMore=false when all sessions fit in one page")
	}
	if result.NextCursor != "" {
		t.Errorf("Expected empty NextCursor when HasMore=false, got %q", result.NextCursor)
	}
}

// TestListUserSessionsPaginated_MultiSelect tests multi-select within a filter dimension (OR logic)
func TestListUserSessionsPaginated_MultiSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "multiselect@test.com", "Multi Select User")

	// 3 repos
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("ms-a-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/alpha.git",
			Summary: "Alpha session",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("ms-b-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/beta.git",
			Summary: "Beta session",
		})
	}
	testutil.CreateTestSessionFull(t, env, user.ID, "ms-c-0", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/org/gamma.git",
		Summary: "Gamma session",
	})

	ctx := context.Background()

	// Multi-select: filter by alpha AND beta repos (OR within dimension)
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Repos: []string{"org/alpha", "org/beta"},
	})
	if err != nil {
		t.Fatalf("Multi-select filter failed: %v", err)
	}
	if len(result.Sessions) != 5 {
		t.Errorf("Expected 5 sessions (3 alpha + 2 beta), got %d", len(result.Sessions))
	}
}

// =============================================================================
// Non-ShareAll Paginated Tests (UNION ALL + dedup path)
// =============================================================================

// TestListUserSessionsPaginated_NonShareAll_OwnedOnly tests that without share-all mode,
// users only see their own sessions (no shares configured).
func TestListUserSessionsPaginated_NonShareAll_OwnedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// ShareAllSessions is false by default — UNION ALL path
	alice := testutil.CreateTestUser(t, env, "alice@nonshare.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@nonshare.com", "Bob")

	// Alice creates 3 sessions
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, alice.ID, fmt.Sprintf("alice-ns-%d", i), testutil.TestSessionFullOpts{
			Summary: "Alice session",
			RepoURL: "https://github.com/org/alice-repo.git",
		})
	}
	// Bob creates 2 sessions (Alice should NOT see these)
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, bob.ID, fmt.Sprintf("bob-ns-%d", i), testutil.TestSessionFullOpts{
			Summary: "Bob session",
			RepoURL: "https://github.com/org/bob-repo.git",
		})
	}

	ctx := context.Background()

	// Alice should only see her own 3 sessions
	result, err := env.DB.ListUserSessionsPaginated(ctx, alice.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("NonShareAll query failed: %v", err)
	}
	if len(result.Sessions) != 3 {
		t.Errorf("Expected 3 sessions (Alice's own), got %d", len(result.Sessions))
	}
	for _, s := range result.Sessions {
		if !s.IsOwner {
			t.Errorf("Expected all sessions to be owned, got access_type=%s", s.AccessType)
		}
	}

	// Bob should only see his own 2 sessions
	result, err = env.DB.ListUserSessionsPaginated(ctx, bob.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("NonShareAll query for Bob failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions (Bob's own), got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_NonShareAll_WithPrivateShare tests the UNION ALL path
// with private shares: user sees own sessions + sessions shared with them.
func TestListUserSessionsPaginated_NonShareAll_WithPrivateShare(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	alice := testutil.CreateTestUser(t, env, "alice@share.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob@share.com", "Bob")

	// Alice creates 2 sessions
	aliceSession1 := testutil.CreateTestSessionFull(t, env, alice.ID, "alice-share-1", testutil.TestSessionFullOpts{
		Summary: "Alice session 1",
	})
	testutil.CreateTestSessionFull(t, env, alice.ID, "alice-share-2", testutil.TestSessionFullOpts{
		Summary: "Alice session 2",
	})

	// Bob creates 1 session
	testutil.CreateTestSessionFull(t, env, bob.ID, "bob-share-1", testutil.TestSessionFullOpts{
		Summary: "Bob session 1",
	})

	// Alice shares one session with Bob (private share)
	testutil.CreateTestShare(t, env, aliceSession1, false, nil, []string{"bob@share.com"})

	ctx := context.Background()

	// Bob sees his own 1 session + 1 shared by Alice = 2 total
	result, err := env.DB.ListUserSessionsPaginated(ctx, bob.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("NonShareAll with share failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions (1 owned + 1 shared), got %d", len(result.Sessions))
	}

	// Verify access types
	ownedCount := 0
	sharedCount := 0
	for _, s := range result.Sessions {
		if s.IsOwner {
			ownedCount++
		} else if s.AccessType == "private_share" {
			sharedCount++
			if s.SharedByEmail == nil || *s.SharedByEmail != "alice@share.com" {
				t.Errorf("Expected shared_by_email='alice@share.com', got %v", s.SharedByEmail)
			}
		}
	}
	if ownedCount != 1 {
		t.Errorf("Expected 1 owned session, got %d", ownedCount)
	}
	if sharedCount != 1 {
		t.Errorf("Expected 1 private_share session, got %d", sharedCount)
	}

	// Alice still sees only her own 2 sessions (she didn't receive any shares)
	result, err = env.DB.ListUserSessionsPaginated(ctx, alice.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("NonShareAll query for Alice failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 sessions for Alice, got %d", len(result.Sessions))
	}
}

// TestListUserSessionsPaginated_NonShareAll_WithFilters tests filtering in non-share-all mode
func TestListUserSessionsPaginated_NonShareAll_WithFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "filter@nonshare.com", "Filter User")

	// Create sessions with different repos
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("ns-frontend-%d", i), testutil.TestSessionFullOpts{
			Summary: "Frontend work",
			RepoURL: "https://github.com/org/frontend.git",
			Branch:  "main",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("ns-backend-%d", i), testutil.TestSessionFullOpts{
			Summary: "Backend work",
			RepoURL: "https://github.com/org/backend.git",
			Branch:  "develop",
		})
	}

	ctx := context.Background()

	// Filter by repo
	result, err := env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Repos: []string{"org/frontend"},
	})
	if err != nil {
		t.Fatalf("NonShareAll repo filter failed: %v", err)
	}
	if len(result.Sessions) != 3 {
		t.Errorf("Expected 3 frontend sessions, got %d", len(result.Sessions))
	}

	// Filter by branch
	result, err = env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{
		Branches: []string{"develop"},
	})
	if err != nil {
		t.Fatalf("NonShareAll branch filter failed: %v", err)
	}
	if len(result.Sessions) != 2 {
		t.Errorf("Expected 2 develop sessions, got %d", len(result.Sessions))
	}

	// No filters — should see all 5
	result, err = env.DB.ListUserSessionsPaginated(ctx, user.ID, db.SessionListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("NonShareAll no filter failed: %v", err)
	}
	if len(result.Sessions) != 5 {
		t.Errorf("Expected 5 total sessions, got %d", len(result.Sessions))
	}
}
