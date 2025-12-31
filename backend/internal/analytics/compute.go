package analytics

import (
	"github.com/shopspring/decimal"
)

// ComputeResult contains the computed analytics from JSONL content.
// This struct aggregates results from all analyzers.
type ComputeResult struct {
	// Token and cost stats (from TokensAnalyzer)
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal

	// Message counts (from SessionAnalyzer)
	TotalMessages     int
	UserMessages      int
	AssistantMessages int

	// Message type breakdown (from SessionAnalyzer)
	HumanPrompts   int
	ToolResults    int
	TextResponses  int
	ToolCalls      int
	ThinkingBlocks int

	// Actual conversational turns (from ConversationAnalyzer)
	UserTurns      int
	AssistantTurns int

	// Session metadata (from SessionAnalyzer)
	DurationMs *int64
	ModelsUsed []string

	// Compaction stats (from SessionAnalyzer)
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int

	// Tools stats (from ToolsAnalyzer)
	TotalToolCalls int
	ToolStats      map[string]*ToolStats
	ToolErrorCount int

	// Code activity stats (from CodeActivityAnalyzer)
	FilesRead         int
	FilesModified     int
	LinesAdded        int
	LinesRemoved      int
	SearchCount       int
	LanguageBreakdown map[string]int

	// Conversation stats (from ConversationAnalyzer)
	AvgAssistantTurnMs       *int64
	AvgUserThinkingMs        *int64
	TotalAssistantDurationMs *int64
	TotalUserDurationMs      *int64
	AssistantUtilization     *float64

	// Agent stats (from AgentsAnalyzer)
	TotalAgentInvocations int
	AgentStats            map[string]*AgentStats

	// Skill stats (from SkillsAnalyzer)
	TotalSkillInvocations int
	SkillStats            map[string]*SkillStats
}

// ComputeFromJSONL computes analytics from JSONL content.
// It uses the analyzer pattern where each analyzer processes the full file collection.
func ComputeFromJSONL(content []byte) (*ComputeResult, error) {
	// Build file collection (with empty agents for now)
	fc, err := NewFileCollection(content)
	if err != nil {
		return nil, err
	}

	return ComputeFromFileCollection(fc)
}

// ComputeFromFileCollection computes analytics from a FileCollection.
// This is the main entry point that runs all analyzers.
func ComputeFromFileCollection(fc *FileCollection) (*ComputeResult, error) {
	// Run all analyzers
	tokens, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	tools, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	codeActivity, err := (&CodeActivityAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	conversation, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	agents, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		return nil, err
	}

	skills, err := (&SkillsAnalyzer{}).Analyze(fc)
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
		DurationMs: session.DurationMs,
		ModelsUsed: session.ModelsUsed,

		// Compaction stats
		CompactionAuto:      session.CompactionAuto,
		CompactionManual:    session.CompactionManual,
		CompactionAvgTimeMs: session.CompactionAvgTimeMs,

		// Tools stats
		TotalToolCalls: tools.TotalCalls,
		ToolStats:      tools.ToolStats,
		ToolErrorCount: tools.ErrorCount,

		// Code activity stats
		FilesRead:         codeActivity.FilesRead,
		FilesModified:     codeActivity.FilesModified,
		LinesAdded:        codeActivity.LinesAdded,
		LinesRemoved:      codeActivity.LinesRemoved,
		SearchCount:       codeActivity.SearchCount,
		LanguageBreakdown: codeActivity.LanguageBreakdown,

		// Conversation stats (turns and timing)
		AvgAssistantTurnMs:       conversation.AvgAssistantTurnMs,
		AvgUserThinkingMs:        conversation.AvgUserThinkingMs,
		TotalAssistantDurationMs: conversation.TotalAssistantDurationMs,
		TotalUserDurationMs:      conversation.TotalUserDurationMs,
		AssistantUtilization:     conversation.AssistantUtilization,

		// Agent stats
		TotalAgentInvocations: agents.TotalInvocations,
		AgentStats:            agents.AgentStats,

		// Skill stats
		TotalSkillInvocations: skills.TotalInvocations,
		SkillStats:            skills.SkillStats,
	}, nil
}
