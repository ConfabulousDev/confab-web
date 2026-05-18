package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// ComputeFromCodexRollout maps a parsed Codex rollout slice (main +
// subagents, CF-403) onto the canonical ComputeResult shape. Per-card compute
// logic lives in analyzer_<card>_codex.go; this orchestrator dispatches each
// card with the appropriate aggregation rule. The Claude side mirrors with
// analyzer_<card>_claude.go. See internal/analytics/README.md for the
// (card, provider) matrix.
//
// Per-card aggregation rules:
//   - Tokens: sum across rollouts (cached is a subset of input under OpenAI
//     semantics — subtract before applying the uncached rate; reasoning tokens
//     fold into output; OpenAI doesn't charge for cache writes).
//   - Session: per-turn counts sum; ModelsUsed unions; DurationMs spans
//     earliest start to latest completion; compactions sum (all "auto").
//   - Conversation: rollouts[0] only — turn counts + timing reflect
//     user-perceived structure, not subagent reasoning overlapping with the
//     main thread.
//   - Tools: per-rollout dispatch. Orphan "<unknown>" tools are dropped from
//     per-tool stats and excluded from TotalToolCalls / ToolErrorCount (CF-438).
//     spawn_agent / wait_agent function_calls are routed out of Turn.ToolCalls
//     by the parser (CF-443) so they only surface in the Agents & Skills card.
//   - Code activity: per-rollout dispatch. apply_patch envelopes drive
//     FilesModified / LinesAdded / Removed and LanguageBreakdown. FilesRead
//     stays 0 (Codex has no Read tool). SearchCount stays 0 — web_search_call
//     is web search, not file search (CF-439).
//   - Agents/skills: per-rollout dispatch (CF-443). SubagentSpawns bucket by
//     agent_role (success = wait_agent "completed", error = any other status
//     or orphan). SkillInvocations bucket by skill name, always success
//     (Codex emits no per-skill error signal in rollout JSONL).
//   - Redactions: per-rollout dispatch over parser-surfaced strings.
//   - ValidationErrorCount: sums across rollouts so the frontend counter
//     reflects the union of main + subagent parse anomalies.
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
