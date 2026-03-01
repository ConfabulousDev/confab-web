package analytics

import (
	"context"
	"os"
	"testing"
)

func TestFileCollection_LineCount(t *testing.T) {
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hi")}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:00:02Z", "world") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	if fc.MainLineCount() != 3 {
		t.Errorf("MainLineCount = %d, want 3", fc.MainLineCount())
	}
}

func TestFileCollection_TimestampMap(t *testing.T) {
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hi")}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	tsMap := fc.Main.BuildTimestampMap()
	if _, ok := tsMap["u1"]; !ok {
		t.Error("TimestampMap should contain u1")
	}
	if _, ok := tsMap["a1"]; !ok {
		t.Error("TimestampMap should contain a1")
	}
}

func TestTokensAnalyzer(t *testing.T) {
	jsonl := makeAssistantMessageFull("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, 20, 30, []map[string]interface{}{makeTextBlock("Hi")}) + "\n" +
		makeAssistantMessage("a2", "2025-01-01T00:00:02Z", "claude-sonnet-4-20241022", 200, 100, []map[string]interface{}{makeTextBlock("Hello")}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", result.InputTokens)
	}
	if result.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150", result.OutputTokens)
	}
	if result.CacheCreationTokens != 20 {
		t.Errorf("CacheCreationTokens = %d, want 20", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 30 {
		t.Errorf("CacheReadTokens = %d, want 30", result.CacheReadTokens)
	}
	if result.EstimatedCostUSD.IsZero() {
		t.Error("EstimatedCostUSD should not be zero")
	}
}

func TestSessionAnalyzer(t *testing.T) {
	// Modern transcript format with content arrays
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessage("a1", "2025-01-01T00:00:10Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hello! How can I help?")}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "continue") + "\n" +
		makeAssistantMessage("a2", "2025-01-01T00:02:00Z", "claude-opus-4", 200, 100, []map[string]interface{}{makeTextBlock("Continuing...")}) + "\n" +
		makeUserMessage("u3", "2025-01-01T00:03:00Z", "done") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("SessionAnalyzer failed: %v", err)
	}

	conversation, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	// Turn counts are in the ConversationResult
	if conversation.UserTurns != 3 {
		t.Errorf("UserTurns = %d, want 3", conversation.UserTurns)
	}
	if conversation.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2", conversation.AssistantTurns)
	}

	if session.DurationMs == nil {
		t.Fatal("DurationMs should not be nil")
	}
	// From 00:00:00 to 00:03:00 = 180000ms
	if *session.DurationMs != 180000 {
		t.Errorf("DurationMs = %d, want 180000", *session.DurationMs)
	}

	if len(session.ModelsUsed) != 2 {
		t.Errorf("ModelsUsed length = %d, want 2", len(session.ModelsUsed))
	}
}

func TestSessionAnalyzer_NoDuration(t *testing.T) {
	// Single message - no duration can be computed
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("SessionAnalyzer failed: %v", err)
	}

	if session.DurationMs != nil {
		t.Error("DurationMs should be nil for single timestamp")
	}
}

func TestSessionAnalyzer_Compaction(t *testing.T) {
	jsonl := makeAssistantMessage("a1", "2025-01-01T00:00:10Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{makeTextBlock("Hi")}) + "\n" +
		makeCompactBoundaryMessage("c1", "2025-01-01T00:00:15Z", "auto", 50000) + "\n" +
		makeAssistantMessage("a2", "2025-01-01T00:01:00Z", "claude-sonnet-4", 80, 40, []map[string]interface{}{makeTextBlock("Hello")}) + "\n" +
		makeCompactBoundaryMessage("c2", "2025-01-01T00:02:00Z", "manual", 60000) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("SessionAnalyzer failed: %v", err)
	}

	if session.CompactionAuto != 1 {
		t.Errorf("CompactionAuto = %d, want 1", session.CompactionAuto)
	}
	if session.CompactionManual != 1 {
		t.Errorf("CompactionManual = %d, want 1", session.CompactionManual)
	}
	// Note: CompactionAvgTimeMs requires logicalParentUuid which is not set in our helper
	// The test previously expected timing, but now we just verify counts
}

