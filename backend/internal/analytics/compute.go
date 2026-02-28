package analytics

import (
	"context"

	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// runAnalyzer executes an analyzer function within a traced span.
func runAnalyzer[T any](ctx context.Context, name string, fn func() (*T, error)) (*T, error) {
	_, span := tracer.Start(ctx, "analytics.analyze_"+name,
		trace.WithAttributes(attribute.String("analyzer.name", name)))
	defer span.End()

	result, err := fn()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return result, nil
}

// ComputeResult contains the computed analytics from JSONL content.
// This struct aggregates results from all analyzers.
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
	TotalUserDurationMs     *int64
	AssistantUtilizationPct *float64

	// Agent stats (from AgentsAnalyzer)
	TotalAgentInvocations int
	AgentStats            map[string]*AgentStats

	// Skill stats (from SkillsAnalyzer)
	TotalSkillInvocations int
	SkillStats            map[string]*SkillStats

	// Redaction stats (from RedactionsAnalyzer)
	TotalRedactions int
	RedactionCounts map[string]int

	// Validation stats (from parsing)
	ValidationErrorCount int

	// Per-card computation errors (graceful degradation)
	CardErrors map[string]string
}

// ComputeFromJSONL computes analytics from JSONL content.
// It uses the analyzer pattern where each analyzer processes the full file collection.
func ComputeFromJSONL(ctx context.Context, content []byte) (*ComputeResult, error) {
	// Build file collection (with empty agents for now)
	fc, err := NewFileCollection(content)
	if err != nil {
		return nil, err
	}

	return ComputeFromFileCollection(ctx, fc)
}

// ComputeFromFileCollection computes analytics from a FileCollection.
// This is the main entry point that runs all analyzers.
// Uses collect-errors pattern: individual card failures don't fail the whole computation.
func ComputeFromFileCollection(ctx context.Context, fc *FileCollection) (*ComputeResult, error) {
	ctx, span := tracer.Start(ctx, "analytics.compute",
		trace.WithAttributes(
			attribute.Int("file.count", 1+len(fc.Agents)),
			attribute.Int64("main.lines", int64(len(fc.Main.Lines))),
		))
	defer span.End()

	cardErrors := make(map[string]string)

	// Run all analyzers with individual spans, collecting errors instead of returning early
	tokens, err := runAnalyzer(ctx, "tokens", func() (*TokensResult, error) {
		return (&TokensAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["tokens"] = err.Error()
	}

	session, err := runAnalyzer(ctx, "session", func() (*SessionResult, error) {
		return (&SessionAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["session"] = err.Error()
	}

	tools, err := runAnalyzer(ctx, "tools", func() (*ToolsResult, error) {
		return (&ToolsAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["tools"] = err.Error()
	}

	codeActivity, err := runAnalyzer(ctx, "code_activity", func() (*CodeActivityResult, error) {
		return (&CodeActivityAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["code_activity"] = err.Error()
	}

	conversation, err := runAnalyzer(ctx, "conversation", func() (*ConversationResult, error) {
		return (&ConversationAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["conversation"] = err.Error()
	}

	agents, err := runAnalyzer(ctx, "agents", func() (*AgentsResult, error) {
		return (&AgentsAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["agents_and_skills"] = err.Error()
	}

	skills, err := runAnalyzer(ctx, "skills", func() (*SkillsResult, error) {
		return (&SkillsAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		// Append to existing error if agents also failed
		if existing, ok := cardErrors["agents_and_skills"]; ok {
			cardErrors["agents_and_skills"] = existing + "; " + err.Error()
		} else {
			cardErrors["agents_and_skills"] = err.Error()
		}
	}

	redactions, err := runAnalyzer(ctx, "redactions", func() (*RedactionsResult, error) {
		return (&RedactionsAnalyzer{}).Analyze(fc)
	})
	if err != nil {
		cardErrors["redactions"] = err.Error()
	}

	// Build result with nil-safe field access
	result := &ComputeResult{
		// Validation stats (always available from file collection)
		ValidationErrorCount: fc.ValidationErrorCount(),
		// Per-card errors (only populated if non-empty)
		CardErrors: cardErrors,
	}

	// Token and cost stats
	if tokens != nil {
		result.InputTokens = tokens.InputTokens
		result.OutputTokens = tokens.OutputTokens
		result.CacheCreationTokens = tokens.CacheCreationTokens
		result.CacheReadTokens = tokens.CacheReadTokens
		result.EstimatedCostUSD = tokens.EstimatedCostUSD
		result.FastTurns = tokens.FastTurns
		result.FastCostUSD = tokens.FastCostUSD
	}

	// Session stats (message counts, breakdown, metadata, compaction)
	if session != nil {
		result.TotalMessages = session.TotalMessages
		result.UserMessages = session.UserMessages
		result.AssistantMessages = session.AssistantMessages
		result.HumanPrompts = session.HumanPrompts
		result.ToolResults = session.ToolResults
		result.TextResponses = session.TextResponses
		result.ToolCalls = session.ToolCalls
		result.ThinkingBlocks = session.ThinkingBlocks
		result.DurationMs = session.DurationMs
		result.ModelsUsed = session.ModelsUsed
		result.CompactionAuto = session.CompactionAuto
		result.CompactionManual = session.CompactionManual
		result.CompactionAvgTimeMs = session.CompactionAvgTimeMs
	}

	// Tools stats
	if tools != nil {
		result.TotalToolCalls = tools.TotalCalls
		result.ToolStats = tools.ToolStats
		result.ToolErrorCount = tools.ErrorCount
	}

	// Code activity stats
	if codeActivity != nil {
		result.FilesRead = codeActivity.FilesRead
		result.FilesModified = codeActivity.FilesModified
		result.LinesAdded = codeActivity.LinesAdded
		result.LinesRemoved = codeActivity.LinesRemoved
		result.SearchCount = codeActivity.SearchCount
		result.LanguageBreakdown = codeActivity.LanguageBreakdown
	}

	// Conversation stats (turns from conversation analyzer, not session)
	if conversation != nil {
		result.UserTurns = conversation.UserTurns
		result.AssistantTurns = conversation.AssistantTurns
		result.AvgAssistantTurnMs = conversation.AvgAssistantTurnMs
		result.AvgUserThinkingMs = conversation.AvgUserThinkingMs
		result.TotalAssistantDurationMs = conversation.TotalAssistantDurationMs
		result.TotalUserDurationMs = conversation.TotalUserDurationMs
		result.AssistantUtilizationPct = conversation.AssistantUtilizationPct
	}

	// Agent stats
	if agents != nil {
		result.TotalAgentInvocations = agents.TotalInvocations
		result.AgentStats = agents.AgentStats
	}

	// Skill stats
	if skills != nil {
		result.TotalSkillInvocations = skills.TotalInvocations
		result.SkillStats = skills.SkillStats
	}

	// Redaction stats
	if redactions != nil {
		result.TotalRedactions = redactions.TotalRedactions
		result.RedactionCounts = redactions.RedactionCounts
	}

	// Record any errors at span level for observability
	if len(cardErrors) > 0 {
		span.SetAttributes(attribute.Int("card_errors.count", len(cardErrors)))
	}

	return result, nil
}
