package til

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-491 — TILs page repo filter must collapse forks the same way the
// Sessions page does. Mapping lives on session_repos.root_name.

// seedTILForkRootMapping mirrors seedForkRootMapping (db/session) and
// seedOrgForkRootMapping (api) — the three live in different packages so
// they can't share trivially.
func seedTILForkRootMapping(t *testing.T, env *testutil.TestEnvironment, fork, root string) {
	t.Helper()
	for _, name := range []string{fork, root} {
		if _, err := env.DB.Conn().ExecContext(env.Ctx,
			`INSERT INTO session_repos (repo_name) VALUES ($1) ON CONFLICT DO NOTHING`,
			name); err != nil {
			t.Fatalf("seed session_repos(%s): %v", name, err)
		}
	}
	if _, err := env.DB.Conn().ExecContext(env.Ctx,
		`UPDATE session_repos SET root_name = $2, root_source = 'pr_inference'
		   WHERE repo_name = $1 AND root_name IS NULL`,
		fork, root); err != nil {
		t.Fatalf("seed mapping %s->%s: %v", fork, root, err)
	}
}

// TestTILRepoRoots_FilterListCollapsesForks verifies the TILs filter list
// returns one chip when a fork and its upstream root both have TILs.
func TestTILRepoRoots_FilterListCollapsesForks(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "til-rr@test.com", "TIL RR")

	forkSession := testutil.CreateTestSessionFull(t, env, user.ID, "til-rr-fork", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork",
	})
	upstreamSession := testutil.CreateTestSessionFull(t, env, user.ID, "til-rr-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream",
	})
	testutil.CreateTestTIL(t, env, user.ID, forkSession, "Fork TIL", "fork-summary", nil)
	testutil.CreateTestTIL(t, env, user.ID, upstreamSession, "Upstream TIL", "upstream-summary", nil)

	seedTILForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	result, err := store.List(context.Background(), user.ID, ListParams{PageSize: 50})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(result.FilterOptions.Repos) != 1 {
		t.Fatalf("expected 1 collapsed chip, got %d: %+v",
			len(result.FilterOptions.Repos), result.FilterOptions.Repos)
	}
	if result.FilterOptions.Repos[0] != "ConfabulousDev/confab-web" {
		t.Errorf("expected chip = 'ConfabulousDev/confab-web', got %q",
			result.FilterOptions.Repos[0])
	}
}

// TestTILRepoRoots_FilterMatchIncludesForkSessions verifies that filtering
// TILs by the upstream root returns TILs from both the fork and upstream
// sessions.
func TestTILRepoRoots_FilterMatchIncludesForkSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "til-rr-m@test.com", "TIL RR Match")

	forkSession := testutil.CreateTestSessionFull(t, env, user.ID, "tilm-fork", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork",
	})
	upstreamSession := testutil.CreateTestSessionFull(t, env, user.ID, "tilm-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream",
	})
	unrelatedSession := testutil.CreateTestSessionFull(t, env, user.ID, "tilm-other", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/other/repo.git",
		Branch:  "main",
		Summary: "Other",
	})
	testutil.CreateTestTIL(t, env, user.ID, forkSession, "Fork TIL", "summary", nil)
	testutil.CreateTestTIL(t, env, user.ID, upstreamSession, "Upstream TIL", "summary", nil)
	testutil.CreateTestTIL(t, env, user.ID, unrelatedSession, "Unrelated TIL", "summary", nil)

	seedTILForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	result, err := store.List(context.Background(), user.ID, ListParams{
		Repos:    []string{"ConfabulousDev/confab-web"},
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(result.TILs) != 2 {
		t.Fatalf("expected 2 TILs (fork + upstream), got %d", len(result.TILs))
	}
	titles := map[string]bool{}
	for _, t := range result.TILs {
		titles[t.Title] = true
	}
	if !titles["Fork TIL"] || !titles["Upstream TIL"] {
		t.Errorf("expected TILs from both fork and upstream, got %v", titles)
	}
	if titles["Unrelated TIL"] {
		t.Error("filter by upstream root incorrectly returned an unrelated TIL")
	}
}

