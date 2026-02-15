package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/api"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// parseSessionListResult parses the new paginated response shape from HandleListSessions
func parseSessionListResult(t *testing.T, rr *httptest.ResponseRecorder) *db.SessionListResult {
	t.Helper()
	var result db.SessionListResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v. Body: %s", err, rr.Body.String())
	}
	return &result
}

func TestListSessionsWithSharedSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create two users
	userA := testutil.CreateTestUser(t, env, "usera@example.com", "User A")
	userB := testutil.CreateTestUser(t, env, "userb@example.com", "User B")

	// User A creates two sessions (with content so they're visible)
	sessionA1 := "session-a1"
	sessionA2 := "session-a2"
	sessionA1PK := testutil.CreateTestSessionFull(t, env, userA.ID, sessionA1, testutil.TestSessionFullOpts{
		Summary: "Session A1",
	})
	testutil.CreateTestSessionFull(t, env, userA.ID, sessionA2, testutil.TestSessionFullOpts{
		Summary: "Session A2",
	})

	// User B creates one session
	sessionB1 := "session-b1"
	sessionB1PK := testutil.CreateTestSessionFull(t, env, userB.ID, sessionB1, testutil.TestSessionFullOpts{
		Summary: "Session B1",
	})

	t.Run("User B sees only their own session before shares are created", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userB.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		result := parseSessionListResult(t, rr)

		if len(result.Sessions) != 1 {
			t.Fatalf("Expected 1 session, got %d", len(result.Sessions))
		}
		if result.Sessions[0].ExternalID != sessionB1 {
			t.Errorf("Expected session %s, got %s", sessionB1, result.Sessions[0].ExternalID)
		}
		if !result.Sessions[0].IsOwner {
			t.Error("Expected IsOwner=true for owned session")
		}
		if result.Sessions[0].AccessType != "owner" {
			t.Errorf("Expected AccessType=owner, got %s", result.Sessions[0].AccessType)
		}
	})

	// User A creates a non-public share for sessionA1, inviting userB by user_id
	var shareID int64
	err := env.DB.QueryRow(ctx,
		`INSERT INTO session_shares (session_id)
		 VALUES ($1) RETURNING id`,
		sessionA1PK).Scan(&shareID)
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

	t.Run("User B sees private share in unified sessions list", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userB.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		result := parseSessionListResult(t, rr)

		if len(result.Sessions) != 2 {
			t.Fatalf("Expected 2 sessions (1 owned + 1 shared), got %d", len(result.Sessions))
		}

		// Find the shared session
		var sharedSession *db.SessionListItem
		var ownedSession *db.SessionListItem
		for i := range result.Sessions {
			if result.Sessions[i].ExternalID == sessionA1 {
				sharedSession = &result.Sessions[i]
			} else if result.Sessions[i].ExternalID == sessionB1 {
				ownedSession = &result.Sessions[i]
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
			`INSERT INTO session_shares (session_id, expires_at)
			 VALUES ($1, $2) RETURNING id`,
			sessionB1PK, yesterday).Scan(&expiredShareID)
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
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userA.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		result := parseSessionListResult(t, rr)

		// User A should only see their own 2 sessions, not the expired share
		if len(result.Sessions) != 2 {
			t.Fatalf("Expected 2 sessions (user A's own), got %d", len(result.Sessions))
		}
		for _, s := range result.Sessions {
			if s.ExternalID == sessionB1 {
				t.Error("Expired share should not appear in list")
			}
		}
	})

	t.Run("User does not see duplicates if they own a session with a share", func(t *testing.T) {
		// User A should only see their 2 sessions once each, not duplicated
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), userA.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		result := parseSessionListResult(t, rr)

		if len(result.Sessions) != 2 {
			t.Errorf("Expected 2 unique sessions for owner, got %d", len(result.Sessions))
		}

		// Both should be owned
		for _, s := range result.Sessions {
			if !s.IsOwner {
				t.Errorf("Session %s should be owned by user A", s.ExternalID)
			}
		}
	})
}

