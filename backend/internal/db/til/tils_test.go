package til

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

func TestCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("success", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-1")
		msgUUID := "msg-uuid-1"

		til := &models.TIL{
			Title:       "Learned about channels",
			Summary:     "Go channels are great for concurrency",
			SessionID:   sessionID,
			MessageUUID: &msgUUID,
			OwnerID:     user.ID,
		}

		created, err := store.Create(context.Background(), til)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.ID == 0 {
			t.Error("expected non-zero ID")
		}
		if created.CreatedAt.IsZero() {
			t.Error("expected non-zero created_at")
		}
		if created.Title != "Learned about channels" {
			t.Errorf("expected title 'Learned about channels', got %q", created.Title)
		}
	})

	t.Run("invalid session ID returns error", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "creator@test.com", "Creator")

		til := &models.TIL{
			Title:     "Test",
			Summary:   "Test",
			SessionID: "not-a-uuid",
			OwnerID:   user.ID,
		}

		_, err := store.Create(context.Background(), til)
		if err == nil {
			t.Fatal("expected error for invalid session ID")
		}
	})
}

func TestGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("success", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "getter@test.com", "Getter")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-get-1")
		tilID := testutil.CreateTestTIL(t, env, user.ID, sessionID, "Test Title", "Test Summary", nil)

		til, err := store.GetByID(context.Background(), tilID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if til.Title != "Test Title" {
			t.Errorf("expected title 'Test Title', got %q", til.Title)
		}
		if til.Summary != "Test Summary" {
			t.Errorf("expected summary 'Test Summary', got %q", til.Summary)
		}
	})

	t.Run("not found", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		_, err := store.GetByID(context.Background(), 99999)
		if err != db.ErrTILNotFound {
			t.Errorf("expected ErrTILNotFound, got %v", err)
		}
	})
}

func TestDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("success", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "deleter@test.com", "Deleter")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-del-1")
		tilID := testutil.CreateTestTIL(t, env, user.ID, sessionID, "To Delete", "Summary", nil)

		err := store.Delete(context.Background(), tilID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify deleted
		_, err = store.GetByID(context.Background(), tilID)
		if err != db.ErrTILNotFound {
			t.Errorf("expected ErrTILNotFound after delete, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		err := store.Delete(context.Background(), 99999)
		if err != db.ErrTILNotFound {
			t.Errorf("expected ErrTILNotFound, got %v", err)
		}
	})
}

func TestListForSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns TILs for session", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "lister@test.com", "Lister")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-list-1")

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL 1", "Summary 1", nil)
		testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL 2", "Summary 2", nil)

		tils, err := store.ListForSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tils) != 2 {
			t.Errorf("expected 2 TILs, got %d", len(tils))
		}
	})

	t.Run("empty result for session without TILs", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "lister@test.com", "Lister")
		sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-empty-1")

		tils, err := store.ListForSession(context.Background(), sessionID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tils) != 0 {
			t.Errorf("expected empty result, got %d TILs", len(tils))
		}
	})
}

