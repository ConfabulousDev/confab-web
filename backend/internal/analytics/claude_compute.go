package analytics

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// AgentProvider yields the next parsed agent file, or io.EOF when done.
// Each call downloads and parses one agent file. The returned TranscriptFile
// may be discarded after all FileProcessors have seen it.
type AgentProvider func(ctx context.Context) (*TranscriptFile, error)

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
// Delegates to ComputeStreaming with an adapter that yields agents from the in-memory collection.
func ComputeFromFileCollection(ctx context.Context, fc *FileCollection) (*ComputeResult, error) {
	idx := 0
	agentProvider := func(_ context.Context) (*TranscriptFile, error) {
		if idx >= len(fc.Agents) {
			return nil, io.EOF
		}
		agent := fc.Agents[idx]
		idx++
		return agent, nil
	}

	return ComputeStreaming(ctx, fc.Main, agentProvider, nil)
}

// WorkflowInputs carries the side data the WorkflowsAnalyzer needs that the
// generic streaming loop doesn't model: the runId for each agent file (resolved
// from file names by the caller, keyed by agent id) and each run's journal
// content. Nil when the session has no workflow files (e.g. Codex, or tests).
type WorkflowInputs struct {
	RunIDByAgentID map[string]string
	Journals       map[string][]byte // runId -> journal.jsonl content
}

