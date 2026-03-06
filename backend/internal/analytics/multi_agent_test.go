package analytics

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// buildMultiAgentFixture creates a FileCollection with a main transcript and multiple agent files.
// The fixture exercises all 8 analyzers:
//
// Main transcript:
//   - User prompt → assistant with text+tool_use (Read /main.go) + Task invocation
//   - Tool result for Read (success)
//   - Agent result for Task (agent1, 10 tool calls fallback, 200/100 tokens fallback)
//   - User prompt → assistant with Skill invocation
//   - Skill result (success)
//   - Text with [REDACTED:API_KEY] marker
//
// Agent 1 (agent1): claude-haiku-3
//   - Assistant: Read /agent1.go (success)
//   - Tool result (success)
//   - Assistant: Write /output.ts (content "line1\nline2\nline3")
//   - Text with [REDACTED:PASSWORD]
//
// Agent 2 (agent2): claude-opus-4
//   - Assistant: Grep + Edit /shared.go (old "foo", new "bar\nbaz")
//   - Tool result for Grep (success), Edit (error)
//   - Text with [REDACTED:API_KEY]
//
// Agent 3 (agent3): empty (0 lines)
func buildMultiAgentFixture(t *testing.T) *FileCollection {
	t.Helper()

	mainJsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "Hello, please explore") + "\n" +
		makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
			makeTextBlock("I'll read and explore"),
			makeToolUseBlock("toolu_read1", "Read", map[string]interface{}{"file_path": "/main.go"}),
			makeToolUseBlock("toolu_task1", "Task", map[string]interface{}{"prompt": "explore", "subagent_type": "Explore"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_read1", "package main", false),
		}) + "\n" +
		makeUserMessageWithToolUseResult("u3", "2025-01-01T00:00:03Z", []map[string]interface{}{
			makeToolResultBlock("toolu_task1", "Done exploring", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(10),
			"usage":             map[string]interface{}{"input_tokens": float64(200), "output_tokens": float64(100)},
		}) + "\n" +
		makeUserMessage("u4", "2025-01-01T00:01:00Z", "Now commit") + "\n" +
		makeAssistantMessageWithStopReason("a2", "2025-01-01T00:01:01Z", "claude-sonnet-4-20241022", 150, 75, []map[string]interface{}{
			makeToolUseBlock("toolu_skill1", "Skill", map[string]interface{}{"skill": "commit", "args": "-m 'test'"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u5", "2025-01-01T00:01:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_skill1", "Committed", false),
		}) + "\n" +
		makeAssistantMessage("a3", "2025-01-01T00:01:03Z", "claude-sonnet-4-20241022", 80, 40, []map[string]interface{}{
			makeTextBlock("Done! Your key is [REDACTED:API_KEY] and done."),
		}) + "\n"

	agent1Jsonl := makeAssistantMessageWithStopReason("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 150, 75, []map[string]interface{}{
		makeToolUseBlock("toolu_a1_read", "Read", map[string]interface{}{"file_path": "/agent1.go"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("au1", "2025-01-01T00:00:01.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_a1_read", "package agent1", false),
		}) + "\n" +
		makeAssistantMessageWithStopReason("aa2", "2025-01-01T00:00:01.7Z", "claude-haiku-3", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_a1_write", "Write", map[string]interface{}{
				"file_path": "/output.ts",
				"content":   "line1\nline2\nline3",
			}),
		}, "tool_use") + "\n" +
		makeAssistantMessage("aa3", "2025-01-01T00:00:01.8Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeTextBlock("Found [REDACTED:PASSWORD] in config"),
		}) + "\n"

	agent2Jsonl := makeAssistantMessageWithStopReason("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 200, 100, []map[string]interface{}{
		makeToolUseBlock("toolu_a2_grep", "Grep", map[string]interface{}{}),
		makeToolUseBlock("toolu_a2_edit", "Edit", map[string]interface{}{
			"file_path":  "/shared.go",
			"old_string": "foo",
			"new_string": "bar\nbaz",
		}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("au2", "2025-01-01T00:00:02.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_a2_grep", "found 3 matches", false),
			makeToolResultBlock("toolu_a2_edit", "edit failed", true),
		}) + "\n" +
		makeAssistantMessage("ab2", "2025-01-01T00:00:02.7Z", "claude-opus-4", 80, 40, []map[string]interface{}{
			makeTextBlock("Error with [REDACTED:API_KEY] credential"),
		}) + "\n"

	agent3Jsonl := "" // empty agent

	agentContents := map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
		"agent3": []byte(agent3Jsonl),
	}

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), agentContents)
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}
	return fc
}

