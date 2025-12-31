package analytics

import "github.com/shopspring/decimal"

// TokensResult contains token usage and cost metrics.
type TokensResult struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal
}

// TokensAnalyzer extracts token usage and cost metrics from transcripts.
// It only processes the main transcript - agent tokens are included via
// toolUseResult.usage which is already in the main transcript.
type TokensAnalyzer struct{}

// Analyze processes the file collection and returns token metrics.
func (a *TokensAnalyzer) Analyze(fc *FileCollection) (*TokensResult, error) {
	result := &TokensResult{
		EstimatedCostUSD: decimal.Zero,
	}

	// Only process main transcript - agent tokens come from toolUseResult.usage
	for _, line := range fc.Main.Lines {
		// Count tokens from assistant messages
		if line.IsAssistantMessage() {
			usage := line.Message.Usage
			result.InputTokens += usage.InputTokens
			result.OutputTokens += usage.OutputTokens
			result.CacheCreationTokens += usage.CacheCreationInputTokens
			result.CacheReadTokens += usage.CacheReadInputTokens

			// Calculate cost for this message
			pricing := GetPricing(line.Message.Model)
			cost := CalculateCost(
				pricing,
				usage.InputTokens,
				usage.OutputTokens,
				usage.CacheCreationInputTokens,
				usage.CacheReadInputTokens,
			)
			result.EstimatedCostUSD = result.EstimatedCostUSD.Add(cost)
		}

		// Count tokens from subagent/Task tool results (in user messages)
		// These contain cumulative token usage for the entire agent session
		for _, usage := range line.GetAgentUsage() {
			result.InputTokens += usage.InputTokens
			result.OutputTokens += usage.OutputTokens
			result.CacheCreationTokens += usage.CacheCreationInputTokens
			result.CacheReadTokens += usage.CacheReadInputTokens

			// Agent usage doesn't include model info, so we use default pricing
			pricing := GetPricing("")
			cost := CalculateCost(
				pricing,
				usage.InputTokens,
				usage.OutputTokens,
				usage.CacheCreationInputTokens,
				usage.CacheReadInputTokens,
			)
			result.EstimatedCostUSD = result.EstimatedCostUSD.Add(cost)
		}
	}

	return result, nil
}
