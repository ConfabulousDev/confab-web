package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
	"github.com/shopspring/decimal"
)

// parityFixturePath locates the repo-root testdata fixture from this package
// directory (backend/internal/analytics → repo root is three levels up).
var parityFixturePath = filepath.Join("..", "..", "..", "testdata", "model_family_parity.json")

// modelFamilyParityCase mirrors one entry of testdata/model_family_parity.json.
type modelFamilyParityCase struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Backend  string `json:"backend"`
	Frontend string `json:"frontend"`
}

// TestGetModelFamily_Parity loads the shared cross-language fixture (also loaded
// by the frontend's tokenStats.test.ts) and asserts this side's getModelFamily
// returns the pinned `backend` family key for every id. Together with the
// frontend test it guards against the two normalizers silently drifting apart
// (nrxr / 5x6e F7). The fixture is provider-agnostic on this side: backend
// getModelFamily sniffs the `gpt-` prefix itself and ignores the provider field.
func TestGetModelFamily_Parity(t *testing.T) {
	raw, err := os.ReadFile(parityFixturePath)
	if err != nil {
		t.Fatalf("read parity fixture %s: %v", parityFixturePath, err)
	}
	var fixture struct {
		Cases []modelFamilyParityCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parse parity fixture: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("parity fixture has no cases")
	}
	for _, c := range fixture.Cases {
		t.Run(c.ID, func(t *testing.T) {
			if got := getModelFamily(c.ID); got != c.Backend {
				t.Errorf("getModelFamily(%q) = %q, want %q (parity fixture `backend`)", c.ID, got, c.Backend)
			}
		})
	}
}

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-fable-5", "fable-5"},
		{"claude-fable-5-20260601", "fable-5"},
		{"claude-opus-4-8-20260515", "opus-4-8"},
		{"claude-opus-4-6-20260201", "opus-4-6"},
		{"claude-opus-4-5-20251101", "opus-4-5"},
		{"claude-sonnet-4-20241022", "sonnet-4"},
		{"claude-haiku-3-5-20241022", "haiku-3-5"},
		{"opus-4-5-20251101", "opus-4-5"},
		{"sonnet-3-7", "sonnet-3-7"},
		{"haiku-3", "haiku-3"},
		{"unknown-model", "unknown-model"},
		// OpenAI / Codex: pass-through with date-suffix stripping.
		{"gpt-5", "gpt-5"},
		{"gpt-5-mini", "gpt-5-mini"},
		{"gpt-5.5", "gpt-5.5"},
		{"gpt-5.4", "gpt-5.4"},
		{"gpt-5-2026-05-01", "gpt-5"},
		{"gpt-5.5-2026-04-15", "gpt-5.5"},
		{"gpt-5.4-2026-05-01", "gpt-5.4"},
		{"o1-mini", "o1-mini"},
		{"o3", "o3"},
		{"o4-mini", "o4-mini"},
		{"gpt-4o", "gpt-4o"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getModelFamily(tt.input)
			if result != tt.expected {
				t.Errorf("getModelFamily(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLookupPricing(t *testing.T) {
	tests := []struct {
		model         string
		wantOK        bool
		expectedInput float64
	}{
		{"claude-opus-4-8-20260515", true, 5},
		{"claude-opus-4-6-20260201", true, 5},
		{"claude-opus-4-5-20251101", true, 5},
		{"claude-opus-5", true, 5},
		{"claude-mythos-5", true, 10},
		{"claude-mythos-5-1", true, 10},
		{"claude-fable-5-1", true, 10},
		{"claude-sonnet-5", true, 2},
		{"claude-sonnet-4-20241022", true, 3},
		{"claude-haiku-3-5-20241022", true, 0.80},
		{"unknown-model", false, 0}, // unknown non-empty model: not found, zero pricing
		{"", false, 0},              // empty model: not found, zero pricing (expected sentinel)
		// OpenAI / Codex
		{"gpt-5", true, 1.25},
		{"gpt-5-mini", true, 0.25},
		{"gpt-5-nano", true, 0.05},
		{"gpt-5.3-codex", true, 1.75},
		{"gpt-5.4", true, 2.50},
		{"gpt-5.4-nano", true, 0.20},
		{"gpt-5.4-pro", true, 30.00},
		{"gpt-5.5", true, 5.00},
		{"gpt-5.5-pro", true, 30.00},
		{"gpt-5.6-sol", true, 4.00},
		{"gpt-5.6-terra", true, 2.00},
		{"gpt-5.6-luna", true, 0.20},
		{"gpt-6-astra", true, 10.00},
		{"gpt-4o", true, 2.50},
		{"gpt-4o-mini", true, 0.15},
		{"o1", true, 15.00},
		{"o3-mini", true, 1.10},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing, ok := LookupPricing(tt.model)
			if ok != tt.wantOK {
				t.Errorf("LookupPricing(%q) ok = %v, want %v", tt.model, ok, tt.wantOK)
			}
			expected := decimal.NewFromFloat(tt.expectedInput)
			if !pricing.Input.Equal(expected) {
				t.Errorf("LookupPricing(%q).Input = %s, want %s", tt.model, pricing.Input, expected)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	// Test with Sonnet 4 pricing: input=$3, output=$15, cacheWrite=$3.75, cacheRead=$0.30 per million
	pricing, _ := LookupPricing("claude-sonnet-4-20241022")

	// 1 million input tokens = $3
	cost := CalculateCost(pricing, 1_000_000, 0, 0, 0)
	expected := decimal.NewFromFloat(3)
	if !cost.Equal(expected) {
		t.Errorf("1M input tokens cost = %s, want %s", cost, expected)
	}

	// 1 million output tokens = $15
	cost = CalculateCost(pricing, 0, 1_000_000, 0, 0)
	expected = decimal.NewFromFloat(15)
	if !cost.Equal(expected) {
		t.Errorf("1M output tokens cost = %s, want %s", cost, expected)
	}

	// Combined: 500k input, 100k output, 200k cache write, 1M cache read
	// = 1.50 + 1.50 + 0.75 + 0.30 = $4.05
	cost = CalculateCost(pricing, 500_000, 100_000, 200_000, 1_000_000)
	expected = decimal.NewFromFloat(4.05)
	if !cost.Equal(expected) {
		t.Errorf("Combined cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_StandardSpeed(t *testing.T) {
	pricing, _ := LookupPricing("claude-sonnet-4-20241022")
	usage := &TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 0,
	}

	cost := CalculateTotalCost(pricing, usage)
	expected := decimal.NewFromFloat(3) // Same as CalculateCost for standard speed
	if !cost.Equal(expected) {
		t.Errorf("Standard speed cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_FastMode(t *testing.T) {
	// Opus 4.6: input=$5, output=$25 per million
	// Fast mode: 2x all token costs ($10/$50 published fast rate vs $5/$25 standard)
	pricing, _ := LookupPricing("claude-opus-4-6-20260201")
	usage := &TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 100_000,
		Speed:        "fast",
	}

	cost := CalculateTotalCost(pricing, usage)
	// Standard: input 1M * $5/M + output 100k * $25/M = $5 + $2.50 = $7.50
	// Fast: $7.50 * 2 = $15
	expected := decimal.NewFromFloat(15)
	if !cost.Equal(expected) {
		t.Errorf("Fast mode cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_FastModeWithCache(t *testing.T) {
	// Verify the fast-mode 2x applies to cache costs too
	pricing, _ := LookupPricing("claude-opus-4-6-20260201")
	usage := &TokenUsage{
		InputTokens:              0,
		OutputTokens:             0,
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
		Speed:                    "fast",
	}

	cost := CalculateTotalCost(pricing, usage)
	// Standard: cacheWrite 1M * $6.25/M + cacheRead 1M * $0.50/M = $6.75
	// Fast: $6.75 * 2 = $13.50
	expected := decimal.NewFromFloat(13.50)
	if !cost.Equal(expected) {
		t.Errorf("Fast mode with cache cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_WebSearchCost(t *testing.T) {
	pricing, _ := LookupPricing("claude-sonnet-4-20241022")
	usage := &TokenUsage{
		InputTokens:  100_000,
		OutputTokens: 10_000,
		ServerToolUse: &ServerToolUse{
			WebSearchRequests: 5,
			WebFetchRequests:  3, // Free, should not add cost
		},
	}

	cost := CalculateTotalCost(pricing, usage)
	// Tokens: input 100k * $3/M + output 10k * $15/M = $0.30 + $0.15 = $0.45
	// Web search: 5 * $0.01 = $0.05
	// Total: $0.50
	expected := decimal.NewFromFloat(0.50)
	if !cost.Equal(expected) {
		t.Errorf("Web search cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_FastModeWithWebSearch(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-6-20260201")
	usage := &TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 0,
		Speed:        "fast",
		ServerToolUse: &ServerToolUse{
			WebSearchRequests: 10,
		},
	}

	cost := CalculateTotalCost(pricing, usage)
	// Token cost: 1M * $5/M = $5, fast: $5 * 2 = $10
	// Web search: 10 * $0.01 = $0.10 (NOT multiplied by fast mode)
	// Total: $10.10
	expected := decimal.NewFromFloat(10.10)
	if !cost.Equal(expected) {
		t.Errorf("Fast mode + web search cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_NilServerToolUse(t *testing.T) {
	pricing, _ := LookupPricing("claude-sonnet-4-20241022")
	usage := &TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 0,
	}

	cost := CalculateTotalCost(pricing, usage)
	expected := decimal.NewFromFloat(3)
	if !cost.Equal(expected) {
		t.Errorf("Nil ServerToolUse cost = %s, want %s", cost, expected)
	}
}

// --------------------------------------------------------------------------
// rd9v: cache-creation billing split by ephemeral tier (5m vs 1h).
// 5m cache writes bill at CacheWrite (1.25x input); 1h at CacheWrite1h (2x input).
// The backend numbers here are pinned identical to the frontend's
// tokenStats.test.ts cache-tier cases so the two cost surfaces stay in lockstep
// (opus-4-8: input 5, cacheWrite 6.25, cacheWrite1h 10 per million).
// --------------------------------------------------------------------------

// TestEmbeddedHasCacheWrite1h asserts the embedded pricing table carries the new
// 1h cache-write rate: 2x input for Claude families, 0 for codex/opencode.
func TestEmbeddedHasCacheWrite1h(t *testing.T) {
	opus, ok := LookupPricing("claude-opus-4-8-20260515")
	if !ok {
		t.Fatal("opus-4-8 not found in embedded pricing")
	}
	if want := decimal.NewFromFloat(10); !opus.CacheWrite1h.Equal(want) {
		t.Errorf("opus-4-8 CacheWrite1h = %s, want %s (2x input)", opus.CacheWrite1h, want)
	}
	gpt5, ok := LookupPricing("gpt-5")
	if !ok {
		t.Fatal("gpt-5 not found in embedded pricing")
	}
	if !gpt5.CacheWrite1h.IsZero() {
		t.Errorf("gpt-5 CacheWrite1h = %s, want 0 (codex has no cache writes)", gpt5.CacheWrite1h)
	}
}

func TestCalculateTotalCost_CacheTierAll1h(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral5m: 0, Ephemeral1h: 1_000_000},
	}
	cost := CalculateTotalCost(pricing, usage)
	// 1M * $10/M (cacheWrite1h) = $10.00
	if want := decimal.NewFromFloat(10); !cost.Equal(want) {
		t.Errorf("all-1h cache cost = %s, want %s", cost, want)
	}
}

func TestCalculateTotalCost_CacheTierAll5m(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral5m: 1_000_000, Ephemeral1h: 0},
	}
	cost := CalculateTotalCost(pricing, usage)
	// 1M * $6.25/M (cacheWrite) = $6.25
	if want := decimal.NewFromFloat(6.25); !cost.Equal(want) {
		t.Errorf("all-5m cache cost = %s, want %s", cost, want)
	}
}

func TestCalculateTotalCost_CacheTierMixed(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral5m: 400_000, Ephemeral1h: 600_000},
	}
	cost := CalculateTotalCost(pricing, usage)
	// 400k * $6.25/M + 600k * $10/M = $2.50 + $6.00 = $8.50
	if want := decimal.NewFromFloat(8.50); !cost.Equal(want) {
		t.Errorf("mixed cache cost = %s, want %s", cost, want)
	}
}

// TestCalculateTotalCost_CacheTierLegacy: no breakdown object (older sessions /
// codex) → all cache-creation tokens bill at the 5m rate (today's behavior).
func TestCalculateTotalCost_CacheTierLegacy(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            nil,
	}
	cost := CalculateTotalCost(pricing, usage)
	if want := decimal.NewFromFloat(6.25); !cost.Equal(want) {
		t.Errorf("legacy (no breakdown) cache cost = %s, want %s (all 5m rate)", cost, want)
	}
}

// TestCalculateTotalCost_CacheTierTrustsBreakdown: when the breakdown disagrees
// with the flat count, the breakdown wins (we never re-derive from the flat sum).
func TestCalculateTotalCost_CacheTierTrustsBreakdown(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 999, // bogus flat count, must be ignored
		CacheCreation:            &CacheCreationBreakdown{Ephemeral5m: 0, Ephemeral1h: 1_000_000},
	}
	cost := CalculateTotalCost(pricing, usage)
	if want := decimal.NewFromFloat(10); !cost.Equal(want) {
		t.Errorf("trust-breakdown cache cost = %s, want %s", cost, want)
	}
}

// TestCalculateTotalCost_CacheTierFastMode: 1h cache cost participates in the
// fast-mode 6x multiplier (it is a token cost).
func TestCalculateTotalCost_CacheTierFastMode(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral1h: 1_000_000},
		Speed:                    "fast",
	}
	cost := CalculateTotalCost(pricing, usage)
	// (1M * $10/M) * 2 = $20.00
	if want := decimal.NewFromFloat(20); !cost.Equal(want) {
		t.Errorf("fast-mode 1h cache cost = %s, want %s", cost, want)
	}
}

// TestCalculateTotalCost_CacheTier1hRateFallback: when CacheWrite1h is missing /
// zero (e.g. a stale remote pricing doc fetched before SaaS redeploys), 1h
// tokens fall back to the 5m CacheWrite rate — never $0.
func TestCalculateTotalCost_CacheTier1hRateFallback(t *testing.T) {
	t.Cleanup(func() { SetActivePricing(pricingsource.Embedded()) })
	SetActivePricing(pricingsource.Document{
		SchemaVersion: 0,
		UpdatedAt:     time.Now(),
		Pricing: map[string]map[string]pricingsource.Rate{
			// CacheWrite1h omitted (zero) — simulates a pre-redeploy remote doc.
			"claude-code": {"opus-4-8": {Input: 5, Output: 25, CacheWrite: 6.25, CacheRead: 0.5}},
		},
	})
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	usage := &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral1h: 1_000_000},
	}
	cost := CalculateTotalCost(pricing, usage)
	// CacheWrite1h missing → 1h priced at the 5m rate $6.25/M, NOT $0.
	if want := decimal.NewFromFloat(6.25); !cost.Equal(want) {
		t.Errorf("1h fallback cost = %s, want %s (5m rate, never $0)", cost, want)
	}
}