// TestComputeFromFileCollection_MultiAgent runs all 8 analyzers on a multi-agent
// fixture and verifies every field of the ComputeResult. This is the primary
// test that locks in current behavior before the streaming refactor.
func TestComputeFromFileCollection_MultiAgent(t *testing.T) {
	fc := buildMultiAgentFixture(t)

	result, err := ComputeFromFileCollection(context.Background(), fc)
	if err != nil {
		t.Fatalf("ComputeFromFileCollection failed: %v", err)
	}

	// --- Tokens (TokensAnalyzer) ---
	t.Run("Tokens", func(t *testing.T) {
		// Main: a1(100,50) + a2(150,75) + a3(80,40) = 330 input, 165 output
		// Agent1: aa1(150,75) + aa2(100,50) + aa3(50,25) = 300 input, 150 output
		// Agent2: ab1(200,100) + ab2(80,40) = 280 input, 140 output
		// Agent1 has file → no fallback. Agent3 is empty → no contribution.
		// Total: 330+300+280 = 910 input, 165+150+140 = 455 output
		if result.InputTokens != 910 {
			t.Errorf("InputTokens = %d, want 910", result.InputTokens)
		}
		if result.OutputTokens != 455 {
			t.Errorf("OutputTokens = %d, want 455", result.OutputTokens)
		}
		if result.EstimatedCostUSD.IsZero() {
			t.Error("EstimatedCostUSD should not be zero")
		}
	})

	// --- Session (SessionAnalyzer) ---
	t.Run("Session", func(t *testing.T) {
		// Main: 8 lines total (u1, a1, u2, u3, u4, a2, u5, a3)
		if result.TotalMessages != 8 {
			t.Errorf("TotalMessages = %d, want 8", result.TotalMessages)
		}
		// User messages: u1, u2, u3, u4, u5 = 5
		if result.UserMessages != 5 {
			t.Errorf("UserMessages = %d, want 5", result.UserMessages)
		}
		// Assistant messages (deduped): a1, a2, a3 = 3
		if result.AssistantMessages != 3 {
			t.Errorf("AssistantMessages = %d, want 3", result.AssistantMessages)
		}
		// Human prompts: u1, u4 = 2
		if result.HumanPrompts != 2 {
			t.Errorf("HumanPrompts = %d, want 2", result.HumanPrompts)
		}
		// Tool results: u2, u3, u5 = 3
		if result.ToolResults != 3 {
			t.Errorf("ToolResults = %d, want 3", result.ToolResults)
		}
		// Models: claude-sonnet-4-20241022 (main) + claude-haiku-3 (agent1) + claude-opus-4 (agent2) = 3
		sort.Strings(result.ModelsUsed)
		wantModels := []string{"claude-haiku-3", "claude-opus-4", "claude-sonnet-4-20241022"}
		if !reflect.DeepEqual(result.ModelsUsed, wantModels) {
			t.Errorf("ModelsUsed = %v, want %v", result.ModelsUsed, wantModels)
		}
		// Duration: 00:00:00 to 00:01:03 = 63000ms
		if result.DurationMs == nil {
			t.Fatal("DurationMs should not be nil")
		}
		if *result.DurationMs != 63000 {
			t.Errorf("DurationMs = %d, want 63000", *result.DurationMs)
		}
	})

	// --- Tools (ToolsAnalyzer) ---
	t.Run("Tools", func(t *testing.T) {
		// Main: Read(toolu_read1), Task(toolu_task1), Skill(toolu_skill1) = 3
		// Agent1: Read(toolu_a1_read), Write(toolu_a1_write) = 2
		// Agent2: Grep(toolu_a2_grep), Edit(toolu_a2_edit) = 2
		// Agent1 has file → no fallback for totalToolUseCount
		// Total: 3+2+2 = 7
		if result.TotalToolCalls != 7 {
			t.Errorf("TotalToolCalls = %d, want 7", result.TotalToolCalls)
		}
		// Read: 1 main + 1 agent1 = 2 success
		if result.ToolStats["Read"] == nil || result.ToolStats["Read"].Success != 2 {
			t.Errorf("Read.Success = %v, want 2", result.ToolStats["Read"])
		}
		// Write: 1 agent1 success
		if result.ToolStats["Write"] == nil || result.ToolStats["Write"].Success != 1 {
			t.Errorf("Write.Success = %v, want 1", result.ToolStats["Write"])
		}
		// Grep: 1 agent2 success
		if result.ToolStats["Grep"] == nil || result.ToolStats["Grep"].Success != 1 {
			t.Errorf("Grep.Success = %v, want 1", result.ToolStats["Grep"])
		}
		// Edit: 1 agent2, errored
		if result.ToolStats["Edit"] == nil || result.ToolStats["Edit"].Errors != 1 {
			t.Errorf("Edit.Errors = %v, want 1", result.ToolStats["Edit"])
		}
		// Total errors: 1 (Edit)
		if result.ToolErrorCount != 1 {
			t.Errorf("ToolErrorCount = %d, want 1", result.ToolErrorCount)
		}
	})

	// --- Code Activity (CodeActivityAnalyzer) ---
	t.Run("CodeActivity", func(t *testing.T) {
		// Files read: /main.go (main), /agent1.go (agent1) = 2
		if result.FilesRead != 2 {
			t.Errorf("FilesRead = %d, want 2", result.FilesRead)
		}
		// Files modified: /output.ts (agent1 Write), /shared.go (agent2 Edit) = 2
		if result.FilesModified != 2 {
			t.Errorf("FilesModified = %d, want 2", result.FilesModified)
		}
		// Lines added: Write content "line1\nline2\nline3" = 3 lines
		//              Edit new_string "bar\nbaz" = 2 lines
		//              Total: 5
		if result.LinesAdded != 5 {
			t.Errorf("LinesAdded = %d, want 5", result.LinesAdded)
		}
		// Lines removed: Edit old_string "foo" = 1 line
		if result.LinesRemoved != 1 {
			t.Errorf("LinesRemoved = %d, want 1", result.LinesRemoved)
		}
		// Searches: Grep (agent2) = 1
		if result.SearchCount != 1 {
			t.Errorf("SearchCount = %d, want 1", result.SearchCount)
		}
		// Language breakdown: .go (main.go, agent1.go, shared.go) = 3, .ts (output.ts) = 1
		if result.LanguageBreakdown["go"] != 3 {
			t.Errorf("LanguageBreakdown[go] = %d, want 3", result.LanguageBreakdown["go"])
		}
		if result.LanguageBreakdown["ts"] != 1 {
			t.Errorf("LanguageBreakdown[ts] = %d, want 1", result.LanguageBreakdown["ts"])
		}
	})

	// --- Conversation (ConversationAnalyzer) ---
	t.Run("Conversation", func(t *testing.T) {
		// Main-only: 2 human prompts = 2 user turns
		if result.UserTurns != 2 {
			t.Errorf("UserTurns = %d, want 2", result.UserTurns)
		}
		// Both prompts triggered responses: u1→a1, u4→a2,a3
		if result.AssistantTurns != 2 {
			t.Errorf("AssistantTurns = %d, want 2", result.AssistantTurns)
		}
	})

	// --- Agents (AgentsAnalyzer) ---
	t.Run("Agents", func(t *testing.T) {
		// 1 Task invocation → 1 agent invocation (Explore)
		if result.TotalAgentInvocations != 1 {
			t.Errorf("TotalAgentInvocations = %d, want 1", result.TotalAgentInvocations)
		}
		if result.AgentStats["Explore"] == nil || result.AgentStats["Explore"].Success != 1 {
			t.Errorf("AgentStats[Explore] = %v, want 1 success", result.AgentStats["Explore"])
		}
	})

	// --- Skills (SkillsAnalyzer) ---
	t.Run("Skills", func(t *testing.T) {
		// 1 Skill invocation (commit)
		if result.TotalSkillInvocations != 1 {
			t.Errorf("TotalSkillInvocations = %d, want 1", result.TotalSkillInvocations)
		}
		if result.SkillStats["commit"] == nil || result.SkillStats["commit"].Success != 1 {
			t.Errorf("SkillStats[commit] = %v, want 1 success", result.SkillStats["commit"])
		}
	})

	// --- Redactions (RedactionsAnalyzer) ---
	t.Run("Redactions", func(t *testing.T) {
		// Main: 1x [REDACTED:API_KEY]
		// Agent1: 1x [REDACTED:PASSWORD]
		// Agent2: 1x [REDACTED:API_KEY]
		// Total: 3 redactions, API_KEY=2, PASSWORD=1
		if result.TotalRedactions != 3 {
			t.Errorf("TotalRedactions = %d, want 3", result.TotalRedactions)
		}
		if result.RedactionCounts["API_KEY"] != 2 {
			t.Errorf("RedactionCounts[API_KEY] = %d, want 2", result.RedactionCounts["API_KEY"])
		}
		if result.RedactionCounts["PASSWORD"] != 1 {
			t.Errorf("RedactionCounts[PASSWORD] = %d, want 1", result.RedactionCounts["PASSWORD"])
		}
	})

	// --- No card errors ---
	t.Run("NoCardErrors", func(t *testing.T) {
		if len(result.CardErrors) != 0 {
			t.Errorf("CardErrors = %v, want empty", result.CardErrors)
		}
	})
}

