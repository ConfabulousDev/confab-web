package til

import (
	"context"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-510 — TILs page repo filter must collapse forks the same way the Sessions
// page does. The upstream is resolved live by db.RepoRootExpr from each fork
// session's own git_info (remotes + tracking_remote).

// forkSessionOpts builds CreateTestSessionFull options for a fork checkout
// whose tracking_remote points at upstreamURL.
func forkSessionOpts(forkURL, upstreamURL, summary string) testutil.TestSessionFullOpts {
	return testutil.TestSessionFullOpts{
		RepoURL: forkURL,
		Branch:  "main",
		Summary: summary,
		Remotes: []testutil.TestGitRemote{
			{Name: "origin", FetchURL: forkURL},
			{Name: "upstream", FetchURL: upstreamURL},
		},
		TrackingRemote: "upstream",
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

	forkSession := testutil.CreateTestSessionFull(t, env, user.ID, "til-rr-fork",
		forkSessionOpts("https://github.com/jackie/confab-web.git", "https://github.com/ConfabulousDev/confab-web.git", "Fork"))
	upstreamSession := testutil.CreateTestSessionFull(t, env, user.ID, "til-rr-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream",
	})
	testutil.CreateTestTIL(t, env, user.ID, forkSession, "Fork TIL", "fork-summary", nil)
	testutil.CreateTestTIL(t, env, user.ID, upstreamSession, "Upstream TIL", "upstream-summary", nil)

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

	forkSession := testutil.CreateTestSessionFull(t, env, user.ID, "tilm-fork",
		forkSessionOpts("https://github.com/jackie/confab-web.git", "https://github.com/ConfabulousDev/confab-web.git", "Fork"))
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
