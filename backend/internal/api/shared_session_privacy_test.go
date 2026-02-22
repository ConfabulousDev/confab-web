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

// createSessionWithPII creates a test session with all PII fields set (hostname, username, cwd, transcript_path)
func createSessionWithPII(t *testing.T, env *testutil.TestEnvironment, userID int64, externalID, hostname, username, cwd, transcriptPath string) string {
	t.Helper()

	sessionID := testutil.CreateTestSession(t, env, userID, externalID)

	query := `UPDATE sessions SET hostname = $1, username = $2, cwd = $3, transcript_path = $4 WHERE id = $5`
	_, err := env.DB.Exec(env.Ctx, query, hostname, username, cwd, transcriptPath, sessionID)
	if err != nil {
		t.Fatalf("failed to set PII fields: %v", err)
	}

	return sessionID
}

// assertPIIRedacted checks that all PII fields are nil in a session detail response
func assertPIIRedacted(t *testing.T, session db.SessionDetail) {
	t.Helper()

	if session.Hostname != nil {
		t.Errorf("PRIVACY VIOLATION: hostname exposed: '%s'", *session.Hostname)
	}
	if session.Username != nil {
		t.Errorf("PRIVACY VIOLATION: username exposed: '%s'", *session.Username)
	}
	if session.CWD != nil {
		t.Errorf("PRIVACY VIOLATION: cwd exposed: '%s'", *session.CWD)
	}
	if session.TranscriptPath != nil {
		t.Errorf("PRIVACY VIOLATION: transcript_path exposed: '%s'", *session.TranscriptPath)
	}
}

// TestSharedSessionPrivacy_OwnerSeesPII tests that all PII fields are visible to the session owner
func TestSharedSessionPrivacy_OwnerSeesPII(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	sessionID := createSessionWithPII(t, env, owner.ID, "detail-test-session",
		"workstation.local", "developer", "/Users/developer/dev/project", "/Users/developer/.claude/transcript.jsonl")

	t.Run("Owner sees all PII fields in session detail", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), owner.ID)

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

		if session.Hostname == nil || *session.Hostname != "workstation.local" {
			t.Errorf("Expected hostname 'workstation.local', got %v", session.Hostname)
		}
		if session.Username == nil || *session.Username != "developer" {
			t.Errorf("Expected username 'developer', got %v", session.Username)
		}
		if session.CWD == nil || *session.CWD != "/Users/developer/dev/project" {
			t.Errorf("Expected cwd '/Users/developer/dev/project', got %v", session.CWD)
		}
		if session.TranscriptPath == nil || *session.TranscriptPath != "/Users/developer/.claude/transcript.jsonl" {
			t.Errorf("Expected transcript_path '/Users/developer/.claude/transcript.jsonl', got %v", session.TranscriptPath)
		}
	})
}

// TestSharedSessionPrivacy_PublicShare tests that PII is redacted for public share access
func TestSharedSessionPrivacy_PublicShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")

	sessionID := createSessionWithPII(t, env, owner.ID, "shared-detail-session",
		"secret-server.internal", "secretuser", "/Users/secretuser/dev/secret-project", "/Users/secretuser/.claude/transcript.jsonl")

	// Create a public share
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionID).Scan(&shareID)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_public (share_id) VALUES ($1)`,
		shareID)
	if err != nil {
		t.Fatalf("Failed to make share public: %v", err)
	}

	t.Run("Public share does NOT expose PII", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

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

		assertPIIRedacted(t, session)
	})
}

// TestSharedSessionPrivacy_PrivateShare tests that PII is redacted for private share recipients
func TestSharedSessionPrivacy_PrivateShare(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	owner := testutil.CreateTestUser(t, env, "owner@example.com", "Owner")
	viewer := testutil.CreateTestUser(t, env, "viewer@example.com", "Viewer")

	sessionID := createSessionWithPII(t, env, owner.ID, "private-shared-session",
		"private-machine.home", "privateuser", "/Users/privateuser/work/internal-project", "/Users/privateuser/.claude/transcript.jsonl")

	// Create a private share for viewer
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionID).Scan(&shareID)
	if err != nil {
		t.Fatalf("Failed to create share: %v", err)
	}

	_, err = env.DB.Exec(ctx,
		`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
		shareID, viewer.Email, viewer.ID)
	if err != nil {
		t.Fatalf("Failed to create recipient: %v", err)
	}

	t.Run("Private share does NOT expose PII", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions/"+sessionID, nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", sessionID)

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

		assertPIIRedacted(t, session)
	})
}
