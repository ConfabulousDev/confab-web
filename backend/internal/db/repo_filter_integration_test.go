package db_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// CF-510 — db.RepoRootExpr resolves a session's fork→upstream root live from
// its own git_info (repo_url + remotes + tracking_remote), with no
// session_repos lookup. This test pins the resolution contract directly
// against the generated SQL, porting the case table from the retired
// git_remote_resolver_test.go and adding CF-509 trailing-slash cases.
//
// "Resolve" means: when tracking_remote names a remote whose URL extracts to a
// different owner/repo than repo_url, RepoRootExpr returns that upstream;
// otherwise it returns the repo_url's own owner/repo (the fork).
func TestRepoRootExpr_ResolvesUpstreamFromGitInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "rre@test.com", "RRE")

	query := fmt.Sprintf("SELECT %s FROM sessions s WHERE s.id = $1", db.RepoRootExpr("s"))

	cases := []struct {
		name    string
		gitInfo string
		want    string
	}{
		{
			"canonical origin fork, upstream tracked",
			`{"repo_url":"git@github.com:jackie/confab-web.git","remotes":[{"name":"origin","fetch_url":"git@github.com:jackie/confab-web.git"},{"name":"upstream","fetch_url":"https://github.com/ConfabulousDev/confab-web.git"}],"tracking_remote":"upstream"}`,
			"ConfabulousDev/confab-web",
		},
		{
			"non-standard tracking remote name",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"},{"name":"canonical","fetch_url":"https://github.com/them/repo.git"}],"tracking_remote":"canonical"}`,
			"them/repo",
		},
		{
			"ssh fork, https upstream",
			`{"repo_url":"git@github.com:me/repo.git","remotes":[{"name":"origin","fetch_url":"git@github.com:me/repo.git"},{"name":"upstream","fetch_url":"https://github.com/them/repo.git"}],"tracking_remote":"upstream"}`,
			"them/repo",
		},
		{
			"duplicate remote names: first wins",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"},{"name":"upstream","fetch_url":"https://github.com/them/repo.git"},{"name":"upstream","fetch_url":"https://github.com/other/repo.git"}],"tracking_remote":"upstream"}`,
			"them/repo",
		},
		{
			"fetch_url empty falls back to push_url",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"upstream","fetch_url":"","push_url":"https://github.com/them/repo.git"}],"tracking_remote":"upstream"}`,
			"them/repo",
		},
		// --- boundary / no-collapse cases (fall through to the fork) ---
		{
			"no tracking_remote: fork passes through",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"}]}`,
			"me/repo",
		},
		{
			"self-loop: tracking points at the fork",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"}],"tracking_remote":"origin"}`,
			"me/repo",
		},
		{
			"self-loop across casings",
			`{"repo_url":"https://github.com/jackie/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/jackie/repo.git"},{"name":"upstream","fetch_url":"https://github.com/Jackie/repo.git"}],"tracking_remote":"upstream"}`,
			"jackie/repo",
		},
		{
			"tracking remote URL not extractable",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"},{"name":"upstream","fetch_url":""}],"tracking_remote":"upstream"}`,
			"me/repo",
		},
		{
			"tracking names unknown remote",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"origin","fetch_url":"https://github.com/me/repo.git"}],"tracking_remote":"nonexistent"}`,
			"me/repo",
		},
		{
			"tracking name is case-sensitive",
			`{"repo_url":"https://github.com/me/repo.git","remotes":[{"name":"upstream","fetch_url":"https://github.com/them/repo.git"}],"tracking_remote":"Upstream"}`,
			"me/repo",
		},
		{
			"remotes missing entirely",
			`{"repo_url":"https://github.com/me/repo.git","tracking_remote":"upstream"}`,
			"me/repo",
		},
		{
			"remotes is not an array",
			`{"repo_url":"https://github.com/me/repo.git","remotes":"oops","tracking_remote":"upstream"}`,
			"me/repo",
		},
		{
			"empty repo_url: no spurious collapse to upstream",
			`{"repo_url":"","remotes":[{"name":"upstream","fetch_url":"https://github.com/them/repo.git"}],"tracking_remote":"upstream"}`,
			"",
		},
		// --- CF-509 trailing-slash extraction (folded into CF-510) ---
		{
			"CF-509: trailing slash on repo_url (fork)",
			`{"repo_url":"https://github.com/jackie/confab-web/"}`,
			"jackie/confab-web",
		},
		{
			"CF-509: trailing slash on both fork and upstream URLs",
			`{"repo_url":"https://github.com/me/repo/","remotes":[{"name":"upstream","fetch_url":"https://github.com/them/repo/"}],"tracking_remote":"upstream"}`,
			"them/repo",
		},
		{
			"CF-509: .git plus trailing slash",
			`{"repo_url":"https://github.com/solo/standalone.git/"}`,
			"solo/standalone",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id := uuid.New().String()
			if _, err := env.DB.Exec(env.Ctx,
				`INSERT INTO sessions (id, user_id, external_id, first_seen, git_info, last_message_at)
				 VALUES ($1, $2, $3, NOW(), $4, NOW())`,
				id, user.ID, c.name, c.gitInfo); err != nil {
				t.Fatalf("insert session: %v", err)
			}
			var got string
			if err := env.DB.Conn().QueryRowContext(env.Ctx, query, id).Scan(&got); err != nil {
				t.Fatalf("query RepoRootExpr: %v", err)
			}
			if got != c.want {
				t.Errorf("RepoRootExpr = %q, want %q", got, c.want)
			}
		})
	}
}