func TestSessionAnalyzer_MessageBreakdown(t *testing.T) {
	// Realistic JSONL with all message types:
	// - 2 human prompts (user with string content)
	// - 3 tool results (user with tool_result array)
	// - 2 text responses (assistant with text blocks)
	// - 2 tool calls (assistant with only tool_use)
	// - 1 thinking only (assistant with only thinking)
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "Hello, please read a file") + "\n" +
		// Text response with tool_use (counts as text response, not tool call)
		makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeTextBlock("I'll read that file for you"),
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}) + "\n" +
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n" +
		// Tool call only (no text)
		makeAssistantMessage("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_2", "Write", map[string]interface{}{}),
		}) + "\n" +
		makeUserMessageWithToolResults("u3", "2025-01-01T00:00:04Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "ok", false),
		}) + "\n" +
		// Tool call only (no text)
		makeAssistantMessage("a3", "2025-01-01T00:00:05Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_3", "Bash", map[string]interface{}{}),
		}) + "\n" +
		makeUserMessageWithToolResults("u4", "2025-01-01T00:00:06Z", []map[string]interface{}{
			makeToolResultBlock("toolu_3", "done", false),
		}) + "\n" +
		// Thinking only
		makeAssistantMessage("a4", "2025-01-01T00:00:07Z", "claude-opus-4", 100, 50, []map[string]interface{}{
			makeThinkingBlock("Let me think about this..."),
		}) + "\n" +
		// Text response only
		makeAssistantMessage("a5", "2025-01-01T00:00:08Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeTextBlock("All done! The task is complete."),
		}) + "\n" +
		makeUserMessage("u5", "2025-01-01T00:00:09Z", "Thanks!") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("SessionAnalyzer failed: %v", err)
	}

	conversation, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	// Message counts
	if session.TotalMessages != 10 {
		t.Errorf("TotalMessages = %d, want 10", session.TotalMessages)
	}
	if session.UserMessages != 5 {
		t.Errorf("UserMessages = %d, want 5", session.UserMessages)
	}
	if session.AssistantMessages != 5 {
		t.Errorf("AssistantMessages = %d, want 5", session.AssistantMessages)
	}

	// Message type breakdown (non-exclusive: a response can be both text + tool_use)
	if session.HumanPrompts != 2 {
		t.Errorf("HumanPrompts = %d, want 2", session.HumanPrompts)
	}
	if session.ToolResults != 3 {
		t.Errorf("ToolResults = %d, want 3", session.ToolResults)
	}
	// a1 has text+tool_use → counts in BOTH TextResponses and ToolCalls
	// a2, a3 have tool_use only → ToolCalls
	// a4 has thinking only → ThinkingBlocks
	// a5 has text only → TextResponses
	if session.TextResponses != 2 {
		t.Errorf("TextResponses = %d, want 2 (a1 text+tool, a5 text)", session.TextResponses)
	}
	if session.ToolCalls != 3 {
		t.Errorf("ToolCalls = %d, want 3 (a1 text+tool, a2 tool, a3 tool)", session.ToolCalls)
	}
	if session.ThinkingBlocks != 1 {
		t.Errorf("ThinkingBlocks = %d, want 1", session.ThinkingBlocks)
	}

	// Turns are in the ConversationResult
	// UserTurns = human prompts
	if conversation.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2 (should equal HumanPrompts)", conversation.UserTurns)
	}
	// AssistantTurns = user-prompt-triggered sequences that got at least one response
	// u1 triggered a1..a5 → 1 turn; u5 ("Thanks!") has no response → 0
	if conversation.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1 (only first prompt triggered responses)", conversation.AssistantTurns)
	}
}

func TestTokensAnalyzer_AgentUsage(t *testing.T) {
	// JSONL with assistant message and a tool_result containing agent usage
	// NOTE: toolUseResult is at the top level of the transcript line, not inside the content block
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{
			"prompt":        "Do something",
			"subagent_type": "Explore",
		}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			{"type": "tool_result", "tool_use_id": "toolu_1", "content": []map[string]interface{}{{"type": "text", "text": "Agent completed"}}},
		}, map[string]interface{}{
			"status":      "completed",
			"agentId":     "abc123",
			"totalTokens": float64(1000),
			"usage": map[string]interface{}{
				"input_tokens":                 float64(50),
				"output_tokens":                float64(200),
				"cache_creation_input_tokens":  float64(100),
				"cache_read_input_tokens":      float64(500),
			},
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main transcript: 100 input + 50 output
	// Agent: 50 input + 200 output + 100 cache_create + 500 cache_read
	if result.InputTokens != 150 {
		t.Errorf("InputTokens = %d, want 150 (100 main + 50 agent)", result.InputTokens)
	}
	if result.OutputTokens != 250 {
		t.Errorf("OutputTokens = %d, want 250 (50 main + 200 agent)", result.OutputTokens)
	}
	if result.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", result.CacheCreationTokens)
	}
	if result.CacheReadTokens != 500 {
		t.Errorf("CacheReadTokens = %d, want 500", result.CacheReadTokens)
	}
}

func TestTokensAnalyzer_NonAgentToolResult(t *testing.T) {
	// JSONL with a regular tool_result (no agentId) - should NOT count extra tokens
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/test.txt"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Only main transcript tokens should be counted
	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
	if result.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", result.OutputTokens)
	}
}

func TestToolsAnalyzer(t *testing.T) {
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Reading file"),
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/tmp/test.txt"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n" +
		makeAssistantMessageWithStopReason("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_2", "Read", map[string]interface{}{}),
			makeToolUseBlock("toolu_3", "Write", map[string]interface{}{}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:04Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "ok", false),
			makeToolResultBlock("toolu_3", "error", true),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalCalls != 3 {
		t.Errorf("TotalCalls = %d, want 3", result.TotalCalls)
	}
	// Read: 2 calls, 0 errors
	if result.ToolStats["Read"] == nil {
		t.Fatal("ToolStats[Read] is nil")
	}
	if result.ToolStats["Read"].Success != 2 {
		t.Errorf("ToolStats[Read].Success = %d, want 2", result.ToolStats["Read"].Success)
	}
	if result.ToolStats["Read"].Errors != 0 {
		t.Errorf("ToolStats[Read].Errors = %d, want 0", result.ToolStats["Read"].Errors)
	}
	// Write: 1 call, 1 error (so 0 success)
	if result.ToolStats["Write"] == nil {
		t.Fatal("ToolStats[Write] is nil")
	}
	if result.ToolStats["Write"].Success != 0 {
		t.Errorf("ToolStats[Write].Success = %d, want 0", result.ToolStats["Write"].Success)
	}
	if result.ToolStats["Write"].Errors != 1 {
		t.Errorf("ToolStats[Write].Errors = %d, want 1", result.ToolStats["Write"].Errors)
	}
	if result.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", result.ErrorCount)
	}
}