// TestCalculateTotalCost_CacheTierUnderbillingFixed encodes the billing-accuracy
// intent: an all-1h turn now costs 1.6x what the old single-rate (5m) path
// charged (2x vs 1.25x input).
func TestCalculateTotalCost_CacheTierUnderbillingFixed(t *testing.T) {
	pricing, _ := LookupPricing("claude-opus-4-8-20260515")
	oldCost := CalculateTotalCost(pricing, &TokenUsage{
		CacheCreationInputTokens: 1_000_000, // legacy single-rate (5m)
	})
	newCost := CalculateTotalCost(pricing, &TokenUsage{
		CacheCreationInputTokens: 1_000_000,
		CacheCreation:            &CacheCreationBreakdown{Ephemeral1h: 1_000_000},
	})
	if want := oldCost.Mul(decimal.NewFromFloat(1.6)); !newCost.Equal(want) {
		t.Errorf("1h cost = %s, want %s (1.6x the old 5m-rate charge)", newCost, want)
	}
}

// TestFlattenEmbeddedNoCollision verifies the embedded provider-nested table
// flattens to a family-keyed table without losing any family to a cross-provider
// key collision (the flatten/LookupPricing invariant). flatten logs-and-skips a
// family that appears under two providers, so a collision silently drops one
// provider's rate; with ~80 families that is no longer self-evident by inspection.
func TestFlattenEmbeddedNoCollision(t *testing.T) {
	doc := pricingsource.Embedded()
	flat := *flatten(doc)

	want := 0
	seen := make(map[string]string) // family → first provider that claimed it
	for provider, fams := range doc.Pricing {
		want += len(fams)
		for family := range fams {
			if other, dup := seen[family]; dup {
				t.Errorf("family %q appears under both %q and %q; flatten drops one", family, other, provider)
				continue
			}
			seen[family] = provider
		}
	}
	if len(flat) != want {
		t.Errorf("flattened family count = %d, want %d (a collision dropped a family)", len(flat), want)
	}
	// Spot-check one family from each provider survived with the right rate.
	if got := flat["opus-4-7"].Input; !got.Equal(decimal.NewFromFloat(5)) {
		t.Errorf("flat[opus-4-7].Input = %s, want 5", got)
	}
	if got := flat["gpt-5"].CacheRead; !got.Equal(decimal.NewFromFloat(0.125)) {
		t.Errorf("flat[gpt-5].CacheRead = %s, want 0.125", got)
	}
}

