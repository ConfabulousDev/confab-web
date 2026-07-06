package analytics

import (
	"log/slog"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/pricingsource"
	"github.com/shopspring/decimal"
)

// ModelPricing contains pricing per million tokens.
// Uses 5-minute cache pricing per Anthropic's pricing page.
type ModelPricing struct {
	Input        decimal.Decimal // Per million input tokens
	Output       decimal.Decimal // Per million output tokens
	CacheWrite   decimal.Decimal // Per million 5-minute cache creation tokens (1.25x input)
	CacheWrite1h decimal.Decimal // Per million 1-hour cache creation tokens (2x input)
	CacheRead    decimal.Decimal // Per million cache read tokens (0.1x input)
}

// activePricing holds the flat family→pricing table currently in effect, keyed
// by family ("opus-4-7", "gpt-5", ...). It defaults to the embedded floor
// (pricingsource.Embedded) and is swapped atomically by SetActivePricing when
// the precompute worker pulls a newer remote table. LookupPricing reads it
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
				Input:        decimal.NewFromFloat(r.Input),
				Output:       decimal.NewFromFloat(r.Output),
				CacheWrite:   decimal.NewFromFloat(r.CacheWrite),
				CacheWrite1h: decimal.NewFromFloat(r.CacheWrite1h),
				CacheRead:    decimal.NewFromFloat(r.CacheRead),
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
//   - Claude:  "claude-fable-5"           -> "fable-5"
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
	if family != "opus" && family != "sonnet" && family != "haiku" && family != "fable" {
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

// LookupPricing resolves a model's pricing from the currently-active table.
// It is pure: no logging, no side effects. The bool reports whether the model
// was recognized. An empty model name is an expected sentinel (some token
// sources, e.g. file-less Claude sub-agents, legitimately carry no model) and
// returns (zeroPricing, false). Callers decide how to surface a miss.
func LookupPricing(modelName string) (ModelPricing, bool) {
	if modelName == "" {
		return zeroPricing, false
	}
	family := getModelFamily(modelName)
	table := *activePricing.Load()
	if pricing, ok := table[family]; ok {
		return pricing, true
	}
	return zeroPricing, false
}

// sonnet5Sep1 is the boundary between Sonnet 5 introductory and standard pricing.
// Sessions whose first_seen is before this instant use the "sonnet-5-intro" rates
// ($2 input, $10 output); sessions on or after use the "sonnet-5" standard rates
// ($3 input, $15 output). The introductory period runs through Aug 31, 2026.
var sonnet5Sep1 = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// pricingForModel resolves pricing and applies the project's logging policy for
// misses, attributing them to the given logger (which upstream enriches with
// session_id + provider so a warning is traceable):
//   - known model  → its pricing (with Sonnet 5 date-routing, see below).
//   - empty model   → zero pricing, DEBUG only. Empty is an expected sentinel, not
//     an anomaly, so it must never spam WARN during precompute.
//   - non-empty but unknown → zero pricing, WARN. This is a genuine gap in
//     pricing.json worth surfacing loudly (carries model + family + context).
//
// sessionAt is the session's first_seen timestamp, used to route Sonnet 5 sessions
// to the correct introductory or standard pricing tier. A zero time.Time (year 0001)
// is before Sep 1 2026, so callers without a real timestamp correctly route to
// intro rates — acceptable for test paths and convenience wrappers.
//
// A nil logger falls back to the default logger (test/Analyze paths that don't
// thread a session-scoped logger).
func pricingForModel(log *slog.Logger, modelName string, sessionAt time.Time) ModelPricing {
	if log == nil {
		log = slog.Default()
	}

	// Sonnet 5 date-aware routing: sessions starting before 2026-09-01 use the
	// introductory rates stored under "sonnet-5-intro"; on or after that date they
	// use the standard "sonnet-5" rates. getModelFamily is called once here to avoid
	// a second call in LookupPricing below.
	family := getModelFamily(modelName)
	if family == "sonnet-5" && sessionAt.Before(sonnet5Sep1) {
		table := *activePricing.Load()
		if p, ok := table["sonnet-5-intro"]; ok {
			return p
		}
	}

	pricing, ok := LookupPricing(modelName)
	if ok {
		return pricing
	}
	if modelName == "" {
		log.Debug("skipping pricing lookup: empty model")
		return zeroPricing
	}
	log.Warn("unknown model for pricing", "model", modelName, "family", family)
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

// effectiveCacheWrite1h is the rate billed for 1-hour cache-creation tokens.
// It falls back to the 5-minute CacheWrite rate when CacheWrite1h is missing or
// zero (e.g. a remote pricing doc fetched before the SaaS redeploys the new
// rate), so 1h tokens degrade to the old behavior rather than billing $0 (rd9v).
func effectiveCacheWrite1h(pricing ModelPricing) decimal.Decimal {
	if pricing.CacheWrite1h.IsPositive() {
		return pricing.CacheWrite1h
	}
	return pricing.CacheWrite
}

// CalculateTotalCost calculates the full cost including token costs,
// fast mode multiplier, and server tool per-request charges.
func CalculateTotalCost(pricing ModelPricing, usage *TokenUsage) decimal.Decimal {
	// Split cache-creation tokens by ephemeral tier when the breakdown is
	// present: 5-minute writes bill at CacheWrite, 1-hour writes at the
	// effective 1h rate. When absent (legacy lines, codex/opencode) all
	// cache-creation bills at the 5-minute rate, preserving prior behavior.
	// The breakdown is trusted over the flat count when both are present.
	cache5m := usage.CacheCreationInputTokens
	var cache1h int64
	if usage.CacheCreation != nil {
		cache5m = usage.CacheCreation.Ephemeral5m
		cache1h = usage.CacheCreation.Ephemeral1h
	}

	cost := CalculateCost(
		pricing,
		usage.InputTokens,
		usage.OutputTokens,
		cache5m,
		usage.CacheReadInputTokens,
	)
	cost = cost.Add(decimal.NewFromInt(cache1h).Mul(effectiveCacheWrite1h(pricing)).Div(oneMillion))

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
