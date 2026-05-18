package analytics

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/shopspring/decimal"
)

func ptrInt64(v int64) *int64 { return &v }

// minimalRollout builds a hand-crafted ParsedRollout with the minimum fields
// each test needs. Avoids depending on the parser implementation (which is
// stubbed during Phase 3b).
func minimalRollout() *codex.ParsedRollout {
	started := time.Date(2026, 5, 13, 1, 0, 0, 0, time.UTC)
	completed := started.Add(11 * time.Second)
	return &codex.ParsedRollout{
		Model:         "gpt-5",
		ModelProvider: "openai",
		Turns: []codex.Turn{
			{
				TurnID:      "t1",
				StartedAt:   &started,
				CompletedAt: &completed,
				DurationMs:  ptrInt64(11000),
				Model:       "gpt-5",
				UserMessages: []codex.Message{
					{Role: "user", Text: "add the linear mcp"},
				},
				AssistantMessages: []codex.Message{
					{Role: "assistant", Text: "ok", Phase: "final"},
				},
				ToolCalls: []codex.ToolCall{
					{CallID: "c1", Name: "exec_command", Arguments: `{"cmd":"pwd"}`, Output: "ok", Status: "completed"},
				},
				ReasoningCount: 1,
			},
		},
		TokenUsage: codex.TokenUsage{
			InputTokens:           10000,
			CachedInputTokens:     4000,
			OutputTokens:          2000,
			ReasoningOutputTokens: 500,
			TotalTokens:           12500,
		},
	}
}

func TestComputeFromCodexRollout_HappyPath(t *testing.T) {
	r := minimalRollout()
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	// Session counts.
	if out.UserMessages != 1 {
		t.Errorf("UserMessages = %d, want 1", out.UserMessages)
	}
	if out.AssistantMessages != 1 {
		t.Errorf("AssistantMessages = %d, want 1", out.AssistantMessages)
	}
	if out.HumanPrompts != 1 {
		t.Errorf("HumanPrompts = %d, want 1", out.HumanPrompts)
	}
	if out.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", out.ToolCalls)
	}
	if out.ToolResults != 1 {
		t.Errorf("ToolResults = %d, want 1", out.ToolResults)
	}
	if out.ThinkingBlocks != 1 {
		t.Errorf("ThinkingBlocks = %d, want 1", out.ThinkingBlocks)
	}
	if out.TextResponses != 1 {
		t.Errorf("TextResponses = %d, want 1", out.TextResponses)
	}
	// Duration = (completed - started) in ms.
	if out.DurationMs == nil || *out.DurationMs != 11000 {
		t.Errorf("DurationMs = %v, want 11000", out.DurationMs)
	}
	// Models used contains gpt-5.
	if len(out.ModelsUsed) != 1 || out.ModelsUsed[0] != "gpt-5" {
		t.Errorf("ModelsUsed = %v, want [gpt-5]", out.ModelsUsed)
	}
	// Tool stats.
	if out.TotalToolCalls != 1 {
		t.Errorf("TotalToolCalls = %d, want 1", out.TotalToolCalls)
	}
	if out.ToolStats == nil || out.ToolStats["exec_command"] == nil {
		t.Fatalf("ToolStats[exec_command] missing: %v", out.ToolStats)
	}
	if out.ToolStats["exec_command"].Success != 1 {
		t.Errorf("ToolStats[exec_command].Success = %d, want 1", out.ToolStats["exec_command"].Success)
	}
	// Compaction (none in this rollout).
	if out.CompactionAuto != 0 || out.CompactionManual != 0 {
		t.Errorf("Compaction Auto/Manual = %d/%d, want 0/0", out.CompactionAuto, out.CompactionManual)
	}
}

func TestComputeFromCodexRollout_EmptyRollout(t *testing.T) {
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{{}})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.UserMessages != 0 || out.AssistantMessages != 0 {
		t.Errorf("expected all zero counts, got users=%d asst=%d", out.UserMessages, out.AssistantMessages)
	}
	if out.TotalToolCalls != 0 {
		t.Errorf("TotalToolCalls = %d, want 0", out.TotalToolCalls)
	}
	// ToolStats may be nil or empty — both acceptable for zero state.
}

