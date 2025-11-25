package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/testutil"
)

// TestCreateWebSession tests creating a new web session
func TestCreateWebSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "session@test.com", "Session User")

	sessionID := "test_session_id_12345"
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	err := env.DB.CreateWebSession(context.Background(), sessionID, user.ID, expiresAt)
	if err != nil {
		t.Fatalf("CreateWebSession failed: %v", err)
	}

	// Verify session can be retrieved
	session, err := env.DB.GetWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}

	if session.ID != sessionID {
		t.Errorf("session.ID = %s, want %s", session.ID, sessionID)
	}
	if session.UserID != user.ID {
		t.Errorf("session.UserID = %d, want %d", session.UserID, user.ID)
	}
}

// TestGetWebSession_Valid tests retrieving a valid session
func TestGetWebSession_Valid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "validsession@test.com", "Valid Session User")

	sessionID := "valid_session_abc123"
	expiresAt := time.Now().UTC().Add(time.Hour) // Expires in 1 hour

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	session, err := env.DB.GetWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}

	if session.UserID != user.ID {
		t.Errorf("UserID = %d, want %d", session.UserID, user.ID)
	}
}

// TestGetWebSession_Expired tests that expired sessions are rejected
func TestGetWebSession_Expired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "expired@test.com", "Expired Session User")

	sessionID := "expired_session_xyz"
	expiresAt := time.Now().UTC().Add(-time.Hour) // Expired 1 hour ago

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Try to get expired session
	_, err := env.DB.GetWebSession(context.Background(), sessionID)
	if err == nil {
		t.Error("expected error for expired session")
	}
}

// TestGetWebSession_NotFound tests retrieving non-existent session
func TestGetWebSession_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	_, err := env.DB.GetWebSession(context.Background(), "nonexistent_session_id")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

// TestDeleteWebSession tests deleting a session (logout)
func TestDeleteWebSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "logout@test.com", "Logout User")

	sessionID := "session_to_delete"
	expiresAt := time.Now().UTC().Add(time.Hour)

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Verify session exists
	_, err := env.DB.GetWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("session should exist before delete: %v", err)
	}

	// Delete session
	err = env.DB.DeleteWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("DeleteWebSession failed: %v", err)
	}

	// Verify session no longer exists
	_, err = env.DB.GetWebSession(context.Background(), sessionID)
	if err == nil {
		t.Error("session should not exist after delete")
	}
}

// TestDeleteWebSession_NotFound tests deleting non-existent session (should not error)
func TestDeleteWebSession_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	// Deleting non-existent session should not error (idempotent)
	err := env.DB.DeleteWebSession(context.Background(), "nonexistent_session")
	if err != nil {
		t.Errorf("DeleteWebSession should not error for non-existent session: %v", err)
	}
}

// TestWebSession_MultipleUsers tests session isolation between users
func TestWebSession_MultipleUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User One")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User Two")

	session1 := "session_user1"
	session2 := "session_user2"
	expiresAt := time.Now().UTC().Add(time.Hour)

	testutil.CreateTestWebSession(t, env, session1, user1.ID, expiresAt)
	testutil.CreateTestWebSession(t, env, session2, user2.ID, expiresAt)

	// Verify each session returns the correct user
	s1, err := env.DB.GetWebSession(context.Background(), session1)
	if err != nil {
		t.Fatalf("GetWebSession(session1) failed: %v", err)
	}
	if s1.UserID != user1.ID {
		t.Errorf("session1 UserID = %d, want %d", s1.UserID, user1.ID)
	}

	s2, err := env.DB.GetWebSession(context.Background(), session2)
	if err != nil {
		t.Fatalf("GetWebSession(session2) failed: %v", err)
	}
	if s2.UserID != user2.ID {
		t.Errorf("session2 UserID = %d, want %d", s2.UserID, user2.ID)
	}

	// Deleting user1's session shouldn't affect user2's
	err = env.DB.DeleteWebSession(context.Background(), session1)
	if err != nil {
		t.Fatalf("DeleteWebSession failed: %v", err)
	}

	// user2's session should still work
	s2Again, err := env.DB.GetWebSession(context.Background(), session2)
	if err != nil {
		t.Fatalf("user2's session should still work: %v", err)
	}
	if s2Again.UserID != user2.ID {
		t.Errorf("session2 UserID = %d, want %d", s2Again.UserID, user2.ID)
	}
}

// TestWebSession_ExpiryBoundary tests session expiry at exact boundary
func TestWebSession_ExpiryBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "boundary@test.com", "Boundary User")

	sessionID := "boundary_session"

	// Create session that expires in 100ms
	expiresAt := time.Now().UTC().Add(100 * time.Millisecond)
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Should be valid immediately
	_, err := env.DB.GetWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("session should be valid immediately: %v", err)
	}

	// Wait for expiry
	time.Sleep(200 * time.Millisecond)

	// Should now be expired
	_, err = env.DB.GetWebSession(context.Background(), sessionID)
	if err == nil {
		t.Error("session should be expired after waiting")
	}
}
