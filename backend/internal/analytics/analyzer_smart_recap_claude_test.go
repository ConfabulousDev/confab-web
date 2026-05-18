package analytics

import (
	"strings"
	"testing"
)

// =============================================================================
// PrepareTranscript integer ID tests
// =============================================================================

func TestPrepareTranscript_IntegerIDs(t *testing.T) {
	jsonl := `{"type":"user","message":{"role":"user","content":"Hello world"},"uuid":"u1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"text","text":"Hi there!"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":5}},"uuid":"a1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"u1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"user","message":{"role":"user","content":"Another question"},"uuid":"u2","timestamp":"2025-01-01T00:00:02Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	xml, idMap := PrepareTranscript(fc)

	// Check that IDs are sequential
	if !strings.Contains(xml, `<user id="1">`) {
		t.Error("expected <user id=\"1\"> in transcript")
	}
	if !strings.Contains(xml, `<assistant id="2">`) {
		t.Error("expected <assistant id=\"2\"> in transcript")
	}
	if !strings.Contains(xml, `<user id="3">`) {
		t.Error("expected <user id=\"3\"> in transcript")
	}

	// Check mapping
	if idMap[1] != "u1" {
		t.Errorf("idMap[1] = %q, want %q", idMap[1], "u1")
	}
	if idMap[2] != "a1" {
		t.Errorf("idMap[2] = %q, want %q", idMap[2], "a1")
	}
	if idMap[3] != "u2" {
		t.Errorf("idMap[3] = %q, want %q", idMap[3], "u2")
	}
}

func TestPrepareTranscript_ToolResultsGetIDs(t *testing.T) {
	jsonl := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"tool_use","id":"tu_1","name":"Read","input":{"path":"/test"}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":5}},"uuid":"a1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":"file contents"}]},"uuid":"u1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"a1","isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	xml, idMap := PrepareTranscript(fc)

	if !strings.Contains(xml, `<assistant id="1">`) {
		t.Error("expected <assistant id=\"1\"> in transcript")
	}
	if !strings.Contains(xml, `<tool_results id="2">`) {
		t.Error("expected <tool_results id=\"2\"> in transcript")
	}

	if idMap[1] != "a1" {
		t.Errorf("idMap[1] = %q, want %q", idMap[1], "a1")
	}
	if idMap[2] != "u1" {
		t.Errorf("idMap[2] = %q, want %q", idMap[2], "u1")
	}
}

func TestPrepareTranscript_SkillExpansionGetsID(t *testing.T) {
	// Skill tool_use from assistant, then skill expansion (isMeta user message with sourceToolUseID)
	jsonl := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"tool_use","id":"tu_skill","name":"Skill","input":{"skill":"commit"}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":5}},"uuid":"a1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
{"type":"user","message":{"role":"user","content":"Expanded skill instructions here"},"uuid":"s1","timestamp":"2025-01-01T00:00:01Z","parentUuid":"a1","isSidechain":false,"isMeta":true,"sourceToolUseID":"tu_skill","userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	xml, idMap := PrepareTranscript(fc)

	// Assistant gets id=1, skill expansion gets id=2
	if !strings.Contains(xml, `<assistant id="1">`) {
		t.Error("expected <assistant id=\"1\"> in transcript")
	}
	if !strings.Contains(xml, `<skill id="2"`) {
		t.Errorf("expected <skill id=\"2\" ...> in transcript, got:\n%s", xml)
	}
	// Skill name should be resolved from the tool_use
	if !strings.Contains(xml, `name="commit"`) {
		t.Errorf("expected skill name=\"commit\" in transcript, got:\n%s", xml)
	}

	if idMap[1] != "a1" {
		t.Errorf("idMap[1] = %q, want %q", idMap[1], "a1")
	}
	if idMap[2] != "s1" {
		t.Errorf("idMap[2] = %q, want %q", idMap[2], "s1")
	}
}

func TestPrepareTranscript_AssistantWithThinkingAndToolsGetsID(t *testing.T) {
	// Assistant message with thinking + text + tool_use — should still get a single ID
	jsonl := `{"type":"assistant","message":{"id":"msg_1","type":"message","model":"claude-sonnet-4","role":"assistant","content":[{"type":"thinking","thinking":"Let me think about this..."},{"type":"text","text":"Here is my response"},{"type":"tool_use","id":"tu_1","name":"Read","input":{"path":"/test"}}],"stop_reason":"tool_use","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":5}},"uuid":"a1","timestamp":"2025-01-01T00:00:00Z","parentUuid":null,"isSidechain":false,"userType":"external","cwd":"/test","sessionId":"test","version":"1.0"}
`

	fc, err := NewFileCollection([]byte(jsonl))
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	xml, idMap := PrepareTranscript(fc)

	if !strings.Contains(xml, `<assistant id="1">`) {
		t.Error("expected <assistant id=\"1\"> in transcript")
	}
	if !strings.Contains(xml, `<thinking>`) {
		t.Error("expected <thinking> in transcript")
	}
	if !strings.Contains(xml, `<tools_called>Read</tools_called>`) {
		t.Errorf("expected <tools_called>Read</tools_called> in transcript, got:\n%s", xml)
	}

	if idMap[1] != "a1" {
		t.Errorf("idMap[1] = %q, want %q", idMap[1], "a1")
	}
	// Should only have 1 entry — all parts of the assistant message share one ID
	if len(idMap) != 1 {
		t.Errorf("idMap has %d entries, want 1", len(idMap))
	}
}