func TestListSessionsWithSystemShares(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	// Create two users
	admin := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	regularUser := testutil.CreateTestUser(t, env, "user@example.com", "Regular User")

	// Admin creates a session (with content so it's visible)
	adminSession := "admin-session"
	adminSessionPK := testutil.CreateTestSessionFull(t, env, admin.ID, adminSession, testutil.TestSessionFullOpts{
		Summary: "Admin session content",
	})

	// Regular user creates their own session
	userSession := "user-session"
	testutil.CreateTestSessionFull(t, env, regularUser.ID, userSession, testutil.TestSessionFullOpts{
		Summary: "User session content",
	})

	// Create a system share for admin's session
	_, err := env.DB.CreateSystemShare(ctx, adminSessionPK, nil)
	if err != nil {
		t.Fatalf("Failed to create system share: %v", err)
	}

	t.Run("System share appears in shared sessions list", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), regularUser.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		result := parseSessionListResult(t, rr)

		// Regular user should see 2 sessions: their own + system share
		if len(result.Sessions) != 2 {
			t.Fatalf("Expected 2 sessions (1 owned + 1 system share), got %d", len(result.Sessions))
		}

		// Find the system shared session
		var systemSession *db.SessionListItem
		var ownedSession *db.SessionListItem
		for i := range result.Sessions {
			if result.Sessions[i].ExternalID == adminSession {
				systemSession = &result.Sessions[i]
			} else if result.Sessions[i].ExternalID == userSession {
				ownedSession = &result.Sessions[i]
			}
		}

		if systemSession == nil {
			t.Fatal("System shared session not found in list")
		}
		if systemSession.IsOwner {
			t.Error("Expected IsOwner=false for system share")
		}
		if systemSession.AccessType != "system_share" {
			t.Errorf("Expected AccessType=system_share, got %s", systemSession.AccessType)
		}
		if systemSession.SharedByEmail == nil || *systemSession.SharedByEmail != admin.Email {
			t.Errorf("Expected SharedByEmail=%s, got %v", admin.Email, systemSession.SharedByEmail)
		}

		// Verify owned session still correct
		if ownedSession == nil {
			t.Fatal("Owned session not found")
		}
		if !ownedSession.IsOwner {
			t.Error("Expected IsOwner=true for owned session")
		}
	})

	t.Run("Owner does not see duplicate from system share", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), admin.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		result := parseSessionListResult(t, rr)

		// Admin should only see their own session once (not duplicated via system share)
		if len(result.Sessions) != 1 {
			t.Fatalf("Expected 1 session for owner, got %d", len(result.Sessions))
		}
		if !result.Sessions[0].IsOwner {
			t.Error("Expected IsOwner=true for owner")
		}
		if result.Sessions[0].AccessType != "owner" {
			t.Errorf("Expected AccessType=owner, got %s", result.Sessions[0].AccessType)
		}
	})

	t.Run("User with both private and system share sees session once", func(t *testing.T) {
		// Also add a private share for regularUser to admin's session
		var privateShareID int64
		err := env.DB.QueryRow(ctx,
			`INSERT INTO session_shares (session_id)
			 VALUES ($1) RETURNING id`,
			adminSessionPK).Scan(&privateShareID)
		if err != nil {
			t.Fatalf("Failed to create private share: %v", err)
		}

		// Add regularUser as a recipient
		_, err = env.DB.Exec(ctx,
			`INSERT INTO session_share_recipients (share_id, email, user_id) VALUES ($1, $2, $3)`,
			privateShareID, regularUser.Email, regularUser.ID)
		if err != nil {
			t.Fatalf("Failed to create recipient: %v", err)
		}

		// regularUser now has access via both system share AND private share
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), regularUser.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		result := parseSessionListResult(t, rr)

		// Should see 2 sessions: their own + admin's (once, not twice)
		if len(result.Sessions) != 2 {
			t.Fatalf("Expected 2 sessions (1 owned + 1 shared), got %d", len(result.Sessions))
		}

		// Find the shared session
		var sharedSession *db.SessionListItem
		for i := range result.Sessions {
			if result.Sessions[i].ExternalID == adminSession {
				sharedSession = &result.Sessions[i]
			}
		}

		if sharedSession == nil {
			t.Fatal("Shared session not found")
		}
		// Private share should take priority over system share
		if sharedSession.AccessType != "private_share" {
			t.Errorf("Expected AccessType=private_share (higher priority), got %s", sharedSession.AccessType)
		}
	})
}

