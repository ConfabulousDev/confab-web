package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestListSessionsWithSharedSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create three users
	userA := testutil.CreateTestUser(t, env, "usera@example.com", "User A")
	userB := testutil.CreateTestUser(t, env, "userb@example.com", "User B")

	// User A creates two sessions
	sessionA1 := "session-a1"
	sessionA2 := "session-a2"
	sessionA1PK := testutil.CreateTestSession(t, env, userA.ID, sessionA1)
	testutil.CreateTestSession(t, env, userA.ID, sessionA2)

	// User B creates one session
	sessionB1 := "session-b1"
	sessionB1PK := testutil.CreateTestSession(t, env, userB.ID, sessionB1)

	t.Run("User B sees only their own session by default", func(t *testing.T) {
		// Test with include_shared=false (default)
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userB.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var sessions []db.SessionListItem
		json.Unmarshal(rr.Body.Bytes(), &sessions)

		if len(sessions) != 1 {
			t.Fatalf("Expected 1 session, got %d", len(sessions))
		}
		if sessions[0].ExternalID != sessionB1 {
			t.Errorf("Expected session %s, got %s", sessionB1, sessions[0].ExternalID)
		}
		if !sessions[0].IsOwner {
			t.Error("Expected IsOwner=true for owned session")
		}
		if sessions[0].AccessType != "owner" {
			t.Errorf("Expected AccessType=owner, got %s", sessions[0].AccessType)
		}
	})

	// User A creates a non-public share for sessionA1, inviting userB by user_id
	var shareID int64
	var shareToken string
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id, share_token)
		 VALUES ($1, $2) RETURNING id, share_token`,
		sessionA1PK, testutil.GenerateShareToken()).Scan(&shareID, &shareToken)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Add userB as a recipient (with resolved user_id)
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
		shareID, userB.Email, userB.ID)
	if err != nil {
		t.Fatalf("Failed to create recipient: %v", err)
	}

	t.Run("User B sees private share when include_shared=true", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?include_shared=true", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userB.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var sessions []db.SessionListItem
		json.Unmarshal(rr.Body.Bytes(), &sessions)

		if len(sessions) != 2 {
			t.Fatalf("Expected 2 sessions (1 owned + 1 shared), got %d", len(sessions))
		}

		// Find the shared session
		var sharedSession *db.SessionListItem
		var ownedSession *db.SessionListItem
		for i := range sessions {
			if sessions[i].ExternalID == sessionA1 {
				sharedSession = &sessions[i]
			} else if sessions[i].ExternalID == sessionB1 {
				ownedSession = &sessions[i]
			}
		}

		if sharedSession == nil {
			t.Fatal("Private shared session not found in list")
		}
		if sharedSession.IsOwner {
			t.Error("Expected IsOwner=false for private share")
		}
		if sharedSession.AccessType != "private_share" {
			t.Errorf("Expected AccessType=private_share, got %s", sharedSession.AccessType)
		}
		if sharedSession.SharedByEmail == nil || *sharedSession.SharedByEmail != userA.Email {
			t.Errorf("Expected SharedByEmail=%s, got %v", userA.Email, sharedSession.SharedByEmail)
		}

		// Verify owned session still correct
		if ownedSession == nil {
			t.Fatal("Owned session not found")
		}
		if !ownedSession.IsOwner {
			t.Error("Expected IsOwner=true for owned session")
		}
	})

	t.Run("Expired shares do not appear in list", func(t *testing.T) {
		// Create an expired share with a recipient
		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		var expiredShareID int64
		err := env.DB.QueryRow(ctx,
			`INSERT INTO session_shares (session_id, share_token, expires_at)
			 VALUES ($1, $2, $3) RETURNING id`,
			sessionB1PK, testutil.GenerateShareToken(), yesterday).Scan(&expiredShareID)
		if err != nil {
			t.Fatalf("Failed to create expired share: %v", err)
		}

		// Add userA as recipient
		_, err = env.DB.Exec(ctx,
			`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
			expiredShareID, userA.Email, userA.ID)
		if err != nil {
			t.Fatalf("Failed to create recipient for expired share: %v", err)
		}

		// User A should NOT see the expired share from userB
		req, _ := http.NewRequest("GET", "/api/v1/sessions?include_shared=true", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userA.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		var sessions []db.SessionListItem
		json.Unmarshal(rr.Body.Bytes(), &sessions)

		// User A should only see their own 2 sessions, not the expired share
		if len(sessions) != 2 {
			t.Fatalf("Expected 2 sessions (user A's own), got %d", len(sessions))
		}
		for _, s := range sessions {
			if s.ExternalID == sessionB1 {
				t.Error("Expired share should not appear in list")
			}
		}
	})

	t.Run("User does not see duplicates if they own a session with a share", func(t *testing.T) {
		// User A should only see their 2 sessions once each, not duplicated
		req, _ := http.NewRequest("GET", "/api/v1/sessions?include_shared=true", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userA.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		var sessions []db.SessionListItem
		json.Unmarshal(rr.Body.Bytes(), &sessions)

		if len(sessions) != 2 {
			t.Errorf("Expected 2 unique sessions for owner, got %d", len(sessions))
		}

		// Both should be owned
		for _, s := range sessions {
			if !s.IsOwner {
				t.Errorf("Session %s should be owned by user A", s.ExternalID)
			}
		}
	})
}
