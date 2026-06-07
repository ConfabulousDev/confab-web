package analytics

import (
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
	"github.com/shopspring/decimal"
)

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
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
		{"gpt-5-2026-05-01", "gpt-5"},
		{"gpt-5.5-2026-04-15", "gpt-5.5"},
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
		{"claude-sonnet-4-20241022", true, 3},
		{"claude-haiku-3-5-20241022", true, 0.80},
		{"unknown-model", false, 0}, // unknown non-empty model: not found, zero pricing
		{"", false, 0},              // empty model: not found, zero pricing (expected sentinel)
		// OpenAI / Codex
		{"gpt-5", true, 1.25},
		{"gpt-5-mini", true, 0.25},
		{"gpt-5-nano", true, 0.05},
		{"gpt-5.5", true, 5.00},
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
	// Fast mode: 6x all token costs
	pricing, _ := LookupPricing("claude-opus-4-6-20260201")
	usage := &TokenUsage{
		InputTokens:  1_000_000,
		OutputTokens: 100_000,
		Speed:        "fast",
	}

	cost := CalculateTotalCost(pricing, usage)
	// Standard: input 1M * $5/M + output 100k * $25/M = $5 + $2.50 = $7.50
	// Fast: $7.50 * 6 = $45
	expected := decimal.NewFromFloat(45)
	if !cost.Equal(expected) {
		t.Errorf("Fast mode cost = %s, want %s", cost, expected)
	}
}

func TestCalculateTotalCost_FastModeWithCache(t *testing.T) {
	// Verify fast mode 6x applies to cache costs too
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
	// Fast: $6.75 * 6 = $40.50
	expected := decimal.NewFromFloat(40.50)
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
	// Token cost: 1M * $5/M = $5, fast: $5 * 6 = $30
	// Web search: 10 * $0.01 = $0.10 (NOT multiplied by fast mode)
	// Total: $30.10
	expected := decimal.NewFromFloat(30.10)
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

// TestFlattenEmbeddedNoCollision verifies the embedded provider-nested table
// flattens to a family-keyed table without losing any family to a cross-provider
// key collision (the flatten/LookupPricing invariant).
func TestFlattenEmbeddedNoCollision(t *testing.T) {
	doc := pricingsource.Embedded()
	flat := *flatten(doc)

	want := 0
	for _, fams := range doc.Pricing {
		want += len(fams)
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
