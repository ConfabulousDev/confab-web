package dbauth_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestCreateWebSession tests creating a new web session
func TestCreateWebSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "session@test.com", "Session User")

	sessionID := "test_session_id_12345"
	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)

	err := store.CreateWebSession(context.Background(), sessionID, user.ID, expiresAt)
	if err != nil {
		t.Fatalf("CreateWebSession failed: %v", err)
	}

	// Verify session can be retrieved
	session, err := store.GetWebSession(context.Background(), sessionID, time.Hour)
	if err != nil {
		t.Fatalf("GetWebSession failed: %v", err)
	}

	// session.ID is stored/returned as the hash of the cookie value (40hj);
	// lookup-by-raw (above) round-trips.
	if session.ID == sessionID {
		t.Error("session.ID must be the hash, not the raw cookie value")
	}
	if session.ID != db.HashToken(sessionID) {
		t.Errorf("session.ID = %s, want sha256(raw) %s", session.ID, db.HashToken(sessionID))
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
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "validsession@test.com", "Valid Session User")

	sessionID := "valid_session_abc123"
	expiresAt := time.Now().UTC().Add(time.Hour) // Expires in 1 hour

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	session, err := store.GetWebSession(context.Background(), sessionID, time.Hour)
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
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "expired@test.com", "Expired Session User")

	sessionID := "expired_session_xyz"
	expiresAt := time.Now().UTC().Add(-time.Hour) // Expired 1 hour ago

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Try to get expired session
	_, err := store.GetWebSession(context.Background(), sessionID, time.Hour)
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
	store := &dbauth.Store{DB: env.DB}

	_, err := store.GetWebSession(context.Background(), "nonexistent_session_id", time.Hour)
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
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "logout@test.com", "Logout User")

	sessionID := "session_to_delete"
	expiresAt := time.Now().UTC().Add(time.Hour)

	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Verify session exists
	_, err := store.GetWebSession(context.Background(), sessionID, time.Hour)
	if err != nil {
		t.Fatalf("session should exist before delete: %v", err)
	}

	// Delete session
	err = store.DeleteWebSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("DeleteWebSession failed: %v", err)
	}

	// Verify session no longer exists
	_, err = store.GetWebSession(context.Background(), sessionID, time.Hour)
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
	store := &dbauth.Store{DB: env.DB}

	// Deleting non-existent session should not error (idempotent)
	err := store.DeleteWebSession(context.Background(), "nonexistent_session")
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
	store := &dbauth.Store{DB: env.DB}

	user1 := testutil.CreateTestUser(t, env, "user1@test.com", "User One")
	user2 := testutil.CreateTestUser(t, env, "user2@test.com", "User Two")

	session1 := "session_user1"
	session2 := "session_user2"
	expiresAt := time.Now().UTC().Add(time.Hour)

	testutil.CreateTestWebSession(t, env, session1, user1.ID, expiresAt)
	testutil.CreateTestWebSession(t, env, session2, user2.ID, expiresAt)

	// Verify each session returns the correct user
	s1, err := store.GetWebSession(context.Background(), session1, time.Hour)
	if err != nil {
		t.Fatalf("GetWebSession(session1) failed: %v", err)
	}
	if s1.UserID != user1.ID {
		t.Errorf("session1 UserID = %d, want %d", s1.UserID, user1.ID)
	}

	s2, err := store.GetWebSession(context.Background(), session2, time.Hour)
	if err != nil {
		t.Fatalf("GetWebSession(session2) failed: %v", err)
	}
	if s2.UserID != user2.ID {
		t.Errorf("session2 UserID = %d, want %d", s2.UserID, user2.ID)
	}

	// Deleting user1's session shouldn't affect user2's
	err = store.DeleteWebSession(context.Background(), session1)
	if err != nil {
		t.Fatalf("DeleteWebSession failed: %v", err)
	}

	// user2's session should still work
	s2Again, err := store.GetWebSession(context.Background(), session2, time.Hour)
	if err != nil {
		t.Fatalf("user2's session should still work: %v", err)
	}
	if s2Again.UserID != user2.ID {
		t.Errorf("session2 UserID = %d, want %d", s2Again.UserID, user2.ID)
	}
}

// --- 60j6: sliding idle-timeout gate ---

// setLastActivity backdates a session's last_activity_at for idle tests.
func setLastActivity(t *testing.T, env *testutil.TestEnvironment, sessionID string, at time.Time) {
	t.Helper()
	_, err := env.DB.Exec(env.Ctx,
		`UPDATE web_sessions SET last_activity_at = $1 WHERE id = $2`, at, db.HashToken(sessionID))
	if err != nil {
		t.Fatalf("setLastActivity: %v", err)
	}
}

// getLastActivity reads a session's stored last_activity_at.
func getLastActivity(t *testing.T, env *testutil.TestEnvironment, sessionID string) time.Time {
	t.Helper()
	var ts time.Time
	err := env.DB.QueryRow(env.Ctx,
		`SELECT last_activity_at FROM web_sessions WHERE id = $1`, db.HashToken(sessionID)).Scan(&ts)
	if err != nil {
		t.Fatalf("getLastActivity: %v", err)
	}
	return ts
}

