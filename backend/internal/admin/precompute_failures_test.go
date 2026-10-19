package admin_test

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// quarantine drives QuarantineAfterAttempts failed attempts so (sessionID,
// bucket) ends up quarantined, the way the worker would.
func quarantine(t *testing.T, env *testutil.TestEnvironment, sessionID, bucket string) {
	t.Helper()
	store := analytics.NewStore(env.DB.Conn())
	for range analytics.QuarantineAfterAttempts {
		if err := store.ClaimAttempt(context.Background(), sessionID, bucket); err != nil {
			t.Fatalf("ClaimAttempt: %v", err)
		}
		if _, _, err := store.RecordFailure(context.Background(), sessionID, bucket, "parse failed"); err != nil {
			t.Fatalf("RecordFailure: %v", err)
		}
	}
}

type precomputeFailuresResponse struct {
	Failures []struct {
		SessionID     string `json:"session_id"`
		UserID        int64  `json:"user_id"`
		ExternalID    string `json:"external_id"`
		Provider      string `json:"provider"`
		Bucket        string `json:"bucket"`
		Attempts      int    `json:"attempts"`
		LastError     string `json:"last_error"`
		QuarantinedAt string `json:"quarantined_at"`
	} `json:"failures"`
}

type releasedResponse struct {
	Released int `json:"released"`
}

func TestPrecomputeFailuresAPI_AuthEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	routes := []struct {
		method, path string
	}{
		{"GET", "/api/v1/admin/precompute-failures"},
		{"POST", "/api/v1/admin/precompute-failures/release"},
		{"POST", "/api/v1/admin/precompute-failures/release-all"},
	}

	t.Run("unauthenticated gets 401", func(t *testing.T) {
		env.CleanDB(t)
		ts := setupTestServer(t, env)
		client := testutil.NewTestClient(t, ts)
		for _, rt := range routes {
			resp, err := client.Request(rt.method, rt.path, map[string]string{})
			if err != nil {
				t.Fatalf("%s %s: %v", rt.method, rt.path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s %s: status %d, want 401", rt.method, rt.path, resp.StatusCode)
			}
		}
	})

	t.Run("non-admin gets 403 and releases nothing", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "user@example.com", "User")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "nonadmin-q")
		quarantine(t, env, sessionID, analytics.BucketRegularCards)

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)
		for _, rt := range routes {
			resp, err := client.Request(rt.method, rt.path, map[string]string{
				"session_id": sessionID, "bucket": analytics.BucketRegularCards,
			})
			if err != nil {
				t.Fatalf("%s %s: %v", rt.method, rt.path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s: status %d, want 403", rt.method, rt.path, resp.StatusCode)
			}
		}
		list, err := analytics.NewStore(env.DB.Conn()).ListQuarantined(context.Background())
		if err != nil || len(list) != 1 {
			t.Errorf("quarantine rows after non-admin attempts = %d (%v), want 1", len(list), err)
		}
	})
}

func TestPrecomputeFailuresAPI_ListsQuarantinedSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
	sessionID := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "list-codex", models.ProviderCodex)
	quarantine(t, env, sessionID, analytics.BucketSearchIndex)

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	resp, err := client.Get("/api/v1/admin/precompute-failures")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)
	var body precomputeFailuresResponse
	testutil.ParseJSON(t, resp, &body)

	if len(body.Failures) != 1 {
		t.Fatalf("failures = %d, want 1: %+v", len(body.Failures), body.Failures)
	}
	f := body.Failures[0]
	if f.SessionID != sessionID || f.UserID != adminUser.ID || f.ExternalID != "list-codex" ||
		f.Provider != models.ProviderCodex || f.Bucket != analytics.BucketSearchIndex ||
		f.Attempts != analytics.QuarantineAfterAttempts || f.LastError != "parse failed" || f.QuarantinedAt == "" {
		t.Errorf("failure row = %+v", f)
	}
}

