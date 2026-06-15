package analytics

import (
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/shopspring/decimal"
)

// TokensResult contains token usage and cost metrics.
type TokensResult struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal

	// Fast mode breakdown
	FastTurns   int
	FastCostUSD decimal.Decimal

	// TokensV2 is the per-provider → per-model breakdown built alongside the flat
	// totals. by_provider has a single canonical-agent key ("claude-code"); model
	// keys are getModelFamily() families, with fast turns under "<family> · fast".
	// Its TotalCostUSD reconciles exactly with EstimatedCostUSD.
	TokensV2 *TokensV2Data
}

// TokensAnalyzer extracts token usage and cost metrics from transcripts.
// It processes all files (main + agents) for accurate model-specific pricing.
// Falls back to toolUseResult.usage for agents without files.
type TokensAnalyzer struct {
	result   TokensResult
	mainFile *TranscriptFile
	// mainModel is the main session's model, used to price file-less sub-agents
	// (their usage carries no model name). Captured from the main transcript.
	mainModel string
	// log is the session-scoped logger (enriched upstream with session_id +
	// provider) used to attribute unknown-model warnings. Nil on test/Analyze
	// paths; pricingForModel falls back to the default logger.
	log *slog.Logger
	// byModel accumulates the per-model tokens_v2 breakdown alongside the flat
	// totals, keyed by getModelFamily() family (fast turns under "<family> · fast").
	// Lazily initialized so a zero-value analyzer needs no constructor.
	byModel map[string]*v2ModelAgg
}

// v2ModelAgg accumulates one model family's tokens + cost for the tokens_v2 tree.
// reasoning stays 0 for Claude (its usage carries none) and is populated by Codex;
// cacheCreation stays 0 for Codex (OpenAI doesn't bill cache writes).
type v2ModelAgg struct {
	input, output, cacheCreation, cacheRead, reasoning int64
	cost                                               decimal.Decimal
}

// buildV2Tree assembles a single-provider tokens_v2 tree from already-aggregated
// per-model entries, keyed under the canonical agent id. Returns nil for a
// token-less session so the card stays empty (unserved), matching the prior
// Claude/Codex behavior. The two providers accumulate byModel at their own sites
// (Claude in ProcessFile/Finalize, Codex in computeCodexTokens) — this only does
// the mechanical map→tree assembly they shared.
func buildV2Tree(providerID string, byModel map[string]*v2ModelAgg) *TokensV2Data {
	if len(byModel) == 0 {
		return nil
	}
	modelEntries := make(map[string]TokensV2Model, len(byModel))
	var totalInput, totalOutput, totalCacheCreation, totalCacheRead int64
	totalCost := decimal.Zero
	for key, agg := range byModel {
		modelEntries[key] = TokensV2Model{
			Input:      agg.input,
			Output:     agg.output,
			CacheRead:  agg.cacheRead,
			CacheWrite: agg.cacheCreation,
			Reasoning:  agg.reasoning,
			CostUSD:    agg.cost.String(),
		}
		totalInput += agg.input
		totalOutput += agg.output
		totalCacheCreation += agg.cacheCreation
		totalCacheRead += agg.cacheRead
		totalCost = totalCost.Add(agg.cost)
	}
	return &TokensV2Data{
		TotalCostUSD:       totalCost.String(),
		TotalInput:         totalInput,
		TotalOutput:        totalOutput,
		TotalCacheCreation: totalCacheCreation,
		TotalCacheRead:     totalCacheRead,
		ByProvider: map[string]TokensV2Provider{
			providerID: {CostUSD: totalCost.String(), Models: modelEntries},
		},
	}
}

