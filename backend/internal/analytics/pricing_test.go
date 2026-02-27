package analytics

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestGetModelFamily(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4-6-20260201", "opus-4-6"},
		{"claude-opus-4-5-20251101", "opus-4-5"},
		{"claude-sonnet-4-20241022", "sonnet-4"},
		{"claude-haiku-3-5-20241022", "haiku-3-5"},
		{"opus-4-5-20251101", "opus-4-5"},
		{"sonnet-3-7", "sonnet-3-7"},
		{"haiku-3", "haiku-3"},
		{"unknown-model", "unknown-model"},
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

func TestGetPricing(t *testing.T) {
	tests := []struct {
		model         string
		expectedInput float64
	}{
		{"claude-opus-4-6-20260201", 5},
		{"claude-opus-4-5-20251101", 5},
		{"claude-sonnet-4-20241022", 3},
		{"claude-haiku-3-5-20241022", 0.80},
		{"unknown-model", 0}, // unknown models return zero pricing
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			pricing := GetPricing(tt.model)
			expected := decimal.NewFromFloat(tt.expectedInput)
			if !pricing.Input.Equal(expected) {
				t.Errorf("GetPricing(%q).Input = %s, want %s", tt.model, pricing.Input, expected)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	// Test with Sonnet 4 pricing: input=$3, output=$15, cacheWrite=$3.75, cacheRead=$0.30 per million
	pricing := GetPricing("claude-sonnet-4-20241022")

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
	pricing := GetPricing("claude-sonnet-4-20241022")
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
	pricing := GetPricing("claude-opus-4-6-20260201")
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
	pricing := GetPricing("claude-opus-4-6-20260201")
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
	pricing := GetPricing("claude-sonnet-4-20241022")
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
	pricing := GetPricing("claude-opus-4-6-20260201")
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
	pricing := GetPricing("claude-sonnet-4-20241022")
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
