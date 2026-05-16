package db_test

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// TestLoadSessionSyncFiles_LoadsAllAndExcludesTodo verifies that the helper
// scans every non-todo sync_file for a session and skips todo files. The
// helper is shared between the session and access sub-packages, so a regression
// here would silently mis-populate session detail responses.
func TestLoadSessionSyncFiles_LoadsAllAndExcludesTodo(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "syncfiles@example.com", "Sync Files User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-sync-files")

	testutil.CreateTestSyncFile(t, env, sessionID, "transcript.jsonl", "transcript", 100)
	testutil.CreateTestSyncFile(t, env, sessionID, "agent.jsonl", "agent", 50)
	testutil.CreateTestSyncFile(t, env, sessionID, "todo.json", "todo", 1) // must be excluded

	session := &db.SessionDetail{ID: sessionID}
	if err := db.LoadSessionSyncFiles(context.Background(), env.DB, session); err != nil {
		t.Fatalf("LoadSessionSyncFiles: %v", err)
	}

	if len(session.Files) != 2 {
		t.Fatalf("expected 2 files (todo excluded), got %d: %+v", len(session.Files), session.Files)
	}
	for _, f := range session.Files {
		if f.FileType == "todo" {
			t.Errorf("todo file should be excluded, got %+v", f)
		}
	}
}

func TestLoadSessionSyncFiles_EmptyResult(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	user := testutil.CreateTestUser(t, env, "empty@example.com", "Empty User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-empty")

	session := &db.SessionDetail{ID: sessionID}
	if err := db.LoadSessionSyncFiles(context.Background(), env.DB, session); err != nil {
		t.Fatalf("LoadSessionSyncFiles: %v", err)
	}
	if session.Files == nil {
		t.Error("Files should be a non-nil empty slice, not nil")
	}
	if len(session.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(session.Files))
	}
}

// TestConnectAndQueryRow exercises Connect, QueryRow, Exec, Conn, and Close
// against a live Postgres container.
func TestConnectAndQueryRow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	ctx := context.Background()

	if _, err := env.DB.Exec(ctx, "SELECT 1"); err != nil {
		t.Fatalf("Exec(SELECT 1) failed: %v", err)
	}

	var n int
	if err := env.DB.QueryRow(ctx, "SELECT 42").Scan(&n); err != nil {
		t.Fatalf("QueryRow scan: %v", err)
	}
	if n != 42 {
		t.Errorf("QueryRow result = %d, want 42", n)
	}

	if env.DB.Conn() == nil {
		t.Error("Conn() returned nil")
	}
	if err := env.DB.Conn().PingContext(ctx); err != nil {
		t.Errorf("Conn().Ping: %v", err)
	}
}