// TestTokensAnalyzer_MultipleAgents verifies token accumulation across multiple agents.
func TestTokensAnalyzer_MultipleAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{"subagent_type": "Explore"}),
		makeToolUseBlock("toolu_2", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{"agentId": "agent1", "usage": map[string]interface{}{"input_tokens": float64(999), "output_tokens": float64(999)}}) + "\n" +
		makeUserMessageWithToolUseResult("u2", "2025-01-01T00:00:03Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "Done", false),
		}, map[string]interface{}{"agentId": "agent2", "usage": map[string]interface{}{"input_tokens": float64(888), "output_tokens": float64(888)}}) + "\n"

	agent1Jsonl := makeAssistantMessageFull("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 200, 100, 10, 20, []map[string]interface{}{
		makeTextBlock("Agent 1"),
	}) + "\n"
	agent2Jsonl := makeAssistantMessageFull("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 300, 150, 30, 40, []map[string]interface{}{
		makeTextBlock("Agent 2"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 100 input, 50 output
	// Agent1 file: 200 input, 100 output, 10 cache_create, 20 cache_read
	// Agent2 file: 300 input, 150 output, 30 cache_create, 40 cache_read
	// Both agents have files → fallback NOT used (999/888 ignored)
	if result.InputTokens != 600 {
		t.Errorf("InputTokens = %d, want 600", result.InputTokens)
	}
	if result.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300", result.OutputTokens)
	}
	if result.CacheCreationTokens != 40 {
		t.Errorf("CacheCreationTokens = %d, want 40", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 60 {
		t.Errorf("CacheReadTokens = %d, want 60", result.CacheReadTokens)
	}
}

// TestTokensAnalyzer_MixedAgentCoverage tests partial agent file coverage:
// agent1 has a file, agent2 does not (fallback used).
func TestTokensAnalyzer_MixedAgentCoverage(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{"subagent_type": "Explore"}),
		makeToolUseBlock("toolu_2", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"agentId": "agent1",
			"usage":   map[string]interface{}{"input_tokens": float64(999), "output_tokens": float64(999)},
		}) + "\n" +
		makeUserMessageWithToolUseResult("u2", "2025-01-01T00:00:03Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "Done", false),
		}, map[string]interface{}{
			"agentId": "agent2",
			"usage":   map[string]interface{}{"input_tokens": float64(50), "output_tokens": float64(25)},
		}) + "\n"

	agent1Jsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 200, 100, []map[string]interface{}{
		makeTextBlock("Agent 1"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		// agent2 has NO file
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 100 input, 50 output
	// Agent1 (file): 200 input, 100 output (fallback 999/999 NOT used)
	// Agent2 (fallback): 50 input, 25 output
	if result.InputTokens != 350 {
		t.Errorf("InputTokens = %d, want 350 (100 main + 200 agent1 file + 50 agent2 fallback)", result.InputTokens)
	}
	if result.OutputTokens != 175 {
		t.Errorf("OutputTokens = %d, want 175 (50 main + 100 agent1 file + 25 agent2 fallback)", result.OutputTokens)
	}
}

// TestToolsAnalyzer_MultipleAgents verifies tool counting across multiple agents.
func TestToolsAnalyzer_MultipleAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/main.go"}),
	}) + "\n"

	agent1Jsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Read", map[string]interface{}{}),
		makeToolUseBlock("toolu_a2", "Write", map[string]interface{}{}),
	}) + "\n"
	agent2Jsonl := makeAssistantMessage("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_b1", "Grep", map[string]interface{}{}),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 1 (Read), Agent1: 2 (Read, Write), Agent2: 1 (Grep) = 4
	if result.TotalCalls != 4 {
		t.Errorf("TotalCalls = %d, want 4", result.TotalCalls)
	}
	if result.ToolStats["Read"].Success != 2 {
		t.Errorf("Read.Success = %d, want 2", result.ToolStats["Read"].Success)
	}
	if result.ToolStats["Write"].Success != 1 {
		t.Errorf("Write.Success = %d, want 1", result.ToolStats["Write"].Success)
	}
	if result.ToolStats["Grep"].Success != 1 {
		t.Errorf("Grep.Success = %d, want 1", result.ToolStats["Grep"].Success)
	}
}

