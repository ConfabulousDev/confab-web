package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
)

// CF-491 — Org analytics surfaces (org repo list and org analytics
// per-user metrics) collapse forks into their upstream root, consistent with
// Sessions and TILs filter behavior.

// seedOrgForkRootMapping inserts the session_repos rows and stamps the
// fork→root mapping directly. Production writes go through the sync
// resolver; these tests bypass that path to focus on the read-side
// COALESCE behavior. Mirrors seedForkRootMapping (db/session) and
// seedTILForkRootMapping (db/til).
func seedOrgForkRootMapping(t *testing.T, env *testutil.TestEnvironment, fork, root string) {
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

// TestOrgRepos_HTTP_CollapsesForks verifies /api/v1/org/repos returns the
// upstream root rather than the fork when a mapping exists.
func TestOrgRepos_HTTP_CollapsesForks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

	alice := testutil.CreateTestUser(t, env, "alice-org-rr@test.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-org-rr@test.com", "Bob")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, alice.ID)

	testutil.CreateTestSessionWithGitInfo(t, env, alice.ID, "alice-fork",
		"https://github.com/jackie/confab-web.git")
	testutil.CreateTestSessionWithGitInfo(t, env, bob.ID, "bob-upstream",
		"https://github.com/ConfabulousDev/confab-web.git")

	seedOrgForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	now := time.Now().UTC()
	resp, err := client.Get(fmt.Sprintf("/api/v1/org/repos?start_ts=%d&end_ts=%d&tz_offset=0",
		now.Add(-7*24*time.Hour).Unix(), now.Add(24*time.Hour).Unix()))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	var result OrgReposResponse
	testutil.ParseJSON(t, resp, &result)

	if len(result.Repos) != 1 {
		t.Fatalf("expected 1 collapsed repo, got %d: %+v", len(result.Repos), result.Repos)
	}
	if result.Repos[0] != "ConfabulousDev/confab-web" {
		t.Errorf("expected repo = 'ConfabulousDev/confab-web', got %q", result.Repos[0])
	}
}

// TestOrgAnalytics_HTTP_FilterByRoot_IncludesForkSessions verifies that the
// per-user org analytics endpoint, when filtered by the upstream root,
// counts sessions whose git_repo_url is the fork. Counts the most basic
// observable metric: number of users with at least one session in the
// filtered result.
func TestOrgAnalytics_HTTP_FilterByRoot_IncludesForkSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

	alice := testutil.CreateTestUser(t, env, "alice-oa-rr@test.com", "Alice")
	bob := testutil.CreateTestUser(t, env, "bob-oa-rr@test.com", "Bob")
	carol := testutil.CreateTestUser(t, env, "carol-oa-rr@test.com", "Carol")
	sessionToken := testutil.CreateTestWebSessionWithToken(t, env, alice.ID)

	// Each session needs a session_type + tokens + conversation cards to be
	// counted by the org analytics aggregate query (see seedOrgSession).
	seedAnalyticsSession := func(userID int64, externalID, repoURL string) {
		sid := testutil.CreateTestSessionWithProvider(t, env, userID, externalID, models.ProviderClaudeCode)
		gitInfo, _ := json.Marshal(map[string]string{"repo_url": repoURL})
		if _, err := env.DB.Conn().ExecContext(env.Ctx,
			`UPDATE sessions SET git_info = $2 WHERE id = $1`,
			sid, gitInfo); err != nil {
			t.Fatalf("set git_info: %v", err)
		}
		store := analytics.NewStore(env.DB.Conn())
		assistantMs := int64(1000)
		userMs := int64(500)
		if err := store.UpsertCards(env.Ctx, &analytics.Cards{
			Tokens: &analytics.TokensCardRecord{
				SessionID:        sid,
				Version:          analytics.TokensCardVersion,
				ComputedAt:       time.Now().UTC(),
				UpToLine:         100,
				EstimatedCostUSD: decimal.NewFromFloat(1.0),
			},
			Conversation: &analytics.ConversationCardRecord{
				SessionID:                sid,
				Version:                  analytics.ConversationCardVersion,
				ComputedAt:               time.Now().UTC(),
				UpToLine:                 100,
				TotalAssistantDurationMs: &assistantMs,
				TotalUserDurationMs:      &userMs,
			},
		}); err != nil {
			t.Fatalf("UpsertCards: %v", err)
		}
	}

	seedAnalyticsSession(alice.ID, "alice-fork-s", "https://github.com/jackie/confab-web.git")
	seedAnalyticsSession(bob.ID, "bob-upstream-s", "https://github.com/ConfabulousDev/confab-web.git")
	seedAnalyticsSession(carol.ID, "carol-other-s", "https://github.com/other/repo.git")

	seedOrgForkRootMapping(t, env, "jackie/confab-web", "ConfabulousDev/confab-web")

	ts := setupTestServerWithEnv(t, env)
	client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

	now := time.Now().UTC()
	v := url.Values{}
	v.Set("start_ts", fmt.Sprintf("%d", now.Add(-7*24*time.Hour).Unix()))
	v.Set("end_ts", fmt.Sprintf("%d", now.Add(24*time.Hour).Unix()))
	v.Set("tz_offset", "0")
	v.Set("repos", "ConfabulousDev/confab-web")
	v.Set("include_no_repo", "false")

	resp, err := client.Get("/api/v1/org/analytics?" + v.Encode())
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	testutil.RequireStatus(t, resp, http.StatusOK)

	// Decode the response as a minimal envelope and count rows with > 0
	// sessions. The full shape is asserted elsewhere; here we only care
	// that the fork session is included.
	body, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Users []struct {
			User struct {
				Email string `json:"email"`
			} `json:"user"`
			SessionCount int `json:"session_count"`
		} `json:"users"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, string(body))
	}

	counted := map[string]int{}
	for _, u := range envelope.Users {
		counted[u.User.Email] = u.SessionCount
	}

	if counted["alice-oa-rr@test.com"] < 1 {
		t.Errorf("expected Alice (fork session) to count when filtering by root, got %d",
			counted["alice-oa-rr@test.com"])
	}
	if counted["bob-oa-rr@test.com"] < 1 {
		t.Errorf("expected Bob (upstream session) to count, got %d",
			counted["bob-oa-rr@test.com"])
	}
	if counted["carol-oa-rr@test.com"] > 0 {
		t.Errorf("expected Carol (unrelated repo) to NOT count, got %d",
			counted["carol-oa-rr@test.com"])
	}
}
