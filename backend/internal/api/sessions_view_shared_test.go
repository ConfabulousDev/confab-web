package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/santaclaude2025/confab/backend/internal/api"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/testutil"
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
	userC := testutil.CreateTestUser(t, env, "userc@example.com", "User C")

	// User A creates two sessions
	sessionA1 := "session-a1"
	sessionA2 := "session-a2"
	sessionA1PK := testutil.CreateTestSession(t, env, userA.ID, sessionA1)
	sessionA2PK := testutil.CreateTestSession(t, env, userA.ID, sessionA2)

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

	// User A creates a private share for sessionA1, inviting userB
	var shareID int64
	var shareToken string
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id, share_token, visibility)
		 VALUES ($1, $2, 'private') RETURNING id, share_token`,
		sessionA1PK, testutil.GenerateShareToken()).Scan(&shareID, &shareToken)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Add userB to invite list
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_invites (share_id, email) VALUES ($1, $2)`,
		shareID, userB.Email)
	if err != nil {
		t.Fatalf("Failed to create invite: %v", err)
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

	// User A creates a public share for sessionA2
	var publicShareID int64
	var publicShareToken string
	err = env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id, share_token, visibility)
		 VALUES ($1, $2, 'public') RETURNING id, share_token`,
		sessionA2PK, testutil.GenerateShareToken()).Scan(&publicShareID, &publicShareToken)
	if err != nil {
		t.Fatalf("Failed to create public share: %v", err)
	}

	// User C accesses the public share (simulating logged-in access)
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_accesses (share_id, user_id, first_accessed_at, last_accessed_at, access_count)
		 VALUES ($1, $2, NOW(), NOW(), 1)`,
		publicShareID, userC.ID)
	if err != nil {
		t.Fatalf("Failed to record share access: %v", err)
	}

	t.Run("User C sees public share they accessed when include_shared=true", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?include_shared=true", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userC.ID)
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
			t.Fatalf("Expected 1 session (accessed public share), got %d", len(sessions))
		}

		if sessions[0].ExternalID != sessionA2 {
			t.Errorf("Expected session %s, got %s", sessionA2, sessions[0].ExternalID)
		}
		if sessions[0].IsOwner {
			t.Error("Expected IsOwner=false for public share")
		}
		if sessions[0].AccessType != "public_share" {
			t.Errorf("Expected AccessType=public_share, got %s", sessions[0].AccessType)
		}
		if sessions[0].SharedByEmail == nil || *sessions[0].SharedByEmail != userA.Email {
			t.Errorf("Expected SharedByEmail=%s, got %v", userA.Email, sessions[0].SharedByEmail)
		}
	})

	t.Run("Expired shares do not appear in list", func(t *testing.T) {
		// Create an expired share
		yesterday := time.Now().UTC().Add(-24 * time.Hour)
		_, err := env.DB.Exec(ctx,
			`INSERT INTO session_shares (session_id, share_token, visibility, expires_at)
			 VALUES ($1, $2, 'public', $3)`,
			sessionB1PK, testutil.GenerateShareToken(), yesterday)
		if err != nil {
			t.Fatalf("Failed to create expired share: %v", err)
		}

		// User C should NOT see the expired share
		req, _ := http.NewRequest("GET", "/api/v1/sessions?include_shared=true", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userC.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		var sessions []db.SessionListItem
		json.Unmarshal(rr.Body.Bytes(), &sessions)

		// Should still only see the non-expired public share from sessionA2
		if len(sessions) != 1 {
			t.Fatalf("Expected 1 session (expired share should be filtered), got %d", len(sessions))
		}
		if sessions[0].ExternalID == sessionB1 {
			t.Error("Expired share should not appear in list")
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
