package analytics

import "github.com/ConfabulousDev/confab-web/internal/codex"

// computeCodexAgentsAndSkills populates AgentStats / SkillStats for a Codex
// rollout. Subagent spawns bucket by agent_role (Codex's per-spawn "agent_type"
// argument, which maps to the child's session_meta.agent_role); a spawn is
// "success" iff its matching wait_agent reported status == "completed", else
// "error" (including orphan spawns with no wait_agent at all). Skill
// invocations bucket by skill name and always count as success — Codex emits
// no per-skill error signal in rollout JSONL. Both spawn_agent and wait_agent
// function_calls are routed out of Turn.ToolCalls by the parser so they do
// not also show up in the Tools card. See CF-443.
func computeCodexAgentsAndSkills(result *ComputeResult, rollout *codex.ParsedRollout) {
	for _, s := range rollout.SubagentSpawns {
		role := s.AgentType
		if role == "" {
			role = "unknown"
		}
		stats := result.AgentStats[role]
		if stats == nil {
			stats = &AgentStats{}
			result.AgentStats[role] = stats
		}
		result.TotalAgentInvocations++
		if s.Completed {
			stats.Success++
		} else {
			stats.Errors++
		}
	}
	for _, sk := range rollout.SkillInvocations {
		name := sk.Name
		if name == "" {
			name = "unknown"
		}
		stats := result.SkillStats[name]
		if stats == nil {
			stats = &SkillStats{}
			result.SkillStats[name] = stats
		}
		result.TotalSkillInvocations++
		stats.Success++
	}
}
