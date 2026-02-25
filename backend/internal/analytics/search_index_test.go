package analytics

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractUserMessagesText_NilFileCollection(t *testing.T) {
	result := ExtractUserMessagesText(nil)
	if result != "" {
		t.Errorf("expected empty string for nil FileCollection, got %q", result)
	}
}

func TestExtractUserMessagesText_EmptyFileCollection(t *testing.T) {
	fc := &FileCollection{
		Main: &TranscriptFile{Lines: []*TranscriptLine{}},
	}
	result := ExtractUserMessagesText(fc)
	if result != "" {
		t.Errorf("expected empty string for empty FileCollection, got %q", result)
	}
}

func TestExtractUserMessagesText_IncludesHumanMessages(t *testing.T) {
	fc := &FileCollection{
		Main: &TranscriptFile{
			Lines: []*TranscriptLine{
				{Type: "user", Message: &MessageContent{Content: "Hello world"}},
				{Type: "assistant", Message: &MessageContent{Content: "Hi there"}},
				{Type: "user", Message: &MessageContent{Content: "How are you?"}},
			},
		},
	}
	result := ExtractUserMessagesText(fc)
	if !strings.Contains(result, "Hello world") {
		t.Error("expected result to contain 'Hello world'")
	}
	if !strings.Contains(result, "How are you?") {
		t.Error("expected result to contain 'How are you?'")
	}
	if strings.Contains(result, "Hi there") {
		t.Error("expected result to NOT contain assistant message 'Hi there'")
	}
}

func TestExtractUserMessagesText_ExcludesToolResults(t *testing.T) {
	// Tool results have content as an array, not a string
	fc := &FileCollection{
		Main: &TranscriptFile{
			Lines: []*TranscriptLine{
				{Type: "user", Message: &MessageContent{Content: "Hello"}},
				{Type: "user", Message: &MessageContent{Content: []interface{}{map[string]interface{}{"type": "tool_result"}}}},
			},
		},
	}
	result := ExtractUserMessagesText(fc)
	if result != "Hello" {
		t.Errorf("expected 'Hello', got %q", result)
	}
}

func TestExtractUserMessagesText_ExcludesSkillExpansion(t *testing.T) {
	fc := &FileCollection{
		Main: &TranscriptFile{
			Lines: []*TranscriptLine{
				{Type: "user", Message: &MessageContent{Content: "Normal message"}},
				{Type: "user", IsMeta: true, SourceToolUseID: "tool-123", Message: &MessageContent{Content: "Skill expansion content"}},
			},
		},
	}
	result := ExtractUserMessagesText(fc)
	if result != "Normal message" {
		t.Errorf("expected 'Normal message', got %q", result)
	}
}

func TestExtractUserMessagesText_ExcludesCommandExpansion(t *testing.T) {
	fc := &FileCollection{
		Main: &TranscriptFile{
			Lines: []*TranscriptLine{
				{Type: "user", Message: &MessageContent{Content: "Normal message"}},
				{Type: "user", Message: &MessageContent{Content: "<command-name>commit</command-name> Do the commit"}},
			},
		},
	}
	result := ExtractUserMessagesText(fc)
	if result != "Normal message" {
		t.Errorf("expected 'Normal message', got %q", result)
	}
}

func TestExtractUserMessagesText_TruncatesAt500KB(t *testing.T) {
	// Create a FileCollection with messages totaling well over 500KB
	var lines []*TranscriptLine
	// Each message is 10KB, need 60 to exceed 500KB
	bigMsg := strings.Repeat("x", 10*1024)
	for i := 0; i < 60; i++ {
		lines = append(lines, &TranscriptLine{
			Type:    "user",
			Message: &MessageContent{Content: bigMsg},
		})
	}
	fc := &FileCollection{
		Main: &TranscriptFile{Lines: lines},
	}
	result := ExtractUserMessagesText(fc)
	if len(result) > maxUserMessagesBytes+1 {
		t.Errorf("expected result to be <= %d bytes, got %d", maxUserMessagesBytes, len(result))
	}
}

func TestExtractUserMessagesText_TruncationPreservesUTF8(t *testing.T) {
	// Create messages with multi-byte UTF-8 characters near the truncation boundary.
	// Each emoji is 4 bytes; we want the truncation point to land mid-character.
	var lines []*TranscriptLine
	emoji := "🔥" // 4-byte UTF-8
	bigMsg := strings.Repeat(emoji, 3*1024) // 12KB per message
	for i := 0; i < 50; i++ {
		lines = append(lines, &TranscriptLine{
			Type:    "user",
			Message: &MessageContent{Content: bigMsg},
		})
	}
	fc := &FileCollection{
		Main: &TranscriptFile{Lines: lines},
	}
	result := ExtractUserMessagesText(fc)
	if !utf8.ValidString(result) {
		t.Error("truncated result contains invalid UTF-8")
	}
}

func TestExtractUserMessagesText_IncludesAgentFiles(t *testing.T) {
	fc := &FileCollection{
		Main: &TranscriptFile{
			Lines: []*TranscriptLine{
				{Type: "user", Message: &MessageContent{Content: "Main message"}},
			},
		},
		Agents: []*TranscriptFile{
			{
				AgentID: "agent-1",
				Lines: []*TranscriptLine{
					{Type: "user", Message: &MessageContent{Content: "Agent message"}},
				},
			},
		},
	}
	result := ExtractUserMessagesText(fc)
	if !strings.Contains(result, "Main message") {
		t.Error("expected result to contain 'Main message'")
	}
	if !strings.Contains(result, "Agent message") {
		t.Error("expected result to contain 'Agent message'")
	}
}

func TestSearchIndexContentCombinedText(t *testing.T) {
	tests := []struct {
		name     string
		content  SearchIndexContent
		expected string
	}{
		{
			name:     "all parts present",
			content:  SearchIndexContent{MetadataText: "title", RecapText: "recap", UserMessagesText: "msgs"},
			expected: "title\nrecap\nmsgs",
		},
		{
			name:     "metadata only",
			content:  SearchIndexContent{MetadataText: "title"},
			expected: "title",
		},
		{
			name:     "empty content",
			content:  SearchIndexContent{},
			expected: "",
		},
		{
			name:     "recap and messages only",
			content:  SearchIndexContent{RecapText: "recap", UserMessagesText: "msgs"},
			expected: "recap\nmsgs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.content.CombinedText()
			if result != tt.expected {
				t.Errorf("CombinedText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFlattenJSONStringArray(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []string
	}{
		{"empty array", []byte(`[]`), nil},
		{"null", []byte(`null`), nil},
		{"empty bytes", nil, nil},
		{"single item", []byte(`["hello"]`), []string{"hello"}},
		{"multiple items", []byte(`["a","b","c"]`), []string{"a", "b", "c"}},
		{"items with spaces", []byte(`["hello world","foo bar"]`), []string{"hello world", "foo bar"}},
		{"escaped quotes", []byte(`["say \"hello\""]`), []string{`say "hello"`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := flattenJSONStringArray(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("len = %d, want %d", len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
