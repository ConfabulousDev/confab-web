package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	dbgithub "github.com/ConfabulousDev/confab-web/internal/db/github"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// setupGitHubStore creates a fresh test environment with a session ready for
// linking. Returns the store, owning session ID, and the env (for cleanup).
func setupGitHubStore(t *testing.T) (*dbgithub.Store, string, *testutil.TestEnvironment) {
	t.Helper()
	env := testutil.SetupTestEnvironment(t)
	user := testutil.CreateTestUser(t, env, "github@example.com", "GitHub User")
	sessionID := testutil.CreateTestSession(t, env, user.ID, "ext-github")
	return &dbgithub.Store{DB: env.DB}, sessionID, env
}

func strPtr(s string) *string { return &s }

func TestCreateGitHubLink_InsertsNew(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	got, err := store.CreateGitHubLink(context.Background(), &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypePullRequest,
		URL:       "https://github.com/foo/bar/pull/42",
		Owner:     "foo",
		Repo:      "bar",
		Ref:       "42",
		Title:     strPtr("first version"),
		Source:    models.GitHubLinkSourceManual,
	}, true)
	if err != nil {
		t.Fatalf("CreateGitHubLink: %v", err)
	}
	if got.ID == 0 {
		t.Error("expected non-zero ID after insert")
	}
	if got.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt after insert")
	}
	if got.URL != "https://github.com/foo/bar/pull/42" {
		t.Errorf("URL = %q", got.URL)
	}
}

// TestCreateGitHubLink_NonexistentSessionFK exercises the error branch when
// the foreign key on session_id is violated.
func TestCreateGitHubLink_NonexistentSessionFK(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	store := &dbgithub.Store{DB: env.DB}
	_, err := store.CreateGitHubLink(context.Background(), &models.GitHubLink{
		// Valid UUID format but no matching session row.
		SessionID: "00000000-0000-0000-0000-000000000000",
		LinkType:  models.GitHubLinkTypePullRequest,
		URL:       "https://github.com/o/r/pull/1",
		Owner:     "o",
		Repo:      "r",
		Ref:       "1",
		Source:    models.GitHubLinkSourceManual,
	}, true)
	if err == nil {
		t.Fatal("expected FK violation for nonexistent session_id")
	}
}

func TestCreateGitHubLink_UpsertUpdatesSourceAndURL(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	got1, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypeCommit,
		URL:       "https://github.com/o/r/commit/aaa",
		Owner:     "o",
		Repo:      "r",
		Ref:       "aaaaaaa",
		Title:     strPtr("commit msg"),
		Source:    models.GitHubLinkSourceCLIHook,
	}, true)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second call: same unique key, different source + URL.
	got2, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypeCommit,
		URL:       "https://github.com/o/r/commit/aaaaaaaaaaaaaaaa", // longer SHA URL
		Owner:     "o",
		Repo:      "r",
		Ref:       "aaaaaaa",
		Title:     strPtr("commit msg v2"),
		Source:    models.GitHubLinkSourceTranscript,
	}, true)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got2.ID != got1.ID {
		t.Errorf("upsert should preserve ID: first=%d, second=%d", got1.ID, got2.ID)
	}

	fetched, err := store.GetGitHubLinkByID(ctx, got1.ID)
	if err != nil {
		t.Fatalf("GetGitHubLinkByID: %v", err)
	}
	if fetched.Source != models.GitHubLinkSourceTranscript {
		t.Errorf("source = %q, want transcript", fetched.Source)
	}
	if fetched.URL != "https://github.com/o/r/commit/aaaaaaaaaaaaaaaa" {
		t.Errorf("URL not updated: %q", fetched.URL)
	}
}

// TestCreateGitHubLink_OverwriteTitle covers the three branches of the
// CASE / COALESCE expression that controls title updates on upsert. The
// subtests use distinct Refs so each writes its own row in the shared session.
func TestCreateGitHubLink_OverwriteTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	cases := []struct {
		name           string
		ref            string
		initialTitle   *string
		overwriteTitle bool
		secondTitle    *string
		wantTitle      string
	}{
		{
			name:           "overwrite_true_replaces_existing",
			ref:            "1",
			initialTitle:   strPtr("original title"),
			overwriteTitle: true,
			secondTitle:    strPtr("replaced title"),
			wantTitle:      "replaced title",
		},
		{
			name:           "overwrite_false_preserves_existing",
			ref:            "2",
			initialTitle:   strPtr("user-set title"),
			overwriteTitle: false,
			secondTitle:    strPtr("enricher would set this"),
			wantTitle:      "user-set title",
		},
		{
			name:           "overwrite_false_fills_null",
			ref:            "3",
			initialTitle:   nil, // existing title is NULL → fill-only still populates
			overwriteTitle: false,
			secondTitle:    strPtr("background-enriched"),
			wantTitle:      "background-enriched",
		},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			first, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
				SessionID: sessionID,
				LinkType:  models.GitHubLinkTypePullRequest,
				URL:       "https://github.com/o/r/pull/" + tc.ref,
				Owner:     "o",
				Repo:      "r",
				Ref:       tc.ref,
				Title:     tc.initialTitle,
				Source:    models.GitHubLinkSourceManual,
			}, true)
			if err != nil {
				t.Fatalf("first insert: %v", err)
			}

			second := *first
			second.Title = tc.secondTitle
			if _, err := store.CreateGitHubLink(ctx, &second, tc.overwriteTitle); err != nil {
				t.Fatalf("upsert with overwriteTitle=%v: %v", tc.overwriteTitle, err)
			}

			got, err := store.GetGitHubLinkByID(ctx, first.ID)
			if err != nil {
				t.Fatalf("GetGitHubLinkByID: %v", err)
			}
			if got.Title == nil || *got.Title != tc.wantTitle {
				t.Errorf("title = %v, want %q", got.Title, tc.wantTitle)
			}
		})
	}
}

