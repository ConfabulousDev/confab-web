package analytics

import "github.com/shopspring/decimal"

// ComputeResult contains the computed analytics from a session.
// It is the provider-agnostic aggregate produced by both the Claude
// orchestrator (ComputeStreaming in claude_compute.go) and the Codex
// orchestrator (ComputeFromCodexRollout in codex_compute.go), then mapped
// onto the per-card DB records by store.go.
type ComputeResult struct {
	// Token and cost stats (from TokensAnalyzer)
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	EstimatedCostUSD    decimal.Decimal

	// Fast mode breakdown (from TokensAnalyzer)
	FastTurns   int
	FastCostUSD decimal.Decimal

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
	AssistantUtilizationPct  *float64

	// Agent stats (from AgentsAnalyzer)
	TotalAgentInvocations int
	AgentStats            map[string]*AgentStats

	// Skill stats (from SkillsAnalyzer)
	TotalSkillInvocations int
	SkillStats            map[string]*SkillStats

	// Redaction stats (from RedactionsAnalyzer)
	TotalRedactions int
	RedactionCounts map[string]int

	// Workflow runs (from WorkflowsAnalyzer; empty for non-workflow sessions)
	Workflows []WorkflowRun

	// Validation stats (from parsing)
	ValidationErrorCount int

	// Per-card computation errors (graceful degradation)
	CardErrors map[string]string

	// Streaming stats
	SkippedAgentFiles int // Number of agent files skipped (cap exceeded, download errors)
}