func TestComputeFromCodexRollout_OnlyEnvContext(t *testing.T) {
	// Parser drops env-context-only messages, so this represents a rollout
	// that, after parser stripping, has no user messages remaining.
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{{
		Turns: []codex.Turn{{TurnID: "t1"}},
	}})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.HumanPrompts != 0 {
		t.Errorf("HumanPrompts = %d, want 0", out.HumanPrompts)
	}
	if out.UserTurns != 0 {
		t.Errorf("UserTurns = %d, want 0", out.UserTurns)
	}
}

func TestComputeFromCodexRollout_ApplyPatch(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{
			TurnID: "t1",
			ToolCalls: []codex.ToolCall{
				{
					Name:      "apply_patch",
					Arguments: "*** Begin Patch\n*** Add File: foo.go\n+package foo\n+\n+func Bar() {}\n*** End Patch",
					Status:    "completed",
				},
				{
					Name:      "apply_patch",
					Arguments: "*** Begin Patch\n*** Update File: bar.py\n-old line\n+new line\n+another new\n*** End Patch",
					Status:    "completed",
				},
			},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2", out.FilesModified)
	}
	if out.LinesAdded != 5 {
		t.Errorf("LinesAdded = %d, want 5 (3 from foo.go + 2 from bar.py)", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", out.LinesRemoved)
	}
	if out.LanguageBreakdown["go"] != 1 {
		t.Errorf("LanguageBreakdown[go] = %d, want 1", out.LanguageBreakdown["go"])
	}
	if out.LanguageBreakdown["python"] != 1 && out.LanguageBreakdown["py"] != 1 {
		t.Errorf("LanguageBreakdown missing python/py entry: %v", out.LanguageBreakdown)
	}
	// Codex has no Read tool — FilesRead must stay zero.
	if out.FilesRead != 0 {
		t.Errorf("FilesRead = %d, want 0 (Codex has no Read tool)", out.FilesRead)
	}
}

// TestComputeFromCodexRollout_ApplyPatch_MissingEndPatch locks in the
// graceful-degradation behavior for truncated rollouts (network drop,
// killed session): parseApplyPatch counts what was seen up to EOF, even
// without a closing "*** End Patch" line. CF-439.
func TestComputeFromCodexRollout_ApplyPatch_MissingEndPatch(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{
			TurnID: "t1",
			ToolCalls: []codex.ToolCall{{
				Name:      "apply_patch",
				Arguments: "*** Begin Patch\n*** Add File: foo.go\n+line1\n+line2\n-removed",
				Status:    "completed",
			}},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.FilesModified != 1 {
		t.Errorf("FilesModified = %d, want 1 (truncated envelope still counts the seen file)", out.FilesModified)
	}
	if out.LinesAdded != 2 {
		t.Errorf("LinesAdded = %d, want 2 (count + lines seen before EOF)", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", out.LinesRemoved)
	}
	if out.LanguageBreakdown["go"] != 1 {
		t.Errorf("LanguageBreakdown[go] = %d, want 1", out.LanguageBreakdown["go"])
	}
}

// TestComputeFromCodexRollout_ApplyPatch_ConsecutiveHeaders verifies that
// back-to-back Update File headers with no diff lines between them still
// count each file. A rename-only patch (zero +/- lines, multiple files) is
// honest activity. CF-439.
func TestComputeFromCodexRollout_ApplyPatch_ConsecutiveHeaders(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{
			TurnID: "t1",
			ToolCalls: []codex.ToolCall{{
				Name: "apply_patch",
				Arguments: "*** Begin Patch\n" +
					"*** Update File: a.go\n" +
					"*** Update File: b.py\n" +
					"*** Update File: c.ts\n" +
					"*** End Patch",
				Status: "completed",
			}},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.FilesModified != 3 {
		t.Errorf("FilesModified = %d, want 3 (each header counts even without diff lines)", out.FilesModified)
	}
	if out.LinesAdded != 0 {
		t.Errorf("LinesAdded = %d, want 0", out.LinesAdded)
	}
	if out.LinesRemoved != 0 {
		t.Errorf("LinesRemoved = %d, want 0", out.LinesRemoved)
	}
	if out.LanguageBreakdown["go"] != 1 {
		t.Errorf("LanguageBreakdown[go] = %d, want 1", out.LanguageBreakdown["go"])
	}
	if out.LanguageBreakdown["python"] != 1 {
		t.Errorf("LanguageBreakdown[python] = %d, want 1", out.LanguageBreakdown["python"])
	}
	if out.LanguageBreakdown["typescript"] != 1 {
		t.Errorf("LanguageBreakdown[typescript] = %d, want 1", out.LanguageBreakdown["typescript"])
	}
}

// TestComputeFromCodexRollout_ApplyPatch_SingleSpaceContextLines verifies that
// classic unified-diff context lines (leading single space) are ignored: only
// "+" and "-" prefixed lines count toward LinesAdded / LinesRemoved. CF-439.
func TestComputeFromCodexRollout_ApplyPatch_SingleSpaceContextLines(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{
			TurnID: "t1",
			ToolCalls: []codex.ToolCall{{
				Name: "apply_patch",
				Arguments: "*** Begin Patch\n" +
					"*** Update File: foo.go\n" +
					" unchanged_a\n" +
					"+added_one\n" +
					" unchanged_b\n" +
					"-removed_one\n" +
					" unchanged_c\n" +
					"*** End Patch",
				Status: "completed",
			}},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.FilesModified != 1 {
		t.Errorf("FilesModified = %d, want 1", out.FilesModified)
	}
	if out.LinesAdded != 1 {
		t.Errorf("LinesAdded = %d, want 1 (single-space context lines must not count)", out.LinesAdded)
	}
	if out.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1 (single-space context lines must not count)", out.LinesRemoved)
	}
}

func TestComputeFromCodexRollout_Compaction(t *testing.T) {
	r := &codex.ParsedRollout{
		Compactions: []codex.CompactionEvent{
			{ReplacementCount: 3},
		},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.CompactionAuto != 1 {
		t.Errorf("CompactionAuto = %d, want 1", out.CompactionAuto)
	}
	if out.CompactionManual != 0 {
		t.Errorf("CompactionManual = %d, want 0", out.CompactionManual)
	}
}

func TestComputeFromCodexRollout_TokenCost_GPT5(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{Model: "gpt-5"}},
		TokenUsage: codex.TokenUsage{
			InputTokens:           10000,
			CachedInputTokens:     4000,
			OutputTokens:          2000,
			ReasoningOutputTokens: 500,
			TotalTokens:           12500,
		},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}

	// Adapter must subtract cached from input: uncached = 10000 - 4000 = 6000.
	if out.InputTokens != 6000 {
		t.Errorf("InputTokens = %d, want 6000 (10000 raw - 4000 cached)", out.InputTokens)
	}
	if out.CacheReadTokens != 4000 {
		t.Errorf("CacheReadTokens = %d, want 4000", out.CacheReadTokens)
	}
	if out.CacheCreationTokens != 0 {
		t.Errorf("CacheCreationTokens = %d, want 0 (OpenAI doesn't charge cache writes)", out.CacheCreationTokens)
	}
	// OutputTokens = output + reasoning_output (both billed as output by OpenAI).
	if out.OutputTokens != 2500 {
		t.Errorf("OutputTokens = %d, want 2500 (2000 + 500 reasoning)", out.OutputTokens)
	}

	// Cost: uncached_input*1.25 + cached*0.125 + output*10 per million.
	//     = (6000*1.25 + 4000*0.125 + 2500*10) / 1_000_000
	//     = (7500 + 500 + 25000) / 1_000_000
	//     = 0.033
	want := decimal.NewFromFloat(0.033)
	if out.EstimatedCostUSD.Sub(want).Abs().GreaterThan(decimal.NewFromFloat(0.001)) {
		t.Errorf("EstimatedCostUSD = %s, want ~0.033", out.EstimatedCostUSD)
	}
}

func TestComputeFromCodexRollout_UnknownModel(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-future-2099",
		Turns: []codex.Turn{{Model: "gpt-future-2099"}},
		TokenUsage: codex.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
		},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if !out.EstimatedCostUSD.IsZero() {
		t.Errorf("EstimatedCostUSD = %s, want 0 for unknown model", out.EstimatedCostUSD)
	}
}

func TestComputeFromCodexRollout_AssistantTurns_MidStreamUserPrompts(t *testing.T) {
	// A single Codex task_started→task_complete cycle that contains two user
	// prompts (user typed mid-stream). Each prompt that triggers ≥1 assistant
	// response before the next prompt should count as one AssistantTurn.
	r := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			UserMessages: []codex.Message{
				{Role: "user", Text: "first prompt", Timestamp: time.Date(2026, 5, 13, 1, 0, 0, 0, time.UTC)},
				{Role: "user", Text: "second prompt", Timestamp: time.Date(2026, 5, 13, 1, 0, 30, 0, time.UTC)},
			},
			AssistantMessages: []codex.Message{
				{Role: "assistant", Text: "reply 1", Phase: "final", Timestamp: time.Date(2026, 5, 13, 1, 0, 15, 0, time.UTC)},
				{Role: "assistant", Text: "reply 2", Phase: "final", Timestamp: time.Date(2026, 5, 13, 1, 0, 45, 0, time.UTC)},
			},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2 (both user prompts counted)", out.UserTurns)
	}
	if out.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2 (one per user-prompt-triggered sequence)", out.AssistantTurns)
	}
}