// TestGetWebSession_IdleExpired: a session inactive longer than the idle window
// is rejected even though its absolute expiry is in the future.
func TestGetWebSession_IdleExpired(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "idle@test.com", "Idle User")

	sessionID := "idle_expired_session"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(7*24*time.Hour))
	setLastActivity(t, env, sessionID, time.Now().UTC().Add(-2*time.Hour)) // idle 2h ago

	if _, err := store.GetWebSession(context.Background(), sessionID, time.Hour); err == nil {
		t.Error("expected error: session idle longer than the 1h window")
	}
}

// TestGetWebSession_ActiveAcceptedAndTouched: a session active within the window
// is accepted, and a read past the throttle window advances last_activity_at.
func TestGetWebSession_ActiveAcceptedAndTouched(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "active@test.com", "Active User")

	sessionID := "active_session"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(7*24*time.Hour))
	// Active 5 min ago: within the 1h idle window, but older than the 60s
	// throttle window so the read should touch it.
	stale := time.Now().UTC().Add(-5 * time.Minute)
	setLastActivity(t, env, sessionID, stale)

	if _, err := store.GetWebSession(context.Background(), sessionID, time.Hour); err != nil {
		t.Fatalf("active session should be accepted: %v", err)
	}
	if got := getLastActivity(t, env, sessionID); !got.After(stale) {
		t.Errorf("last_activity_at = %v, want advanced past %v (touch should fire)", got, stale)
	}
}

// TestGetWebSession_ThrottleSuppressesWrite: a read while last_activity_at is
// within the 60s throttle window must NOT write.
func TestGetWebSession_ThrottleSuppressesWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "throttle@test.com", "Throttle User")

	sessionID := "throttle_session"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(7*24*time.Hour))
	recent := time.Now().UTC().Add(-10 * time.Second) // inside the 60s window
	setLastActivity(t, env, sessionID, recent)

	if _, err := store.GetWebSession(context.Background(), sessionID, time.Hour); err != nil {
		t.Fatalf("session should be accepted: %v", err)
	}
	if got := getLastActivity(t, env, sessionID); !got.Equal(recent) {
		t.Errorf("last_activity_at = %v, want unchanged %v (throttle should suppress write)", got, recent)
	}
}

// TestGetWebSession_AbsoluteCapStillEnforced: fresh activity does NOT rescue an
// absolutely-expired session — idle is an additional gate, not a replacement.
func TestGetWebSession_AbsoluteCapStillEnforced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "abscap@test.com", "AbsCap User")

	sessionID := "abscap_session"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(-time.Hour)) // expired
	setLastActivity(t, env, sessionID, time.Now().UTC())                                        // active now

	if _, err := store.GetWebSession(context.Background(), sessionID, time.Hour); err == nil {
		t.Error("expected error: absolute expiry must still reject despite fresh activity")
	}
}

// TestGetWebSession_NullLastActivityLegacy: a rollout-gap row with NULL
// last_activity_at falls back to created_at and is not force-logged-out while
// within the absolute cap.
func TestGetWebSession_NullLastActivityLegacy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "legacy@test.com", "Legacy User")

	sessionID := "legacy_null_session"
	// CreateTestWebSession leaves last_activity_at NULL (created_at = NOW).
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(7*24*time.Hour))

	if _, err := store.GetWebSession(context.Background(), sessionID, time.Hour); err != nil {
		t.Errorf("NULL last_activity_at should COALESCE to created_at and be accepted: %v", err)
	}
}

// TestGetWebSession_DemoExempt: a non-positive idleTimeout disables the idle
// gate AND the touch (the CF-483 demo shared session).
func TestGetWebSession_DemoExempt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}
	user := testutil.CreateTestUser(t, env, "demo-exempt@test.com", "Demo User")

	sessionID := "demo_shared_session"
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, time.Now().UTC().Add(100*365*24*time.Hour))
	veryStale := time.Now().UTC().Add(-365 * 24 * time.Hour) // a year idle
	setLastActivity(t, env, sessionID, veryStale)

	// idleTimeout = 0 → exempt: accepted despite a year of inactivity.
	if _, err := store.GetWebSession(context.Background(), sessionID, 0); err != nil {
		t.Fatalf("demo session (idleTimeout=0) should be accepted despite long idle: %v", err)
	}
	// And the touch must be skipped (no write thrash on the shared row).
	if got := getLastActivity(t, env, sessionID); !got.Equal(veryStale) {
		t.Errorf("last_activity_at = %v, want unchanged %v (touch must be skipped for demo)", got, veryStale)
	}
}

// TestWebSession_ExpiryBoundary tests session expiry at exact boundary
func TestWebSession_ExpiryBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbauth.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "boundary@test.com", "Boundary User")

	sessionID := "boundary_session"

	// Create session that expires in 100ms
	expiresAt := time.Now().UTC().Add(100 * time.Millisecond)
	testutil.CreateTestWebSession(t, env, sessionID, user.ID, expiresAt)

	// Should be valid immediately
	_, err := store.GetWebSession(context.Background(), sessionID, time.Hour)
	if err != nil {
		t.Fatalf("session should be valid immediately: %v", err)
	}

	// Wait for expiry
	time.Sleep(200 * time.Millisecond)

	// Should now be expired
	_, err = store.GetWebSession(context.Background(), sessionID, time.Hour)
	if err == nil {
		t.Error("session should be expired after waiting")
	}
}