// TestToolsAnalyzer_AgentToolErrors verifies tool error counting from agent files.
func TestToolsAnalyzer_AgentToolErrors(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("ok"),
	}) + "\n"

	agentJsonl := makeAssistantMessageWithStopReason("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Write", map[string]interface{}{}),
		makeToolUseBlock("toolu_a2", "Write", map[string]interface{}{}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("au1", "2025-01-01T00:00:01.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_a1", "ok", false),
			makeToolResultBlock("toolu_a2", "permission denied", true),
		}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{"agent1": []byte(agentJsonl)})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2", result.TotalCalls)
	}
	if result.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", result.ErrorCount)
	}
	if result.ToolStats["Write"].Success != 1 {
		t.Errorf("Write.Success = %d, want 1", result.ToolStats["Write"].Success)
	}
	if result.ToolStats["Write"].Errors != 1 {
		t.Errorf("Write.Errors = %d, want 1", result.ToolStats["Write"].Errors)
	}
}

// TestRedactionsAnalyzer_WithAgents verifies redaction counting across agents.
func TestRedactionsAnalyzer_WithAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Key is [REDACTED:API_KEY] and [REDACTED:API_KEY]"),
	}) + "\n"
	agent1Jsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("[REDACTED:PASSWORD] found [REDACTED:SECRET_KEY]"),
	}) + "\n"
	agent2Jsonl := makeAssistantMessage("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 50, 25, []map[string]interface{}{
		makeTextBlock("No redactions here"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 4 {
		t.Errorf("TotalRedactions = %d, want 4", result.TotalRedactions)
	}
	if result.RedactionCounts["API_KEY"] != 2 {
		t.Errorf("RedactionCounts[API_KEY] = %d, want 2", result.RedactionCounts["API_KEY"])
	}
	if result.RedactionCounts["PASSWORD"] != 1 {
		t.Errorf("RedactionCounts[PASSWORD] = %d, want 1", result.RedactionCounts["PASSWORD"])
	}
	if result.RedactionCounts["SECRET_KEY"] != 1 {
		t.Errorf("RedactionCounts[SECRET_KEY] = %d, want 1", result.RedactionCounts["SECRET_KEY"])
	}
}

// TestCodeActivityAnalyzer_MultipleAgents verifies code activity merging across agents.
func TestCodeActivityAnalyzer_MultipleAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/main.go"}),
	}) + "\n"

	agent1Jsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Read", map[string]interface{}{"file_path": "/main.go"}), // same file
		makeToolUseBlock("toolu_a2", "Write", map[string]interface{}{
			"file_path": "/new.py",
			"content":   "print('hello')\nprint('world')",
		}),
	}) + "\n"

	agent2Jsonl := makeAssistantMessage("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_b1", "Edit", map[string]interface{}{
			"file_path":  "/main.go",
			"old_string": "old line",
			"new_string": "new line 1\nnew line 2",
		}),
		makeToolUseBlock("toolu_b2", "Glob", map[string]interface{}{}),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&CodeActivityAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Files read: /main.go (deduplicated across main + agent1) = 1
	if result.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1 (deduplicated)", result.FilesRead)
	}
	// Files modified: /new.py (agent1 Write), /main.go (agent2 Edit) = 2
	if result.FilesModified != 2 {
		t.Errorf("FilesModified = %d, want 2", result.FilesModified)
	}
	// Lines added: Write "print('hello')\nprint('world')" = 2, Edit new "new line 1\nnew line 2" = 2 → 4
	if result.LinesAdded != 4 {
		t.Errorf("LinesAdded = %d, want 4", result.LinesAdded)
	}
	// Lines removed: Edit old "old line" = 1
	if result.LinesRemoved != 1 {
		t.Errorf("LinesRemoved = %d, want 1", result.LinesRemoved)
	}
	// Searches: Glob (agent2) = 1
	if result.SearchCount != 1 {
		t.Errorf("SearchCount = %d, want 1", result.SearchCount)
	}
	// Language: .go = 3 (main read + agent1 read + agent2 edit), .py = 1
	if result.LanguageBreakdown["go"] != 3 {
		t.Errorf("LanguageBreakdown[go] = %d, want 3", result.LanguageBreakdown["go"])
	}
	if result.LanguageBreakdown["py"] != 1 {
		t.Errorf("LanguageBreakdown[py] = %d, want 1", result.LanguageBreakdown["py"])
	}
}

