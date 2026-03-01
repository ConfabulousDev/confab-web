package analytics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSmartRecapResponse_WithSuggestedTitle(t *testing.T) {
	input := `{
		"suggested_session_title": "Implement dark mode feature",
		"recap": "User implemented dark mode with Claude's help.",
		"went_well": ["Clear requirements"],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "Implement dark mode feature" {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, "Implement dark mode feature")
	}
	if result.Recap != "User implemented dark mode with Claude's help." {
		t.Errorf("Recap = %q, want %q", result.Recap, "User implemented dark mode with Claude's help.")
	}
}

func TestParseSmartRecapResponse_TruncatesLongTitle(t *testing.T) {
	// Create a title that's over 100 characters
	longTitle := "This is a very long session title that exceeds the maximum allowed length of one hundred characters by quite a bit"
	input := `{
		"suggested_session_title": "` + longTitle + `",
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if len(result.SuggestedSessionTitle) > 100 {
		t.Errorf("SuggestedSessionTitle length = %d, want <= 100", len(result.SuggestedSessionTitle))
	}
	if result.SuggestedSessionTitle != longTitle[:100] {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, longTitle[:100])
	}
}

func TestParseSmartRecapResponse_EmptyTitle(t *testing.T) {
	input := `{
		"suggested_session_title": "",
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "" {
		t.Errorf("SuggestedSessionTitle = %q, want empty string", result.SuggestedSessionTitle)
	}
}

func TestParseSmartRecapResponse_MissingTitle(t *testing.T) {
	// Title field completely missing from JSON
	input := `{
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	// Should be empty string (zero value)
	if result.SuggestedSessionTitle != "" {
		t.Errorf("SuggestedSessionTitle = %q, want empty string", result.SuggestedSessionTitle)
	}
}

func TestParseSmartRecapResponse_ExtractsJSONFromText(t *testing.T) {
	// Sometimes LLMs add text around the JSON
	input := `Here is the analysis:
	{
		"suggested_session_title": "Debug authentication flow",
		"recap": "Fixed auth bug",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}
	That's my analysis.`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "Debug authentication flow" {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, "Debug authentication flow")
	}
}

// =============================================================================
// AnnotatedItem UnmarshalJSON tests
// =============================================================================

func TestAnnotatedItem_UnmarshalJSON_String(t *testing.T) {
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`"plain text"`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "plain text" {
		t.Errorf("Text = %q, want %q", item.Text, "plain text")
	}
	if item.MessageID != "" {
		t.Errorf("MessageID = %q, want empty", item.MessageID)
	}
}

func TestAnnotatedItem_UnmarshalJSON_Object(t *testing.T) {
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`{"text":"item text","message_id":"uuid-123"}`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "item text" {
		t.Errorf("Text = %q, want %q", item.Text, "item text")
	}
	if item.MessageID != "uuid-123" {
		t.Errorf("MessageID = %q, want %q", item.MessageID, "uuid-123")
	}
}

func TestAnnotatedItem_UnmarshalJSON_ObjectWithIntegerID(t *testing.T) {
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`{"text":"item text","message_id":42}`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "item text" {
		t.Errorf("Text = %q, want %q", item.Text, "item text")
	}
	if item.MessageID != "42" {
		t.Errorf("MessageID = %q, want %q", item.MessageID, "42")
	}
}

func TestAnnotatedItem_UnmarshalJSON_ObjectWithoutMessageID(t *testing.T) {
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`{"text":"no ref"}`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "no ref" {
		t.Errorf("Text = %q, want %q", item.Text, "no ref")
	}
	if item.MessageID != "" {
		t.Errorf("MessageID = %q, want empty", item.MessageID)
	}
}

