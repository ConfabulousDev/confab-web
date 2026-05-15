package analytics

import (
	"log/slog"
	"regexp"
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

// modelPricingTable contains pricing for all supported model families.
//
// Claude entries are keyed by family ("opus-4-7", "sonnet-4-6", ...) after
// stripping the "claude-" prefix and trailing date suffix.
//
// OpenAI/Codex entries are keyed by their full short name ("gpt-5",
// "gpt-5.5", "o3-mini") because OpenAI uses both dashes and dots in version
// numbers. Date-pinned snapshots (e.g. "gpt-5-2026-05-01") are normalized
// down to the unpinned key via stripOpenAIDateSuffix.
//
// Anthropic source: https://www.anthropic.com/pricing
// OpenAI source: https://developers.openai.com/api/docs/pricing
var modelPricingTable = map[string]ModelPricing{
	// Opus 4.7
	"opus-4-7": {
		Input:      decimal.NewFromFloat(5),
		Output:     decimal.NewFromFloat(25),
		CacheWrite: decimal.NewFromFloat(6.25),
		CacheRead:  decimal.NewFromFloat(0.50),
	},
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
	// Sonnet 4.6, 4.5, 4, 3.7
	"sonnet-4-6": {
		Input:      decimal.NewFromFloat(3),
		Output:     decimal.NewFromFloat(15),
		CacheWrite: decimal.NewFromFloat(3.75),
		CacheRead:  decimal.NewFromFloat(0.30),
	},
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

	// =====================
	// OpenAI / Codex models
	// =====================
	// All entries set CacheWrite=0: OpenAI's prompt caching is automatic and
	// free to write. Cached tokens are billed at the documented "cached input"
	// rate, which the adapter handles by subtracting CachedInputTokens from
	// InputTokens before applying the uncached rate.
	// Source: https://developers.openai.com/api/docs/models/<model> (May 2026).

	"gpt-5": {
		Input:      decimal.NewFromFloat(1.25),
		Output:     decimal.NewFromFloat(10.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.125),
	},
	"gpt-5-mini": {
		Input:      decimal.NewFromFloat(0.25),
		Output:     decimal.NewFromFloat(2.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.025),
	},
	"gpt-5-nano": {
		Input:      decimal.NewFromFloat(0.05),
		Output:     decimal.NewFromFloat(0.40),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.005),
	},
	"gpt-5.4-mini": {
		Input:      decimal.NewFromFloat(0.75),
		Output:     decimal.NewFromFloat(4.50),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.075),
	},
	"gpt-5.5": {
		// Note: prompts >272K input tokens are billed at 2x input / 1.5x output
		// for the session. Per-session size escalation is not currently modeled.
		Input:      decimal.NewFromFloat(5.00),
		Output:     decimal.NewFromFloat(30.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.50),
	},
	"gpt-4o": {
		Input:      decimal.NewFromFloat(2.50),
		Output:     decimal.NewFromFloat(10.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(1.25),
	},
	"gpt-4o-mini": {
		Input:      decimal.NewFromFloat(0.15),
		Output:     decimal.NewFromFloat(0.60),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.075),
	},
	"gpt-4-turbo": {
		// Deprecated; no cached-input rate documented.
		Input:      decimal.NewFromFloat(10.00),
		Output:     decimal.NewFromFloat(30.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0),
	},
	"o1": {
		Input:      decimal.NewFromFloat(15.00),
		Output:     decimal.NewFromFloat(60.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(7.50),
	},
	"o1-mini": {
		Input:      decimal.NewFromFloat(1.10),
		Output:     decimal.NewFromFloat(4.40),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.55),
	},
	"o3": {
		Input:      decimal.NewFromFloat(2.00),
		Output:     decimal.NewFromFloat(8.00),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.50),
	},
	"o3-mini": {
		Input:      decimal.NewFromFloat(1.10),
		Output:     decimal.NewFromFloat(4.40),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.55),
	},
	"o4-mini": {
		Input:      decimal.NewFromFloat(1.10),
		Output:     decimal.NewFromFloat(4.40),
		CacheWrite: decimal.NewFromFloat(0),
		CacheRead:  decimal.NewFromFloat(0.275),
	},
}