// TestSessionAnalyzer_MultipleAgentModels verifies model collection from multiple agents.
func TestSessionAnalyzer_MultipleAgentModels(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n"
	agent1Jsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent 1"),
	}) + "\n"
	agent2Jsonl := makeAssistantMessage("ab1", "2025-01-01T00:00:02.5Z", "claude-opus-4", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent 2"),
	}) + "\n"
	// agent3 uses same model as main
	agent3Jsonl := makeAssistantMessage("ac1", "2025-01-01T00:00:03.5Z", "claude-sonnet-4", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent 3"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agent1Jsonl),
		"agent2": []byte(agent2Jsonl),
		"agent3": []byte(agent3Jsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Models: sonnet (main+agent3 deduped), haiku (agent1), opus (agent2) = 3
	sort.Strings(result.ModelsUsed)
	want := []string{"claude-haiku-3", "claude-opus-4", "claude-sonnet-4"}
	if !reflect.DeepEqual(result.ModelsUsed, want) {
		t.Errorf("ModelsUsed = %v, want %v", result.ModelsUsed, want)
	}
}

// TestEmptyAgentFile verifies an empty agent file is handled gracefully.
func TestEmptyAgentFile(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": {},          // empty
		"agent2": []byte(""), // empty string
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	// Empty agents should be skipped
	if fc.AgentCount() != 0 {
		t.Errorf("AgentCount = %d, want 0 (empty agents skipped)", fc.AgentCount())
	}

	// Should still compute fine
	result, err := ComputeFromFileCollection(context.Background(), fc)
	if err != nil {
		t.Fatalf("ComputeFromFileCollection failed: %v", err)
	}
	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
}

// TestAgentWithInvalidLines verifies an agent with only invalid JSON lines is handled.
func TestAgentWithInvalidLines(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n"

	invalidAgent := "not valid json\nalso not valid\n"
	validAgent := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Valid agent"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"bad_agent":  []byte(invalidAgent),
		"good_agent": []byte(validAgent),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	// Bad agent should be skipped (parseTranscriptFile returns error for unparseable)
	// Actually, invalid JSON lines become validation errors but the file itself parses.
	// The agent file will have 0 valid lines but still exist.
	// Let's verify computation still works.
	result, err := ComputeFromFileCollection(context.Background(), fc)
	if err != nil {
		t.Fatalf("ComputeFromFileCollection failed: %v", err)
	}

	// Main: 100 input, good_agent: 50 input
	if result.InputTokens != 150 {
		t.Errorf("InputTokens = %d, want 150", result.InputTokens)
	}
}

// TestPrepareTranscript_WithAgents verifies that PrepareTranscript includes agent content.
func TestPrepareTranscript_WithAgents(t *testing.T) {
	mainJsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "Hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeTextBlock("Main response"),
		}) + "\n"

	agentJsonl := makeUserMessage("au1", "2025-01-01T00:00:01.5Z", "Agent prompt") + "\n" +
		makeAssistantMessage("aa1", "2025-01-01T00:00:01.6Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeTextBlock("Agent response"),
		}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agentJsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	transcript, idMap := PrepareTranscript(fc)

	// Should contain content from both main and agent
	if len(transcript) == 0 {
		t.Fatal("PrepareTranscript returned empty string")
	}
	if !strings.Contains(transcript, "Main response") {
		t.Error("Transcript should contain main response text")
	}
	if !strings.Contains(transcript, "Agent response") {
		t.Error("Transcript should contain agent response text")
	}
	if !strings.Contains(transcript, "<transcript>") {
		t.Error("Transcript should start with <transcript> tag")
	}
	if !strings.Contains(transcript, "</transcript>") {
		t.Error("Transcript should end with </transcript> tag")
	}

	// idMap should have entries for messages with UUIDs
	if len(idMap) == 0 {
		t.Error("idMap should not be empty")
	}
}

