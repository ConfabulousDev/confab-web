package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// ComputeFromCodexRollout maps a parsed Codex rollout slice (main +
// subagents) onto the canonical ComputeResult shape. Per-card compute logic
// lives in analyzer_<card>_codex.go; this orchestrator initializes the
// result and dispatches each card. The Claude side mirrors with
// analyzer_<card>_claude.go. See internal/analytics/README.md for the
// (card, provider) matrix and per-card mapping decisions.
func ComputeFromCodexRollout(rollouts []*codex.ParsedRollout) *ComputeResult {
	if len(rollouts) == 0 || rollouts[0] == nil {
		return &ComputeResult{}
	}

	result := &ComputeResult{
		ToolStats:         make(map[string]*ToolStats),
		LanguageBreakdown: make(map[string]int),
		AgentStats:        make(map[string]*AgentStats),
		SkillStats:        make(map[string]*SkillStats),
		RedactionCounts:   make(map[string]int),
	}

	// Tokens and Session aggregate across all rollouts internally.
	computeCodexTokens(result, rollouts)
	computeCodexSession(result, rollouts)

	// Conversation stays main-only: turn counts + timing reflect user-perceived
	// structure, not subagent reasoning overlapping invisibly with the main thread.
	computeCodexConversation(result, rollouts[0])

	// Remaining analyzers accumulate via += on result fields, so per-rollout
	// dispatch produces the cross-rollout total.
	for _, r := range rollouts {
		if r == nil {
			continue
		}
		computeCodexTools(result, r)
		computeCodexCodeActivity(result, r)
		computeCodexAgentsAndSkills(result, r)
		computeCodexRedactions(result, r)
		result.ValidationErrorCount += len(r.ValidationErrors)
	}

	return result
}