func TestGetGitHubLinksForSession_OrderedByCreatedAtDesc(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	for i, ref := range []string{"10", "20", "30"} {
		_, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  models.GitHubLinkTypePullRequest,
			URL:       "https://github.com/o/r/pull/" + ref,
			Owner:     "o",
			Repo:      "r",
			Ref:       ref,
			Source:    models.GitHubLinkSourceManual,
		}, true)
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
		// Ensure created_at differs enough for ordering to be deterministic.
		time.Sleep(2 * time.Millisecond)
	}

	got, err := store.GetGitHubLinksForSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetGitHubLinksForSession: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 links, got %d", len(got))
	}
	if got[0].Ref != "30" || got[1].Ref != "20" || got[2].Ref != "10" {
		t.Errorf("order = [%s, %s, %s], want [30, 20, 10]", got[0].Ref, got[1].Ref, got[2].Ref)
	}
}

func TestGetGitHubLinksForSession_EmptyResult(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	got, err := store.GetGitHubLinksForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("GetGitHubLinksForSession: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 links for session with none, got %d", len(got))
	}
}

func TestGetGitHubLinksForSession_InvalidUUIDReturnsErrSessionNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	store := &dbgithub.Store{DB: env.DB}
	_, err := store.GetGitHubLinksForSession(context.Background(), "not-a-valid-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
	if !errors.Is(err, db.ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestGetGitHubLinkByID_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	link, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypePullRequest,
		URL:       "https://github.com/o/r/pull/5",
		Owner:     "o",
		Repo:      "r",
		Ref:       "5",
		Title:     strPtr("hi"),
		Source:    models.GitHubLinkSourceManual,
	}, true)
	if err != nil {
		t.Fatalf("CreateGitHubLink: %v", err)
	}

	got, err := store.GetGitHubLinkByID(ctx, link.ID)
	if err != nil {
		t.Fatalf("GetGitHubLinkByID: %v", err)
	}
	if got.ID != link.ID {
		t.Errorf("ID = %d, want %d", got.ID, link.ID)
	}
	if got.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, sessionID)
	}
}

func TestDeleteGitHubLink_HappyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	store, sessionID, env := setupGitHubStore(t)
	defer env.Cleanup(t)

	ctx := context.Background()
	link, err := store.CreateGitHubLink(ctx, &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypePullRequest,
		URL:       "https://github.com/o/r/pull/9",
		Owner:     "o",
		Repo:      "r",
		Ref:       "9",
		Source:    models.GitHubLinkSourceManual,
	}, true)
	if err != nil {
		t.Fatalf("CreateGitHubLink: %v", err)
	}

	if err := store.DeleteGitHubLink(ctx, link.ID); err != nil {
		t.Fatalf("DeleteGitHubLink: %v", err)
	}

	if _, err := store.GetGitHubLinkByID(ctx, link.ID); !errors.Is(err, db.ErrGitHubLinkNotFound) {
		t.Errorf("after delete, expected ErrGitHubLinkNotFound, got %v", err)
	}
}

// TestGitHubLinkByID_NotFound covers the ErrGitHubLinkNotFound branch on both
// the Get and Delete paths, since they hit the same sentinel via different
// code paths (sql.ErrNoRows vs zero RowsAffected).
func TestGitHubLinkByID_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)

	store := &dbgithub.Store{DB: env.DB}
	ctx := context.Background()
	const missingID int64 = 99999

	t.Run("get_returns_not_found", func(t *testing.T) {
		_, err := store.GetGitHubLinkByID(ctx, missingID)
		if !errors.Is(err, db.ErrGitHubLinkNotFound) {
			t.Errorf("expected ErrGitHubLinkNotFound, got %v", err)
		}
	})

	t.Run("delete_returns_not_found", func(t *testing.T) {
		err := store.DeleteGitHubLink(ctx, missingID)
		if !errors.Is(err, db.ErrGitHubLinkNotFound) {
			t.Errorf("expected ErrGitHubLinkNotFound, got %v", err)
		}
	})
}