// TestPrepareTranscript_ToolNameResolution verifies tool_use ID to name mapping within agent files.
func TestPrepareTranscript_ToolNameResolution(t *testing.T) {
	mainJsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "Read a file") + "\n" +
		makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_main_1", "Read", map[string]interface{}{"file_path": "/test.txt"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_main_1", "file contents", false),
		}) + "\n"

	agentJsonl := makeAssistantMessageWithStopReason("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_agent_1", "Grep", map[string]interface{}{}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("au1", "2025-01-01T00:00:01.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_agent_1", "matches found", false),
		}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agentJsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	transcript, _ := PrepareTranscript(fc)

	// Tool names should be resolved in the transcript
	if !strings.Contains(transcript, "Read") {
		t.Error("Transcript should contain 'Read' tool name from main")
	}
	if !strings.Contains(transcript, "Grep") {
		t.Error("Transcript should contain 'Grep' tool name from agent")
	}
}

// TestComputeStreaming_MatchesFileCollection verifies that ComputeStreaming produces
// the same results as ComputeFromFileCollection for the same data.
func TestComputeStreaming_MatchesFileCollection(t *testing.T) {
	fc := buildMultiAgentFixture(t)

	// Get result from the FileCollection path (which delegates to ComputeStreaming)
	fcResult, err := ComputeFromFileCollection(context.Background(), fc)
	if err != nil {
		t.Fatalf("ComputeFromFileCollection failed: %v", err)
	}

	// Get result from ComputeStreaming directly with an explicit provider
	idx := 0
	provider := func(_ context.Context) (*TranscriptFile, error) {
		if idx >= len(fc.Agents) {
			return nil, io.EOF
		}
		agent := fc.Agents[idx]
		idx++
		return agent, nil
	}
	streamResult, err := ComputeStreaming(context.Background(), fc.Main, provider)
	if err != nil {
		t.Fatalf("ComputeStreaming failed: %v", err)
	}

	// Compare key fields
	if fcResult.InputTokens != streamResult.InputTokens {
		t.Errorf("InputTokens: fc=%d, stream=%d", fcResult.InputTokens, streamResult.InputTokens)
	}
	if fcResult.OutputTokens != streamResult.OutputTokens {
		t.Errorf("OutputTokens: fc=%d, stream=%d", fcResult.OutputTokens, streamResult.OutputTokens)
	}
	if fcResult.TotalToolCalls != streamResult.TotalToolCalls {
		t.Errorf("TotalToolCalls: fc=%d, stream=%d", fcResult.TotalToolCalls, streamResult.TotalToolCalls)
	}
	if fcResult.TotalRedactions != streamResult.TotalRedactions {
		t.Errorf("TotalRedactions: fc=%d, stream=%d", fcResult.TotalRedactions, streamResult.TotalRedactions)
	}
	if fcResult.FilesRead != streamResult.FilesRead {
		t.Errorf("FilesRead: fc=%d, stream=%d", fcResult.FilesRead, streamResult.FilesRead)
	}
	if fcResult.FilesModified != streamResult.FilesModified {
		t.Errorf("FilesModified: fc=%d, stream=%d", fcResult.FilesModified, streamResult.FilesModified)
	}
	if fcResult.TotalAgentInvocations != streamResult.TotalAgentInvocations {
		t.Errorf("TotalAgentInvocations: fc=%d, stream=%d", fcResult.TotalAgentInvocations, streamResult.TotalAgentInvocations)
	}
	if fcResult.TotalSkillInvocations != streamResult.TotalSkillInvocations {
		t.Errorf("TotalSkillInvocations: fc=%d, stream=%d", fcResult.TotalSkillInvocations, streamResult.TotalSkillInvocations)
	}

	sort.Strings(fcResult.ModelsUsed)
	sort.Strings(streamResult.ModelsUsed)
	if !reflect.DeepEqual(fcResult.ModelsUsed, streamResult.ModelsUsed) {
		t.Errorf("ModelsUsed: fc=%v, stream=%v", fcResult.ModelsUsed, streamResult.ModelsUsed)
	}
}

