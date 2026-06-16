package admin_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// installFixturePricing pins a two-family pricing table (claude opus-4-5, codex
// gpt-5) for the test so the priced/unpriced split is deterministic regardless of
// the shipped pricing.json. Restored to the embedded floor on cleanup.
func installFixturePricing(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { analytics.SetActivePricing(pricingsource.Embedded()) })
	rate := pricingsource.Rate{Input: 1, Output: 2, CacheWrite: 1, CacheWrite1h: 2, CacheRead: 1}
	analytics.SetActivePricing(pricingsource.Document{
		Pricing: map[string]map[string]pricingsource.Rate{
			models.ProviderClaudeCode: {"opus-4-5": rate},
			models.ProviderCodex:      {"gpt-5": rate},
		},
	})
}

func TestUnpricedModelsAPI_AuthEnforcement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)

	t.Run("unauthenticated gets 401", func(t *testing.T) {
		env.CleanDB(t)
		ts := setupTestServer(t, env)
		client := testutil.NewTestClient(t, ts)

		resp, err := client.Get("/api/v1/admin/unpriced-models")
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("non-admin gets 403", func(t *testing.T) {
		env.CleanDB(t)
		user := testutil.CreateTestUser(t, env, "user@example.com", "User")
		testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

		ts := setupTestServer(t, env)
		client := adminClient(t, env, ts, user.ID)

		resp, err := client.Get("/api/v1/admin/unpriced-models")
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}
	})
}

// unpricedModelsResponse mirrors the JSON wire shape of the endpoint so the test
// asserts the actual serialized fields (not just the Go struct).
type unpricedModelsResponse struct {
	Models []struct {
		Provider     string `json:"provider"`
		Family       string `json:"family"`
		SessionCount int    `json:"session_count"`
		LastSeen     string `json:"last_seen"`
	} `json:"models"`
}

func (r unpricedModelsResponse) find(provider, family string) (int, string, bool) {
	for _, m := range r.Models {
		if m.Provider == provider && m.Family == family {
			return m.SessionCount, m.LastSeen, true
		}
	}
	return 0, "", false
}

func TestUnpricedModelsAPI_ReturnsGap(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	installFixturePricing(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	// One session with a priced family + an unpriced family.
	sess := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "unpriced-api", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sess, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 100, CostUSD: "0"}, // priced → excluded
				"opus-9":   {Input: 50, CostUSD: "0"},  // unpriced
			}},
		},
	})

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	resp, err := client.Get("/api/v1/admin/unpriced-models")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body unpricedModelsResponse
	testutil.ParseJSON(t, resp, &body)

	if len(body.Models) != 1 {
		t.Fatalf("expected 1 unpriced model, got %d: %+v", len(body.Models), body.Models)
	}
	count, lastSeen, ok := body.find(models.ProviderClaudeCode, "opus-9")
	if !ok {
		t.Fatalf("expected (claude-code, opus-9); got %+v", body.Models)
	}
	if count != 1 {
		t.Errorf("session_count = %d, want 1", count)
	}
	if lastSeen == "" {
		t.Error("last_seen is empty")
	}
	if _, _, ok := body.find(models.ProviderClaudeCode, "opus-4-5"); ok {
		t.Error("priced family opus-4-5 must not appear")
	}
}

func TestUnpricedModelsAPI_EmptyWhenAllPriced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	os.Setenv("LOG_FORMAT", "json")

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	installFixturePricing(t)

	adminUser := testutil.CreateTestUser(t, env, "admin@example.com", "Admin")
	testutil.SetEnvForTest(t, "SUPER_ADMIN_EMAILS", "admin@example.com")

	sess := testutil.CreateTestSessionWithProvider(t, env, adminUser.ID, "all-priced-api", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sess, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 100, CostUSD: "0"},
			}},
		},
	})

	ts := setupTestServer(t, env)
	client := adminClient(t, env, ts, adminUser.ID)

	resp, err := client.Get("/api/v1/admin/unpriced-models")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.RequireStatus(t, resp, http.StatusOK)

	var body unpricedModelsResponse
	testutil.ParseJSON(t, resp, &body)
	if len(body.Models) != 0 {
		t.Fatalf("expected 0 unpriced models, got %d: %+v", len(body.Models), body.Models)
	}
}