func TestPrecomputeFailuresAPI_EmptyListIsAnArray(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)
	resp, err := client.Get("/api/v1/admin/precompute-failures")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)
	var raw map[string]any
	testutil.ParseJSON(t, resp, &raw)
	if arr, ok := raw["failures"].([]any); !ok || len(arr) != 0 {
		t.Errorf("failures = %#v, want []", raw["failures"])
	}
}

func TestPrecomputeFailuresAPI_ReleaseDeletesRowAndAudits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
	target := testutil.CreateTestSession(t, env, adminUser.ID, "release-target")
	other := testutil.CreateTestSession(t, env, adminUser.ID, "release-other")
	quarantine(t, env, target, analytics.BucketRegularCards)
	quarantine(t, env, target, analytics.BucketSmartRecap)
	quarantine(t, env, other, analytics.BucketRegularCards)

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	var body releasedResponse
	lines := captureAuditLog(t, func() {
		resp, err := client.Post("/api/v1/admin/precompute-failures/release", map[string]string{
			"session_id": target, "bucket": analytics.BucketRegularCards,
		})
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)
		testutil.ParseJSON(t, resp, &body)
	})
	if body.Released != 1 {
		t.Errorf("released = %d, want 1", body.Released)
	}
	entry := findAuditEntry(t, lines, string(admin.ActionPrecomputeRelease))
	if entry["session_id"] != target || entry["bucket"] != analytics.BucketRegularCards {
		t.Errorf("audit entry = %v, want session_id and bucket", entry)
	}

	list, err := analytics.NewStore(env.DB.Conn()).ListQuarantined(context.Background())
	if err != nil {
		t.Fatalf("ListQuarantined: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("remaining quarantined = %d, want 2 (only the named row released): %+v", len(list), list)
	}
	for _, q := range list {
		if q.SessionID == target && q.Bucket == analytics.BucketRegularCards {
			t.Error("released row is still quarantined")
		}
	}
}

func TestPrecomputeFailuresAPI_ReleaseValidatesInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
	sessionID := testutil.CreateTestSession(t, env, adminUser.ID, "validate")

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	cases := []struct {
		name string
		body any
	}{
		{"unknown bucket", map[string]string{"session_id": sessionID, "bucket": "tokens_v2"}},
		{"missing bucket", map[string]string{"session_id": sessionID}},
		{"non-uuid session", map[string]string{"session_id": "not-a-uuid", "bucket": analytics.BucketRegularCards}},
		{"missing session", map[string]string{"bucket": analytics.BucketRegularCards}},
		{"not json", "garbage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Post("/api/v1/admin/precompute-failures/release", tc.body)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestPrecomputeFailuresAPI_ReleaseAllDeletesQuarantinedRowsAndAudits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
	a := testutil.CreateTestSession(t, env, adminUser.ID, "all-a")
	b := testutil.CreateTestSession(t, env, adminUser.ID, "all-b")
	quarantine(t, env, a, analytics.BucketRegularCards)
	quarantine(t, env, b, analytics.BucketSessionTitle)

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	var body releasedResponse
	lines := captureAuditLog(t, func() {
		resp, err := client.Post("/api/v1/admin/precompute-failures/release-all", map[string]string{})
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		testutil.RequireStatus(t, resp, http.StatusOK)
		testutil.ParseJSON(t, resp, &body)
	})
	if body.Released != 2 {
		t.Errorf("released = %d, want 2", body.Released)
	}
	entry := findAuditEntry(t, lines, string(admin.ActionPrecomputeReleaseAll))
	if entry["released"] != float64(2) {
		t.Errorf("audit entry released = %v, want 2", entry["released"])
	}
	if list, _ := analytics.NewStore(env.DB.Conn()).ListQuarantined(context.Background()); len(list) != 0 {
		t.Errorf("remaining quarantined = %d, want 0", len(list))
	}
}