// TestComputeStreaming_ProviderErrors verifies that agent provider errors are skipped gracefully.
func TestComputeStreaming_ProviderErrors(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n"

	main, err := parseTranscriptFile([]byte(mainJsonl), "")
	if err != nil {
		t.Fatalf("parseTranscriptFile failed: %v", err)
	}

	validAgent, err := parseTranscriptFile([]byte(
		makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeTextBlock("Agent"),
		})+"\n",
	), "agent1")
	if err != nil {
		t.Fatalf("parseTranscriptFile failed: %v", err)
	}

	callCount := 0
	provider := func(_ context.Context) (*TranscriptFile, error) {
		callCount++
		switch callCount {
		case 1:
			return nil, fmt.Errorf("download failed")
		case 2:
			return validAgent, nil
		default:
			return nil, io.EOF
		}
	}

	result, err := ComputeStreaming(context.Background(), main, provider)
	if err != nil {
		t.Fatalf("ComputeStreaming failed: %v", err)
	}

	// Main: 100, agent1: 50 → total 150
	if result.InputTokens != 150 {
		t.Errorf("InputTokens = %d, want 150", result.InputTokens)
	}
	// 1 skipped agent
	if result.SkippedAgentFiles != 1 {
		t.Errorf("SkippedAgentFiles = %d, want 1", result.SkippedAgentFiles)
	}
}