func TestAnnotatedItem_MixedArrayUnmarshal(t *testing.T) {
	// Test that a JSON array with mixed string and object items unmarshals correctly
	input := `[
		"plain string",
		{"text": "object item", "message_id": 5},
		{"text": "no ref item"}
	]`
	var items []AnnotatedItem
	if err := json.Unmarshal([]byte(input), &items); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if items[0].Text != "plain string" || items[0].MessageID != "" {
		t.Errorf("item[0] = %+v, want {Text:plain string, MessageID:}", items[0])
	}
	if items[1].Text != "object item" || items[1].MessageID != "5" {
		t.Errorf("item[1] = %+v, want {Text:object item, MessageID:5}", items[1])
	}
	if items[2].Text != "no ref item" || items[2].MessageID != "" {
		t.Errorf("item[2] = %+v, want {Text:no ref item, MessageID:}", items[2])
	}
}

// =============================================================================
// truncateAnnotatedSlice tests
// =============================================================================

func TestTruncateAnnotatedSlice(t *testing.T) {
	tests := []struct {
		name   string
		input  []AnnotatedItem
		maxLen int
		want   int
	}{
		{"nil returns empty", nil, 3, 0},
		{"empty returns empty", []AnnotatedItem{}, 3, 0},
		{"under limit unchanged", []AnnotatedItem{{Text: "a"}, {Text: "b"}}, 3, 2},
		{"at limit unchanged", []AnnotatedItem{{Text: "a"}, {Text: "b"}, {Text: "c"}}, 3, 3},
		{"over limit truncated", []AnnotatedItem{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}}, 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateAnnotatedSlice(tt.input, tt.maxLen)
			if got == nil {
				t.Fatal("result should never be nil")
			}
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

// =============================================================================
// resolveMessageIDs tests
// =============================================================================

func TestResolveMessageIDs_ValidMapping(t *testing.T) {
	result := &SmartRecapResult{
		WentWell:     []AnnotatedItem{{Text: "good", MessageID: "1"}, {Text: "also good", MessageID: "3"}},
		WentBad:      []AnnotatedItem{{Text: "bad", MessageID: "2"}},
		HumanSuggestions: []AnnotatedItem{},
		EnvironmentSuggestions: []AnnotatedItem{},
		DefaultContextSuggestions: []AnnotatedItem{},
	}
	idMap := map[int]string{1: "uuid-aaa", 2: "uuid-bbb", 3: "uuid-ccc"}

	resolveMessageIDs(result, idMap)

	if result.WentWell[0].MessageID != "uuid-aaa" {
		t.Errorf("WentWell[0].MessageID = %q, want uuid-aaa", result.WentWell[0].MessageID)
	}
	if result.WentWell[1].MessageID != "uuid-ccc" {
		t.Errorf("WentWell[1].MessageID = %q, want uuid-ccc", result.WentWell[1].MessageID)
	}
	if result.WentBad[0].MessageID != "uuid-bbb" {
		t.Errorf("WentBad[0].MessageID = %q, want uuid-bbb", result.WentBad[0].MessageID)
	}
}

func TestResolveMessageIDs_InvalidID(t *testing.T) {
	result := &SmartRecapResult{
		WentWell:     []AnnotatedItem{{Text: "good", MessageID: "999"}},
		WentBad:      []AnnotatedItem{},
		HumanSuggestions: []AnnotatedItem{},
		EnvironmentSuggestions: []AnnotatedItem{},
		DefaultContextSuggestions: []AnnotatedItem{},
	}
	idMap := map[int]string{1: "uuid-aaa"}

	resolveMessageIDs(result, idMap)

	// 999 not in mapping — should be cleared
	if result.WentWell[0].MessageID != "" {
		t.Errorf("MessageID = %q, want empty (invalid ID should be cleared)", result.WentWell[0].MessageID)
	}
	// Text should be preserved
	if result.WentWell[0].Text != "good" {
		t.Errorf("Text = %q, want %q", result.WentWell[0].Text, "good")
	}
}

func TestResolveMessageIDs_NonIntegerID(t *testing.T) {
	result := &SmartRecapResult{
		WentWell:     []AnnotatedItem{{Text: "good", MessageID: "not-a-number"}},
		WentBad:      []AnnotatedItem{},
		HumanSuggestions: []AnnotatedItem{},
		EnvironmentSuggestions: []AnnotatedItem{},
		DefaultContextSuggestions: []AnnotatedItem{},
	}
	idMap := map[int]string{1: "uuid-aaa"}

	resolveMessageIDs(result, idMap)

	if result.WentWell[0].MessageID != "" {
		t.Errorf("MessageID = %q, want empty (non-integer should be cleared)", result.WentWell[0].MessageID)
	}
}

func TestResolveMessageIDs_EmptyID(t *testing.T) {
	result := &SmartRecapResult{
		WentWell:     []AnnotatedItem{{Text: "good", MessageID: ""}},
		WentBad:      []AnnotatedItem{},
		HumanSuggestions: []AnnotatedItem{},
		EnvironmentSuggestions: []AnnotatedItem{},
		DefaultContextSuggestions: []AnnotatedItem{},
	}
	idMap := map[int]string{1: "uuid-aaa"}

	resolveMessageIDs(result, idMap)

	// Empty should stay empty
	if result.WentWell[0].MessageID != "" {
		t.Errorf("MessageID = %q, want empty", result.WentWell[0].MessageID)
	}
}

// =============================================================================
// parseSmartRecapResponse with AnnotatedItem format tests
// =============================================================================

func TestParseSmartRecapResponse_AnnotatedItems(t *testing.T) {
	input := `{
		"suggested_session_title": "Test session",
		"recap": "Test recap.",
		"went_well": [{"text": "Good thing", "message_id": 1}, {"text": "Another good thing"}],
		"went_bad": [{"text": "Bad thing", "message_id": 3}],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": [{"text": "Add docs"}]
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if len(result.WentWell) != 2 {
		t.Fatalf("WentWell length = %d, want 2", len(result.WentWell))
	}
	if result.WentWell[0].Text != "Good thing" {
		t.Errorf("WentWell[0].Text = %q, want %q", result.WentWell[0].Text, "Good thing")
	}
	if result.WentWell[0].MessageID != "1" {
		t.Errorf("WentWell[0].MessageID = %q, want %q", result.WentWell[0].MessageID, "1")
	}
	if result.WentWell[1].MessageID != "" {
		t.Errorf("WentWell[1].MessageID = %q, want empty", result.WentWell[1].MessageID)
	}
	if result.WentBad[0].MessageID != "3" {
		t.Errorf("WentBad[0].MessageID = %q, want %q", result.WentBad[0].MessageID, "3")
	}
}

func TestParseSmartRecapResponse_LegacyStringItems(t *testing.T) {
	// Verify backwards compat: old-style string arrays still parse
	input := `{
		"suggested_session_title": "Old session",
		"recap": "Old recap.",
		"went_well": ["Good thing 1", "Good thing 2"],
		"went_bad": ["Bad thing"],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if len(result.WentWell) != 2 {
		t.Fatalf("WentWell length = %d, want 2", len(result.WentWell))
	}
	if result.WentWell[0].Text != "Good thing 1" {
		t.Errorf("WentWell[0].Text = %q, want %q", result.WentWell[0].Text, "Good thing 1")
	}
	if result.WentWell[0].MessageID != "" {
		t.Errorf("WentWell[0].MessageID = %q, want empty", result.WentWell[0].MessageID)
	}
}

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

func TestAnnotatedItem_UnmarshalJSON_NullMessageID(t *testing.T) {
	// LLM could theoretically send null for message_id
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`{"text":"item","message_id":null}`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "item" {
		t.Errorf("Text = %q, want %q", item.Text, "item")
	}
	if item.MessageID != "" {
		t.Errorf("MessageID = %q, want empty (null should be cleared)", item.MessageID)
	}
}

func TestAnnotatedItem_UnmarshalJSON_BoolMessageID(t *testing.T) {
	// Unexpected type — should be cleared gracefully
	var item AnnotatedItem
	if err := json.Unmarshal([]byte(`{"text":"item","message_id":true}`), &item); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if item.Text != "item" {
		t.Errorf("Text = %q, want %q", item.Text, "item")
	}
	if item.MessageID != "" {
		t.Errorf("MessageID = %q, want empty (bool should be cleared)", item.MessageID)
	}
}
