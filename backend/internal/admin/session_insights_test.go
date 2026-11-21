package admin_test

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

const (
	providerSharePath = "/api/v1/admin/session-insights/provider-share"
	sessionPointsPath = "/api/v1/admin/session-insights/points"
)

func TestSessionInsightsAPI_AuthEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)

	for _, path := range []string{providerSharePath, sessionPointsPath} {
		t.Run(path+" unauthenticated gets 401", func(t *testing.T) {
			env.CleanDB(t)
			ts := setupTestServer(t, env)
			resp, err := testutil.NewTestClient(t, ts).Get(path)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401, got %d", resp.StatusCode)
			}
		})

		t.Run(path+" non-admin gets 403", func(t *testing.T) {
			env.CleanDB(t)
			user := testutil.CreateTestUser(t, env, "user@example.com", "User")
			testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
			ts := setupTestServer(t, env)
			resp, err := adminClient(t, env, ts, user.ID).Get(path)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("expected 403, got %d", resp.StatusCode)
			}
		})
	}
}

func TestSessionInsightsAPI_RejectsBadParams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")
	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	cases := map[string]string{
		"unknown provider":       "?provider=gemini",
		"legacy provider form":   "?provider=Claude%20Code",
		"unparseable from":       "?from=yesterday",
		"timestamp without tz":   "?from=2026-03-01T00:00:00",
		"from not before to":     "?from=2026-03-08&to=2026-03-01",
		"non-numeric user_id":    "?user_id=abc",
		"non-positive user_id":   "?user_id=0",
		"unparseable all flag":   "?all=maybe",
		"all combined with from": "?all=true&from=2026-03-01",
	}
	for _, path := range []string{providerSharePath, sessionPointsPath} {
		for name, query := range cases {
			t.Run(path+" "+name, func(t *testing.T) {
				resp, err := client.Get(path + query)
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("%s: expected 400, got %d", query, resp.StatusCode)
				}
			})
		}
	}
}

type providerShareWire struct {
	From       string `json:"from"`
	To         string `json:"to"`
	BucketSize string `json:"bucket_size"`
	Buckets    []struct {
		Start  string         `json:"start"`
		Counts map[string]int `json:"counts"`
	} `json:"buckets"`
	Totals map[string]int `json:"totals"`
}

type sessionPointsWire struct {
	Total    int `json:"total"`
	Returned int `json:"returned"`
	Limit    int `json:"limit"`
	Points   []struct {
		ID         string   `json:"id"`
		FirstSeen  string   `json:"first_seen"`
		Provider   string   `json:"provider"`
		Title      string   `json:"title"`
		OwnerEmail string   `json:"owner_email"`
		Repo       *string  `json:"repo"`
		Branch     *string  `json:"branch"`
		Lines      int64    `json:"lines"`
		Tokens     *int64   `json:"tokens"`
		CostUSD    *float64 `json:"cost_usd"`
		UserTurns  *int64   `json:"user_turns"`
		DurationMs *int64   `json:"duration_ms"`
	} `json:"points"`
}

