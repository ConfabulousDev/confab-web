package analytics

import (
	"log/slog"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/models"
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
// Pricing uses the main rollout's model. sessionAt is forwarded to pricingForModel
// for date-aware pricing; zero value routes to introductory rates (before Sep 1 2026).
func computeCodexTokens(log *slog.Logger, out *ComputeResult, rollouts []*codex.ParsedRollout, sessionAt time.Time) {
	var totalUncached, totalCached, totalOutput int64

	// Per-model accumulation for tokens_v2, grouped by model family with
	// per-rollout pricing (7eje). pricingByFamily memoizes the lookup so an
	// unknown model WARNs at most once per session regardless of rollout count.
	byModel := make(map[string]*v2ModelAgg)
	pricingByFamily := make(map[string]ModelPricing)

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

		family := getModelFamily(r.Model)
		pricing, ok := pricingByFamily[family]
		if !ok {
			pricing = pricingForModel(log, r.Model, sessionAt)
			pricingByFamily[family] = pricing
		}
		agg := byModel[family]
		if agg == nil {
			agg = &v2ModelAgg{}
			byModel[family] = agg
		}
		agg.input += uncached
		agg.output += tu.OutputTokens
		agg.cacheRead += tu.CachedInputTokens
		agg.reasoning += tu.ReasoningOutputTokens
		// Cache writes stay 0 (OpenAI bills none); reasoning is a subset of output
		// (CF-471), so it bills implicitly at the output rate — not added here.
		agg.cost = agg.cost.Add(CalculateCost(pricing, uncached, tu.OutputTokens, 0, tu.CachedInputTokens))
	}

	out.InputTokens = totalUncached
	out.CacheReadTokens = totalCached
	out.CacheCreationTokens = 0
	out.OutputTokens = totalOutput

	pricingModel := ""
	if len(rollouts) > 0 && rollouts[0] != nil {
		pricingModel = rollouts[0].Model
	}
	pricing := pricingForModel(log, pricingModel, sessionAt)
	out.EstimatedCostUSD = CalculateCost(
		pricing,
		out.InputTokens,
		out.OutputTokens,
		out.CacheCreationTokens,
		out.CacheReadTokens,
	)
	out.FastTurns = 0
	out.FastCostUSD = decimal.Zero

	out.TokensV2 = buildV2Tree(models.ProviderCodex, byModel)
}
