package analytics

import "github.com/shopspring/decimal"

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
}

// ProcessFile accumulates token counts from a single file.
func (a *TokensAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.mainFile = file
		a.result.EstimatedCostUSD = decimal.Zero
		a.result.FastCostUSD = decimal.Zero
	}

	for _, group := range file.AssistantMessageGroups() {
		if group.FinalUsage == nil {
			continue
		}

		usage := group.FinalUsage
		a.result.InputTokens += usage.InputTokens
		a.result.OutputTokens += usage.OutputTokens
		a.result.CacheCreationTokens += usage.CacheCreationInputTokens
		a.result.CacheReadTokens += usage.CacheReadInputTokens

		pricing := GetPricing(group.Model)
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

			pricing := GetPricing("")
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