// TestSetActivePricingSwapsRates verifies a refreshed document changes what
// LookupPricing returns — the mechanism that lets a self-host pick up new prices
// without a redeploy.
func TestSetActivePricingSwapsRates(t *testing.T) {
	t.Cleanup(func() { SetActivePricing(pricingsource.Embedded()) }) // restore the floor

	// gpt-5 input is 1.25 in the embedded table; swap in a doc that changes it.
	updated := pricingsource.Document{
		SchemaVersion: 0,
		UpdatedAt:     time.Now(),
		Pricing: map[string]map[string]pricingsource.Rate{
			"codex": {"gpt-5": {Input: 99, Output: 10, CacheWrite: 0, CacheRead: 0.125}},
		},
	}
	SetActivePricing(updated)

	if got, _ := LookupPricing("gpt-5"); !got.Input.Equal(decimal.NewFromFloat(99)) {
		t.Errorf("after swap LookupPricing(gpt-5).Input = %s, want 99", got.Input)
	}

	SetActivePricing(pricingsource.Embedded())
	if got, _ := LookupPricing("gpt-5"); !got.Input.Equal(decimal.NewFromFloat(1.25)) {
		t.Errorf("after restore LookupPricing(gpt-5).Input = %s, want embedded 1.25", got.Input)
	}
}