// accumulateV2 folds one group/agent's usage into the per-model tree. The cost is
// the same value already added to the flat total, so the v2 tree reconciles with
// EstimatedCostUSD exactly. Fast turns route to a synthetic "<family> · fast" key.
func (a *TokensAnalyzer) accumulateV2(model string, fast bool, usage *TokenUsage, cost decimal.Decimal) {
	// <synthetic> turns carry no real model/usage ($0, 0 tokens); exclude them from
	// the v2 tree at the source so no recomputed session surfaces a synthetic model
	// entry on any card (xz6g). A synthetic-only session thus leaves byModel empty,
	// and buildV2Tree returns nil — an unserved/empty card, matching the flat path.
	if model == syntheticModelKey {
		return
	}
	key := getModelFamily(model)
	if fast {
		key += fastModelKeySuffix
	}
	if a.byModel == nil {
		a.byModel = make(map[string]*v2ModelAgg)
	}
	agg := a.byModel[key]
	if agg == nil {
		agg = &v2ModelAgg{}
		a.byModel[key] = agg
	}
	agg.input += usage.InputTokens
	agg.output += usage.OutputTokens
	agg.cacheCreation += usage.CacheCreationInputTokens
	agg.cacheRead += usage.CacheReadInputTokens
	agg.cost = agg.cost.Add(cost)
}

// ProcessFile accumulates token counts from a single file.
func (a *TokensAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.mainFile = file
		a.result.EstimatedCostUSD = decimal.Zero
		a.result.FastCostUSD = decimal.Zero
	}

	for _, group := range file.AssistantMessageGroups() {
		// Capture the first concrete main-session model; file-less sub-agents
		// inherit it for pricing (see Finalize).
		if isMain && a.mainModel == "" && group.Model != "" && group.Model != syntheticModelKey {
			a.mainModel = group.Model
		}

		if group.FinalUsage == nil {
			continue
		}

		usage := group.FinalUsage
		a.result.InputTokens += usage.InputTokens
		a.result.OutputTokens += usage.OutputTokens
		a.result.CacheCreationTokens += usage.CacheCreationInputTokens
		a.result.CacheReadTokens += usage.CacheReadInputTokens

		pricing := pricingForModel(a.log, group.Model)
		cost := CalculateTotalCost(pricing, usage)
		a.result.EstimatedCostUSD = a.result.EstimatedCostUSD.Add(cost)
		a.accumulateV2(group.Model, group.IsFastMode, usage, cost)

		if group.IsFastMode {
			a.result.FastTurns++
			a.result.FastCostUSD = a.result.FastCostUSD.Add(cost)
		}
	}
}

// Finalize runs fallback logic for agents without files.
func (a *TokensAnalyzer) Finalize(hasAgentFile func(string) bool) {
	if a.mainFile == nil {
		return
	}
	for _, line := range a.mainFile.Lines {
		for _, agentResult := range line.GetAgentResults() {
			if hasAgentFile(agentResult.AgentID) {
				continue
			}
			if agentResult.Usage == nil {
				continue
			}

			usage := agentResult.Usage
			a.result.InputTokens += usage.InputTokens
			a.result.OutputTokens += usage.OutputTokens
			a.result.CacheCreationTokens += usage.CacheCreationInputTokens
			a.result.CacheReadTokens += usage.CacheReadInputTokens

			// File-less sub-agents carry token usage but no model name, so price
			// them at the main session model they were spawned under. If the main
			// session itself has no resolvable model, pricingForModel treats the
			// empty name as an expected sentinel (zero cost, DEBUG, no WARN).
			pricing := pricingForModel(a.log, a.mainModel)
			cost := CalculateTotalCost(pricing, usage)
			a.result.EstimatedCostUSD = a.result.EstimatedCostUSD.Add(cost)
			a.accumulateV2(a.mainModel, usage.Speed == SpeedFast, usage, cost)

			if usage.Speed == SpeedFast {
				a.result.FastTurns++
				a.result.FastCostUSD = a.result.FastCostUSD.Add(cost)
			}
		}
	}
}

// Result returns the accumulated token metrics, including the per-model tokens_v2
// tree assembled from byModel. Idempotent: callers (claude_compute, Analyze) may
// call it once after ProcessFile/Finalize have run.
func (a *TokensAnalyzer) Result() *TokensResult {
	a.result.TokensV2 = buildV2Tree(models.ProviderClaudeCode, a.byModel)
	return &a.result
}

// Analyze processes the file collection and returns token metrics.
func (a *TokensAnalyzer) Analyze(fc *FileCollection) (*TokensResult, error) {
	a.ProcessFile(fc.Main, true)
	for _, agent := range fc.Agents {
		a.ProcessFile(agent, false)
	}
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}
