package analytics

import (
	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/shopspring/decimal"
)

// computeCodexTokens fills InputTokens/OutputTokens/cache fields and cost.
//
// OpenAI semantics: CachedInputTokens is a subset of InputTokens, so we
// subtract it before billing the uncached portion at the full input rate.
// Reasoning tokens are billed as output (same rate), so they fold in there.
// CacheCreationTokens stays 0 — OpenAI doesn't charge for cache writes.
func computeCodexTokens(out *ComputeResult, r *codex.ParsedRollout) {
	tu := r.TokenUsage
	uncached := tu.InputTokens - tu.CachedInputTokens
	if uncached < 0 {
		uncached = 0
	}
	out.InputTokens = uncached
	out.CacheReadTokens = tu.CachedInputTokens
	out.CacheCreationTokens = 0
	out.OutputTokens = tu.OutputTokens + tu.ReasoningOutputTokens

	pricing := GetPricing(r.Model)
	out.EstimatedCostUSD = CalculateCost(
		pricing,
		out.InputTokens,
		out.OutputTokens,
		out.CacheCreationTokens,
		out.CacheReadTokens,
	)
	// Codex doesn't expose a "fast mode" toggle.
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero
}