// TestPricingForModel_Sonnet5_StandardRate pins the corrected Sonnet 5 rate.
// Anthropic cancelled the scheduled 2026-09-01 increase to $3/$15, so the $2/$10
// launch pricing is now the standard price and there is no date routing at all
// (khjx D2). This test is deliberately date-independent: it takes no timestamp,
// which is the property that makes the 50%-overcharge regression unrepeatable.
func TestPricingForModel_Sonnet5_StandardRate(t *testing.T) {
	for _, model := range []string{"claude-sonnet-5", "claude-sonnet-5-20260701"} {
		t.Run(model, func(t *testing.T) {
			assertRate(t, pricingForModel(nil, model), rateSpec{model, 2, 10, 2.5, 4, 0.2})
		})
	}
}

// TestEmbeddedHasNoSonnet5Intro guards the deleted introductory tier. The row
// and both routing branches were removed; a table that still carries the key
// means the machinery leaked back in.
func TestEmbeddedHasNoSonnet5Intro(t *testing.T) {
	if _, ok := embeddedTable()["sonnet-5-intro"]; ok {
		t.Error("embedded table still carries sonnet-5-intro; the introductory tier was removed (khjx D2)")
	}
}

// embeddedTable flattens the embedded document into the family-keyed table the
// lookup path actually reads.
func embeddedTable() map[string]ModelPricing {
	return *flatten(pricingsource.Embedded())
}

