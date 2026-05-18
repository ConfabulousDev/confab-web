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
	"github.com/ConfabulousDev/confab-web/internal/testutil"
	"github.com/shopspring/decimal"
)

func TestOrgAnalytics_HTTP_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 401 without session", func(t *testing.T) {
		env.CleanDB(t)

		// Enable org analytics
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestOrgAnalytics_HTTP_InvalidParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 400 for invalid start_ts", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics?start_ts=notanumber")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("returns 400 when range exceeds 90 days", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		now := time.Now().UTC()
		startTS := now.Add(-100 * 24 * time.Hour).Unix()
		endTS := now.Unix()

		resp, err := client.Get(fmt.Sprintf("/api/v1/org/analytics?start_ts=%d&end_ts=%d", startTS, endTS))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusBadRequest)
	})
}

func TestOrgAnalytics_HTTP_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns org analytics for authenticated user", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.OrgAnalyticsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// User should appear in results
		if len(result.Users) != 1 {
			t.Fatalf("Users length = %d, want 1", len(result.Users))
		}
		if result.Users[0].User.Email != "test@example.com" {
			t.Errorf("User email = %s, want test@example.com", result.Users[0].User.Email)
		}
	})
}

// seedOrgSession creates a session with the given provider plus tokens +
// conversation cards so it qualifies for inclusion in the org aggregate.
// Pass provider="Claude Code" for the legacy session_type variant.
func seedOrgSession(t *testing.T, env *testutil.TestEnvironment, userID int64, externalID, provider string, costUSD float64, assistantMs, userMs int64) {
	t.Helper()
	var sessionID string
	if provider == "Claude Code" {
		sessionID = testutil.CreateTestSessionLegacyClaudeCode(t, env, userID, externalID)
	} else {
		sessionID = testutil.CreateTestSessionWithProvider(t, env, userID, externalID, provider)
	}
	store := analytics.NewStore(env.DB.Conn())
	err := store.UpsertCards(env.Ctx, &analytics.Cards{
		Tokens: &analytics.TokensCardRecord{
			SessionID:        sessionID,
			Version:          analytics.TokensCardVersion,
			ComputedAt:       time.Now().UTC(),
			UpToLine:         100,
			EstimatedCostUSD: decimal.NewFromFloat(costUSD),
		},
		Conversation: &analytics.ConversationCardRecord{
			SessionID:                sessionID,
			Version:                  analytics.ConversationCardVersion,
			ComputedAt:               time.Now().UTC(),
			UpToLine:                 100,
			TotalAssistantDurationMs: &assistantMs,
			TotalUserDurationMs:      &userMs,
		},
	})
	if err != nil {
		t.Fatalf("UpsertCards (%s): %v", sessionID, err)
	}
}

// orgAnalyticsURL builds a /api/v1/org/analytics URL with the given query
// params plus a generous date range covering "now".
func orgAnalyticsURL(extra url.Values) string {
	now := time.Now().UTC()
	v := url.Values{}
	v.Set("start_ts", fmt.Sprintf("%d", now.Add(-7*24*time.Hour).Unix()))
	v.Set("end_ts", fmt.Sprintf("%d", now.Add(24*time.Hour).Unix()))
	v.Set("tz_offset", "0")
	for k, vals := range extra {
		for _, vv := range vals {
			v.Add(k, vv)
		}
	}
	return "/api/v1/org/analytics?" + v.Encode()
}

// TestOrgAnalytics_HTTP_WireContract pins the renamed wire contract:
//
//  1. `total_assistant_time_ms` / `avg_assistant_time_ms` are present.
//  2. Stale `total_claude_time_ms` / `avg_claude_time_ms` are NOT present
//     (negative assertion via raw JSON to catch drift past struct rename).
//  3. `providers_present` is always non-nil and reports providers in the date
//     range × repo filter — independent of the provider selection.
//  4. `?provider=` narrows session counting; legacy `Claude Code` rows fold
//     into the canonical `claude-code` filter via models.ExpandWithAliases.
func TestOrgAnalytics_HTTP_WireContract(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("no filter: renamed fields present, old fields absent", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "alice@test.com", "Alice")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		seedOrgSession(t, env, user.ID, "alice-claude", "claude-code", 2.50, 30000, 60000)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(orgAnalyticsURL(nil))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		users, ok := raw["users"].([]any)
		if !ok || len(users) != 1 {
			t.Fatalf("users = %v, want length 1", raw["users"])
		}
		u := users[0].(map[string]any)

		// Strict map-key checks beat substring search: a value that happens to
		// contain "total_claude_time_ms" (an email, a name) won't false-fail.
		for _, stale := range []string{"total_claude_time_ms", "avg_claude_time_ms"} {
			if _, present := u[stale]; present {
				t.Errorf("response carries stale %q key", stale)
			}
		}
		for _, required := range []string{"total_assistant_time_ms", "avg_assistant_time_ms"} {
			if _, present := u[required]; !present {
				t.Errorf("missing required field %q", required)
			}
		}
		providers, ok := raw["providers_present"].([]any)
		if !ok {
			t.Fatalf("providers_present = %v, want array", raw["providers_present"])
		}
		if len(providers) != 1 || providers[0] != "claude-code" {
			t.Errorf("providers_present = %v, want [claude-code]", providers)
		}
	})

	t.Run("provider=codex narrows to codex only", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "bob@test.com", "Bob")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		seedOrgSession(t, env, user.ID, "bob-claude", "claude-code", 2.00, 10000, 20000)
		seedOrgSession(t, env, user.ID, "bob-codex", "codex", 3.00, 15000, 30000)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(orgAnalyticsURL(url.Values{"provider": {"codex"}}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.OrgAnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Users) != 1 {
			t.Fatalf("Users = %v, want length 1", result.Users)
		}
		got := result.Users[0]
		if got.SessionCount != 1 {
			t.Errorf("SessionCount = %d, want 1 (codex only)", got.SessionCount)
		}
		if got.TotalCostUSD != "3.00" {
			t.Errorf("TotalCostUSD = %s, want 3.00", got.TotalCostUSD)
		}
		// providers_present is independent of the provider filter — both
		// providers seeded in the date range should appear so the dropdown
		// stays widenable when a user pins one provider.
		if len(result.ProvidersPresent) != 2 ||
			result.ProvidersPresent[0] != "claude-code" || result.ProvidersPresent[1] != "codex" {
			t.Errorf("ProvidersPresent = %v, want [claude-code codex]", result.ProvidersPresent)
		}
	})

	t.Run("legacy Claude Code session_type folds into claude-code filter", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		user := testutil.CreateTestUser(t, env, "carol@test.com", "Carol")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)
		seedOrgSession(t, env, user.ID, "carol-legacy", "Claude Code", 5.00, 20000, 40000)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get(orgAnalyticsURL(url.Values{"provider": {"claude-code"}}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result analytics.OrgAnalyticsResponse
		testutil.ParseJSON(t, resp, &result)

		if len(result.Users) != 1 {
			t.Fatalf("Users = %v, want length 1", result.Users)
		}
		if result.Users[0].SessionCount != 1 {
			t.Errorf("SessionCount = %d, want 1 (legacy row should fold into claude-code)", result.Users[0].SessionCount)
		}
		if len(result.ProvidersPresent) != 1 || result.ProvidersPresent[0] != "claude-code" {
			t.Errorf("ProvidersPresent = %v, want [claude-code] (legacy normalized)", result.ProvidersPresent)
		}
	})
}

