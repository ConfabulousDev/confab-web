package github_test

import (
	"context"
	"testing"

	dbgithub "github.com/ConfabulousDev/confab-web/internal/db/github"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-491 — the manual API path (HandleCreateGitHubLink → CreateGitHubLink)
// must NOT trigger the fork→root resolver. Per the Phase 3a decision, the
// resolver lives in the sync chunk handler. CreateGitHubLink stays a pure
// CRUD function. Future sync chunks on the same session pick up the
// mapping; the manual path is best-effort and intentionally skips the
// inference write.
func TestCreateGitHubLink_ManualPRWithMismatch_DoesNotWriteRootName(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	defer env.Cleanup(t)
	store := &dbgithub.Store{DB: env.DB}

	user := testutil.CreateTestUser(t, env, "manual-pr@test.com", "Manual PR")
	sessionID := testutil.CreateTestSessionFull(t, env, user.ID, "manual-pr-session", testutil.TestSessionFullOpts{
		RepoURL: "https://github.com/jackie/confab-web.git",
		Branch:  "main",
		Summary: "Fork session w/ manual PR link",
	})

	// Seed session_repos for the fork. The manual CreateGitHubLink we are
	// about to call must leave root_name NULL on this row.
	if _, err := env.DB.Conn().ExecContext(env.Ctx,
		`INSERT INTO session_repos (repo_name) VALUES ($1) ON CONFLICT DO NOTHING`,
		"jackie/confab-web"); err != nil {
		t.Fatalf("seed session_repos: %v", err)
	}

	// Manually create a PR link whose owner/repo differs from the session's
	// extracted repo. In the sync path this would trigger the resolver; in
	// the manual path it must not.
	title := "PR Title"
	_, err := store.CreateGitHubLink(context.Background(), &models.GitHubLink{
		SessionID: sessionID,
		LinkType:  models.GitHubLinkTypePullRequest,
		URL:       "https://github.com/ConfabulousDev/confab-web/pull/99",
		Owner:     "ConfabulousDev",
		Repo:      "confab-web",
		Ref:       "99",
		Title:     &title,
		Source:    models.GitHubLinkSourceManual,
	}, true)
	if err != nil {
		t.Fatalf("CreateGitHubLink: %v", err)
	}

	var rootName *string
	err = env.DB.Conn().QueryRowContext(env.Ctx,
		`SELECT root_name FROM session_repos WHERE repo_name = $1`,
		"jackie/confab-web").Scan(&rootName)
	if err != nil {
		t.Fatalf("read session_repos: %v", err)
	}
	if rootName != nil {
		t.Errorf("manual CreateGitHubLink must not populate root_name, got %q", *rootName)
	}
}
