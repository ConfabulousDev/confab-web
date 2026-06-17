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
