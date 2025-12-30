package analytics

import "github.com/shopspring/decimal"

// TokensCollector accumulates token usage and cost metrics.
type TokensCollector struct {
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal
}

// NewTokensCollector creates a new TokensCollector.
func NewTokensCollector() *TokensCollector {
	return &TokensCollector{
		EstimatedCostUSD: decimal.Zero,
	}
}

// Collect processes a line for token metrics.
func (c *TokensCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	// Count tokens from assistant messages
	if line.IsAssistantMessage() {
		usage := line.Message.Usage
		c.InputTokens += usage.InputTokens
		c.OutputTokens += usage.OutputTokens
		c.CacheCreationTokens += usage.CacheCreationInputTokens
		c.CacheReadTokens += usage.CacheReadInputTokens

		// Calculate cost for this message
		pricing := GetPricing(line.Message.Model)
		cost := CalculateCost(
			pricing,
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		c.EstimatedCostUSD = c.EstimatedCostUSD.Add(cost)
	}

	// Count tokens from subagent/Task tool results (in user messages)
	// These contain cumulative token usage for the entire agent session
	for _, usage := range line.GetAgentUsage() {
		c.InputTokens += usage.InputTokens
		c.OutputTokens += usage.OutputTokens
		c.CacheCreationTokens += usage.CacheCreationInputTokens
		c.CacheReadTokens += usage.CacheReadInputTokens

		// Agent usage doesn't include model info, so we use default pricing
		// This is a reasonable approximation since agents typically use the same model
		pricing := GetPricing("")
		cost := CalculateCost(
			pricing,
			usage.InputTokens,
			usage.OutputTokens,
			usage.CacheCreationInputTokens,
			usage.CacheReadInputTokens,
		)
		c.EstimatedCostUSD = c.EstimatedCostUSD.Add(cost)
	}
}

// Finalize completes token collection (no-op for this collector).
func (c *TokensCollector) Finalize(ctx *CollectContext) {
	// No post-processing needed
}