func TestToolsAnalyzer_AgentToolCalls(t *testing.T) {
	// JSONL with a main tool call (Read) and a Task tool that spawned an agent with 25 tool calls
	// NOTE: toolUseResult is at the top level of the user message, not inside content blocks
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/test.txt"}),
		makeToolUseBlock("toolu_2", "Task", map[string]interface{}{"prompt": "Do something", "subagent_type": "Explore"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1a", "2025-01-01T00:00:01.5Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			{"type": "tool_result", "tool_use_id": "toolu_2", "content": []map[string]interface{}{{"type": "text", "text": "Done"}}},
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "abc123",
			"totalToolUseCount": float64(25),
			"usage":             map[string]interface{}{"input_tokens": float64(500), "output_tokens": float64(1000)},
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// TotalCalls should be 2 (main) + 25 (agent) = 27
	if result.TotalCalls != 27 {
		t.Errorf("TotalCalls = %d, want 27 (2 main + 25 agent)", result.TotalCalls)
	}

	// Per-tool breakdown only includes main transcript tools
	if result.ToolStats["Read"] == nil || result.ToolStats["Read"].Success != 1 {
		t.Errorf("Read tool should have 1 success call")
	}
	if result.ToolStats["Task"] == nil || result.ToolStats["Task"].Success != 1 {
		t.Errorf("Task tool should have 1 success call")
	}
}

func TestComputeFromJSONL(t *testing.T) {
	// Integration test using the main entry point
	jsonl := makeAssistantMessage("a1", "2025-01-01T00:00:10Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n" +
		makeCompactBoundaryMessageWithParent("c1", "2025-01-01T00:00:15Z", "auto", 50000, "a1") + "\n"

	result, err := ComputeFromJSONL(context.Background(), []byte(jsonl))
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Verify both tokens and session data are computed
	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
	if result.CompactionAuto != 1 {
		t.Errorf("CompactionAuto = %d, want 1", result.CompactionAuto)
	}
}

func TestFileCollectionWithAgents(t *testing.T) {
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(10),
			"usage":             map[string]interface{}{"input_tokens": float64(200), "output_tokens": float64(100)},
		}) + "\n"
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 200, 100, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Read", map[string]interface{}{}),
	}) + "\n" +
		makeUserMessageWithToolResults("au1", "2025-01-01T00:00:01.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_a1", "file contents", false),
		}) + "\n"

	agentContents := map[string][]byte{
		"agent1": []byte(agentJsonl),
	}

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), agentContents)
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	// Test HasAgentFile
	if !fc.HasAgentFile("agent1") {
		t.Error("HasAgentFile should return true for agent1")
	}
	if fc.HasAgentFile("agent2") {
		t.Error("HasAgentFile should return false for agent2")
	}

	// Test AgentCount
	if fc.AgentCount() != 1 {
		t.Errorf("AgentCount = %d, want 1", fc.AgentCount())
	}

	// Test TotalLineCount
	if fc.TotalLineCount() != 4 { // 2 main + 2 agent
		t.Errorf("TotalLineCount = %d, want 4", fc.TotalLineCount())
	}

	// Test AllFiles
	allFiles := fc.AllFiles()
	if len(allFiles) != 2 {
		t.Errorf("AllFiles length = %d, want 2", len(allFiles))
	}
}

func TestTokensAnalyzer_WithAgentFile(t *testing.T) {
	// Main transcript with Task tool and toolUseResult
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(5),
			"usage":             map[string]interface{}{"input_tokens": float64(200), "output_tokens": float64(100)},
		}) + "\n"
	// Agent file with actual token usage
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 150, 75, []map[string]interface{}{
		makeTextBlock("Thinking..."),
	}) + "\n" +
		makeAssistantMessage("aa2", "2025-01-01T00:00:01.6Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeTextBlock("Done"),
		}) + "\n"

	// With agent file: should use agent file tokens (not toolUseResult fallback)
	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{"agent1": []byte(agentJsonl)})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 100 input, 50 output
	// Agent: 150+50=200 input, 75+25=100 output
	// Total: 300 input, 150 output
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300 (100 main + 200 agent)", result.InputTokens)
	}
	if result.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150 (50 main + 100 agent)", result.OutputTokens)
	}
}

func TestTokensAnalyzer_FallbackWithoutAgentFile(t *testing.T) {
	// Main transcript with Task tool and toolUseResult, but NO agent file
	// NOTE: toolUseResult is at the top level of the user message
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(5),
			"usage":             map[string]interface{}{"input_tokens": float64(200), "output_tokens": float64(100)},
		}) + "\n"

	// Without agent file: should use toolUseResult fallback
	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), nil)
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 100 input, 50 output
	// Fallback: 200 input, 100 output
	// Total: 300 input, 150 output
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300 (100 main + 200 fallback)", result.InputTokens)
	}
	if result.OutputTokens != 150 {
		t.Errorf("OutputTokens = %d, want 150 (50 main + 100 fallback)", result.OutputTokens)
	}
}