func getJSON(t *testing.T, client *testutil.TestClient, path string, out any) {
	t.Helper()
	resp, err := client.Get(path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func TestSessionInsightsAPI_WireShape(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")
	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	other := testutil.CreateTestUser(t, env, "other@example.com", "Other")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	seen := time.Date(2026, 3, 4, 15, 0, 0, 0, time.UTC)
	// Another user's session: admin endpoints are instance-wide, not visibility-scoped.
	rich := testutil.CreateTestSessionFull(t, env, other.ID, "rich", testutil.TestSessionFullOpts{
		RepoURL: "git@github.com:acme/widgets.git", Branch: "feat", Summary: "Widget work", SyncLines: 42,
	})
	codex := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "cx", models.ProviderCodex)
	testutil.MakeSessionListable(t, env, codex)
	legacy := testutil.CreateTestSessionLegacyClaudeCode(t, env, other.ID, "legacy")
	testutil.MakeSessionListable(t, env, legacy)
	for _, id := range []string{rich, codex, legacy} {
		if _, err := env.DB.Exec(env.Ctx, `UPDATE sessions SET first_seen = $2 WHERE id = $1`, id, seen); err != nil {
			t.Fatal(err)
		}
	}
	testutil.SeedTokensV2Card(t, env, rich, analytics.TokensV2Data{TotalInput: 10, TotalOutput: 5, TotalCostUSD: "0.5"})

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)
	window := "?from=2026-03-01&to=2026-03-07"

	t.Run("provider-share serializes buckets, totals, and the effective range", func(t *testing.T) {
		var got providerShareWire
		getJSON(t, client, providerSharePath+window, &got)
		if got.BucketSize != "day" || got.From != "2026-03-01T00:00:00Z" {
			t.Errorf("bucket_size/from = %q/%q", got.BucketSize, got.From)
		}
		// A date-only `to` is inclusive of that day.
		if got.To != "2026-03-08T00:00:00Z" {
			t.Errorf("to = %q, want 2026-03-08T00:00:00Z", got.To)
		}
		if len(got.Buckets) != 1 || got.Buckets[0].Start != "2026-03-04T00:00:00Z" {
			t.Fatalf("buckets = %+v, want one starting 2026-03-04", got.Buckets)
		}
		if got.Buckets[0].Counts["claude-code"] != 2 || got.Buckets[0].Counts["codex"] != 1 {
			t.Errorf("counts = %v, want claude-code:2 codex:1", got.Buckets[0].Counts)
		}
		if got.Totals["claude-code"] != 2 || got.Totals["codex"] != 1 {
			t.Errorf("totals = %v", got.Totals)
		}
	})

	t.Run("provider and user filters narrow; provider accepts repeats and commas", func(t *testing.T) {
		cases := []struct {
			query string
			want  map[string]int
		}{
			{"&provider=codex&provider=cursor", map[string]int{"codex": 1}},
			{"&provider=CODEX,cursor", map[string]int{"codex": 1}},
			{"&user_id=" + strconv.FormatInt(other.ID, 10), map[string]int{"claude-code": 2}},
		}
		for _, tc := range cases {
			var got providerShareWire
			getJSON(t, client, providerSharePath+window+tc.query, &got)
			if len(got.Totals) != len(tc.want) {
				t.Errorf("%s: totals = %v, want %v", tc.query, got.Totals, tc.want)
			}
			for k, v := range tc.want {
				if got.Totals[k] != v {
					t.Errorf("%s: totals = %v, want %v", tc.query, got.Totals, tc.want)
				}
			}
		}
	})

	t.Run("all=true spans from the earliest session", func(t *testing.T) {
		var got providerShareWire
		getJSON(t, client, providerSharePath+"?all=true", &got)
		if got.From != "2026-03-04T15:00:00Z" {
			t.Errorf("all-time from = %q, want the earliest first_seen", got.From)
		}
	})

	t.Run("points serialize every field with nulls for missing metrics", func(t *testing.T) {
		var got sessionPointsWire
		getJSON(t, client, sessionPointsPath+window, &got)
		if got.Total != 3 || got.Returned != 3 || got.Limit != analytics.SessionInsightsPointLimit {
			t.Fatalf("total/returned/limit = %d/%d/%d", got.Total, got.Returned, got.Limit)
		}
		var found bool
		for _, p := range got.Points {
			if p.ID != rich {
				if p.Tokens != nil || p.CostUSD != nil {
					t.Errorf("session without a tokens card must have null tokens/cost: %+v", p)
				}
				continue
			}
			found = true
			if p.Provider != "claude-code" || p.Title != "Widget work" || p.OwnerEmail != "other@example.com" {
				t.Errorf("identity = %+v", p)
			}
			if p.FirstSeen != "2026-03-04T15:00:00Z" {
				t.Errorf("first_seen = %q", p.FirstSeen)
			}
			if p.Repo == nil || *p.Repo != "acme/widgets" || p.Branch == nil || *p.Branch != "feat" {
				t.Errorf("repo/branch = %v/%v, want sanitized acme/widgets / feat", p.Repo, p.Branch)
			}
			if p.Lines != 42 || p.Tokens == nil || *p.Tokens != 15 || p.CostUSD == nil || *p.CostUSD != 0.5 {
				t.Errorf("metrics lines=%d tokens=%v cost=%v", p.Lines, p.Tokens, p.CostUSD)
			}
			if p.UserTurns != nil || p.DurationMs != nil {
				t.Errorf("missing session/conversation cards must serialize null")
			}
		}
		if !found {
			t.Errorf("another user's session must appear (instance-wide)")
		}
	})

	t.Run("an empty window returns empty arrays, not null", func(t *testing.T) {
		resp, err := client.Get(sessionPointsPath + "?from=2020-01-01&to=2020-01-02")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var raw map[string]json.RawMessage
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["points"]) != "[]" {
			t.Errorf("points = %s, want []", raw["points"])
		}
	})
}
