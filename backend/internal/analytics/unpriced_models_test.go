package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
	"github.com/ConfabulousDev/confab-web/internal/testutil"
)

// unpricedFixturePricing is a tiny known pricing document installed for the
// duration of an UnpricedModels test so the priced/unpriced split is
// deterministic regardless of what ships in the embedded pricing.json. It prices
// exactly two families: Claude "opus-4-5" and OpenAI "gpt-5".
func unpricedFixturePricing() pricingsource.Document {
	rate := pricingsource.Rate{Input: 1, Output: 2, CacheWrite: 1, CacheWrite1h: 2, CacheRead: 1}
	return pricingsource.Document{
		Pricing: map[string]map[string]pricingsource.Rate{
			models.ProviderClaudeCode: {"opus-4-5": rate},
			models.ProviderCodex:      {"gpt-5": rate},
		},
	}
}

// findUnpriced returns the row for a (provider, family) pair, if present.
func findUnpriced(rows []analytics.UnpricedModel, provider, family string) (analytics.UnpricedModel, bool) {
	for _, r := range rows {
		if r.Provider == provider && r.Family == family {
			return r, true
		}
	}
	return analytics.UnpricedModel{}, false
}

// TestUnpricedModels_GapAgainstActivePricing is the core correctness test: only
// model families absent from the active pricing table are returned, keyed by the
// normalized family (not the raw dated id), with a distinct-session count and a
// last-seen timestamp. Priced families, the empty/Unknown key, the synthetic
// sentinel, and the Claude " · fast" variant of a priced family must NOT appear.
func TestUnpricedModels_GapAgainstActivePricing(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	t.Cleanup(func() { analytics.SetActivePricing(pricingsource.Embedded()) })
	analytics.SetActivePricing(unpricedFixturePricing())

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "unpriced@test.com", "Unpriced User")

	// Session A (claude-code): a priced family + an UNPRICED family ("opus-9").
	sessA := testutil.CreateTestSessionWithProvider(t, env, user.ID, "unpriced-a", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sessA, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"opus-4-5":        {Input: 100, CostUSD: "0"}, // priced → excluded
				"opus-4-5 · fast": {Input: 10, CostUSD: "0"},  // priced family fast variant → excluded
				"opus-9":          {Input: 50, CostUSD: "0"},  // UNPRICED
			}},
		},
	})

	// Session B (claude-code): the SAME unpriced family "opus-9" again, so the
	// distinct-session count for (claude-code, opus-9) must be 2.
	sessB := testutil.CreateTestSessionWithProvider(t, env, user.ID, "unpriced-b", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sessB, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"opus-9": {Input: 5, CostUSD: "0"},
			}},
		},
	})

	// Session C (codex): an unpriced OpenAI family with a DATED raw id; it must
	// be reported under its bare family "gpt-9", not the dated id.
	sessC := testutil.CreateTestSessionWithProvider(t, env, user.ID, "unpriced-c", models.ProviderCodex)
	testutil.SeedTokensV2Card(t, env, sessC, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderCodex: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"gpt-5": {Input: 1, CostUSD: "0"}, // priced → excluded
				"gpt-9": {Input: 2, CostUSD: "0"}, // UNPRICED
			}},
		},
	})

	// Session D (opencode): RAW vendor-keyed dated ids. The priced one
	// ("claude-opus-4-5-20251101" → opus-4-5) is excluded; the unpriced one
	// ("claude-opus-9-20260601" → opus-9) collapses to the SAME family as the
	// claude-code "opus-9" but under the canonical opencode provider, so it is a
	// SEPARATE row (provider-keyed).
	sessD := testutil.CreateTestSessionWithProvider(t, env, user.ID, "unpriced-d", models.ProviderOpencode)
	testutil.SeedTokensV2Card(t, env, sessD, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			"anthropic": {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"claude-opus-4-5-20251101": {Input: 1, CostUSD: "0"}, // priced → excluded
				"claude-opus-9-20260601":   {Input: 1, CostUSD: "0"}, // UNPRICED → opus-9
			}},
		},
	})

	// Session E (claude-code): only the empty (Unknown) key and the synthetic
	// sentinel — neither is a real model, so neither may appear.
	sessE := testutil.CreateTestSessionWithProvider(t, env, user.ID, "unpriced-e", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sessE, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"":            {Input: 1, CostUSD: "0"},
				"<synthetic>": {Input: 1, CostUSD: "0"},
			}},
		},
	})

	store := analytics.NewStore(env.DB.Conn())
	rows, err := store.UnpricedModels(context.Background())
	if err != nil {
		t.Fatalf("UnpricedModels: %v", err)
	}

	// Exactly three unpriced rows: (claude-code, opus-9), (codex, gpt-9),
	// (opencode, opus-9).
	if len(rows) != 3 {
		t.Fatalf("expected 3 unpriced rows, got %d: %+v", len(rows), rows)
	}

	claudeOpus9, ok := findUnpriced(rows, models.ProviderClaudeCode, "opus-9")
	if !ok {
		t.Fatalf("expected (claude-code, opus-9) row; got %+v", rows)
	}
	if claudeOpus9.SessionCount != 2 {
		t.Errorf("(claude-code, opus-9) session count = %d, want 2", claudeOpus9.SessionCount)
	}
	if claudeOpus9.LastSeen.IsZero() {
		t.Error("(claude-code, opus-9) last seen is zero")
	}
	if time.Since(claudeOpus9.LastSeen) > time.Hour {
		t.Errorf("(claude-code, opus-9) last seen too old: %v", claudeOpus9.LastSeen)
	}

	if gpt9, ok := findUnpriced(rows, models.ProviderCodex, "gpt-9"); !ok {
		t.Errorf("expected (codex, gpt-9) row; got %+v", rows)
	} else if gpt9.SessionCount != 1 {
		t.Errorf("(codex, gpt-9) session count = %d, want 1", gpt9.SessionCount)
	}

	if opencodeOpus9, ok := findUnpriced(rows, models.ProviderOpencode, "opus-9"); !ok {
		t.Errorf("expected (opencode, opus-9) row; got %+v", rows)
	} else if opencodeOpus9.SessionCount != 1 {
		t.Errorf("(opencode, opus-9) session count = %d, want 1", opencodeOpus9.SessionCount)
	}

	// Negative assertions: priced families and non-models must be absent.
	for _, bad := range []struct{ provider, family string }{
		{models.ProviderClaudeCode, "opus-4-5"},
		{models.ProviderClaudeCode, "opus-4-5 · fast"},
		{models.ProviderCodex, "gpt-5"},
		{models.ProviderOpencode, "opus-4-5"},
		{models.ProviderClaudeCode, ""},
		{models.ProviderClaudeCode, "<synthetic>"},
	} {
		if _, ok := findUnpriced(rows, bad.provider, bad.family); ok {
			t.Errorf("did not expect (%s, %s) in unpriced rows", bad.provider, bad.family)
		}
	}
}