func TestToolsAnalyzer_WithAgentFile(t *testing.T) {
	// Main transcript with Task tool
	// NOTE: toolUseResult is at the top level of the user message
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		makeToolUseBlock("toolu_2", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolResults("u1a", "2025-01-01T00:00:01.5Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "Done", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(10),
			"usage":             map[string]interface{}{},
		}) + "\n"
	// Agent file with 3 tool calls
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Read", map[string]interface{}{}),
	}) + "\n" +
		makeUserMessageWithToolResults("au1", "2025-01-01T00:00:01.6Z", []map[string]interface{}{
			makeToolResultBlock("toolu_a1", "ok", false),
		}) + "\n" +
		makeAssistantMessage("aa2", "2025-01-01T00:00:01.7Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
			makeToolUseBlock("toolu_a2", "Write", map[string]interface{}{}),
			makeToolUseBlock("toolu_a3", "Grep", map[string]interface{}{}),
		}) + "\n"

	// With agent file: should count agent tool calls directly (not use totalToolUseCount)
	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{"agent1": []byte(agentJsonl)})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 2 tool calls (Read, Task)
	// Agent: 3 tool calls (Read, Write, Grep)
	// Total: 5 (NOT 2 + 10 from totalToolUseCount)
	if result.TotalCalls != 5 {
		t.Errorf("TotalCalls = %d, want 5 (2 main + 3 agent)", result.TotalCalls)
	}

	// Should have per-tool breakdown from agent
	if result.ToolStats["Read"].Success != 2 { // 1 main + 1 agent
		t.Errorf("Read.Success = %d, want 2", result.ToolStats["Read"].Success)
	}
	if result.ToolStats["Write"].Success != 1 {
		t.Errorf("Write.Success = %d, want 1", result.ToolStats["Write"].Success)
	}
	if result.ToolStats["Grep"].Success != 1 {
		t.Errorf("Grep.Success = %d, want 1", result.ToolStats["Grep"].Success)
	}
}

func TestToolsAnalyzer_FallbackWithoutAgentFile(t *testing.T) {
	// Main transcript with Task tool, but NO agent file
	// NOTE: toolUseResult is at the top level of the user message
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		makeToolUseBlock("toolu_2", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}) + "\n" +
		makeUserMessageWithToolResults("u1a", "2025-01-01T00:00:01.5Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_2", "Done", false),
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent1",
			"totalToolUseCount": float64(10),
			"usage":             map[string]interface{}{},
		}) + "\n"

	// Without agent file: should use totalToolUseCount fallback
	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), nil)
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&ToolsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Main: 2 tool calls + 10 from totalToolUseCount = 12
	if result.TotalCalls != 12 {
		t.Errorf("TotalCalls = %d, want 12 (2 main + 10 fallback)", result.TotalCalls)
	}
}

func TestSessionAnalyzer_AgentModels(t *testing.T) {
	// Main transcript uses sonnet
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeTextBlock("Hello"),
	}) + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{
			"agentId": "agent1",
		}) + "\n"
	// Agent uses haiku
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeTextBlock("Agent response"),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{"agent1": []byte(agentJsonl)})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should include both models
	if len(result.ModelsUsed) != 2 {
		t.Errorf("ModelsUsed length = %d, want 2", len(result.ModelsUsed))
	}

	hasHaiku := false
	hasSonnet := false
	for _, m := range result.ModelsUsed {
		if m == "claude-haiku-3" {
			hasHaiku = true
		}
		if m == "claude-sonnet-4" {
			hasSonnet = true
		}
	}
	if !hasHaiku {
		t.Error("ModelsUsed should include claude-haiku-3 from agent")
	}
	if !hasSonnet {
		t.Error("ModelsUsed should include claude-sonnet-4 from main")
	}
}

func TestCodeActivityAnalyzer_WithAgentFile(t *testing.T) {
	// Main transcript reads one file
	mainJsonl := makeAssistantMessage("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/main.go"}),
	}) + "\n"
	// Agent reads another file
	agentJsonl := makeAssistantMessage("aa1", "2025-01-01T00:00:01.5Z", "claude-haiku-3", 50, 25, []map[string]interface{}{
		makeToolUseBlock("toolu_a1", "Read", map[string]interface{}{"file_path": "/agent.go"}),
	}) + "\n"

	fc, err := NewFileCollectionWithAgents([]byte(mainJsonl), map[string][]byte{"agent1": []byte(agentJsonl)})
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&CodeActivityAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should count files from both main and agent
	if result.FilesRead != 2 {
		t.Errorf("FilesRead = %d, want 2", result.FilesRead)
	}
}

func TestAgentsAnalyzer_BasicAgentInvocation(t *testing.T) {
	// Real JSONL format: Task tool_use followed by tool_result with top-level toolUseResult
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_01ABC123", "Task", map[string]interface{}{
			"description":   "Explore codebase",
			"prompt":        "Find the main function",
			"subagent_type": "Explore",
		}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:10Z", []map[string]interface{}{
			{"type": "tool_result", "tool_use_id": "toolu_01ABC123", "content": []map[string]interface{}{{"type": "text", "text": "Found main function in main.go"}}},
		}, map[string]interface{}{
			"status":            "completed",
			"agentId":           "agent_xyz",
			"totalTokens":       float64(5000),
			"totalToolUseCount": float64(12),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 1 {
		t.Errorf("TotalInvocations = %d, want 1", result.TotalInvocations)
	}

	if result.AgentStats["Explore"] == nil {
		t.Fatal("AgentStats should have 'Explore' entry")
	}
	if result.AgentStats["Explore"].Success != 1 {
		t.Errorf("AgentStats[Explore].Success = %d, want 1", result.AgentStats["Explore"].Success)
	}
	if result.AgentStats["Explore"].Errors != 0 {
		t.Errorf("AgentStats[Explore].Errors = %d, want 0", result.AgentStats["Explore"].Errors)
	}
}

