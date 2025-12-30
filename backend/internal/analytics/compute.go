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

	// Actual conversational turns (from ConversationCollector)
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

	// Code activity stats (from CodeActivityCollector)
	FilesRead         int
	FilesModified     int
	LinesAdded        int
	LinesRemoved      int
	SearchCount       int
	LanguageBreakdown map[string]int

	// Conversation stats (from ConversationCollector)
	AvgAssistantTurnMs *int64
	AvgUserThinkingMs  *int64
}

// ComputeFromJSONL computes analytics from JSONL content.
// It performs a single pass through the content using the collector pattern.
func ComputeFromJSONL(content []byte) (*ComputeResult, error) {
	tokens := NewTokensCollector()
	session := NewSessionCollector()
	tools := NewToolsCollector()
	codeActivity := NewCodeActivityCollector()
	conversation := NewConversationCollector()

	_, err := RunCollectors(content, tokens, session, tools, codeActivity, conversation)
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
		UserTurns:      conversation.UserTurns,
		AssistantTurns: conversation.AssistantTurns,

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

		// Code activity stats
		FilesRead:         codeActivity.FilesRead(),
		FilesModified:     codeActivity.FilesModified(),
		LinesAdded:        codeActivity.LinesAdded,
		LinesRemoved:      codeActivity.LinesRemoved,
		SearchCount:       codeActivity.SearchCount,
		LanguageBreakdown: codeActivity.LanguageBreakdown(),

		// Conversation stats (turns and timing)
		AvgAssistantTurnMs: conversation.AvgAssistantTurnMs,
		AvgUserThinkingMs:  conversation.AvgUserThinkingMs,
	}, nil
}