// ComputeStreaming computes analytics by streaming agent files one at a time through all analyzers.
// The main file is processed first, then each agent file from the provider is processed and discarded.
// Peak memory: O(main) + O(largest single agent) instead of O(all agents).
// Uses collect-errors pattern: individual card failures don't fail the whole computation.
//
// wf is optional: when non-nil, the WorkflowsAnalyzer is driven explicitly
// alongside the generic processors (it is not a FileProcessor — see
// analyzer_workflows.go).
func ComputeStreaming(ctx context.Context, main *TranscriptFile, agentProvider AgentProvider, wf *WorkflowInputs) (*ComputeResult, error) {
	ctx, span := tracer.Start(ctx, "analytics.compute_streaming",
		trace.WithAttributes(
			attribute.Int64("main.lines", int64(len(main.Lines))),
		))
	defer span.End()

	// Initialize all analyzers
	tokensAnalyzer := &TokensAnalyzer{}
	sessionAnalyzer := &SessionAnalyzer{}
	toolsAnalyzer := &ToolsAnalyzer{}
	codeActivityAnalyzer := &CodeActivityAnalyzer{}
	conversationAnalyzer := &ConversationAnalyzer{}
	agentsAnalyzer := &AgentsAnalyzer{}
	skillsAnalyzer := &SkillsAnalyzer{}
	redactionsAnalyzer := &RedactionsAnalyzer{}

	processors := []FileProcessor{
		tokensAnalyzer,
		sessionAnalyzer,
		toolsAnalyzer,
		codeActivityAnalyzer,
		conversationAnalyzer,
		agentsAnalyzer,
		skillsAnalyzer,
		redactionsAnalyzer,
	}

	// Phase 1: Process main file through all analyzers
	for _, p := range processors {
		p.ProcessFile(main, true)
	}

	// WorkflowsAnalyzer is driven explicitly (not a FileProcessor): it needs a
	// runId per agent + the run journals, neither of which the generic loop models.
	var workflowsAnalyzer *WorkflowsAnalyzer
	if wf != nil {
		workflowsAnalyzer = &WorkflowsAnalyzer{}
	}

	// Phase 2: Stream agent files one at a time
	agentFilesSeen := make(map[string]bool)
	skippedAgentFiles := 0
	validationErrorCount := len(main.ValidationErrors)

	for {
		agent, err := agentProvider(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("agent provider error, skipping agent", "error", err)
			skippedAgentFiles++
			continue
		}

		validationErrorCount += len(agent.ValidationErrors)
		if agent.AgentID != "" {
			agentFilesSeen[agent.AgentID] = true
		}

		for _, p := range processors {
			p.ProcessFile(agent, false)
		}
		if workflowsAnalyzer != nil {
			workflowsAnalyzer.ProcessAgent(agent, wf.RunIDByAgentID[agent.AgentID])
		}
	}

	// Phase 3: Finalize all analyzers
	for _, p := range processors {
		p.Finalize(func(agentID string) bool { return agentFilesSeen[agentID] })
	}

	// Phase 3b: fold in per-run journal status, then collect workflow runs.
	var workflowRuns []WorkflowRun
	if workflowsAnalyzer != nil {
		for runID, content := range wf.Journals {
			workflowsAnalyzer.ProcessJournal(runID, content)
		}
		workflowRuns = workflowsAnalyzer.Result()
	}

	span.SetAttributes(
		attribute.Int("agent_files.processed", len(agentFilesSeen)),
		attribute.Int("agent_files.skipped", skippedAgentFiles),
	)

	// Build result from analyzer outputs
	tokens := tokensAnalyzer.Result()
	session := sessionAnalyzer.Result()
	tools := toolsAnalyzer.Result()
	codeActivity := codeActivityAnalyzer.Result()
	conversation := conversationAnalyzer.Result()
	agents := agentsAnalyzer.Result()
	skills := skillsAnalyzer.Result()
	redactions := redactionsAnalyzer.Result()

	return &ComputeResult{
		// Tokens and cost
		InputTokens:         tokens.InputTokens,
		OutputTokens:        tokens.OutputTokens,
		CacheCreationTokens: tokens.CacheCreationTokens,
		CacheReadTokens:     tokens.CacheReadTokens,
		EstimatedCostUSD:    tokens.EstimatedCostUSD,
		FastTurns:           tokens.FastTurns,
		FastCostUSD:         tokens.FastCostUSD,

		// Session
		TotalMessages:       session.TotalMessages,
		UserMessages:        session.UserMessages,
		AssistantMessages:   session.AssistantMessages,
		HumanPrompts:        session.HumanPrompts,
		ToolResults:         session.ToolResults,
		TextResponses:       session.TextResponses,
		ToolCalls:           session.ToolCalls,
		ThinkingBlocks:      session.ThinkingBlocks,
		DurationMs:          session.DurationMs,
		ModelsUsed:          session.ModelsUsed,
		CompactionAuto:      session.CompactionAuto,
		CompactionManual:    session.CompactionManual,
		CompactionAvgTimeMs: session.CompactionAvgTimeMs,

		// Tools
		TotalToolCalls: tools.TotalCalls,
		ToolStats:      tools.ToolStats,
		ToolErrorCount: tools.ErrorCount,

		// Code activity
		FilesRead:         codeActivity.FilesRead,
		FilesModified:     codeActivity.FilesModified,
		LinesAdded:        codeActivity.LinesAdded,
		LinesRemoved:      codeActivity.LinesRemoved,
		SearchCount:       codeActivity.SearchCount,
		LanguageBreakdown: codeActivity.LanguageBreakdown,

		// Conversation
		UserTurns:                conversation.UserTurns,
		AssistantTurns:           conversation.AssistantTurns,
		AvgAssistantTurnMs:       conversation.AvgAssistantTurnMs,
		AvgUserThinkingMs:        conversation.AvgUserThinkingMs,
		TotalAssistantDurationMs: conversation.TotalAssistantDurationMs,
		TotalUserDurationMs:      conversation.TotalUserDurationMs,
		AssistantUtilizationPct:  conversation.AssistantUtilizationPct,

		// Agents and skills
		TotalAgentInvocations: agents.TotalInvocations,
		AgentStats:            agents.AgentStats,
		TotalSkillInvocations: skills.TotalInvocations,
		SkillStats:            skills.SkillStats,

		// Redactions
		TotalRedactions: redactions.TotalRedactions,
		RedactionCounts: redactions.RedactionCounts,

		// Workflows
		Workflows: workflowRuns,

		// Metadata
		ValidationErrorCount: validationErrorCount,
		SkippedAgentFiles:    skippedAgentFiles,
	}, nil
}
