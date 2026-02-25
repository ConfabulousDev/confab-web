package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestUpsertSearchIndex_InsertAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "searchidx@test.com", "SearchIdx User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-search-idx")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	now := time.Now().UTC().Truncate(time.Microsecond)
	record := &analytics.SearchIndexRecord{
		SessionID:      sessionID,
		Version:        analytics.SearchIndexVersion,
		IndexedUpToLine: 42,
		RecapIndexedAt: &now,
		MetadataHash:   "abc123hash",
	}
	content := &analytics.SearchIndexContent{
		MetadataText:     "Fix authentication bug",
		RecapText:        "Session involved fixing an auth bug",
		UserMessagesText: "help me fix the auth flow",
	}

	// Insert
	err := store.UpsertSearchIndex(ctx, record, content)
	if err != nil {
		t.Fatalf("UpsertSearchIndex insert failed: %v", err)
	}

	// Get
	retrieved, err := store.GetSearchIndex(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSearchIndex failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected search index record to be retrieved")
	}

	if retrieved.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", retrieved.SessionID, sessionID)
	}
	if retrieved.Version != analytics.SearchIndexVersion {
		t.Errorf("Version = %d, want %d", retrieved.Version, analytics.SearchIndexVersion)
	}
	if retrieved.IndexedUpToLine != 42 {
		t.Errorf("IndexedUpToLine = %d, want 42", retrieved.IndexedUpToLine)
	}
	if retrieved.MetadataHash != "abc123hash" {
		t.Errorf("MetadataHash = %q, want %q", retrieved.MetadataHash, "abc123hash")
	}
	if retrieved.ContentText != content.CombinedText() {
		t.Errorf("ContentText = %q, want %q", retrieved.ContentText, content.CombinedText())
	}
	if retrieved.RecapIndexedAt == nil {
		t.Fatal("expected RecapIndexedAt to be set")
	}
	if !retrieved.RecapIndexedAt.Equal(now) {
		t.Errorf("RecapIndexedAt = %v, want %v", retrieved.RecapIndexedAt, now)
	}
}

func TestUpsertSearchIndex_Upsert(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "searchupsert@test.com", "SearchUpsert User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-search-upsert")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	// First insert
	record := &analytics.SearchIndexRecord{
		SessionID:       sessionID,
		Version:         analytics.SearchIndexVersion,
		IndexedUpToLine: 10,
		MetadataHash:    "hash1",
	}
	content := &analytics.SearchIndexContent{
		MetadataText: "Original title",
	}
	err := store.UpsertSearchIndex(ctx, record, content)
	if err != nil {
		t.Fatalf("first UpsertSearchIndex failed: %v", err)
	}

	// Update with new content
	record.IndexedUpToLine = 50
	record.MetadataHash = "hash2"
	content.MetadataText = "Updated title"
	content.UserMessagesText = "new user messages"

	err = store.UpsertSearchIndex(ctx, record, content)
	if err != nil {
		t.Fatalf("second UpsertSearchIndex failed: %v", err)
	}

	// Verify updated values
	retrieved, err := store.GetSearchIndex(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSearchIndex failed: %v", err)
	}
	if retrieved.IndexedUpToLine != 50 {
		t.Errorf("IndexedUpToLine = %d, want 50", retrieved.IndexedUpToLine)
	}
	if retrieved.MetadataHash != "hash2" {
		t.Errorf("MetadataHash = %q, want %q", retrieved.MetadataHash, "hash2")
	}
	if retrieved.ContentText != content.CombinedText() {
		t.Errorf("ContentText = %q, want %q", retrieved.ContentText, content.CombinedText())
	}
}

func TestUpsertSearchIndex_FTSQueryMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "ftsquery@test.com", "FTSQuery User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-fts-query")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	record := &analytics.SearchIndexRecord{
		SessionID:       sessionID,
		Version:         analytics.SearchIndexVersion,
		IndexedUpToLine: 10,
		MetadataHash:    "hash",
	}
	content := &analytics.SearchIndexContent{
		MetadataText:     "Implementing authentication flow",
		RecapText:        "Session focused on OAuth2 integration",
		UserMessagesText: "help me set up login with Google",
	}

	err := store.UpsertSearchIndex(ctx, record, content)
	if err != nil {
		t.Fatalf("UpsertSearchIndex failed: %v", err)
	}

	// Verify FTS query works using raw SQL
	tests := []struct {
		name    string
		tsquery string
		match   bool
	}{
		{"exact word from metadata", "authentication", true},
		{"prefix match", "auth:*", true},
		{"word from recap", "OAuth2", true},
		{"word from user messages", "login", true},
		{"stemmed form", "authenticate", true}, // stems to 'authent'
		{"non-matching word", "kubernetes", false},
		{"multi-word AND match", "authentication & login", true},
		{"multi-word AND no match", "authentication & kubernetes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found bool
			query := `SELECT EXISTS(
				SELECT 1 FROM session_search_index
				WHERE session_id = $1 AND search_vector @@ to_tsquery('english', $2)
			)`
			err := env.DB.Conn().QueryRowContext(ctx, query, sessionID, tt.tsquery).Scan(&found)
			if err != nil {
				t.Fatalf("FTS query failed: %v", err)
			}
			if found != tt.match {
				t.Errorf("FTS match for %q = %v, want %v", tt.tsquery, found, tt.match)
			}
		})
	}
}

func TestGetSearchIndex_NonExistent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	user := testutil.CreateTestUser(t, env, "nosearchidx@test.com", "NoSearchIdx User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "test-no-search-idx")
	ctx := context.Background()

	store := analytics.NewStore(env.DB.Conn())

	record, err := store.GetSearchIndex(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSearchIndex failed: %v", err)
	}
	if record != nil {
		t.Error("expected nil for non-existent search index")
	}
}
