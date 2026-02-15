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

// NOTE: hostname/username are no longer included in the session list API response (SessionListItem).
// They are only available in the session detail API (SessionDetail). The list-API privacy tests
// have been removed since there's nothing to test — the fields don't exist in the list response.

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
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

		// Add URL parameters
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)

		// Set up context with route params and authenticated user
		reqCtx := context.WithValue(ctx, chi.RouteCtxKey, rctx)
		reqCtx = auth.SetUserIDForTest(reqCtx, viewer.ID)
		req = req.WithContext(reqCtx)

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