// zeroPricing is used when model is not found. Returns $0 cost rather than
// silently defaulting to a specific model's pricing.
var zeroPricing = ModelPricing{}

// openAIDateSuffix matches the YYYY-MM-DD suffix OpenAI sometimes appends to
// pinned model snapshots (e.g. "gpt-5-2026-05-01"). Stripping it normalizes
// the name to its family key.
var openAIDateSuffix = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

// stripOpenAIDateSuffix removes a trailing -YYYY-MM-DD if present.
// Pure-version names like "gpt-5" or "gpt-5.5" are returned unchanged.
func stripOpenAIDateSuffix(name string) string {
	return openAIDateSuffix.ReplaceAllString(name, "")
}

// isOpenAIModel returns true for model names that belong to OpenAI families.
// We pass these through getModelFamily unchanged (after date-suffix stripping)
// because their naming convention uses both dashes and dots, unlike Claude.
func isOpenAIModel(name string) bool {
	return strings.HasPrefix(name, "gpt-") || strings.HasPrefix(name, "o1") ||
		strings.HasPrefix(name, "o3") || strings.HasPrefix(name, "o4")
}

// getModelFamily extracts the pricing-table key from a full model name.
//   - Claude:  "claude-opus-4-5-20251101" -> "opus-4-5"
//   - OpenAI:  "gpt-5-2026-05-01"          -> "gpt-5"
//   - OpenAI:  "gpt-5.5"                   -> "gpt-5.5" (pass-through)
func getModelFamily(modelName string) string {
	if isOpenAIModel(modelName) {
		return stripOpenAIDateSuffix(modelName)
	}

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

// Server tool pricing (per request, not per token).
// Source: https://docs.anthropic.com/en/about-claude/pricing
var webSearchPricePerRequest = decimal.NewFromFloat(0.01) // $10 per 1,000 searches

// fastModeMultiplier is applied to all token costs when speed is "fast".
// Source: https://docs.anthropic.com/en/build-with-claude/fast-mode
var fastModeMultiplier = decimal.NewFromInt(6)

// CalculateCost calculates token-only cost for the given counts.
func CalculateCost(pricing ModelPricing, inputTokens, outputTokens, cacheWriteTokens, cacheReadTokens int64) decimal.Decimal {
	input := decimal.NewFromInt(inputTokens).Mul(pricing.Input).Div(oneMillion)
	output := decimal.NewFromInt(outputTokens).Mul(pricing.Output).Div(oneMillion)
	cacheWrite := decimal.NewFromInt(cacheWriteTokens).Mul(pricing.CacheWrite).Div(oneMillion)
	cacheRead := decimal.NewFromInt(cacheReadTokens).Mul(pricing.CacheRead).Div(oneMillion)

	return input.Add(output).Add(cacheWrite).Add(cacheRead)
}

// CalculateTotalCost calculates the full cost including token costs,
// fast mode multiplier, and server tool per-request charges.
func CalculateTotalCost(pricing ModelPricing, usage *TokenUsage) decimal.Decimal {
	cost := CalculateCost(
		pricing,
		usage.InputTokens,
		usage.OutputTokens,
		usage.CacheCreationInputTokens,
		usage.CacheReadInputTokens,
	)

	// Fast mode: 6x all token costs
	if usage.Speed == SpeedFast {
		cost = cost.Mul(fastModeMultiplier)
	}

	// Server tool costs (per-request pricing, not affected by fast mode)
	if usage.ServerToolUse != nil {
		searches := decimal.NewFromInt(int64(usage.ServerToolUse.WebSearchRequests))
		cost = cost.Add(searches.Mul(webSearchPricePerRequest))
	}

	return cost
}
