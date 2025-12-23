package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// createSessionWithHostnameUsername creates a test session with hostname and username set
func createSessionWithHostnameUsername(t *testing.T, env *testutil.TestEnvironment, userID int64, externalID, hostname, username string) string {
	t.Helper()

	sessionID := testutil.CreateTestSession(t, env, userID, externalID)

	// Update the session with hostname and username
	query := `UPDATE sessions SET hostname = $1, username = $2 WHERE id = $3`
	_, err := env.DB.Exec(env.Ctx, query, hostname, username, sessionID)
	if err != nil {
		t.Fatalf("failed to set hostname/username: %v", err)
	}

	return sessionID
}

// TestHostnameUsernamePrivacy_SessionList tests that hostname/username are only visible to owners in the list API
func TestHostnameUsernamePrivacy_SessionList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create two users
	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	// Owner creates a session with hostname and username
	sessionID := createSessionWithHostnameUsername(t, env, owner.ID, "test-session", "owners-macbook.local", "owneruser")

	t.Run("Owner sees hostname and username in their own sessions", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), owner.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var sessions []db.SessionListItem
		if err := json.Unmarshal(rr.Body.Bytes(), &sessions); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(sessions) != 1 {
			t.Fatalf("Expected 1 session, got %d", len(sessions))
		}

		session := sessions[0]
		if session.Hostname == nil {
			t.Error("Expected hostname to be set for owner, got nil")
		} else if *session.Hostname != "owners-macbook.local" {
			t.Errorf("Expected hostname 'owners-macbook.local', got '%s'", *session.Hostname)
		}

		if session.Username == nil {
			t.Error("Expected username to be set for owner, got nil")
		} else if *session.Username != "owneruser" {
			t.Errorf("Expected username 'owneruser', got '%s'", *session.Username)
		}
	})

	// Create a private share for viewer
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionID).Scan(&shareID)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Add viewer as a recipient
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
		shareID, viewer.Email, viewer.ID)
	if err != nil {
		t.Fatalf("Failed to create recipient: %v", err)
	}

	t.Run("Viewer does NOT see hostname/username for shared sessions (private share)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?view=shared", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), viewer.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var sessions []db.SessionListItem
		if err := json.Unmarshal(rr.Body.Bytes(), &sessions); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(sessions) != 1 {
			t.Fatalf("Expected 1 shared session, got %d", len(sessions))
		}

		session := sessions[0]
		if session.Hostname != nil {
			t.Errorf("Expected hostname to be nil for shared session, got '%s'", *session.Hostname)
		}
		if session.Username != nil {
			t.Errorf("Expected username to be nil for shared session, got '%s'", *session.Username)
		}

		// Verify it's actually a shared session
		if session.IsOwner {
			t.Error("Expected IsOwner=false for shared session")
		}
		if session.AccessType != "private_share" {
			t.Errorf("Expected AccessType=private_share, got %s", session.AccessType)
		}
	})
}

// TestHostnameUsernamePrivacy_SessionListSystemShare tests hostname/username visibility for system shares
func TestHostnameUsernamePrivacy_SessionListSystemShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create two users
	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	// Owner creates a session with hostname and username
	sessionID := createSessionWithHostnameUsername(t, env, owner.ID, "system-shared-session", "server.internal", "admin")

	// Create a system share
	_, err := env.DB.CreateSystemShare(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("Failed to create system share: %v", err)
	}

	t.Run("Viewer does NOT see hostname/username for system shared sessions", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?view=shared", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), viewer.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var sessions []db.SessionListItem
		if err := json.Unmarshal(rr.Body.Bytes(), &sessions); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(sessions) != 1 {
			t.Fatalf("Expected 1 system shared session, got %d", len(sessions))
		}

		session := sessions[0]
		if session.Hostname != nil {
			t.Errorf("Expected hostname to be nil for system share, got '%s'", *session.Hostname)
		}
		if session.Username != nil {
			t.Errorf("Expected username to be nil for system share, got '%s'", *session.Username)
		}

		// Verify it's a system share
		if session.AccessType != "system_share" {
			t.Errorf("Expected AccessType=system_share, got %s", session.AccessType)
		}
	})
}