// TestListSessionsWithFilters_API tests the HTTP handler with filter query params
func TestListSessionsWithFilters_API(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "apifilter@test.com", "API Filter User")

	// Create sessions with different repos
	for i := 0; i < 3; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("api-fe-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/frontend.git",
			Branch:  "main",
			Summary: "Frontend work",
		})
	}
	for i := 0; i < 2; i++ {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("api-be-%d", i), testutil.TestSessionFullOpts{
			RepoURL: "https://github.com/org/backend.git",
			Branch:  "main",
			Summary: "Backend work",
		})
	}

	t.Run("Filter by repo via query param", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?repo=org/frontend", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), user.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d: %s", rr.Code, rr.Body.String())
		}

		result := parseSessionListResult(t, rr)

		if len(result.Sessions) != 3 {
			t.Errorf("Expected 3 frontend sessions, got %d", len(result.Sessions))
		}
		if result.Total != 3 {
			t.Errorf("Expected total=3, got %d", result.Total)
		}
		if result.Page != 1 {
			t.Errorf("Expected page=1, got %d", result.Page)
		}
		if result.PageSize != 50 {
			t.Errorf("Expected page_size=50, got %d", result.PageSize)
		}
	})

	t.Run("Response includes filter_options", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), user.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		result := parseSessionListResult(t, rr)

		if len(result.FilterOptions.Repos) != 2 {
			t.Errorf("Expected 2 repos in filter_options, got %d", len(result.FilterOptions.Repos))
		}
		if len(result.FilterOptions.Branches) < 1 {
			t.Errorf("Expected at least 1 branch in filter_options, got %d", len(result.FilterOptions.Branches))
		}
		if len(result.FilterOptions.Owners) < 1 {
			t.Errorf("Expected at least 1 owner in filter_options, got %d", len(result.FilterOptions.Owners))
		}
	})

	t.Run("Multi-select repos via comma-separated param", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/sessions?repo=org/frontend,org/backend", nil)
		reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), user.ID)
		req = req.WithContext(reqCtx)

		rr := httptest.NewRecorder()
		handler := api.HandleListSessions(env.DB)
		handler.ServeHTTP(rr, req)

		result := parseSessionListResult(t, rr)

		if len(result.Sessions) != 5 {
			t.Errorf("Expected 5 sessions (3 fe + 2 be), got %d", len(result.Sessions))
		}
	})
}

// TestListSessionsWithFilters_InvalidPage tests that invalid page params return 400
func TestListSessionsWithFilters_InvalidPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	user := testutil.CreateTestUser(t, env, "invalidpage@test.com", "Invalid Page User")

	tests := []struct {
		name     string
		page     string
		wantCode int
	}{
		{"page=0", "0", http.StatusBadRequest},
		{"page=-1", "-1", http.StatusBadRequest},
		{"page=abc", "abc", http.StatusBadRequest},
		{"page=1 (valid)", "1", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/api/v1/sessions?page="+tc.page, nil)
			reqCtx := context.WithValue(ctx, auth.GetUserIDContextKey(), user.ID)
			req = req.WithContext(reqCtx)

			rr := httptest.NewRecorder()
			handler := api.HandleListSessions(env.DB)
			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantCode {
				t.Errorf("page=%s: expected status %d, got %d: %s", tc.page, tc.wantCode, rr.Code, rr.Body.String())
			}
		})
	}
}
