package sessions_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/db/dbadmincardinvalidations"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestSessionTitleRecompute_HiddenCodexSessionReappearsInList is the nbrd
// end-to-end contract: a Codex session whose first_user_message was never sent
// (so ListableSessionPredicate hid it) becomes listable after an admin
// session_title invalidation plus one worker recompute from its stored chunk 1.
func TestSessionTitleRecompute_HiddenCodexSessionReappearsInList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	user := testutil.CreateTestUser(t, env, "codex@example.com", "Codex User")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

	sessionID := uuid.NewString()
	externalID := "codex-hidden-" + sessionID[:8]
	if _, err := env.DB.Exec(env.Ctx, `
		INSERT INTO sessions (id, user_id, external_id, first_seen, last_message_at, session_type)
		VALUES ($1, $2, $3, NOW(), NOW(), $4)
	`, sessionID, user.ID, externalID, models.ProviderCodex); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	const fileName = "rollout-2026-08-26T10-00-00.jsonl"
	chunk := []byte(`{"type":"session_meta","payload":{"id":"x","cli_version":"0.149.1"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n</environment_context>"}]}}
{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"UserMessage","content":[{"type":"text","text":"repair the hidden codex session"}]}}}
`)
	testutil.CreateTestSyncFile(t, env, sessionID, fileName, "transcript", 3)
	testutil.UploadTestChunk(t, env, user.ID, models.ProviderCodex, externalID, fileName, 1, 3, chunk)

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	listSessions := func() db.SessionListResult {
		t.Helper()
		resp, err := client.Get("/api/v1/sessions")
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)
		var result db.SessionListResult
		testutil.ParseJSON(t, resp, &result)
		return result
	}

	if got := listSessions(); len(got.Sessions) != 0 {
		t.Fatalf("precondition: untitled codex session should be hidden, got %d sessions", len(got.Sessions))
	}

	ctx := context.Background()
	store := &dbadmincardinvalidations.Store{DB: env.DB}
	if _, err := store.Execute(ctx, dbadmincardinvalidations.ExecuteRequest{
		CountRequest: dbadmincardinvalidations.CountRequest{
			StartDate: time.Now().UTC().Add(-time.Hour),
			CardTypes: []string{analytics.SessionTitleInvalidationTarget},
			Providers: models.ExpandWithAliases([]string{models.ProviderCodex}),
		},
		AdminUserID: adminUser.ID,
		Reason:      "codex 0.149.1 title regression",
	}); err != nil {
		t.Fatalf("invalidate session_title: %v", err)
	}

	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analytics.NewStore(env.DB.Conn()), analytics.PrecomputeConfig{})
	marked, err := precomputer.FindTitleRecomputeSessions(ctx, 10)
	if err != nil {
		t.Fatalf("FindTitleRecomputeSessions: %v", err)
	}
	if len(marked) != 1 {
		t.Fatalf("marked sessions = %d, want 1", len(marked))
	}
	if err := precomputer.RecomputeSessionTitle(ctx, marked[0]); err != nil {
		t.Fatalf("RecomputeSessionTitle: %v", err)
	}

	got := listSessions()
	if len(got.Sessions) != 1 {
		t.Fatalf("after recompute: %d sessions listed, want 1", len(got.Sessions))
	}
	item := got.Sessions[0]
	if item.ID != sessionID {
		t.Errorf("listed session id = %s, want %s", item.ID, sessionID)
	}
	if item.FirstUserMessage == nil || *item.FirstUserMessage != "repair the hidden codex session" {
		t.Errorf("first_user_message on the wire = %v, want the derived prompt", item.FirstUserMessage)
	}
}