// TestOrgRepos_HTTP_Integration covers the new `/api/v1/org/repos` endpoint:
// org-wide DISTINCT repos in the date range, sorted, deduped across users,
// gated on ENABLE_ORG_ANALYTICS.
func TestOrgRepos_HTTP_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns sorted distinct repos across users", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		alice := testutil.CreateTestUser(t, env, "alice@test.com", "Alice")
		bob := testutil.CreateTestUser(t, env, "bob@test.com", "Bob")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, alice.ID)

		// alice touches confab-web; bob touches confab-web AND cli; one no-repo
		// session must NOT appear in the listing.
		testutil.CreateTestSessionWithGitInfo(t, env, alice.ID, "alice-a", "https://github.com/ConfabulousDev/confab-web.git")
		testutil.CreateTestSessionWithGitInfo(t, env, bob.ID, "bob-a", "git@github.com:ConfabulousDev/confab-web.git")
		testutil.CreateTestSessionWithGitInfo(t, env, bob.ID, "bob-b", "https://github.com/ConfabulousDev/cli.git")
		testutil.CreateTestSession(t, env, alice.ID, "alice-norepo")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		now := time.Now().UTC()
		url := fmt.Sprintf("/api/v1/org/repos?start_ts=%d&end_ts=%d&tz_offset=0",
			now.Add(-7*24*time.Hour).Unix(), now.Add(24*time.Hour).Unix())

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusOK)

		var result OrgReposResponse
		testutil.ParseJSON(t, resp, &result)

		want := []string{"ConfabulousDev/cli", "ConfabulousDev/confab-web"}
		if len(result.Repos) != len(want) {
			t.Fatalf("Repos = %v, want %v", result.Repos, want)
		}
		for i, r := range want {
			if result.Repos[i] != r {
				t.Errorf("Repos[%d] = %q, want %q", i, result.Repos[i], r)
			}
		}
	})

	t.Run("returns 404 when feature flag disabled", func(t *testing.T) {
		env.CleanDB(t)
		// Do NOT set ENABLE_ORG_ANALYTICS

		user := testutil.CreateTestUser(t, env, "dave@test.com", "Dave")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		now := time.Now().UTC()
		url := fmt.Sprintf("/api/v1/org/repos?start_ts=%d&end_ts=%d&tz_offset=0",
			now.Add(-7*24*time.Hour).Unix(), now.Add(24*time.Hour).Unix())

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 404 or 405 when feature disabled", resp.StatusCode)
		}
	})

	t.Run("returns 401 without session", func(t *testing.T) {
		env.CleanDB(t)
		testutil.SetEnvForTest(t, "ENABLE_ORG_ANALYTICS", "true")

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts) // No session

		now := time.Now().UTC()
		url := fmt.Sprintf("/api/v1/org/repos?start_ts=%d&end_ts=%d&tz_offset=0",
			now.Add(-7*24*time.Hour).Unix(), now.Add(24*time.Hour).Unix())

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		testutil.RequireStatus(t, resp, http.StatusUnauthorized)
	})
}

func TestOrgAnalytics_HTTP_DisabledRoute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTTP integration test in short mode")
	}

	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	t.Run("returns 404 when feature is disabled", func(t *testing.T) {
		env.CleanDB(t)
		// Do NOT set ENABLE_ORG_ANALYTICS

		user := testutil.CreateTestUser(t, env, "test@example.com", "Test User")
		sessionToken := testutil.CreateTestWebSessionWithToken(t, env, user.ID)

		ts := setupTestServerWithEnv(t, env)
		client := testutil.NewTestClient(t, ts).WithSession(sessionToken)

		resp, err := client.Get("/api/v1/org/analytics")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		// Route not registered → 404 or 405
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 404 or 405 when feature disabled", resp.StatusCode)
		}
	})
}