// TestHostnameUsernamePrivacy_SessionDetail tests that hostname/username are visible in the owner detail endpoint
func TestHostnameUsernamePrivacy_SessionDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	// Owner creates a session with hostname and username
	sessionID := createSessionWithHostnameUsername(t, env, owner.ID, "detail-test-session", "workstation.local", "developer")

	t.Run("Owner sees hostname and username in session detail", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), owner.ID)

		// Add URL parameter
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(reqCtx, chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		handler := api.HandleGetSession(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var session db.SessionDetail
		if err := json.Unmarshal(rr.Body.Bytes(), &session); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if session.Hostname == nil {
			t.Error("Expected hostname to be set for owner, got nil")
		} else if *session.Hostname != "workstation.local" {
			t.Errorf("Expected hostname 'workstation.local', got '%s'", *session.Hostname)
		}

		if session.Username == nil {
			t.Error("Expected username to be set for owner, got nil")
		} else if *session.Username != "developer" {
			t.Errorf("Expected username 'developer', got '%s'", *session.Username)
		}
	})
}

// TestHostnameUsernamePrivacy_SharedSession tests that hostname/username are NOT visible via shared session endpoint
func TestHostnameUsernamePrivacy_SharedSession(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	// Owner creates a session with hostname and username
	sessionID := createSessionWithHostnameUsername(t, env, owner.ID, "shared-detail-session", "secret-server.internal", "secretuser")

	// Create a public share
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionID).Scan(&shareID)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Make it public
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_public (share_id) VALUES ($1)`,
		shareID)
	if err != nil {
		t.Fatalf("Failed to make share public: %v", err)
	}

	t.Run("Canonical session endpoint does NOT expose hostname/username for shared access", func(t *testing.T) {
		// Use canonical endpoint (CF-132) instead of old /shared/{token} endpoint
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)
		req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))

		rr := httptest.NewRecorder()
		handler := api.HandleGetSession(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var session db.SessionDetail
		if err := json.Unmarshal(rr.Body.Bytes(), &session); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// The critical privacy check: hostname and username must NOT be exposed
		if session.Hostname != nil {
			t.Errorf("PRIVACY VIOLATION: hostname exposed via canonical session endpoint: '%s'", *session.Hostname)
		}
		if session.Username != nil {
			t.Errorf("PRIVACY VIOLATION: username exposed via canonical session endpoint: '%s'", *session.Username)
		}
	})
}

// TestHostnameUsernamePrivacy_SharedSessionPrivate tests private shares also don't leak hostname/username
func TestHostnameUsernamePrivacy_SharedSessionPrivate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	// Owner creates a session with hostname and username
	sessionID := createSessionWithHostnameUsername(t, env, owner.ID, "private-shared-session", "private-machine.home", "privateuser")

	// Create a private share for viewer
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionID).Scan(&shareID)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	// Add viewer as recipient
	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
		shareID, viewer.Email, viewer.ID)
	if err != nil {
		t.Fatalf("Failed to create recipient: %v", err)
	}

	t.Run("Private shared session via canonical endpoint does NOT expose hostname/username", func(t *testing.T) {
		// Use canonical endpoint (CF-132) instead of old /shared/{token} endpoint
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)

		// Simulate viewer being logged in (for private share access check)
		reqCtx := context.WithValue(ctx, chi.RouteCtxKey, rctx)
		req = req.WithContext(reqCtx)

		// Create a web session for the viewer to simulate being logged in
		webSessionToken := "test-web-session-token-for-privacy-test"
		_, err := env.DB.Exec(ctx,
			`INSERT INTO web_sessions (id, user_id, expires_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour')`,
			webSessionToken, viewer.ID)
		if err != nil {
			t.Fatalf("Failed to create web session: %v", err)
		}

		// Add cookie
		req.AddCookie(&http.Cookie{
			Name:  "confab_session",
			Value: webSessionToken,
		})

		rr := httptest.NewRecorder()
		handler := api.HandleGetSession(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		var session db.SessionDetail
		if err := json.Unmarshal(rr.Body.Bytes(), &session); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// The critical privacy check: hostname and username must NOT be exposed
		if session.Hostname != nil {
			t.Errorf("PRIVACY VIOLATION: hostname exposed via canonical session endpoint: '%s'", *session.Hostname)
		}
		if session.Username != nil {
			t.Errorf("PRIVACY VIOLATION: username exposed via canonical session endpoint: '%s'", *session.Username)
		}
	})
}
