package analytics

import (
	"testing"
)

func TestParseLine_UserMessage(t *testing.T) {
	jsonl := `{"type":"user","message":{"role":"user","content":"hello"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !line.IsUserMessage() {
		t.Error("IsUserMessage should return true")
	}
	if line.IsAssistantMessage() {
		t.Error("IsAssistantMessage should return false")
	}
	if line.UUID != "u1" {
		t.Errorf("UUID = %s, want u1", line.UUID)
	}
}

func TestParseLine_AssistantMessage(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if line.IsUserMessage() {
		t.Error("IsUserMessage should return false")
	}
	if !line.IsAssistantMessage() {
		t.Error("IsAssistantMessage should return true")
	}
	if line.GetModel() != "claude-sonnet-4" {
		t.Errorf("GetModel = %s, want claude-sonnet-4", line.GetModel())
	}
	if line.GetStopReason() != "end_turn" {
		t.Errorf("GetStopReason = %s, want end_turn", line.GetStopReason())
	}
}

func TestParseLine_ToolUse(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Let me read that file"},{"type":"tool_use","id":"toolu_123","name":"Read","input":{"file_path":"/tmp/test.txt"}},{"type":"tool_use","id":"toolu_456","name":"Bash","input":{"command":"ls"}}],"stop_reason":"tool_use"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	blocks := line.GetContentBlocks()
	if len(blocks) != 3 {
		t.Fatalf("GetContentBlocks returned %d blocks, want 3", len(blocks))
	}

	if blocks[0].Type != "text" {
		t.Errorf("blocks[0].Type = %s, want text", blocks[0].Type)
	}
	if blocks[1].Type != "tool_use" {
		t.Errorf("blocks[1].Type = %s, want tool_use", blocks[1].Type)
	}
	if blocks[1].Name != "Read" {
		t.Errorf("blocks[1].Name = %s, want Read", blocks[1].Name)
	}
	if blocks[1].ID != "toolu_123" {
		t.Errorf("blocks[1].ID = %s, want toolu_123", blocks[1].ID)
	}

	tools := line.GetToolUses()
	if len(tools) != 2 {
		t.Fatalf("GetToolUses returned %d tools, want 2", len(tools))
	}
	if tools[0].Name != "Read" {
		t.Errorf("tools[0].Name = %s, want Read", tools[0].Name)
	}
	if tools[1].Name != "Bash" {
		t.Errorf("tools[1].Name = %s, want Bash", tools[1].Name)
	}
}

func TestParseLine_ThinkingBlock(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"thinking","thinking":"Let me think about this...","signature":"abc123"},{"type":"text","text":"Here is my answer"}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !line.HasThinking() {
		t.Error("HasThinking should return true")
	}

	blocks := line.GetContentBlocks()
	if len(blocks) != 2 {
		t.Fatalf("GetContentBlocks returned %d blocks, want 2", len(blocks))
	}
	if blocks[0].Type != "thinking" {
		t.Errorf("blocks[0].Type = %s, want thinking", blocks[0].Type)
	}
}

func TestParseLine_NoThinking(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Simple response"}]},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if line.HasThinking() {
		t.Error("HasThinking should return false")
	}
}

func TestParseLine_CompactBoundary(t *testing.T) {
	jsonl := `{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":50000},"logicalParentUuid":"a1","uuid":"c1","timestamp":"2025-01-01T00:00:15Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !line.IsCompactBoundary() {
		t.Error("IsCompactBoundary should return true")
	}
	if line.CompactMetadata == nil {
		t.Fatal("CompactMetadata should not be nil")
	}
	if line.CompactMetadata.Trigger != "auto" {
		t.Errorf("CompactMetadata.Trigger = %s, want auto", line.CompactMetadata.Trigger)
	}
	if line.LogicalParentUUID != "a1" {
		t.Errorf("LogicalParentUUID = %s, want a1", line.LogicalParentUUID)
	}
}

func TestParseLine_StringContent(t *testing.T) {
	// User messages have string content, not array
	jsonl := `{"type":"user","message":{"role":"user","content":"hello world"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	// GetContentBlocks should return nil for string content
	blocks := line.GetContentBlocks()
	if blocks != nil {
		t.Error("GetContentBlocks should return nil for string content")
	}
}

func TestGetTimestamp(t *testing.T) {
	jsonl := `{"type":"user","uuid":"u1","timestamp":"2025-01-01T12:30:45.123Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	ts, err := line.GetTimestamp()
	if err != nil {
		t.Fatalf("GetTimestamp failed: %v", err)
	}

	if ts.Hour() != 12 {
		t.Errorf("Hour = %d, want 12", ts.Hour())
	}
	if ts.Minute() != 30 {
		t.Errorf("Minute = %d, want 30", ts.Minute())
	}
}

func TestGetTimestamp_Empty(t *testing.T) {
	jsonl := `{"type":"summary","summary":"test"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	_, err = line.GetTimestamp()
	if err != ErrNoTimestamp {
		t.Errorf("GetTimestamp should return ErrNoTimestamp for empty timestamp, got: %v", err)
	}
}

// =============================================================================
// Tests for message type detection (for turn counting)
// =============================================================================

func TestIsHumanMessage(t *testing.T) {
	t.Run("user message with string content returns true", func(t *testing.T) {
		jsonl := `{"type":"user","message":{"role":"user","content":"hello world"},"uuid":"u1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.IsHumanMessage() {
			t.Error("IsHumanMessage should return true for user message with string content")
		}
	})

	t.Run("user message with tool_result array returns false", func(t *testing.T) {
		jsonl := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"file contents here"}]},"uuid":"u1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.IsHumanMessage() {
			t.Error("IsHumanMessage should return false for user message with tool_result array")
		}
	})

	t.Run("assistant message returns false", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":"response"},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.IsHumanMessage() {
			t.Error("IsHumanMessage should return false for assistant messages")
		}
	})

	t.Run("user message with empty string content returns true", func(t *testing.T) {
		jsonl := `{"type":"user","message":{"role":"user","content":""},"uuid":"u1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		// Empty string is still a human message (user hit enter with no text)
		if !line.IsHumanMessage() {
			t.Error("IsHumanMessage should return true for user message with empty string")
		}
	})
}

func TestIsToolResultMessage(t *testing.T) {
	t.Run("user message with tool_result array returns true", func(t *testing.T) {
		jsonl := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"file contents"}]},"uuid":"u1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.IsToolResultMessage() {
			t.Error("IsToolResultMessage should return true for user message with tool_result array")
		}
	})

	t.Run("user message with string content returns false", func(t *testing.T) {
		jsonl := `{"type":"user","message":{"role":"user","content":"hello world"},"uuid":"u1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.IsToolResultMessage() {
			t.Error("IsToolResultMessage should return false for user message with string content")
		}
	})

	t.Run("assistant message returns false", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"response"}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.IsToolResultMessage() {
			t.Error("IsToolResultMessage should return false for assistant messages")
		}
	})
}