func TestComputeFromCodexRollout_Redactions_Recursive(t *testing.T) {
	r := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			TurnID: "t1",
			AssistantMessages: []codex.Message{
				{Role: "assistant", Text: "user email is [REDACTED:EMAIL]"},
			},
			ToolCalls: []codex.ToolCall{
				{Name: "exec_command", Output: "found token [REDACTED:API_KEY] in env"},
			},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", out.TotalRedactions)
	}
	if out.RedactionCounts["EMAIL"] != 1 {
		t.Errorf("RedactionCounts[EMAIL] = %d, want 1", out.RedactionCounts["EMAIL"])
	}
	if out.RedactionCounts["API_KEY"] != 1 {
		t.Errorf("RedactionCounts[API_KEY] = %d, want 1", out.RedactionCounts["API_KEY"])
	}
}

// TestComputeFromCodexRollout_OrphanToolCallSkipped locks in the CF-438
// contract: orphan "<unknown>" tool calls (synthetic placeholders the parser
// emits when a function_call_output arrives without a matching function_call)
// are dropped from the Tools card. The data anomaly is recorded as a
// ParsedRollout.ValidationError at parse time instead.
func TestComputeFromCodexRollout_OrphanToolCallSkipped(t *testing.T) {
	r := &codex.ParsedRollout{
		Turns: []codex.Turn{{
			ToolCalls: []codex.ToolCall{
				{Name: "<unknown>", Output: "orphan", Status: "completed"},
			},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.TotalToolCalls != 0 {
		t.Errorf("TotalToolCalls = %d, want 0 (orphan must be skipped)", out.TotalToolCalls)
	}
	if out.ToolErrorCount != 0 {
		t.Errorf("ToolErrorCount = %d, want 0", out.ToolErrorCount)
	}
	if _, ok := out.ToolStats["<unknown>"]; ok {
		t.Errorf("ToolStats must not contain orphan <unknown> key: %v", out.ToolStats)
	}
}

// TestComputeFromCodexRollout_FailedTool exercises CF-438 acceptance #2:
// failed custom_tool_call payloads must increment both ToolErrorCount and the
// per-tool Errors counter, while completed payloads land in Success.
func TestComputeFromCodexRollout_FailedTool(t *testing.T) {
	r := &codex.ParsedRollout{
		Model: "gpt-5",
		Turns: []codex.Turn{{
			TurnID: "t1",
			ToolCalls: []codex.ToolCall{
				{Name: "apply_patch", Arguments: "*** Begin Patch\n*** Add File: a.txt\n+ok\n*** End Patch", Status: "completed"},
				{Name: "apply_patch", Arguments: "*** Begin Patch\n*** Update File: b.txt\n-old\n+new\n*** End Patch", Status: "failed"},
			},
		}},
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{r})
	if out == nil {
		t.Fatal("ComputeFromCodexRollout returned nil")
	}
	if out.TotalToolCalls != 2 {
		t.Errorf("TotalToolCalls = %d, want 2", out.TotalToolCalls)
	}
	if out.ToolErrorCount != 1 {
		t.Errorf("ToolErrorCount = %d, want 1 (one failed apply_patch)", out.ToolErrorCount)
	}
	stats := out.ToolStats["apply_patch"]
	if stats == nil {
		t.Fatalf("ToolStats[apply_patch] missing: %v", out.ToolStats)
	}
	if stats.Success != 1 {
		t.Errorf("ToolStats[apply_patch].Success = %d, want 1", stats.Success)
	}
	if stats.Errors != 1 {
		t.Errorf("ToolStats[apply_patch].Errors = %d, want 1", stats.Errors)
	}
}

// ============================================================================
// CF-443: end-to-end Codex agents/skills support
// ============================================================================

// TestComputeFromCodexRollout_SanitizedRealFixture_PopulatesBoth runs the full
// pipeline on the synthetic parent-with-spawns fixture and verifies that the
// resulting ComputeResult carries non-zero agent and skill counts. (Skills
// are zero here because the parent fixture doesn't invoke any; this exercises
// the agents path.)
func TestComputeFromCodexRollout_ParentFixture_PopulatesAgents(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", "sample_rollout_parent_with_spawns.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{rollout})
	if out.TotalAgentInvocations != 2 {
		t.Errorf("TotalAgentInvocations = %d, want 2", out.TotalAgentInvocations)
	}
	// AgentStats keyed by role.
	if out.AgentStats["default"] == nil || out.AgentStats["default"].Success != 1 {
		t.Errorf("AgentStats[default] = %+v, want {Success:1}", out.AgentStats["default"])
	}
	if out.AgentStats["explorer"] == nil || out.AgentStats["explorer"].Errors != 1 {
		t.Errorf("AgentStats[explorer] = %+v, want {Errors:1}", out.AgentStats["explorer"])
	}
	// Skills are absent in this fixture.
	if out.TotalSkillInvocations != 0 {
		t.Errorf("TotalSkillInvocations = %d, want 0", out.TotalSkillInvocations)
	}
}

// TestComputeFromCodexRollout_SkillFixture_PopulatesSkills runs the pipeline
// on the synthetic skill-invocation fixture.
func TestComputeFromCodexRollout_SkillFixture_PopulatesSkills(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", "sample_rollout_with_skill_invocation.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{rollout})
	if out.TotalSkillInvocations != 1 {
		t.Errorf("TotalSkillInvocations = %d, want 1", out.TotalSkillInvocations)
	}
	if out.SkillStats["audit-documentation"] == nil ||
		out.SkillStats["audit-documentation"].Success != 1 {
		t.Errorf("SkillStats[audit-documentation] = %+v, want {Success:1}",
			out.SkillStats["audit-documentation"])
	}
}

// TestComputeFromCodexRollout_NoSpawnAgentInTools verifies spawn_agent does
// not appear in the Tools card stats produced by the adapter.
func TestComputeFromCodexRollout_NoSpawnAgentInTools(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", "sample_rollout_parent_with_spawns.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{rollout})
	if out.ToolStats["spawn_agent"] != nil {
		t.Errorf("ToolStats[spawn_agent] = %+v, want nil (routed to AgentsAndSkills)",
			out.ToolStats["spawn_agent"])
	}
}

