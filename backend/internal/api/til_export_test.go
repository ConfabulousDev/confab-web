package api

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// =============================================================================
// GET /api/v1/tils/export - Export TILs (API key auth, external API)
// =============================================================================

func TestExportTILs_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns own TILs with session URLs", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "exporter@test.com", "Exporter")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "export-key")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-export-1", testutil.TestSessionFullOpts{
			Summary: "My session",
			RepoURL: "https://github.com/org/repo.git",
			Branch:  "main",
		})

		msgUUID := "msg-uuid-123"
		testutil.CreateTestTIL(t, env, user.ID, sessionID, "Learned Go channels", "Channels are typed conduits", &msgUUID)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result ExportTILsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Count != 1 {
			t.Fatalf("expected count 1, got %d", result.Count)
		}
		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}

		til := result.TILs[0]
		if til.Title != "Learned Go channels" {
			t.Errorf("expected title 'Learned Go channels', got %q", til.Title)
		}
		if til.SessionID != sessionID {
			t.Errorf("expected session_id %q, got %q", sessionID, til.SessionID)
		}
		if til.SessionURL != "http://localhost:3000/sessions/"+sessionID {
			t.Errorf("unexpected session_url: %q", til.SessionURL)
		}
		if til.TranscriptDeepLink != "http://localhost:3000/sessions/"+sessionID+"?msg=msg-uuid-123" {
			t.Errorf("unexpected transcript_deep_link: %q", til.TranscriptDeepLink)
		}
		if til.GitRepo == nil || *til.GitRepo != "org/repo" {
			t.Errorf("expected git_repo 'org/repo', got %v", til.GitRepo)
		}
		if til.GitBranch == nil || *til.GitBranch != "main" {
			t.Errorf("expected git_branch 'main', got %v", til.GitBranch)
		}
		if til.OwnerEmail != "exporter@test.com" {
			t.Errorf("expected owner_email 'exporter@test.com', got %q", til.OwnerEmail)
		}
	})

	t.Run("transcript_deep_link equals session_url when no message_uuid", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "exporter@test.com", "Exporter")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "export-key")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-no-msg", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "No message UUID", "summary", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result ExportTILsResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}
		if result.TILs[0].TranscriptDeepLink != result.TILs[0].SessionURL {
			t.Errorf("expected deep_link == session_url when no message_uuid, got deep_link=%q session_url=%q",
				result.TILs[0].TranscriptDeepLink, result.TILs[0].SessionURL)
		}
	})

	t.Run("owner filter returns only matching owner", func(t *testing.T) {
		env.CleanDB(t)

		user1 := testutil.CreateTestUser(t, env, "alice@test.com", "Alice")
		user2 := testutil.CreateTestUser(t, env, "bob@test.com", "Bob")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user1.ID, "alice-key")

		session1 := testutil.CreateTestSessionFull(t, env, user1.ID, "ext-alice", testutil.TestSessionFullOpts{Summary: "s"})
		session2 := testutil.CreateTestSessionFull(t, env, user2.ID, "ext-bob", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user1.ID, session1, "Alice TIL", "alice summary", nil)
		testutil.CreateTestTIL(t, env, user2.ID, session2, "Bob TIL", "bob summary", nil)

		// System share bob's session so alice can see it
		testutil.CreateTestSystemShare(t, env, session2, nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Without filter: sees both (own + system shared)
		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var allResult ExportTILsResponse
		testutil.ParseJSON(t, resp, &allResult)

		if allResult.Count != 2 {
			t.Fatalf("expected 2 TILs without filter, got %d", allResult.Count)
		}

		// With owner filter: only alice's TILs
		resp2, err := client.Get("/api/v1/tils/export?owner=alice@test.com")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		var filteredResult ExportTILsResponse
		testutil.ParseJSON(t, resp2, &filteredResult)

		if filteredResult.Count != 1 {
			t.Fatalf("expected 1 TIL with owner filter, got %d", filteredResult.Count)
		}
		if filteredResult.TILs[0].Title != "Alice TIL" {
			t.Errorf("expected 'Alice TIL', got %q", filteredResult.TILs[0].Title)
		}
	})

	t.Run("date range filtering with semi-open interval", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "dater@test.com", "Dater")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "date-key")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-date", testutil.TestSessionFullOpts{Summary: "s"})

		// Create TILs with different timestamps by inserting directly
		env.DB.Exec(env.Ctx, `INSERT INTO tils (title, summary, session_id, owner_id, created_at) VALUES ($1, $2, $3, $4, '2026-03-01T10:00:00Z')`,
			"Early TIL", "early", sessionID, user.ID)
		env.DB.Exec(env.Ctx, `INSERT INTO tils (title, summary, session_id, owner_id, created_at) VALUES ($1, $2, $3, $4, '2026-03-10T10:00:00Z')`,
			"Middle TIL", "middle", sessionID, user.ID)
		env.DB.Exec(env.Ctx, `INSERT INTO tils (title, summary, session_id, owner_id, created_at) VALUES ($1, $2, $3, $4, '2026-03-20T10:00:00Z')`,
			"Late TIL", "late", sessionID, user.ID)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// from only: everything from March 10 onward
		resp, err := client.Get("/api/v1/tils/export?from=2026-03-10T00:00:00Z")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var fromResult ExportTILsResponse
		testutil.ParseJSON(t, resp, &fromResult)

		if fromResult.Count != 2 {
			t.Fatalf("expected 2 TILs with from filter, got %d", fromResult.Count)
		}

		// to only: everything before March 10 (exclusive)
		resp2, err := client.Get("/api/v1/tils/export?to=2026-03-10T00:00:00Z")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		var toResult ExportTILsResponse
		testutil.ParseJSON(t, resp2, &toResult)

		if toResult.Count != 1 {
			t.Fatalf("expected 1 TIL with to filter (exclusive), got %d", toResult.Count)
		}
		if toResult.TILs[0].Title != "Early TIL" {
			t.Errorf("expected 'Early TIL', got %q", toResult.TILs[0].Title)
		}

		// Both: semi-open [March 5, March 15)
		resp3, err := client.Get("/api/v1/tils/export?from=2026-03-05T00:00:00Z&to=2026-03-15T00:00:00Z")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp3.Body.Close()

		testutil.RequireStatus(t, resp3, http.StatusOK)

		var bothResult ExportTILsResponse
		testutil.ParseJSON(t, resp3, &bothResult)

		if bothResult.Count != 1 {
			t.Fatalf("expected 1 TIL with both filters, got %d", bothResult.Count)
		}
		if bothResult.TILs[0].Title != "Middle TIL" {
			t.Errorf("expected 'Middle TIL', got %q", bothResult.TILs[0].Title)
		}
	})

	t.Run("pagination with cursor", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "pager@test.com", "Pager")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "page-key")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-page", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL 1", "s1", nil)
		testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL 2", "s2", nil)
		testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL 3", "s3", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		// Page 1: page_size=2
		resp, err := client.Get("/api/v1/tils/export?page_size=2")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var page1 ExportTILsResponse
		testutil.ParseJSON(t, resp, &page1)

		if page1.Count != 2 {
			t.Fatalf("expected 2 TILs on page 1, got %d", page1.Count)
		}
		if !page1.HasMore {
			t.Error("expected has_more=true on page 1")
		}
		if page1.NextCursor == "" {
			t.Fatal("expected non-empty next_cursor on page 1")
		}

		// Page 2: use cursor
		resp2, err := client.Get("/api/v1/tils/export?page_size=2&cursor=" + page1.NextCursor)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp2.Body.Close()

		testutil.RequireStatus(t, resp2, http.StatusOK)

		var page2 ExportTILsResponse
		testutil.ParseJSON(t, resp2, &page2)

		if page2.Count != 1 {
			t.Fatalf("expected 1 TIL on page 2, got %d", page2.Count)
		}
		if page2.HasMore {
			t.Error("expected has_more=false on page 2")
		}
	})

	t.Run("empty result returns empty array", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "empty@test.com", "Empty")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "empty-key")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result ExportTILsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Count != 0 {
			t.Errorf("expected count 0, got %d", result.Count)
		}
		if result.TILs == nil {
			t.Error("expected empty array, got nil")
		}
		if len(result.TILs) != 0 {
			t.Errorf("expected 0 TILs, got %d", len(result.TILs))
		}
	})

	t.Run("excludes TILs on inaccessible sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "viewer-key")

		// User's own session
		ownSession := testutil.CreateTestSessionFull(t, env, user.ID, "ext-own", testutil.TestSessionFullOpts{Summary: "s"})
		testutil.CreateTestTIL(t, env, user.ID, ownSession, "My TIL", "visible", nil)

		// Other's unshared session — must NOT appear
		otherSession := testutil.CreateTestSessionFull(t, env, other.ID, "ext-other", testutil.TestSessionFullOpts{Summary: "s"})
		testutil.CreateTestTIL(t, env, other.ID, otherSession, "Secret TIL", "hidden", nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result ExportTILsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Count != 1 {
			t.Fatalf("expected 1 TIL (own only), got %d", result.Count)
		}
		if result.TILs[0].Title != "My TIL" {
			t.Errorf("expected 'My TIL', got %q", result.TILs[0].Title)
		}
	})

	t.Run("includes TILs on shared sessions", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		other := testutil.CreateTestUser(t, env, "other@test.com", "Other")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "viewer-key")

		// Private share
		sharedSession := testutil.CreateTestSessionFull(t, env, other.ID, "ext-shared", testutil.TestSessionFullOpts{Summary: "s"})
		testutil.CreateTestTIL(t, env, other.ID, sharedSession, "Private Shared TIL", "via share", nil)
		testutil.CreateTestShare(t, env, sharedSession, false, nil, []string{"viewer@test.com"})

		// System share
		sysSession := testutil.CreateTestSessionFull(t, env, other.ID, "ext-sys", testutil.TestSessionFullOpts{Summary: "s"})
		testutil.CreateTestTIL(t, env, other.ID, sysSession, "System Shared TIL", "via system", nil)
		testutil.CreateTestSystemShare(t, env, sysSession, nil)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result ExportTILsResponse
		testutil.ParseJSON(t, resp, &result)

		if result.Count != 2 {
			t.Fatalf("expected 2 TILs (private + system share), got %d", result.Count)
		}
	})

	// --- Error cases ---

	t.Run("returns 400 for invalid from date", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "err@test.com", "Err")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "err-key")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export?from=not-a-date")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 for invalid to date", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "err@test.com", "Err")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "err-key")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export?to=2026-03-15")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// "2026-03-15" is not valid RFC 3339 (missing time component)
		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 for invalid page_size", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "err@test.com", "Err")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "err-key")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export?page_size=501")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 for non-numeric page_size", func(t *testing.T) {
		env.CleanDB(t)

		user := testutil.CreateTestUser(t, env, "err@test.com", "Err")
		apiKey := testutil.CreateTestAPIKeyWithToken(t, env, user.ID, "err-key")

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts).WithAPIKey(apiKey.RawToken)

		resp, err := client.Get("/api/v1/tils/export?page_size=abc")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 401 without API key", func(t *testing.T) {
		env.CleanDB(t)

		ts := setupTILsTestServer(t, env)
		client := testutil.NewTestClient(t, ts) // No auth

		resp, err := client.Get("/api/v1/tils/export")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}
