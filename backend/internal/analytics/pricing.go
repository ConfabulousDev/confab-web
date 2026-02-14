package analytics

import (
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
)

// ModelPricing contains pricing per million tokens.
// Uses 5-minute cache pricing per Anthropic's pricing page.
type ModelPricing struct {
	Input      decimal.Decimal // Per million input tokens
	Output     decimal.Decimal // Per million output tokens
	CacheWrite decimal.Decimal // Per million cache creation tokens (1.25x input)
	CacheRead  decimal.Decimal // Per million cache read tokens (0.1x input)
}

// modelPricingTable contains pricing for all model families.
// Source: https://www.anthropic.com/pricing
var modelPricingTable = map[string]ModelPricing{
	// Opus 4.6
	"opus-4-6": {
		Input:      decimal.NewFromFloat(5),
		Output:     decimal.NewFromFloat(25),
		CacheWrite: decimal.NewFromFloat(6.25),
		CacheRead:  decimal.NewFromFloat(0.50),
	},
	// Opus 4.5
	"opus-4-5": {
		Input:      decimal.NewFromFloat(5),
		Output:     decimal.NewFromFloat(25),
		CacheWrite: decimal.NewFromFloat(6.25),
		CacheRead:  decimal.NewFromFloat(0.50),
	},
	// Opus 4.1 and 4
	"opus-4-1": {
		Input:      decimal.NewFromFloat(15),
		Output:     decimal.NewFromFloat(75),
		CacheWrite: decimal.NewFromFloat(18.75),
		CacheRead:  decimal.NewFromFloat(1.50),
	},
	"opus-4": {
		Input:      decimal.NewFromFloat(15),
		Output:     decimal.NewFromFloat(75),
		CacheWrite: decimal.NewFromFloat(18.75),
		CacheRead:  decimal.NewFromFloat(1.50),
	},
	// Sonnet 4.5, 4, 3.7
	"sonnet-4-5": {
		Input:      decimal.NewFromFloat(3),
		Output:     decimal.NewFromFloat(15),
		CacheWrite: decimal.NewFromFloat(3.75),
		CacheRead:  decimal.NewFromFloat(0.30),
	},
	"sonnet-4": {
		Input:      decimal.NewFromFloat(3),
		Output:     decimal.NewFromFloat(15),
		CacheWrite: decimal.NewFromFloat(3.75),
		CacheRead:  decimal.NewFromFloat(0.30),
	},
	"sonnet-3-7": {
		Input:      decimal.NewFromFloat(3),
		Output:     decimal.NewFromFloat(15),
		CacheWrite: decimal.NewFromFloat(3.75),
		CacheRead:  decimal.NewFromFloat(0.30),
	},
	// Haiku 4.5
	"haiku-4-5": {
		Input:      decimal.NewFromFloat(1),
		Output:     decimal.NewFromFloat(5),
		CacheWrite: decimal.NewFromFloat(1.25),
		CacheRead:  decimal.NewFromFloat(0.10),
	},
	// Haiku 3.5
	"haiku-3-5": {
		Input:      decimal.NewFromFloat(0.80),
		Output:     decimal.NewFromFloat(4),
		CacheWrite: decimal.NewFromFloat(1.00),
		CacheRead:  decimal.NewFromFloat(0.08),
	},
	// Opus 3 (deprecated)
	"opus-3": {
		Input:      decimal.NewFromFloat(15),
		Output:     decimal.NewFromFloat(75),
		CacheWrite: decimal.NewFromFloat(18.75),
		CacheRead:  decimal.NewFromFloat(1.50),
	},
	// Haiku 3
	"haiku-3": {
		Input:      decimal.NewFromFloat(0.25),
		Output:     decimal.NewFromFloat(1.25),
		CacheWrite: decimal.NewFromFloat(0.30),
		CacheRead:  decimal.NewFromFloat(0.03),
	},
}

// zeroPricing is used when model is not found. Returns $0 cost rather than
// silently defaulting to a specific model's pricing.
var zeroPricing = ModelPricing{}

// getModelFamily extracts model family from full model name.
// e.g., "claude-opus-4-5-20251101" -> "opus-4-5"
// e.g., "claude-sonnet-4-20241022" -> "sonnet-4"
func getModelFamily(modelName string) string {
	// Remove "claude-" prefix if present
	name := strings.TrimPrefix(modelName, "claude-")

	// Split by dash and reconstruct
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return name
	}

	family := parts[0]
	if family != "opus" && family != "sonnet" && family != "haiku" {
		return name
	}

	// parts[1] should be major version (single digit)
	if len(parts[1]) != 1 || parts[1][0] < '0' || parts[1][0] > '9' {
		return name
	}
	major := parts[1]

	// Check for minor version in parts[2]
	// Minor version is a single digit; date suffixes are 8+ characters
	if len(parts) >= 3 && len(parts[2]) == 1 && parts[2][0] >= '0' && parts[2][0] <= '9' {
		return family + "-" + major + "-" + parts[2]
	}

	return family + "-" + major
}

// GetPricing returns pricing for a model.
// Returns zero pricing for unknown models and logs a warning.
func GetPricing(modelName string) ModelPricing {
	family := getModelFamily(modelName)
	if pricing, ok := modelPricingTable[family]; ok {
		return pricing
	}
	slog.Warn("unknown model for pricing", "model", modelName, "family", family)
	return zeroPricing
}

// oneMillion is used for price calculation (pricing is per million tokens).
var oneMillion = decimal.NewFromInt(1_000_000)

// CalculateCost calculates cost for token usage.
func CalculateCost(pricing ModelPricing, inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int64) decimal.Decimal {
	input := decimal.NewFromInt(inputTokens).Mul(pricing.Input).Div(oneMillion)
	output := decimal.NewFromInt(outputTokens).Mul(pricing.Output).Div(oneMillion)
	cacheWrite := decimal.NewFromInt(cacheWriteTokens).Mul(pricing.CacheWrite).Div(oneMillion)
	cacheRead := decimal.NewFromInt(cacheReadTokens).Mul(pricing.CacheRead).Div(oneMillion)

	return input.Add(output).Add(cacheWrite).Add(cacheRead)
}