func TestAgentsAnalyzer_MultipleAgentTypes(t *testing.T) {
	// Multiple agent invocations of different types
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_explore", "Task", map[string]interface{}{"subagent_type": "Explore"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_explore", "Done", false),
		}, map[string]interface{}{"agentId": "agent1"}) + "\n" +
		makeAssistantMessageWithStopReason("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_plan", "Task", map[string]interface{}{"subagent_type": "Plan"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u2", "2025-01-01T00:00:04Z", []map[string]interface{}{
			makeToolResultBlock("toolu_plan", "Done", false),
		}, map[string]interface{}{"agentId": "agent2"}) + "\n" +
		makeAssistantMessageWithStopReason("a3", "2025-01-01T00:00:05Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_explore2", "Task", map[string]interface{}{"subagent_type": "Explore"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u3", "2025-01-01T00:00:06Z", []map[string]interface{}{
			makeToolResultBlock("toolu_explore2", "error", true),
		}, map[string]interface{}{"agentId": "agent3"}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 3 {
		t.Errorf("TotalInvocations = %d, want 3", result.TotalInvocations)
	}

	// Check Explore stats (2 invocations: 1 success, 1 error)
	if result.AgentStats["Explore"] == nil {
		t.Fatal("AgentStats should have 'Explore' entry")
	}
	if result.AgentStats["Explore"].Success != 1 {
		t.Errorf("AgentStats[Explore].Success = %d, want 1", result.AgentStats["Explore"].Success)
	}
	if result.AgentStats["Explore"].Errors != 1 {
		t.Errorf("AgentStats[Explore].Errors = %d, want 1", result.AgentStats["Explore"].Errors)
	}

	// Check Plan stats (1 success)
	if result.AgentStats["Plan"] == nil {
		t.Fatal("AgentStats should have 'Plan' entry")
	}
	if result.AgentStats["Plan"].Success != 1 {
		t.Errorf("AgentStats[Plan].Success = %d, want 1", result.AgentStats["Plan"].Success)
	}
}

func TestAgentsAnalyzer_NoAgentInvocations(t *testing.T) {
	// Regular tool usage without Task/agents
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/test.txt"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 0 {
		t.Errorf("TotalInvocations = %d, want 0", result.TotalInvocations)
	}
	if len(result.AgentStats) != 0 {
		t.Errorf("AgentStats length = %d, want 0", len(result.AgentStats))
	}
}

func TestAgentsAnalyzer_UnknownAgentType(t *testing.T) {
	// Task tool without subagent_type but with agentId in result (should count as "unknown")
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Task", map[string]interface{}{"prompt": "Do something"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolUseResult("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "Done", false),
		}, map[string]interface{}{"agentId": "agent_orphan"}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 1 {
		t.Errorf("TotalInvocations = %d, want 1", result.TotalInvocations)
	}

	if result.AgentStats["unknown"] == nil {
		t.Fatal("AgentStats should have 'unknown' entry for Task without subagent_type")
	}
	if result.AgentStats["unknown"].Success != 1 {
		t.Errorf("AgentStats[unknown].Success = %d, want 1", result.AgentStats["unknown"].Success)
	}
}

// TestAgentsAnalyzer_RealSession tests the analyzer against a real session transcript.
// The test fixture is a copy of an actual Claude Code session.
// Expected values derived from testdata/session_comprehensive.jsonl:
//   - 1 Task tool invocation with subagent_type "Explore"
//   - 1 successful agent result
func TestAgentsAnalyzer_RealSession(t *testing.T) {
	content, err := os.ReadFile("testdata/session_comprehensive.jsonl")
	if err != nil {
		t.Fatalf("Failed to read test fixture: %v", err)
	}

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&AgentsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	t.Run("TotalInvocations", func(t *testing.T) {
		expected := 1
		if result.TotalInvocations != expected {
			t.Errorf("TotalInvocations = %d, want %d", result.TotalInvocations, expected)
		}
	})

	t.Run("ExploreAgentStats", func(t *testing.T) {
		if result.AgentStats["Explore"] == nil {
			t.Fatal("AgentStats should have 'Explore' entry")
		}
		if result.AgentStats["Explore"].Success != 1 {
			t.Errorf("AgentStats[Explore].Success = %d, want 1", result.AgentStats["Explore"].Success)
		}
		if result.AgentStats["Explore"].Errors != 0 {
			t.Errorf("AgentStats[Explore].Errors = %d, want 0", result.AgentStats["Explore"].Errors)
		}
	})
}

func TestSkillsAnalyzer_BasicSkillInvocation(t *testing.T) {
	// Skill tool_use followed by tool_result
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_skill1", "Skill", map[string]interface{}{"skill": "commit", "args": "-m 'test'"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_skill1", "Skill executed successfully", false),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 1 {
		t.Errorf("TotalInvocations = %d, want 1", result.TotalInvocations)
	}

	if result.SkillStats["commit"] == nil {
		t.Fatal("SkillStats should have 'commit' entry")
	}
	if result.SkillStats["commit"].Success != 1 {
		t.Errorf("SkillStats[commit].Success = %d, want 1", result.SkillStats["commit"].Success)
	}
	if result.SkillStats["commit"].Errors != 0 {
		t.Errorf("SkillStats[commit].Errors = %d, want 0", result.SkillStats["commit"].Errors)
	}
}

