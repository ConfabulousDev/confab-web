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
	if !line.IsAssistantMessage() {
		return
	}

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

// Finalize completes token collection (no-op for this collector).
func (c *TokensCollector) Finalize(ctx *CollectContext) {
	// No post-processing needed
}
