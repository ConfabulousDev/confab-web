package analytics_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	dbcursor "github.com/ConfabulousDev/confab-web/internal/db/cursor"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// cursorFixtureBytes loads the committed sanitized Cursor wire-format fixture
// (the same one the in-package compute tests use) so the precompute path runs
// against a real Cursor transcript shape, not a synthetic stub.
func cursorFixtureBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cursor", "main.jsonl"))
	if err != nil {
		t.Fatalf("read cursor fixture: %v", err)
	}
	return raw
}

// setupCursorPrecompute creates a cursor session with its transcript uploaded to
// S3 and a sync_files row, then runs PrecomputeRegularCards and returns the
// session card. The caller seeds (or omits) the cursor_session_meta row before
// calling to control whether a model is recovered.
func runCursorPrecompute(t *testing.T, env *testutil.TestEnvironment, sessionID, externalID string, userID int64) *analytics.SessionCardRecord {
	t.Helper()

	raw := cursorFixtureBytes(t)
	testutil.UploadTestTranscript(t, env, userID, models.ProviderCursor, externalID, "main.jsonl", raw)

	// Match the uploaded line count so the precompute reads the whole transcript.
	lines := 0
	for _, b := range raw {
		if b == '\n' {
			lines++
		}
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		lines++
	}
	testutil.CreateTestSyncFile(t, env, sessionID, "main.jsonl", "transcript", lines)

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())

	stale := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     userID,
		ExternalID: externalID,
		Provider:   models.ProviderCursor,
		TotalLines: int64(lines),
	}
	if err := precomputer.PrecomputeRegularCards(context.Background(), stale); err != nil {
		t.Fatalf("PrecomputeRegularCards: %v", err)
	}

	cards, err := analyticsStore.GetCards(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if cards == nil || cards.Session == nil {
		t.Fatal("expected a session card to be computed")
	}
	return cards.Session
}

// TestCursorPrecompute_ModelInSidecar_SurfacesInModelsUsed is the zsr6 wire-level
// acceptance: a cursor_session_meta row drives the session card's models_used.
func TestCursorPrecompute_ModelInSidecar_SurfacesInModelsUsed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cursor-model@test.com", "Cursor User")
	const externalID = "ext-cursor-model"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, "cursor")

	// Seed the model the sync handler would have persisted.
	cursorStore := &dbcursor.Store{DB: env.DB}
	if err := cursorStore.UpsertModel(context.Background(), sessionID, "composer-2.5"); err != nil {
		t.Fatalf("seed cursor model: %v", err)
	}

	session := runCursorPrecompute(t, env, sessionID, externalID, user.ID)

	if got := session.ModelsUsed; len(got) != 1 || got[0] != "composer-2.5" {
		t.Errorf("session.models_used = %v, want [composer-2.5]", got)
	}
}

// TestCursorPrecompute_NoSidecar_LeavesModelsUsedEmpty is the absent-metadata
// regression: no cursor_session_meta row → models_used is an empty (non-nil)
// array, never an invented model, never null (y0kc).
func TestCursorPrecompute_NoSidecar_LeavesModelsUsedEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cursor-nomodel@test.com", "Cursor User")
	const externalID = "ext-cursor-nomodel"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, "cursor")

	session := runCursorPrecompute(t, env, sessionID, externalID, user.ID)

	if session.ModelsUsed == nil {
		t.Fatal("session.models_used = nil, want non-nil empty slice")
	}
	if len(session.ModelsUsed) != 0 {
		t.Errorf("session.models_used = %v, want [] (no sidecar model → no invented model)", session.ModelsUsed)
	}
}

// countJSONLines returns the line count of a JSONL blob for the sync_files
// last_synced_line column (every non-empty line, accounting for a trailing
// newline or its absence).
func countJSONLines(raw []byte) int {
	lines := 0
	for _, b := range raw {
		if b == '\n' {
			lines++
		}
	}
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		lines++
	}
	return lines
}

// cursorSubagentFixtureBytes loads the committed sanitized Cursor subagent
// fixture (testdata/cursor/subagent.jsonl) so the precompute path runs against a
// real subagent transcript shape — main thread plus a subagent rollout uploaded
// as file_type='agent'.
func cursorSubagentFixtureBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "cursor", "subagent.jsonl"))
	if err != nil {
		t.Fatalf("read cursor subagent fixture: %v", err)
	}
	return raw
}

