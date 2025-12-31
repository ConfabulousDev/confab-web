package analytics

import (
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
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4-20241022","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_1","name":"Task","input":{"prompt":"Do something"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"Agent completed"}],"toolUseResult":{"status":"completed","agentId":"abc123","totalTokens":1000,"usage":{"input_tokens":50,"output_tokens":200,"cache_creation_input_tokens":100,"cache_read_input_tokens":500}}}]},"uuid":"u1","timestamp":"2025-01-01T00:00:02Z"}
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

func TestComputeFromJSONL(t *testing.T) {
	// Integration test using the main entry point
	jsonl := `{"type":"assistant","uuid":"a1","timestamp":"2025-01-01T00:00:10Z","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50}}}
{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}
`

	result, err := ComputeFromJSONL([]byte(jsonl))
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