// rateSpec is one family's (or model's) expected per-million rates.
type rateSpec struct {
	name                  string
	in, out, cw, cw1h, cr float64
}

// assertRate checks all five per-million rates of a resolved family.
func assertRate(t *testing.T, got ModelPricing, want rateSpec) {
	t.Helper()
	for _, f := range []struct {
		field string
		got   decimal.Decimal
		want  float64
	}{
		{"input", got.Input, want.in},
		{"output", got.Output, want.out},
		{"cacheWrite", got.CacheWrite, want.cw},
		{"cacheWrite1h", got.CacheWrite1h, want.cw1h},
		{"cacheRead", got.CacheRead, want.cr},
	} {
		if w := decimal.NewFromFloat(f.want); !f.got.Equal(w) {
			t.Errorf("%s.%s = %s, want %s", want.name, f.field, f.got, w)
		}
	}
}

// TestEmbeddedRates pins every rate this ticket corrected or added, keyed by the
// family string the table is looked up with. A row here is the contract; if a
// vendor changes a price, this test and pricing.json move together.
func TestEmbeddedRates(t *testing.T) {
	tests := []rateSpec{
		// claude-code — corrections and additions. The 5.1 generation bills cache
		// hits at 0.025x base input ($0.25/MTok), not the usual 0.1x the 5.0 rows
		// below keep ($1.00) — a deliberate exception, so both generations are
		// pinned here. "Normalizing" the 5.1 rows would be a silent 4x overcharge
		// on the dominant token category in agentic sessions.
		{"sonnet-5", 2, 10, 2.5, 4, 0.2},
		{"fable-5-1", 10, 50, 12.5, 20, 0.25},
		{"mythos-5-1", 10, 50, 12.5, 20, 0.25},
		{"fable-5", 10, 50, 12.5, 20, 1.0},
		{"mythos-5", 10, 50, 12.5, 20, 1.0},

		// codex — corrections and additions.
		{"gpt-5.1", 1.25, 10.0, 0, 0, 0.125},
		{"gpt-5.2", 1.75, 14.0, 0, 0, 0.175},
		{"gpt-5.5-cyber", 12.5, 75.0, 0, 0, 1.25},
		{"gpt-5.6-sol", 4.0, 20.0, 0, 0, 0.4},
		{"gpt-5.6-cyber", 12.5, 75.0, 0, 0, 1.25},
		{"gpt-6-astra", 10.0, 50.0, 0, 0, 1.0},
		{"chat-latest", 5.0, 30.0, 0, 0, 0.5},

		// opencode — Gemini corrections and additions.
		{"gemini-2.5-pro", 1.25, 10.0, 0, 0, 0.125},
		{"gemini-2.5-flash", 0.3, 2.5, 0, 0, 0.03},
		{"gemini-2.5-flash-lite", 0.1, 0.4, 0, 0, 0.01},
		{"gemini-3.1-pro-preview", 2.0, 12.0, 0, 0, 0.2},
		{"gemini-3.8-flash", 0.75, 3.75, 0, 0, 0.075},
		{"gemini-3.7-flash", 0.75, 3.75, 0, 0, 0.075},
		{"gemini-3.6-flash", 0.75, 3.75, 0, 0, 0.075},
		{"gemini-3.5-flash", 1.5, 9.0, 0, 0, 0.15},
		{"gemini-3.5-flash-lite", 0.3, 2.5, 0, 0, 0},
		{"gemini-3.1-flash-lite", 0.25, 1.5, 0, 0, 0.025},

		// opencode — xAI Grok additions.
		{"grok-4.6", 2.0, 6.0, 0, 0, 0.5},
		{"grok-4.5", 2.0, 6.0, 0, 0, 0.3},
		{"grok-4.3", 1.25, 2.5, 0, 0, 0.2},
		{"grok-4.20-0309-reasoning", 1.25, 2.5, 0, 0, 0.2},
		{"grok-4.20-0309-non-reasoning", 1.25, 2.5, 0, 0, 0.2},
		{"grok-4.20-multi-agent-0309", 1.25, 2.5, 0, 0, 0.2},
		{"grok-build-0.1", 1.0, 2.0, 0, 0, 0.2},

		// opencode — Mistral additions. Cached input is unpublished per model,
		// so cacheRead stays 0 (cache hits bill at the full input rate).
		{"mistral-medium-3504", 1.5, 7.5, 0, 0, 0},
		{"mistral-large-2512", 0.5, 1.5, 0, 0, 0},
		{"mistral-small-2603", 0.15, 0.6, 0, 0, 0},
		{"ministral-3-14b-2512", 0.2, 0.2, 0, 0, 0},
		{"ministral-3-8b-2512", 0.15, 0.15, 0, 0, 0},
		{"ministral-3-3b-2512", 0.1, 0.1, 0, 0, 0},
		{"codestral-2508", 0.3, 0.9, 0, 0, 0},

		// opencode — DeepSeek V4 additions, stored at off-peak rates (khjx D9).
		{"deepseek-v4-flash", 0.22, 0.66, 0, 0, 0.007},
		{"deepseek-v4-pro", 0.66, 1.98, 0, 0, 0.022},
		{"deepseek-v4-flash-vision-exp", 0.22, 0.66, 0, 0, 0.007},
	}

	table := embeddedTable()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := table[tt.name]
			if !ok {
				t.Fatalf("family %q missing from the embedded pricing table", tt.name)
			}
			assertRate(t, got, tt)
		})
	}
}