// TestCursorPrecompute_SubagentMergesIntoTools is the wc9t wire-level acceptance:
// a cursor session with a main transcript AND a subagent file (file_type='agent')
// must aggregate the subagent's tool calls into the parent session's Tools card.
// The subagent fixture adds Glob×2, Read×2, Grep×1.
func TestCursorPrecompute_SubagentMergesIntoTools(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cursor-subagent@test.com", "Cursor User")
	const externalID = "ext-cursor-subagent"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, "cursor")

	// Upload main transcript + subagent file under the same session.
	mainRaw := cursorFixtureBytes(t)
	subRaw := cursorSubagentFixtureBytes(t)
	testutil.UploadTestTranscript(t, env, user.ID, models.ProviderCursor, externalID, "main.jsonl", mainRaw)
	testutil.UploadTestTranscript(t, env, user.ID, models.ProviderCursor, externalID, "subagent.jsonl", subRaw)
	mainLines := countJSONLines(mainRaw)
	testutil.CreateTestSyncFile(t, env, sessionID, "main.jsonl", "transcript", mainLines)
	testutil.CreateTestSyncFile(t, env, sessionID, "subagent.jsonl", "agent", countJSONLines(subRaw))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())
	stale := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: externalID,
		Provider:   models.ProviderCursor,
		TotalLines: int64(mainLines),
	}
	if err := precomputer.PrecomputeRegularCards(context.Background(), stale); err != nil {
		t.Fatalf("PrecomputeRegularCards: %v", err)
	}

	cards, err := analyticsStore.GetCards(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetCards: %v", err)
	}
	if cards == nil || cards.Tools == nil {
		t.Fatal("expected a tools card")
	}

	// The merged Tools card must include the subagent's tool calls.
	stats := cards.Tools.ToolStats
	got := func(name string) int {
		if s := stats[name]; s != nil {
			return s.Success + s.Errors
		}
		return 0
	}
	// Main fixture: Read×1, Grep×1, Glob×1; subagent: Read×2, Grep×1, Glob×2.
	if want := 3; got("Read") != want {
		t.Errorf("Read calls = %d, want %d (main 1 + subagent 2)", got("Read"), want)
	}
	if want := 3; got("Glob") != want {
		t.Errorf("Glob calls = %d, want %d (main 1 + subagent 2)", got("Glob"), want)
	}
	if want := 2; got("Grep") != want {
		t.Errorf("Grep calls = %d, want %d (main 1 + subagent 1)", got("Grep"), want)
	}
	// UpdateCurrentStep is a subagent progress marker, not a tool (D2).
	if s := stats["UpdateCurrentStep"]; s != nil {
		t.Errorf("UpdateCurrentStep must not appear in the merged Tools card, got %+v", s)
	}
}

// TestCursorPrecompute_SubagentTextIsSearchable is decision D1: subagent
// transcript text feeds the parent session's search index, so a phrase that
// appears only in the subagent matches the session.
func TestCursorPrecompute_SubagentTextIsSearchable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cursor-subsearch@test.com", "Cursor User")
	const externalID = "ext-cursor-subsearch"
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, externalID, "cursor")

	mainRaw := cursorFixtureBytes(t)
	subRaw := cursorSubagentFixtureBytes(t)
	testutil.UploadTestTranscript(t, env, user.ID, models.ProviderCursor, externalID, "main.jsonl", mainRaw)
	testutil.UploadTestTranscript(t, env, user.ID, models.ProviderCursor, externalID, "subagent.jsonl", subRaw)
	mainLines := countJSONLines(mainRaw)
	testutil.CreateTestSyncFile(t, env, sessionID, "main.jsonl", "transcript", mainLines)
	testutil.CreateTestSyncFile(t, env, sessionID, "subagent.jsonl", "agent", countJSONLines(subRaw))

	analyticsStore := analytics.NewStore(env.DB.Conn())
	precomputer := analytics.NewPrecomputer(env.DB.Conn(), env.Storage, analyticsStore, defaultTestConfig())
	stale := analytics.StaleSession{
		SessionID:  sessionID,
		UserID:     user.ID,
		ExternalID: externalID,
		Provider:   models.ProviderCursor,
		TotalLines: int64(mainLines),
	}
	if err := precomputer.BuildSearchIndexOnly(context.Background(), stale); err != nil {
		t.Fatalf("BuildSearchIndexOnly: %v", err)
	}

	// "traversal" appears only in the subagent transcript text, not the main
	// fixture. An FTS match proves the subagent text reached the parent index.
	var found bool
	query := `SELECT EXISTS(
		SELECT 1 FROM session_search_index
		WHERE session_id = $1 AND search_vector @@ to_tsquery('english', $2)
	)`
	if err := env.DB.Conn().QueryRowContext(context.Background(), query, sessionID, "traversal").Scan(&found); err != nil {
		t.Fatalf("FTS query: %v", err)
	}
	if !found {
		t.Error("subagent-only term 'traversal' not found in the session search index (subagent text must feed search recall)")
	}
}
