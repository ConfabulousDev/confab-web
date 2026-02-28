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
type TokensAnalyzer struct{}

// Analyze processes the file collection and returns token metrics.
func (a *TokensAnalyzer) Analyze(fc *FileCollection) (*TokensResult, error) {
	result := &TokensResult{
		EstimatedCostUSD: decimal.Zero,
		FastCostUSD:      decimal.Zero,
	}

	// Process all files - main and agents
	for _, file := range fc.AllFiles() {
		for _, line := range file.Lines {
			if !line.IsAssistantMessage() {
				continue
			}

			usage := line.Message.Usage
			result.InputTokens += usage.InputTokens
			result.OutputTokens += usage.OutputTokens
			result.CacheCreationTokens += usage.CacheCreationInputTokens
			result.CacheReadTokens += usage.CacheReadInputTokens

			// Calculate cost with model-specific pricing (includes fast mode + server tools)
			pricing := GetPricing(line.Message.Model)
			cost := CalculateTotalCost(pricing, usage)
			result.EstimatedCostUSD = result.EstimatedCostUSD.Add(cost)

			if usage.Speed == SpeedFast {
				result.FastTurns++
				result.FastCostUSD = result.FastCostUSD.Add(cost)
			}
		}
	}

	// Fallback: count tokens from toolUseResult for agents we don't have files for
	for _, line := range fc.Main.Lines {
		for _, agentResult := range line.GetAgentResults() {
			if fc.HasAgentFile(agentResult.AgentID) {
				continue // Already counted from agent file
			}
			if agentResult.Usage == nil {
				continue
			}

			usage := agentResult.Usage
			result.InputTokens += usage.InputTokens
			result.OutputTokens += usage.OutputTokens
			result.CacheCreationTokens += usage.CacheCreationInputTokens
			result.CacheReadTokens += usage.CacheReadInputTokens

			// Agent usage doesn't include model info, so we use default pricing
			pricing := GetPricing("")
			cost := CalculateTotalCost(pricing, usage)
			result.EstimatedCostUSD = result.EstimatedCostUSD.Add(cost)

			if usage.Speed == SpeedFast {
				result.FastTurns++
				result.FastCostUSD = result.FastCostUSD.Add(cost)
			}
		}
	}

	return result, nil
}
