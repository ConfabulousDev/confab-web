package analytics

import (
	"github.com/shopspring/decimal"
)

// ComputeResult contains the computed analytics from JSONL content.
// This struct aggregates results from all collectors for backward compatibility.
type ComputeResult struct {
	// Token stats (from TokensCollector)
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64

	// Cost (from TokensCollector)
	EstimatedCostUSD decimal.Decimal

	// Compaction stats (from CompactionCollector)
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int

	// Session stats (from SessionCollector)
	UserTurns      int
	AssistantTurns int
	DurationMs     *int64
	ModelsUsed     []string

	// Tools stats (from ToolsCollector)
	TotalToolCalls int
	ToolBreakdown  map[string]int
	ToolErrorCount int
}

// ComputeFromJSONL computes analytics from JSONL content.
// It performs a single pass through the content using the collector pattern.
func ComputeFromJSONL(content []byte) (*ComputeResult, error) {
	tokens := NewTokensCollector()
	compaction := NewCompactionCollector()
	session := NewSessionCollector()
	tools := NewToolsCollector()

	_, err := RunCollectors(content, tokens, compaction, session, tools)
	if err != nil {
		return nil, err
	}

	return &ComputeResult{
		// Token stats
		InputTokens:         tokens.InputTokens,
		OutputTokens:        tokens.OutputTokens,
		CacheCreationTokens: tokens.CacheCreationTokens,
		CacheReadTokens:     tokens.CacheReadTokens,
		EstimatedCostUSD:    tokens.EstimatedCostUSD,

		// Compaction stats
		CompactionAuto:      compaction.AutoCount,
		CompactionManual:    compaction.ManualCount,
		CompactionAvgTimeMs: compaction.AvgTimeMs,

		// Session stats
		UserTurns:      session.UserTurns,
		AssistantTurns: session.AssistantTurns,
		DurationMs:     session.DurationMs(),
		ModelsUsed:     session.ModelsList(),

		// Tools stats
		TotalToolCalls: tools.TotalCalls,
		ToolBreakdown:  tools.ToolBreakdown,
		ToolErrorCount: tools.ErrorCount,
	}, nil
}