func TestSkillsAnalyzer_MultipleSkillTypes(t *testing.T) {
	// Multiple skill invocations of different types
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_s1", "Skill", map[string]interface{}{"skill": "commit"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_s1", "Done", false),
		}) + "\n" +
		makeAssistantMessageWithStopReason("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_s2", "Skill", map[string]interface{}{"skill": "codebase-maintenance"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u2", "2025-01-01T00:00:04Z", []map[string]interface{}{
			makeToolResultBlock("toolu_s2", "Done", false),
		}) + "\n" +
		makeAssistantMessageWithStopReason("a3", "2025-01-01T00:00:05Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
			makeToolUseBlock("toolu_s3", "Skill", map[string]interface{}{"skill": "commit"}),
		}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u3", "2025-01-01T00:00:06Z", []map[string]interface{}{
			makeToolResultBlock("toolu_s3", "error", true),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 3 {
		t.Errorf("TotalInvocations = %d, want 3", result.TotalInvocations)
	}

	// Check commit stats (2 invocations: 1 success, 1 error)
	if result.SkillStats["commit"] == nil {
		t.Fatal("SkillStats should have 'commit' entry")
	}
	if result.SkillStats["commit"].Success != 1 {
		t.Errorf("SkillStats[commit].Success = %d, want 1", result.SkillStats["commit"].Success)
	}
	if result.SkillStats["commit"].Errors != 1 {
		t.Errorf("SkillStats[commit].Errors = %d, want 1", result.SkillStats["commit"].Errors)
	}

	// Check codebase-maintenance stats (1 success)
	if result.SkillStats["codebase-maintenance"] == nil {
		t.Fatal("SkillStats should have 'codebase-maintenance' entry")
	}
	if result.SkillStats["codebase-maintenance"].Success != 1 {
		t.Errorf("SkillStats[codebase-maintenance].Success = %d, want 1", result.SkillStats["codebase-maintenance"].Success)
	}
}

