package cursor

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setupCursorEnv creates a user + cursor session + the cursor store. The
// session provider is "cursor" because in production all writers to
// cursor_session_meta go through a cursor session.
func setupCursorEnv(t *testing.T) (*Store, string, context.Context) {
	t.Helper()
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "cursor-meta@test.com", "Cursor User")
	sessionID := testutil.CreateTestSessionWithProvider(t, env, user.ID, "ext-cursor", "cursor")
	return &Store{DB: env.DB}, sessionID, context.Background()
}

func TestGetModel_NoRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, sessionID, ctx := setupCursorEnv(t)

	model, found, err := store.GetModel(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if found {
		t.Errorf("found = true, want false for a session with no meta row")
	}
	if model != "" {
		t.Errorf("model = %q, want empty", model)
	}
}

func TestUpsertModel_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, sessionID, ctx := setupCursorEnv(t)

	if err := store.UpsertModel(ctx, sessionID, "composer-2.5"); err != nil {
		t.Fatalf("UpsertModel: %v", err)
	}

	model, found, err := store.GetModel(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if !found {
		t.Fatal("found = false, want true after upsert")
	}
	if model != "composer-2.5" {
		t.Errorf("model = %q, want %q", model, "composer-2.5")
	}
}

func TestUpsertModel_FirstNonEmptyWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	store, sessionID, ctx := setupCursorEnv(t)

	if err := store.UpsertModel(ctx, sessionID, "composer-2.5"); err != nil {
		t.Fatalf("first UpsertModel: %v", err)
	}
	// A later, different model must NOT clobber the first one.
	if err := store.UpsertModel(ctx, sessionID, "gpt-5"); err != nil {
		t.Fatalf("second UpsertModel: %v", err)
	}

	model, _, err := store.GetModel(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetModel: %v", err)
	}
	if model != "composer-2.5" {
		t.Errorf("model = %q, want %q (first write wins)", model, "composer-2.5")
	}
}
