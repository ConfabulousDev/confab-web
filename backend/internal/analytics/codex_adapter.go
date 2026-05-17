package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// ComputeFromCodexRollout maps a parsed Codex rollout onto the same
// ComputeResult shape produced by ComputeStreaming for Claude transcripts.
//
// Per-card compute logic lives in analyzer_<card>_codex.go files; this
// orchestrator just initializes the result struct and dispatches each card
// in order. The Claude side lives in analyzer_<card>_claude.go files.
//
// Per-card mapping decisions (locked at interview time, see /tmp/plan-CF-350.md
// and follow-ups CF-441, CF-443, CF-445):
//   - Tokens: cached is a subset of input (OpenAI semantics) — subtract before
//     applying the uncached rate. Reasoning tokens add to output (same rate).
//   - Session: full population (TotalMessages, breakdowns, ModelsUsed, Duration).
//     Compactions all classified as "auto" (Codex doesn't distinguish auto vs manual).
//   - Tools: standard success/error breakdown. Orphan "<unknown>" tools
//     (synthetic placeholders for function_call_output with no matching
//     function_call) are dropped from per-tool stats and excluded from
//     TotalToolCalls / ToolErrorCount. The anomaly is surfaced via
//     ParsedRollout.ValidationErrors at parse time. CF-438.
//   - Code activity: apply_patch envelopes drive FilesModified/LinesAdded/Removed
//     and LanguageBreakdown. FilesRead stays 0 (Codex has no Read tool).
//   - Conversation: UserTurns / AssistantTurns plus the five timing fields
//     (CF-441). Reasoning extends the assistant window via a synthetic event
//     at Turn.CompletedAt — a Codex-specific divergence from Claude that's
//     documented inline in analyzer_conversation_codex.go.
//   - Agents/skills: zero (no Codex equivalent). See
//     analyzer_agents_and_skills_codex.go.
//   - Redactions: walk all parser-surfaced strings.
func ComputeFromCodexRollout(rollout *codex.ParsedRollout) *ComputeResult {
	if rollout == nil {
		return &ComputeResult{}
	}

	result := &ComputeResult{
		ToolStats:         make(map[string]*ToolStats),
		LanguageBreakdown: make(map[string]int),
		AgentStats:        make(map[string]*AgentStats),
		SkillStats:        make(map[string]*SkillStats),
		RedactionCounts:   make(map[string]int),
	}

	computeCodexTokens(result, rollout)
	computeCodexSession(result, rollout)
	computeCodexTools(result, rollout)
	computeCodexCodeActivity(result, rollout)
	computeCodexConversation(result, rollout)
	computeCodexAgentsAndSkills(result, rollout)
	computeCodexRedactions(result, rollout)

	return result
}
