package analytics

import (
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/shopspring/decimal"
)

// computeCodexTokens sums token usage across rollouts and computes cost.
// OpenAI semantics:
//   - CachedInputTokens and CacheWriteInputTokens are both SUBSETS of
//     InputTokens (OpenAI's prompt-caching guide; Codex maps the wire's
//     input_tokens_details.cache_write_tokens straight through). Ordinary input
//     is input - cached - cacheWrite, clamped at 0, and bills at the input rate.
//   - Cache writes surface as CacheCreationTokens and bill at the table's
//     cacheWrite rate, which is OpenAI's 30-minute tier (1.25x input). The
//     rollout carries no 30m/12h split, so an explicitly requested 12-hour write
//     (2.5x) is undercharged at the 30-minute rate.
//   - Models priced with cacheWrite = 0 (everything before gpt-5.6) get no
//     fallback rate: their writes leave ordinary input and bill at $0. No such
//     model has ever reported a write.
//   - ReasoningOutputTokens is a subset of OutputTokens (CF-471); the wire's
//     output_tokens already includes reasoning, so we surface it unchanged.
//     Reasoning bills at the output rate implicitly.
//
// Pricing uses the main rollout's model.
func computeCodexTokens(log *slog.Logger, out *ComputeResult, rollouts []*codex.ParsedRollout) {
	var totalUncached, totalCached, totalCacheWrite, totalOutput int64

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
		cacheWrite := tu.CacheWriteInputTokens
		uncached := max(tu.InputTokens-tu.CachedInputTokens-cacheWrite, 0)
		totalUncached += uncached
		totalCached += tu.CachedInputTokens
		totalCacheWrite += cacheWrite
		totalOutput += tu.OutputTokens

		family := getModelFamily(r.Model)
		pricing, ok := pricingByFamily[family]
		if !ok {
			pricing = pricingForModel(log, r.Model)
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
		agg.cacheCreation += cacheWrite
		agg.reasoning += tu.ReasoningOutputTokens
		// Reasoning is a subset of output (CF-471), so it bills implicitly at the
		// output rate and is not added here.
		agg.cost = agg.cost.Add(CalculateCost(pricing, uncached, tu.OutputTokens, cacheWrite, tu.CachedInputTokens))
	}

	out.InputTokens = totalUncached
	out.CacheReadTokens = totalCached
	out.CacheCreationTokens = totalCacheWrite
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

	out.TokensV2 = buildV2Tree(models.ProviderCodex, byModel)
}