// TestUnpricedModels_EmptyWhenAllPriced confirms a clean (no-gap) state returns
// an empty slice, not an error — the common case once pricing.json is current.
func TestUnpricedModels_EmptyWhenAllPriced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	t.Cleanup(func() { analytics.SetActivePricing(pricingsource.Embedded()) })
	analytics.SetActivePricing(unpricedFixturePricing())

	env := testutil.SetupTestEnvironment(t)
	env.CleanDB(t)
	user := testutil.CreateTestUser(t, env, "allpriced@test.com", "All Priced User")

	sess := testutil.CreateTestSessionWithProvider(t, env, user.ID, "all-priced", models.ProviderClaudeCode)
	testutil.SeedTokensV2Card(t, env, sess, analytics.TokensV2Data{
		TotalCostUSD: "0", ByProvider: map[string]analytics.TokensV2Provider{
			models.ProviderClaudeCode: {CostUSD: "0", Models: map[string]analytics.TokensV2Model{
				"opus-4-5": {Input: 100, CostUSD: "0"},
			}},
		},
	})

	store := analytics.NewStore(env.DB.Conn())
	rows, err := store.UnpricedModels(context.Background())
	if err != nil {
		t.Fatalf("UnpricedModels: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no unpriced rows, got %d: %+v", len(rows), rows)
	}
}

// TestActivePricingFamilies_ReflectsActiveTable confirms the exported helper
// returns exactly the family keys in the active pricing table.
func TestActivePricingFamilies_ReflectsActiveTable(t *testing.T) {
	t.Cleanup(func() { analytics.SetActivePricing(pricingsource.Embedded()) })
	analytics.SetActivePricing(unpricedFixturePricing())

	fams := analytics.ActivePricingFamilies()
	if _, ok := fams["opus-4-5"]; !ok {
		t.Errorf("expected opus-4-5 in active families, got %v", fams)
	}
	if _, ok := fams["gpt-5"]; !ok {
		t.Errorf("expected gpt-5 in active families, got %v", fams)
	}
	if _, ok := fams["opus-9"]; ok {
		t.Errorf("did not expect opus-9 in active families")
	}
	if len(fams) != 2 {
		t.Errorf("expected exactly 2 active families, got %d: %v", len(fams), fams)
	}
}