// TestComputeFromCodexRollout_NoWaitAgentInTools verifies wait_agent does
// not appear in the Tools card stats.
func TestComputeFromCodexRollout_NoWaitAgentInTools(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", "sample_rollout_parent_with_spawns.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{rollout})
	if out.ToolStats["wait_agent"] != nil {
		t.Errorf("ToolStats[wait_agent] = %+v, want nil (excluded from Tools card)",
			out.ToolStats["wait_agent"])
	}
}

// TestComputeFromCodexRollout_ConversationTimingUnchanged_NoSpawnFixture is the
// CF-441 regression guard. The legacy fixture (sample_rollout.jsonl) contains
// no spawn_agent/wait_agent calls — the CF-443 parser refactor must not change
// the timing numbers it produces.
func TestComputeFromCodexRollout_ConversationTimingUnchanged_NoSpawnFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "codex", "testdata", "sample_rollout.jsonl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseRollout: %v", err)
	}
	out := ComputeFromCodexRollout([]*codex.ParsedRollout{rollout})
	// Hard-coded baselines captured from main before CF-443. If any of these
	// shift, CF-443 has inadvertently altered Conversation card behavior — fix
	// the parser refactor, not the baseline.
	if out.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2 (CF-441 baseline)", out.UserTurns)
	}
	if out.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2 (CF-441 baseline)", out.AssistantTurns)
	}
	if out.UserMessages != 2 {
		t.Errorf("UserMessages = %d, want 2 (CF-441 baseline)", out.UserMessages)
	}
	if out.AssistantMessages != 3 {
		t.Errorf("AssistantMessages = %d, want 3 (CF-441 baseline)", out.AssistantMessages)
	}
	if out.TotalToolCalls != 4 {
		t.Errorf("TotalToolCalls = %d, want 4 (CF-441 baseline)", out.TotalToolCalls)
	}
}