// TestEmbeddedRetainsLegacyFamilies pins the retired-but-retained rows (khjx
// D10). Historical sessions still reference them, and dropping a key silently
// reprices those sessions to $0 rather than failing loudly.
func TestEmbeddedRetainsLegacyFamilies(t *testing.T) {
	table := embeddedTable()
	legacy := []string{
		"sonnet-3-7", "opus-3", "haiku-3",
		"o1-mini", "o4-mini", "gpt-4-turbo",
		"gemini-2.5-pro", "gemini-2.5-flash",
		"deepseek-v3", "deepseek-r1",
		"grok-3", "grok-3-mini",
		"mistral-large-2411", "mistral-small-2501",
	}
	for _, family := range legacy {
		if _, ok := table[family]; !ok {
			t.Errorf("legacy family %q was dropped from the pricing table; historical sessions would reprice to $0", family)
		}
	}
}

// TestFastModeMultiplier pins the fast-mode premium at 2x. Anthropic publishes
// fast mode at $10/$50 per million against a $5/$25 standard rate for Claude
// Opus 5 and Opus 4.8 — a 2x multiplier, not the 6x this constant previously
// carried (khjx D5). Caching multipliers stack on top of the fast base price,
// which is why the multiplier is applied to the whole token subtotal.
func TestFastModeMultiplier(t *testing.T) {
	if want := decimal.NewFromInt(2); !fastModeMultiplier.Equal(want) {
		t.Errorf("fastModeMultiplier = %s, want %s (published fast mode is $10/$50 vs $5/$25 standard)", fastModeMultiplier, want)
	}

	// End to end: the same usage costs exactly twice as much at "fast" speed.
	pricing, _ := LookupPricing("claude-opus-5-20260301")
	usage := func(speed string) *TokenUsage {
		return &TokenUsage{InputTokens: 1_000_000, OutputTokens: 200_000, Speed: speed}
	}
	standard := CalculateTotalCost(pricing, usage(""))
	fast := CalculateTotalCost(pricing, usage("fast"))
	if want := standard.Mul(decimal.NewFromInt(2)); !fast.Equal(want) {
		t.Errorf("fast cost = %s, want %s (2x the standard %s)", fast, want, standard)
	}
}