func TestList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	env := testutil.SetupTestEnvironment(t)

	t.Run("returns own TILs with session context", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-own-1", testutil.TestSessionFullOpts{
			Summary: "Test Session",
			RepoURL: "https://github.com/org/repo.git",
			Branch:  "main",
		})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "My TIL", "My Summary", nil)

		result, err := store.List(context.Background(), user.ID, ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL, got %d", len(result.TILs))
		}

		til := result.TILs[0]
		if til.Title != "My TIL" {
			t.Errorf("expected title 'My TIL', got %q", til.Title)
		}
		if til.SessionTitle == nil || *til.SessionTitle != "Test Session" {
			t.Errorf("expected session title 'Test Session', got %v", til.SessionTitle)
		}
		if til.GitRepo == nil || *til.GitRepo != "org/repo" {
			t.Errorf("expected git repo 'org/repo', got %v", til.GitRepo)
		}
		if !til.IsOwner {
			t.Error("expected is_owner to be true")
		}
		if til.AccessType != "owner" {
			t.Errorf("expected access_type 'owner', got %q", til.AccessType)
		}
	})

	t.Run("search filter", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "search@test.com", "Searcher")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-search-1", testutil.TestSessionFullOpts{Summary: "s"})

		testutil.CreateTestTIL(t, env, user.ID, sessionID, "Go channels are cool", "Learned about goroutines", nil)
		testutil.CreateTestTIL(t, env, user.ID, sessionID, "Python decorators", "Learned about decorators", nil)

		result, err := store.List(context.Background(), user.ID, ListParams{Query: "channels"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 1 {
			t.Errorf("expected 1 result for 'channels' search, got %d", len(result.TILs))
		}
	})

	t.Run("repo filter", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "repo@test.com", "RepoUser")
		session1 := testutil.CreateTestSessionFull(t, env, user.ID, "ext-repo-1", testutil.TestSessionFullOpts{
			Summary: "s1", RepoURL: "https://github.com/org/alpha.git",
		})
		session2 := testutil.CreateTestSessionFull(t, env, user.ID, "ext-repo-2", testutil.TestSessionFullOpts{
			Summary: "s2", RepoURL: "https://github.com/org/beta.git",
		})

		testutil.CreateTestTIL(t, env, user.ID, session1, "Alpha TIL", "alpha", nil)
		testutil.CreateTestTIL(t, env, user.ID, session2, "Beta TIL", "beta", nil)

		result, err := store.List(context.Background(), user.ID, ListParams{Repos: []string{"org/alpha"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 TIL for repo filter, got %d", len(result.TILs))
		}
		if result.TILs[0].Title != "Alpha TIL" {
			t.Errorf("expected 'Alpha TIL', got %q", result.TILs[0].Title)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "page@test.com", "Pager")
		sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "ext-page-1", testutil.TestSessionFullOpts{Summary: "s"})

		for i := 0; i < 5; i++ {
			testutil.CreateTestTIL(t, env, user.ID, sessionID, "TIL "+string(rune('A'+i)), "summary", nil)
		}

		// Page 1 (size 2)
		result, err := store.List(context.Background(), user.ID, ListParams{PageSize: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 2 {
			t.Fatalf("expected 2 TILs on page 1, got %d", len(result.TILs))
		}
		if !result.HasMore {
			t.Error("expected has_more to be true")
		}
		if result.NextCursor == "" {
			t.Error("expected non-empty next_cursor")
		}

		// Page 2
		result2, err := store.List(context.Background(), user.ID, ListParams{PageSize: 2, Cursor: result.NextCursor})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result2.TILs) != 2 {
			t.Fatalf("expected 2 TILs on page 2, got %d", len(result2.TILs))
		}
		if !result2.HasMore {
			t.Error("expected has_more to be true on page 2")
		}

		// Page 3 (last)
		result3, err := store.List(context.Background(), user.ID, ListParams{PageSize: 2, Cursor: result2.NextCursor})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result3.TILs) != 1 {
			t.Fatalf("expected 1 TIL on page 3, got %d", len(result3.TILs))
		}
		if result3.HasMore {
			t.Error("expected has_more to be false on last page")
		}
	})

	t.Run("filter options reflect TILs sessions", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		user := testutil.CreateTestUser(t, env, "opts@test.com", "Opts")
		session1 := testutil.CreateTestSessionFull(t, env, user.ID, "ext-opts-1", testutil.TestSessionFullOpts{
			Summary: "s1", RepoURL: "https://github.com/org/repo-a.git", Branch: "main",
		})
		session2 := testutil.CreateTestSessionFull(t, env, user.ID, "ext-opts-2", testutil.TestSessionFullOpts{
			Summary: "s2", RepoURL: "https://github.com/org/repo-b.git", Branch: "dev",
		})
		// Session without TILs — should not appear in filter options
		testutil.CreateTestSessionFull(t, env, user.ID, "ext-opts-3", testutil.TestSessionFullOpts{
			Summary: "s3", RepoURL: "https://github.com/org/repo-c.git", Branch: "staging",
		})

		testutil.CreateTestTIL(t, env, user.ID, session1, "TIL A", "a", nil)
		testutil.CreateTestTIL(t, env, user.ID, session2, "TIL B", "b", nil)

		result, err := store.List(context.Background(), user.ID, ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		fo := result.FilterOptions
		if len(fo.Repos) != 2 {
			t.Errorf("expected 2 repos in filter options, got %d: %v", len(fo.Repos), fo.Repos)
		}
		if len(fo.Branches) != 2 {
			t.Errorf("expected 2 branches in filter options, got %d: %v", len(fo.Branches), fo.Branches)
		}
	})

	t.Run("shared session TILs visible", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-share-1", testutil.TestSessionFullOpts{Summary: "shared session"})

		testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Shared TIL", "visible via share", nil)

		// Create share to viewer
		testutil.CreateTestShare(t, env, sessionID, false, nil, []string{"viewer@test.com"})

		result, err := store.List(context.Background(), viewer.ID, ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 1 {
			t.Fatalf("expected 1 shared TIL, got %d", len(result.TILs))
		}
		if result.TILs[0].IsOwner {
			t.Error("expected is_owner to be false for shared TIL")
		}
		if result.TILs[0].AccessType != "private_share" {
			t.Errorf("expected access_type 'private_share', got %q", result.TILs[0].AccessType)
		}
	})

	t.Run("expired share excludes TILs", func(t *testing.T) {
		env.CleanDB(t)
		store := &Store{DB: env.DB}

		owner := testutil.CreateTestUser(t, env, "owner@test.com", "Owner")
		viewer := testutil.CreateTestUser(t, env, "viewer@test.com", "Viewer")
		sessionID := testutil.CreateTestSessionFull(t, env, owner.ID, "ext-expired-1", testutil.TestSessionFullOpts{Summary: "expired share"})

		testutil.CreateTestTIL(t, env, owner.ID, sessionID, "Expired Share TIL", "should not see", nil)

		// Create share with expiry in the past
		pastTime := time.Now().Add(-24 * time.Hour)
		testutil.CreateTestShare(t, env, sessionID, false, &pastTime, []string{"viewer@test.com"})

		result, err := store.List(context.Background(), viewer.ID, ListParams{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.TILs) != 0 {
			t.Errorf("expected 0 TILs for expired share, got %d", len(result.TILs))
		}
	})
}
