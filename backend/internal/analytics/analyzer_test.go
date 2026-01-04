package analytics

import (
	"context"
	"os"
	"testing"
)

func TestFileCollection_LineCount(t *testing.T) {
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":"world"},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z"}
`

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	if fc.MainLineCount() != 3 {
		t.Errorf("MainLineCount = %d, want 3", fc.MainLineCount())
	}
}

func TestFileCollection_TimestampMap(t *testing.T) {
	jsonl := `{"type":"user","uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:01Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":20,"cache_read_input_tokens":30}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":200,"output_tokens":100}},"uuid":"a2","timestamp":"2025-01-01T00:00:02Z"}
`

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
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Hello! How can I help?"}]},"uuid":"a1","timestamp":"2025-01-01T00:00:10Z"}
{"type":"user","message":{"role":"user","content":"continue"},"uuid":"u2","timestamp":"2025-01-01T00:01:00Z"}
{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":200,"output_tokens":100},"content":[{"type":"text","text":"Continuing..."}]},"uuid":"a2","timestamp":"2025-01-01T00:02:00Z"}
{"type":"user","message":{"role":"user","content":"done"},"uuid":"u3","timestamp":"2025-01-01T00:03:00Z"}
`

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
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
`

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
	jsonl := `{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:10Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
{"type":"assistant","uuid":"a2","timestamp":"2025-01-01T00:01:00Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":80,"output_tokens":40}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":60000},"logicalParentUuid":"a2","uuid":"c2","timestamp":"2025-01-01T00:02:00Z"}
`

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
	if session.CompactionAvgTimeMs == nil {
		t.Fatal("CompactionAvgTimeMs should not be nil")
	}
	// a1 at 00:00:10, c1 at 00:00:15 = 5000ms
	if *session.CompactionAvgTimeMs != 5000 {
		t.Errorf("CompactionAvgTimeMs = %d, want 5000", *session.CompactionAvgTimeMs)
	}
}

func TestSessionAnalyzer_MessageBreakdown(t *testing.T) {
	// Realistic JSONL with all message types:
	// - 2 human prompts (user with string content)
	// - 3 tool results (user with tool_result array)
	// - 2 text responses (assistant with text blocks)
	// - 2 tool calls (assistant with only tool_use)
	// - 1 thinking only (assistant with only thinking)
	jsonl := `{"type":"user","message":{"role":"user","content":"Hello, please read a file"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"I'll read that file for you"},{"type":"tool_use","id":"toolu_1","name":"Read","input":{}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_2","name":"Write","input":{}}]},"uuid":"a2","timestamp":"2025-01-01T00:00:03Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"ok"}]},"uuid":"u3","timestamp":"2025-01-01T00:00:04Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_3","name":"Bash","input":{}}]},"uuid":"a3","timestamp":"2025-01-01T00:00:05Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_3","content":"done"}]},"uuid":"u4","timestamp":"2025-01-01T00:00:06Z"}
{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"thinking","thinking":"Let me think about this..."}]},"uuid":"a4","timestamp":"2025-01-01T00:00:07Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"All done! The task is complete."}]},"uuid":"a5","timestamp":"2025-01-01T00:00:08Z"}
{"type":"user","message":{"role":"user","content":"Thanks!"},"uuid":"u5","timestamp":"2025-01-01T00:00:09Z"}
`

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

	// Message type breakdown
	if session.HumanPrompts != 2 {
		t.Errorf("HumanPrompts = %d, want 2", session.HumanPrompts)
	}
	if session.ToolResults != 3 {
		t.Errorf("ToolResults = %d, want 3", session.ToolResults)
	}
	if session.TextResponses != 2 {
		t.Errorf("TextResponses = %d, want 2", session.TextResponses)
	}
	if session.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2 (tool_use only, not text+tool_use)", session.ToolCalls)
	}
	if session.ThinkingBlocks != 1 {
		t.Errorf("ThinkingBlocks = %d, want 1", session.ThinkingBlocks)
	}

	// Turns are in the ConversationResult
	if conversation.UserTurns != 2 {
		t.Errorf("UserTurns = %d, want 2 (should equal HumanPrompts)", conversation.UserTurns)
	}
	if conversation.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2 (should equal TextResponses)", conversation.AssistantTurns)
	}
}

func TestTokensAnalyzer_AgentUsage(t *testing.T) {
	// JSONL with assistant message and a tool_result containing agent usage
	// NOTE: toolUseResult is at the top level of the transcript line, not inside the content block
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{"prompt":"Do something","subagent_type":"Explore"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"Agent completed"}]}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"status":"completed","agentId":"abc123","totalTokens":1000,"usage":{"input_tokens":50,"output_tokens":200,"cache_creation_input_tokens":100,"cache_read_input_tokens":500}}}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/test.txt"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Reading file"},{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/tmp/test.txt"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_2","name":"Read","input":{}},{"type":"tool_use","id":"toolu_3","name":"Write","input":{}}],"stop_reason":"tool_use"},"uuid":"a2","timestamp":"2025-01-01T00:00:03Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"ok"},{"type":"tool_result","tool_use_id":"toolu_3","content":"error","is_error":true}]},"uuid":"u2","timestamp":"2025-01-01T00:00:04Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/test.txt"}},{"type":"tool_use","id":"toolu_2","name":"Task","input":{"prompt":"Do something","subagent_type":"Explore"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1a","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":[{"type":"text","text":"Done"}]}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"status":"completed","agentId":"abc123","totalToolUseCount":25,"usage":{"input_tokens":500,"output_tokens":1000}}}
`

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
	jsonl := `{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:10Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Done","toolUseResult":{"status":"completed","agentId":"agent1","totalToolUseCount":10,"usage":{"input_tokens":200,"output_tokens":100}}}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`
	agentJsonl := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":200,"output_tokens":100},"content":[{"type":"tool_use","id":"toolu_a1","name":"Read","input":{}}]},"uuid":"aa1","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_a1","content":"file contents"}]},"uuid":"au1","timestamp":"2025-01-01T00:00:01.6Z"}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Done","toolUseResult":{"status":"completed","agentId":"agent1","totalToolUseCount":5,"usage":{"input_tokens":200,"output_tokens":100}}}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`
	// Agent file with actual token usage
	agentJsonl := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":150,"output_tokens":75}},"uuid":"aa1","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":50,"output_tokens":25}},"uuid":"aa2","timestamp":"2025-01-01T00:00:01.6Z"}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{"subagent_type":"Explore"}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"status":"completed","agentId":"agent1","totalToolUseCount":5,"usage":{"input_tokens":200,"output_tokens":100}}}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{}},{"type":"tool_use","id":"toolu_2","name":"Task","input":{"subagent_type":"Explore"}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1a","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"status":"completed","agentId":"agent1","totalToolUseCount":10,"usage":{}}}
`
	// Agent file with 3 tool calls
	agentJsonl := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":50,"output_tokens":25},"content":[{"type":"tool_use","id":"toolu_a1","name":"Read","input":{}}]},"uuid":"aa1","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_a1","content":"ok"}]},"uuid":"au1","timestamp":"2025-01-01T00:00:01.6Z"}
{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":50,"output_tokens":25},"content":[{"type":"tool_use","id":"toolu_a2","name":"Write","input":{}},{"type":"tool_use","id":"toolu_a3","name":"Grep","input":{}}]},"uuid":"aa2","timestamp":"2025-01-01T00:00:01.7Z"}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{}},{"type":"tool_use","id":"toolu_2","name":"Task","input":{"subagent_type":"Explore"}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1a","timestamp":"2025-01-01T00:00:01.5Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"status":"completed","agentId":"agent1","totalToolUseCount":10,"usage":{}}}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Done","toolUseResult":{"agentId":"agent1"}}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`
	// Agent uses haiku
	agentJsonl := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":50,"output_tokens":25}},"uuid":"aa1","timestamp":"2025-01-01T00:00:01.5Z"}
`

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
	mainJsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/main.go"}}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
