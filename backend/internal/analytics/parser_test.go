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