func TestHasTextContent(t *testing.T) {
	t.Run("assistant with text block returns true", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Here is my response"}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.HasTextContent() {
			t.Error("HasTextContent should return true for assistant with text block")
		}
	})

	t.Run("assistant with only tool_use returns false", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_123","name":"Read","input":{}}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.HasTextContent() {
			t.Error("HasTextContent should return false for assistant with only tool_use")
		}
	})

	t.Run("assistant with only thinking returns false", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-opus-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"thinking","thinking":"Let me think..."}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.HasTextContent() {
			t.Error("HasTextContent should return false for assistant with only thinking")
		}
	})

	t.Run("assistant with text and tool_use returns true", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Let me read that"},{"type":"tool_use","id":"toolu_123","name":"Read","input":{}}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.HasTextContent() {
			t.Error("HasTextContent should return true for assistant with text and tool_use")
		}
	})

	t.Run("assistant with string content returns true", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":"Simple string response"},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.HasTextContent() {
			t.Error("HasTextContent should return true for assistant with string content")
		}
	})
}

func TestParseLine_ServerToolUse(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"model":"claude-opus-4-6","usage":{"input_tokens":500,"output_tokens":200,"server_tool_use":{"web_search_requests":3,"web_fetch_requests":1},"speed":"fast"},"content":[{"type":"text","text":"Search results"}],"stop_reason":"end_turn"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !line.IsAssistantMessage() {
		t.Fatal("IsAssistantMessage should return true")
	}

	usage := line.Message.Usage
	if usage.Speed != "fast" {
		t.Errorf("Speed = %q, want %q", usage.Speed, "fast")
	}
	if usage.ServerToolUse == nil {
		t.Fatal("ServerToolUse should not be nil")
	}
	if usage.ServerToolUse.WebSearchRequests != 3 {
		t.Errorf("WebSearchRequests = %d, want 3", usage.ServerToolUse.WebSearchRequests)
	}
	if usage.ServerToolUse.WebFetchRequests != 1 {
		t.Errorf("WebFetchRequests = %d, want 1", usage.ServerToolUse.WebFetchRequests)
	}
	if usage.ServerToolUse.CodeExecutionRequests != 0 {
		t.Errorf("CodeExecutionRequests = %d, want 0", usage.ServerToolUse.CodeExecutionRequests)
	}
}