// TestComputeStreaming_NoAgents verifies streaming works with zero agents.
func TestComputeStreaming_NoAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n"

	main, err := parseTranscriptFile([]byte(mainJsonl), "")
	if err != nil {
		t.Fatalf("parseTranscriptFile failed: %v", err)
	}

	provider := func(_ context.Context) (*TranscriptFile, error) {
		return nil, io.EOF
	}

	result, err := ComputeStreaming(context.Background(), main, provider)
	if err != nil {
		t.Fatalf("ComputeStreaming failed: %v", err)
	}

	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
	if result.SkippedAgentFiles != 0 {
		t.Errorf("SkippedAgentFiles = %d, want 0", result.SkippedAgentFiles)
	}
}

// TestExtractUserMessagesText_WithAgents verifies user messages from agents are included.
func TestExtractUserMessagesText_WithAgents(t *testing.T) {
	mainJsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "Main user message") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeTextBlock("Response"),
		}) + "\n"

	agentJsonl := makeUserMessage("au1", "2025-01-01T00:00:01.5Z", "Agent user message") + "\n" +
		makeAssistantMessage("aa1", "2025-01-01T00:00:01.6Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeTextBlock("Agent response"),
		}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{
		"agent1": []byte(agentJsonl),
	})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	text := ExtractUserMessagesText(fc)

	if !strings.Contains(text, "Main user message") {
		t.Error("Should contain main user message")
	}
	if !strings.Contains(text, "Agent user message") {
		t.Error("Should contain agent user message")
	}
}
