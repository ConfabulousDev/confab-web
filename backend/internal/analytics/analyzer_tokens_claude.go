package analytics

import (
	"log/slog"

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
		if isMain && a.mainModel == "" && group.Model != "" && group.Model != "<synthetic>" {
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

			if usage.Speed == SpeedFast {
				a.result.FastTurns++
				a.result.FastCostUSD = a.result.FastCostUSD.Add(cost)
			}
		}
	}
}

// Result returns the accumulated token metrics.
func (a *TokensAnalyzer) Result() *TokensResult {
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
