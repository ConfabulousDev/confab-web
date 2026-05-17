package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// computeCodexAgentsAndSkills is intentionally a no-op. Codex has no concept
// of Anthropic-style Agents (Task tool / subagent_type) or Skills (slash
// commands), so AgentInvocations / SkillInvocations / AgentStats / SkillStats
// stay at their zero/empty initial values from ComputeFromCodexRollout.
//
// This file exists to make the (card, provider) matrix complete: a glance at
// `ls analyzer_*.go` shows every (card, provider) pair, with this file
// explicitly documenting the empty quadrant rather than leaving it implicit.
//
// CF-443 will decide whether to skip upserting an empty card record for
// Codex sessions entirely.
func computeCodexAgentsAndSkills(_ *ComputeResult, _ *codex.ParsedRollout) {
}
