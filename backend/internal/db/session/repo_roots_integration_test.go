package session_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/db"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-491 — Sessions repo filter must collapse forks into their upstream root.
// Mapping lives on session_repos.root_name (NULL for non-forks; populated for
// observed forks via the PR-inference resolver).

// seedForkRootMapping stamps both the fork and upstream-root rows in
// session_repos and links the fork to the root. Lets these tests assert
// behavior without depending on the production resolver path, which is
// exercised by sync_repo_roots_integration_test.go. Mirrors
// seedTILForkRootMapping (db/til) and seedOrgForkRootMapping (api).
func seedForkRootMapping(t *testing.T, env *testutil.TestEnvironment, fork, root string) {
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

// TestRepoRoots_FilterListGlobal_CollapsesForks verifies that the global
// (ShareAllSessions=true) filter list returns a single chip when a fork and
// its upstream root are both seen.
func TestRepoRoots_FilterListGlobal_CollapsesForks(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbsession.Store{DB: env.DB}

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "rr-global@test.com", "RR Global")

	// Fork session
	testutil.CreateTestSessionFull(t, env, user.ID, "rr-fork", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork work",
	})
	// Upstream session
	testutil.CreateTestSessionFull(t, env, user.ID, "rr-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream work",
	})

	seedForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	result, err := store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated: %v", err)
	}

	if len(result.FilterOptions.Repos) != 1 {
		t.Fatalf("expected 1 collapsed repo chip, got %d: %+v",
			len(result.FilterOptions.Repos), result.FilterOptions.Repos)
	}
	if result.FilterOptions.Repos[0] != "ConfabulousDev/confab-web" {
		t.Errorf("expected chip = 'ConfabulousDev/confab-web', got %q",
			result.FilterOptions.Repos[0])
	}
}

// TestRepoRoots_FilterListScoped_CollapsesForks verifies the scoped
// (ShareAllSessions=false) filter list path also collapses.
func TestRepoRoots_FilterListScoped_CollapsesForks(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbsession.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "rr-scoped@test.com", "RR Scoped")

	testutil.CreateTestSessionFull(t, env, user.ID, "rrs-fork", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork",
	})
	testutil.CreateTestSessionFull(t, env, user.ID, "rrs-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream",
	})

	seedForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	result, err := store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated: %v", err)
	}

	if len(result.FilterOptions.Repos) != 1 {
		t.Fatalf("expected 1 collapsed repo chip, got %d: %+v",
			len(result.FilterOptions.Repos), result.FilterOptions.Repos)
	}
	if result.FilterOptions.Repos[0] != "ConfabulousDev/confab-web" {
		t.Errorf("expected chip = 'ConfabulousDev/confab-web', got %q",
			result.FilterOptions.Repos[0])
	}
}

// TestRepoRoots_FilterMatch_IncludesForkSessions verifies that filtering by
// the upstream root returns both the upstream session and any fork sessions
// mapped to that root.
func TestRepoRoots_FilterMatch_IncludesForkSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbsession.Store{DB: env.DB}

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "rr-match@test.com", "RR Match")

	testutil.CreateTestSessionFull(t, env, user.ID, "rrm-fork", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork session",
	})
	testutil.CreateTestSessionFull(t, env, user.ID, "rrm-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream session",
	})
	// Unrelated session — must NOT appear when filtering by the upstream root.
	testutil.CreateTestSessionFull(t, env, user.ID, "rrm-other", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/other-org/other-repo.git",
		Branch:  "main",
		Summary: "Other session",
	})

	seedForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	result, err := store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{
		Repos:    []string{"ConfabulousDev/confab-web"},
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated: %v", err)
	}

	if len(result.Sessions) != 2 {
		t.Fatalf("expected 2 sessions (fork + upstream), got %d", len(result.Sessions))
	}
	seen := map[string]bool{}
	for _, s := range result.Sessions {
		seen[s.ExternalID] = true
	}
	if !seen["rrm-fork"] {
		t.Error("filter by upstream root did not return the fork session")
	}
	if !seen["rrm-upstream"] {
		t.Error("filter by upstream root did not return the upstream session")
	}
	if seen["rrm-other"] {
		t.Error("filter by upstream root incorrectly returned an unrelated session")
	}
}

// TestRepoRoots_ThreeForksOneUpstream verifies dedupe + match across multiple
// forks of the same root.
func TestRepoRoots_ThreeForksOneUpstream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbsession.Store{DB: env.DB}

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "rr-multi@test.com", "RR Multi")

	forks := []string{"alice", "bob", "carol"}
	for _, owner := range forks {
		testutil.CreateTestSessionFull(t, env, user.ID, fmt.Sprintf("rrm3-%s", owner), testutil.TestSessionFullOpts{
			RepoURL: fmt.Sprintf("https://github.com/%s/confab-web.git", owner),
			Branch:  "main",
			Summary: owner + " fork",
		})
		seedForkRootMapping(t, env, fmt.Sprintf("%s/confab-web", owner), "ConfabulousDev/confab-web")
	}
	testutil.CreateTestSessionFull(t, env, user.ID, "rrm3-upstream", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/ConfabulousDev/confab-web.git",
		Branch:  "main",
		Summary: "Upstream",
	})

	// Filter list returns a single chip.
	result, err := store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated: %v", err)
	}
	if len(result.FilterOptions.Repos) != 1 {
		t.Fatalf("expected 1 chip across 3 forks + upstream, got %d: %+v",
			len(result.FilterOptions.Repos), result.FilterOptions.Repos)
	}

	// Filtering by the root returns all four sessions.
	result, err = store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{
		Repos:    []string{"ConfabulousDev/confab-web"},
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated (filtered): %v", err)
	}
	if len(result.Sessions) != 4 {
		t.Fatalf("expected 4 sessions (3 forks + upstream), got %d", len(result.Sessions))
	}
}

// TestRepoRoots_NonFork_PassesThrough verifies a repo with NULL root_name
// appears under its raw name in the chip list and matches its own sessions.
func TestRepoRoots_NonFork_PassesThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	store := &dbsession.Store{DB: env.DB}

	env.DB.ShareAllSessions = true
	defer func() { env.DB.ShareAllSessions = false }()

	user := testutil.CreateTestUser(t, env, "rr-nofork@test.com", "RR NoFork")

	testutil.CreateTestSessionFull(t, env, user.ID, "rrn-standalone", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/solo/standalone.git",
		Branch:  "main",
		Summary: "Standalone repo",
	})

	// No seedForkRootMapping call — this repo has no known root.

	result, err := store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated: %v", err)
	}
	if len(result.FilterOptions.Repos) != 1 || result.FilterOptions.Repos[0] != "solo/standalone" {
		t.Fatalf("expected chip = ['solo/standalone'], got %+v", result.FilterOptions.Repos)
	}

	result, err = store.ListUserSessionsPaginated(context.Background(), user.ID, db.SessionListParams{
		Repos:    []string{"solo/standalone"},
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("ListUserSessionsPaginated (filtered): %v", err)
	}
	if len(result.Sessions) != 1 {
		t.Fatalf("expected 1 session for standalone repo, got %d", len(result.Sessions))
	}
}
