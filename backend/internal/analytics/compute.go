package analytics

import (
	"github.com/shopspring/decimal"
)

// ComputeResult contains the computed analytics from JSONL content.
// This struct aggregates results from all collectors.
type ComputeResult struct {
	// Token and cost stats (from TokensCollector)
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal

	// Message counts (from SessionCollector)
	TotalMessages     int
	UserMessages      int
	AssistantMessages int

	// Message type breakdown (from SessionCollector)
	HumanPrompts   int
	ToolResults    int
	TextResponses  int
	ToolCalls      int
	ThinkingBlocks int

	// Actual conversational turns (from SessionCollector)
	UserTurns      int
	AssistantTurns int

	// Session metadata (from SessionCollector)
	DurationMs *int64
	ModelsUsed []string

	// Compaction stats (from SessionCollector)
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int

	// Tools stats (from ToolsCollector)
	TotalToolCalls int
	ToolStats      map[string]*ToolStats
	ToolErrorCount int
}

// ComputeFromJSONL computes analytics from JSONL content.
// It performs a single pass through the content using the collector pattern.
func ComputeFromJSONL(content []byte) (*ComputeResult, error) {
	tokens := NewTokensCollector()
	session := NewSessionCollector()
	tools := NewToolsCollector()

	_, err := RunCollectors(content, tokens, session, tools)
	if err != nil {
		return nil, err
	}

	return &ComputeResult{
		// Token and cost stats
		InputTokens:         tokens.InputTokens,
		OutputTokens:        tokens.OutputTokens,
		CacheCreationTokens: tokens.CacheCreationTokens,
		CacheReadTokens:     tokens.CacheReadTokens,
		EstimatedCostUSD:    tokens.EstimatedCostUSD,

		// Message counts
		TotalMessages:     session.TotalMessages,
		UserMessages:      session.UserMessages,
		AssistantMessages: session.AssistantMessages,

		// Message type breakdown
		HumanPrompts:   session.HumanPrompts,
		ToolResults:    session.ToolResults,
		TextResponses:  session.TextResponses,
		ToolCalls:      session.ToolCalls,
		ThinkingBlocks: session.ThinkingBlocks,

		// Actual conversational turns
		UserTurns:      session.UserTurns,
		AssistantTurns: session.AssistantTurns,

		// Session metadata
		DurationMs: session.DurationMs(),
		ModelsUsed: session.ModelsList(),

		// Compaction stats
		CompactionAuto:      session.CompactionAuto,
		CompactionManual:    session.CompactionManual,
		CompactionAvgTimeMs: session.CompactionAvgTimeMs,

		// Tools stats
		TotalToolCalls: tools.TotalCalls,
		ToolStats:      tools.ToolStats,
		ToolErrorCount: tools.ErrorCount,
	}, nil
}
