package analytics

import (
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
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

// activePricing holds the flat family→pricing table currently in effect, keyed
// by family ("opus-4-7", "gpt-5", ...). It defaults to the embedded floor
// (pricingsource.Embedded) and is swapped atomically by SetActivePricing when
// the precompute worker pulls a newer remote table. GetPricing reads it
// lock-free, so price updates land without a redeploy.
//
// The single source of truth for the data is internal/pricingsource/pricing.json.
var activePricing atomic.Pointer[map[string]ModelPricing]

func init() {
	activePricing.Store(flatten(pricingsource.Embedded()))
}

// SetActivePricing swaps in a (validated) pricing document. The precompute
// worker calls this with pricingsource.Effective() at the start of each cycle
// so newly analyzed sessions cost out at the freshest prices.
func SetActivePricing(doc pricingsource.Document) {
	activePricing.Store(flatten(doc))
}

// flatten collapses the provider-nested document into a family-keyed table.
// Family keys are unique across providers (Claude families like "opus-4-7" vs
// OpenAI names like "gpt-5" are disjoint); a collision in a fetched document is
// logged and the duplicate skipped (the embedded doc is collision-free by test).
func flatten(doc pricingsource.Document) *map[string]ModelPricing {
	table := make(map[string]ModelPricing)
	for provider, families := range doc.Pricing {
		for family, r := range families {
			if _, dup := table[family]; dup {
				slog.Warn("duplicate pricing family across providers; skipping", "family", family, "provider", provider)
				continue
			}
			table[family] = ModelPricing{
				Input:      decimal.NewFromFloat(r.Input),
				Output:     decimal.NewFromFloat(r.Output),
				CacheWrite: decimal.NewFromFloat(r.CacheWrite),
				CacheRead:  decimal.NewFromFloat(r.CacheRead),
			}
		}
	}
	return &table
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

// GetPricing returns pricing for a model from the currently-active table.
// Returns zero pricing for unknown models and logs a warning.
func GetPricing(modelName string) ModelPricing {
	family := getModelFamily(modelName)
	table := *activePricing.Load()
	if pricing, ok := table[family]; ok {
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
