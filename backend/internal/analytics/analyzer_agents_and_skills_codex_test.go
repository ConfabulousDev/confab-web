package analytics

import (
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
)

// TestComputeCodexAgentsAndSkills_Empty: a ParsedRollout with no spawns and
// no skill invocations leaves AgentStats / SkillStats untouched and counts at
// zero.
func TestComputeCodexAgentsAndSkills_Empty(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	computeCodexAgentsAndSkills(result, &codex.ParsedRollout{})
	if result.TotalAgentInvocations != 0 {
		t.Errorf("TotalAgentInvocations = %d, want 0", result.TotalAgentInvocations)
	}
	if result.TotalSkillInvocations != 0 {
		t.Errorf("TotalSkillInvocations = %d, want 0", result.TotalSkillInvocations)
	}
	if len(result.AgentStats) != 0 {
		t.Errorf("AgentStats len = %d, want 0", len(result.AgentStats))
	}
	if len(result.SkillStats) != 0 {
		t.Errorf("SkillStats len = %d, want 0", len(result.SkillStats))
	}
}

// TestComputeCodexAgentsAndSkills_TwoSpawnsMixedOutcomes: two spawn rows with
// the same agent role but one completed and one failed → AgentStats[role]
// counts {Success:1, Errors:1}.
func TestComputeCodexAgentsAndSkills_TwoSpawnsMixedOutcomes(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	rollout := &codex.ParsedRollout{
		SubagentSpawns: []codex.SubagentSpawn{
			{CallID: "s1", AgentType: "default", Completed: true},
			{CallID: "s2", AgentType: "default", Completed: false},
		},
	}
	computeCodexAgentsAndSkills(result, rollout)
	if result.TotalAgentInvocations != 2 {
		t.Errorf("TotalAgentInvocations = %d, want 2", result.TotalAgentInvocations)
	}
	stats := result.AgentStats["default"]
	if stats == nil {
		t.Fatal("AgentStats[default] = nil")
	}
	if stats.Success != 1 {
		t.Errorf("AgentStats[default].Success = %d, want 1", stats.Success)
	}
	if stats.Errors != 1 {
		t.Errorf("AgentStats[default].Errors = %d, want 1", stats.Errors)
	}
}

// TestComputeCodexAgentsAndSkills_TwoSkillsSameName: two invocations of the
// same skill both bucket under SkillStats[name].Success (Codex skills are
// always success — no per-skill error signal exists in rollout JSONL).
func TestComputeCodexAgentsAndSkills_TwoSkillsSameName(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	rollout := &codex.ParsedRollout{
		SkillInvocations: []codex.SkillInvocation{
			{Name: "audit-documentation", Timestamp: time.Now()},
			{Name: "audit-documentation", Timestamp: time.Now()},
		},
	}
	computeCodexAgentsAndSkills(result, rollout)
	if result.TotalSkillInvocations != 2 {
		t.Errorf("TotalSkillInvocations = %d, want 2", result.TotalSkillInvocations)
	}
	stats := result.SkillStats["audit-documentation"]
	if stats == nil {
		t.Fatal("SkillStats[audit-documentation] = nil")
	}
	if stats.Success != 2 {
		t.Errorf("SkillStats.Success = %d, want 2", stats.Success)
	}
	if stats.Errors != 0 {
		t.Errorf("SkillStats.Errors = %d, want 0 (Codex skills have no error signal)", stats.Errors)
	}
}

// TestComputeCodexAgentsAndSkills_Mixed: spawns and skills coexist without
// cross-pollution; the totals are independent.
func TestComputeCodexAgentsAndSkills_Mixed(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	rollout := &codex.ParsedRollout{
		SubagentSpawns: []codex.SubagentSpawn{
			{AgentType: "explorer", Completed: true},
		},
		SkillInvocations: []codex.SkillInvocation{
			{Name: "interview"},
		},
	}
	computeCodexAgentsAndSkills(result, rollout)
	if result.TotalAgentInvocations != 1 {
		t.Errorf("TotalAgentInvocations = %d, want 1", result.TotalAgentInvocations)
	}
	if result.TotalSkillInvocations != 1 {
		t.Errorf("TotalSkillInvocations = %d, want 1", result.TotalSkillInvocations)
	}
	if result.AgentStats["explorer"] == nil || result.AgentStats["explorer"].Success != 1 {
		t.Errorf("AgentStats[explorer] = %+v, want {Success:1}", result.AgentStats["explorer"])
	}
	if result.SkillStats["interview"] == nil || result.SkillStats["interview"].Success != 1 {
		t.Errorf("SkillStats[interview] = %+v, want {Success:1}", result.SkillStats["interview"])
	}
}

// TestComputeCodexAgentsAndSkills_EmptyAgentType_BucketsAsUnknown: defensive
// case for a spawn whose AgentType wasn't extracted.
func TestComputeCodexAgentsAndSkills_EmptyAgentType_BucketsAsUnknown(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	rollout := &codex.ParsedRollout{
		SubagentSpawns: []codex.SubagentSpawn{
			{AgentType: "", Completed: true},
		},
	}
	computeCodexAgentsAndSkills(result, rollout)
	if result.AgentStats["unknown"] == nil || result.AgentStats["unknown"].Success != 1 {
		t.Errorf("AgentStats[unknown] = %+v, want {Success:1}", result.AgentStats["unknown"])
	}
}

// TestComputeCodexAgentsAndSkills_EmptySkillName_BucketsAsUnknown: defensive
// case for a skill whose Name wasn't extracted.
func TestComputeCodexAgentsAndSkills_EmptySkillName_BucketsAsUnknown(t *testing.T) {
	result := &ComputeResult{
		AgentStats: make(map[string]*AgentStats),
		SkillStats: make(map[string]*SkillStats),
	}
	rollout := &codex.ParsedRollout{
		SkillInvocations: []codex.SkillInvocation{
			{Name: ""},
		},
	}
	computeCodexAgentsAndSkills(result, rollout)
	if result.SkillStats["unknown"] == nil || result.SkillStats["unknown"].Success != 1 {
		t.Errorf("SkillStats[unknown] = %+v, want {Success:1}", result.SkillStats["unknown"])
	}
}
