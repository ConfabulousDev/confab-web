package analytics

import (
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/shopspring/decimal"
)

// computeCodexTokens sums token usage across rollouts and computes cost.
// OpenAI semantics:
//   - CachedInputTokens is a subset of InputTokens; subtract it before billing
//     the uncached portion at the input rate.
//   - ReasoningOutputTokens is a subset of OutputTokens (CF-471); the wire's
//     output_tokens already includes reasoning, so we surface it unchanged.
//     Reasoning bills at the output rate implicitly.
//   - CacheCreationTokens stays 0; OpenAI doesn't charge for cache writes.
//
// Pricing uses the main rollout's model.
func computeCodexTokens(log *slog.Logger, out *ComputeResult, rollouts []*codex.ParsedRollout) {
	var totalUncached, totalCached, totalOutput int64
	for _, r := range rollouts {
		if r == nil {
			continue
		}
		tu := r.TokenUsage
		uncached := tu.InputTokens - tu.CachedInputTokens
		if uncached < 0 {
			uncached = 0
		}
		totalUncached += uncached
		totalCached += tu.CachedInputTokens
		totalOutput += tu.OutputTokens
	}
	out.InputTokens = totalUncached
	out.CacheReadTokens = totalCached
	out.CacheCreationTokens = 0
	out.OutputTokens = totalOutput

	pricingModel := ""
	if len(rollouts) > 0 && rollouts[0] != nil {
		pricingModel = rollouts[0].Model
	}
	pricing := pricingForModel(log, pricingModel)
	out.EstimatedCostUSD = CalculateCost(
		pricing,
		out.InputTokens,
		out.OutputTokens,
		out.CacheCreationTokens,
		out.CacheReadTokens,
	)
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero
}