func TestSkillsAnalyzer_NoSkillInvocations(t *testing.T) {
	// Regular tool usage without Skills
	jsonl := makeAssistantMessageWithStopReason("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", 100, 50, []map[string]interface{}{
		makeToolUseBlock("toolu_1", "Read", map[string]interface{}{"file_path": "/test.txt"}),
	}, "tool_use") + "\n" +
		makeUserMessageWithToolResults("u1", "2025-01-01T00:00:02Z", []map[string]interface{}{
			makeToolResultBlock("toolu_1", "file contents", false),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&SkillsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalInvocations != 0 {
		t.Errorf("TotalInvocations = %d, want 0", result.TotalInvocations)
	}
	if len(result.SkillStats) != 0 {
		t.Errorf("SkillStats length = %d, want 0", len(result.SkillStats))
	}
}

// =============================================================================
// Message ID deduplication tests
// =============================================================================

func TestAssistantMessageGroups_MultiLinePerResponse(t *testing.T) {
	// Same message ID appears 3 times (thinking, text, tool_use) — one API response
	// Output tokens grow incrementally: 10 → 50 → 80 (last is final)
	jsonl := makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 10, []map[string]interface{}{
		makeThinkingBlock("Let me think..."),
	}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("Here's my response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a3", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 80, []map[string]interface{}{
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	groups := fc.Main.AssistantMessageGroups()

	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.MessageID != "msg-001" {
		t.Errorf("MessageID = %q, want msg-001", g.MessageID)
	}
	if g.Model != "claude-sonnet-4" {
		t.Errorf("Model = %q, want claude-sonnet-4", g.Model)
	}
	if !g.HasThinking {
		t.Error("HasThinking should be true")
	}
	if !g.HasText {
		t.Error("HasText should be true")
	}
	if !g.HasToolUse {
		t.Error("HasToolUse should be true")
	}
	if g.FinalUsage == nil {
		t.Fatal("FinalUsage should not be nil")
	}
	// Should use the LAST line's output_tokens (80), not the sum
	if g.FinalUsage.OutputTokens != 80 {
		t.Errorf("FinalUsage.OutputTokens = %d, want 80 (last occurrence)", g.FinalUsage.OutputTokens)
	}
	if g.FinalUsage.InputTokens != 100 {
		t.Errorf("FinalUsage.InputTokens = %d, want 100", g.FinalUsage.InputTokens)
	}
}

func TestAssistantMessageGroups_ContextReplay(t *testing.T) {
	// msg-001 appears, then msg-002, then msg-001 again (context replay).
	// Replayed message has identical usage to original — shouldn't be double-counted.
	jsonl := makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
		makeTextBlock("First response"),
	}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:02Z", "claude-sonnet-4", "msg-002", 200, 100, []map[string]interface{}{
			makeTextBlock("Second response"),
		}) + "\n" +
		// Context replay of msg-001 (hundreds of lines later in practice)
		makeAssistantMessageWithMsgID("a3", "2025-01-01T00:00:03Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("First response"),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	groups := fc.Main.AssistantMessageGroups()

	if len(groups) != 2 {
		t.Fatalf("Expected 2 groups (msg-001 deduped), got %d", len(groups))
	}

	if groups[0].MessageID != "msg-001" {
		t.Errorf("First group MessageID = %q, want msg-001", groups[0].MessageID)
	}
	if groups[1].MessageID != "msg-002" {
		t.Errorf("Second group MessageID = %q, want msg-002", groups[1].MessageID)
	}
}

func TestTokensAnalyzer_Dedup(t *testing.T) {
	// Same message ID with 3 lines (incremental output_tokens: 10, 50, 80)
	// Plus a distinct message
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", "msg-001", 100, 10, []map[string]interface{}{
			makeThinkingBlock("thinking..."),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a3", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", "msg-001", 100, 80, []map[string]interface{}{
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}) + "\n" +
		// Second distinct message
		makeAssistantMessageWithMsgID("a4", "2025-01-01T00:00:02Z", "claude-sonnet-4-20241022", "msg-002", 200, 60, []map[string]interface{}{
			makeTextBlock("another response"),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// msg-001: input=100, output=80 (final line)
	// msg-002: input=200, output=60
	// Total: 300 input, 140 output
	// WITHOUT dedup it would be: 300+200=500 input, 10+50+80+60=200 output
	if result.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", result.InputTokens)
	}
	if result.OutputTokens != 140 {
		t.Errorf("OutputTokens = %d, want 140 (80 final from msg-001 + 60 from msg-002)", result.OutputTokens)
	}
}

func TestSessionAnalyzer_Dedup(t *testing.T) {
	// 3 lines with same message ID → should count as 1 assistant message
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 10, []map[string]interface{}{
			makeThinkingBlock("thinking..."),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a3", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 80, []map[string]interface{}{
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	session, err := (&SessionAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("SessionAnalyzer failed: %v", err)
	}

	// Should count 1 assistant message (not 3)
	if session.AssistantMessages != 1 {
		t.Errorf("AssistantMessages = %d, want 1 (3 lines deduped to 1)", session.AssistantMessages)
	}

	// Non-exclusive breakdown: all content types present
	if session.TextResponses != 1 {
		t.Errorf("TextResponses = %d, want 1", session.TextResponses)
	}
	if session.ToolCalls != 1 {
		t.Errorf("ToolCalls = %d, want 1", session.ToolCalls)
	}
	if session.ThinkingBlocks != 1 {
		t.Errorf("ThinkingBlocks = %d, want 1", session.ThinkingBlocks)
	}

	// TotalMessages is still raw line count (user + 3 assistant lines = 4)
	if session.TotalMessages != 4 {
		t.Errorf("TotalMessages = %d, want 4 (raw line count)", session.TotalMessages)
	}
}

func TestConversationAnalyzer_Dedup(t *testing.T) {
	// User prompt → 3 assistant lines (same msg ID) → should be 1 assistant turn
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 10, []map[string]interface{}{
			makeThinkingBlock("thinking..."),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a3", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 80, []map[string]interface{}{
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "thanks") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	if result.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2", result.UserTurns)
	}
	// First user prompt triggered an assistant response → 1 assistant turn
	// Second user prompt has no assistant response → no assistant turn
	if result.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1 (user-prompt-triggered sequence)", result.AssistantTurns)
	}
}

func TestConversationAnalyzer_ContextReplayDedup(t *testing.T) {
	// msg-001 appears, then later replayed by context management
	// Should still count as 1 assistant turn
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		// Context replay of msg-001
		makeAssistantMessageWithMsgID("a1r", "2025-01-01T00:00:02Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:00:03Z", "claude-sonnet-4", "msg-002", 200, 100, []map[string]interface{}{
			makeTextBlock("another response"),
		}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "bye") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	if result.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2", result.UserTurns)
	}
	// First user prompt triggered assistant responses → 1 assistant turn
	if result.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1", result.AssistantTurns)
	}
}

func TestConversationAnalyzer_CrossTurnContextReplay(t *testing.T) {
	// msg-001 in turn 1, then replayed in turn 2 alongside new msg-002.
	// Turn 2 should count as an assistant turn (has new msg-002),
	// but msg-001 replay should not inflate the count.
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response 1"),
		}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "continue") + "\n" +
		// Context replay of msg-001 (re-logged by context management)
		makeAssistantMessageWithMsgID("a1r", "2025-01-01T00:01:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response 1"),
		}) + "\n" +
		// Actual new response to u2
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:01:02Z", "claude-sonnet-4", "msg-002", 200, 100, []map[string]interface{}{
			makeTextBlock("response 2"),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	if result.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2", result.UserTurns)
	}
	// Both turns got responses: u1→msg-001, u2→msg-002
	if result.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2 (both turns had new responses)", result.AssistantTurns)
	}
}

func TestConversationAnalyzer_CrossTurnReplayOnly(t *testing.T) {
	// msg-001 in turn 1, then replayed in turn 2 with NO new response.
	// Turn 2 should NOT count as an assistant turn.
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "thanks") + "\n" +
		// Context replay of msg-001 only — no new assistant response
		makeAssistantMessageWithMsgID("a1r", "2025-01-01T00:01:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&ConversationAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("ConversationAnalyzer failed: %v", err)
	}

	if result.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2", result.UserTurns)
	}
	// Only turn 1 had a new response. Turn 2 only has replay → not counted.
	if result.AssistantTurns != 1 {
		t.Errorf("AssistantTurns = %d, want 1 (turn 2 only has replay)", result.AssistantTurns)
	}
}