func TestParseLine_ServerToolUse_Absent(t *testing.T) {
	// Verify existing messages without server_tool_use still parse fine
	jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"stop_reason":"end_turn"},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z"}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	usage := line.Message.Usage
	if usage.Speed != "" {
		t.Errorf("Speed = %q, want empty", usage.Speed)
	}
	if usage.ServerToolUse != nil {
		t.Errorf("ServerToolUse should be nil, got %+v", usage.ServerToolUse)
	}
}

func TestParseLine_AgentUsageWithServerToolUse(t *testing.T) {
	jsonl := `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_123","content":"result"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","toolUseResult":{"agentId":"agent-1","usage":{"input_tokens":200,"output_tokens":100,"speed":"fast","server_tool_use":{"web_search_requests":2}}}}`

	line, err := ParseLine([]byte(jsonl))
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	results := line.GetAgentResults()
	if len(results) != 1 {
		t.Fatalf("GetAgentResults returned %d results, want 1", len(results))
	}

	agentUsage := results[0].Usage
	if agentUsage == nil {
		t.Fatal("Agent usage should not be nil")
	}
	if agentUsage.Speed != "fast" {
		t.Errorf("Agent usage Speed = %q, want %q", agentUsage.Speed, "fast")
	}
	if agentUsage.ServerToolUse == nil {
		t.Fatal("Agent ServerToolUse should not be nil")
	}
	if agentUsage.ServerToolUse.WebSearchRequests != 2 {
		t.Errorf("Agent WebSearchRequests = %d, want 2", agentUsage.ServerToolUse.WebSearchRequests)
	}
}

func TestHasToolUse(t *testing.T) {
	t.Run("assistant with tool_use block returns true", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"tool_use","id":"toolu_123","name":"Read","input":{}}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.HasToolUse() {
			t.Error("HasToolUse should return true for assistant with tool_use block")
		}
	})

	t.Run("assistant with only text returns false", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Just a text response"}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if line.HasToolUse() {
			t.Error("HasToolUse should return false for assistant with only text")
		}
	})

	t.Run("assistant with text and tool_use returns true", func(t *testing.T) {
		jsonl := `{"type":"assistant","message":{"model":"claude-sonnet-4","usage":{"input_tokens":100,"output_tokens":50},"content":[{"type":"text","text":"Let me do that"},{"type":"tool_use","id":"toolu_123","name":"Bash","input":{}}]},"uuid":"a1"}`
		line, err := ParseLine([]byte(jsonl))
		if err != nil {
			t.Fatalf("ParseLine failed: %v", err)
		}
		if !line.HasToolUse() {
			t.Error("HasToolUse should return true for assistant with text and tool_use")
		}
	})
}