`
	// Agent reads another file
	agentJsonl := `{"type":"assistant","message":{"model":"claude-haiku-3","usage":{"input_tokens":50,"output_tokens":25},"content":[{"type":"tool_use","id":"toolu_a1","name":"Read","input":{"file_path":"/agent.go"}}]},"uuid":"aa1","timestamp":"2025-01-01T00:00:01.5Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_01ABC123","name":"Task","input":{"description":"Explore codebase","prompt":"Find the main function","subagent_type":"Explore"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_01ABC123","content":[{"type":"text","text":"Found main function in main.go"}]}]},"uuid":"u1","timestamp":"2025-01-01T00:00:10Z","toolUseResult":{"status":"completed","agentId":"agent_xyz","totalTokens":5000,"totalToolUseCount":12}}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_explore","name":"Task","input":{"subagent_type":"Explore"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_explore","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"agentId":"agent1"}}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_plan","name":"Task","input":{"subagent_type":"Plan"}}],"stop_reason":"tool_use"},"uuid":"a2","timestamp":"2025-01-01T00:00:03Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_plan","content":"Done"}]},"uuid":"u2","timestamp":"2025-01-01T00:00:04Z","toolUseResult":{"agentId":"agent2"}}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_explore2","name":"Task","input":{"subagent_type":"Explore"}}],"stop_reason":"tool_use"},"uuid":"a3","timestamp":"2025-01-01T00:00:05Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_explore2","content":"error","is_error":true}]},"uuid":"u3","timestamp":"2025-01-01T00:00:06Z","toolUseResult":{"agentId":"agent3"}}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/test.txt"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{"prompt":"Do something"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z","toolUseResult":{"agentId":"agent_orphan"}}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_skill1","name":"Skill","input":{"skill":"commit","args":"-m 'test'"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_skill1","content":"Skill executed successfully"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_s1","name":"Skill","input":{"skill":"commit"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_s1","content":"Done"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_s2","name":"Skill","input":{"skill":"codebase-maintenance"}}],"stop_reason":"tool_use"},"uuid":"a2","timestamp":"2025-01-01T00:00:03Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_s2","content":"Done"}]},"uuid":"u2","timestamp":"2025-01-01T00:00:04Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_s3","name":"Skill","input":{"skill":"commit"}}],"stop_reason":"tool_use"},"uuid":"a3","timestamp":"2025-01-01T00:00:05Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_s3","content":"error","is_error":true}]},"uuid":"u3","timestamp":"2025-01-01T00:00:06Z"}
`

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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"/test.txt"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file contents"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
`

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