func TestTokensAnalyzer_ContextReplayDedup(t *testing.T) {
	// msg-001 appears twice (context replay). Tokens should not be double-counted.
	jsonl := makeUserMessage("u1", "2025-01-01T00:00:00Z", "hello") + "\n" +
		makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4-20241022", "msg-001", 500, 200, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeUserMessage("u2", "2025-01-01T00:01:00Z", "continue") + "\n" +
		// Context replay of msg-001
		makeAssistantMessageWithMsgID("a1r", "2025-01-01T00:01:01Z", "claude-sonnet-4-20241022", "msg-001", 500, 200, []map[string]interface{}{
			makeTextBlock("response"),
		}) + "\n" +
		makeAssistantMessageWithMsgID("a2", "2025-01-01T00:01:02Z", "claude-sonnet-4-20241022", "msg-002", 300, 100, []map[string]interface{}{
			makeTextBlock("new response"),
		}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&TokensAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// msg-001: input=500, output=200 (counted once despite replay)
	// msg-002: input=300, output=100
	// Total: 800 input, 300 output
	// WITHOUT dedup: 1300 input, 500 output (2x from replay)
	if result.InputTokens != 800 {
		t.Errorf("InputTokens = %d, want 800 (replay not double-counted)", result.InputTokens)
	}
	if result.OutputTokens != 300 {
		t.Errorf("OutputTokens = %d, want 300 (replay not double-counted)", result.OutputTokens)
	}
}

func TestAssistantMessageGroups_NoMessageID(t *testing.T) {
	// The validator requires message.id for assistant messages, so no-ID messages
	// are dropped during parsing. However, AssistantMessageGroups() defensively
	// handles this case with standalone groups. Test via direct struct construction
	// to exercise this path.
	tf := &TranscriptFile{
		Lines: []*TranscriptLine{
			{
				Type: "assistant",
				Message: &MessageContent{
					// No ID field set — empty string
					Model: "claude-sonnet-4",
					Usage: &TokenUsage{InputTokens: 100, OutputTokens: 50},
				},
			},
			{
				Type: "assistant",
				Message: &MessageContent{
					ID:    "msg-001",
					Model: "claude-sonnet-4",
					Usage: &TokenUsage{InputTokens: 200, OutputTokens: 100},
				},
			},
			{
				Type: "assistant",
				Message: &MessageContent{
					// No ID field set — empty string
					Model: "claude-sonnet-4",
					Usage: &TokenUsage{InputTokens: 150, OutputTokens: 75},
				},
			},
		},
	}

	groups := tf.AssistantMessageGroups()

	// 3 groups: no-ID standalone, msg-001, no-ID standalone (interleaved in order)
	if len(groups) != 3 {
		t.Fatalf("Expected 3 groups, got %d", len(groups))
	}

	// First: no-ID standalone
	if groups[0].MessageID != "" {
		t.Errorf("Group 0 MessageID = %q, want empty", groups[0].MessageID)
	}
	if groups[0].FinalUsage.InputTokens != 100 {
		t.Errorf("Group 0 InputTokens = %d, want 100", groups[0].FinalUsage.InputTokens)
	}

	// Second: msg-001
	if groups[1].MessageID != "msg-001" {
		t.Errorf("Group 1 MessageID = %q, want msg-001", groups[1].MessageID)
	}

	// Third: no-ID standalone
	if groups[2].MessageID != "" {
		t.Errorf("Group 2 MessageID = %q, want empty", groups[2].MessageID)
	}
	if groups[2].FinalUsage.InputTokens != 150 {
		t.Errorf("Group 2 InputTokens = %d, want 150", groups[2].FinalUsage.InputTokens)
	}
}

func TestAssistantMessageGroups_FastModeFlag(t *testing.T) {
	// Only one of three lines has speed="fast". The group should have IsFastMode=true.
	jsonl := makeAssistantMessageWithMsgIDAndSpeed("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 10, []map[string]interface{}{
		makeThinkingBlock("thinking..."),
	}, "") + "\n" +
		makeAssistantMessageWithMsgIDAndSpeed("a2", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
			makeTextBlock("response"),
		}, "fast") + "\n" +
		makeAssistantMessageWithMsgIDAndSpeed("a3", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 80, []map[string]interface{}{
			makeToolUseBlock("toolu_1", "Read", map[string]interface{}{}),
		}, "") + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	groups := fc.Main.AssistantMessageGroups()
	if len(groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups))
	}
	if !groups[0].IsFastMode {
		t.Error("IsFastMode should be true (any line with speed=fast)")
	}
}

func TestAssistantMessageGroups_CacheConsistency(t *testing.T) {
	jsonl := makeAssistantMessageWithMsgID("a1", "2025-01-01T00:00:01Z", "claude-sonnet-4", "msg-001", 100, 50, []map[string]interface{}{
		makeTextBlock("response"),
	}) + "\n"

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	groups1 := fc.Main.AssistantMessageGroups()
	groups2 := fc.Main.AssistantMessageGroups()

	if len(groups1) != len(groups2) {
		t.Fatalf("Cache inconsistency: first call returned %d groups, second returned %d", len(groups1), len(groups2))
	}
	if len(groups1) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(groups1))
	}
	if groups1[0].MessageID != groups2[0].MessageID {
		t.Error("Cache returned different MessageID")
	}
}
