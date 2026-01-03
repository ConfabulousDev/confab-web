package analytics

import (
	"testing"
)

// Test the regex pattern directly
func TestRedactionPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected []string // expected TYPE values
	}{
		{"[REDACTED:KEY1]", []string{"KEY1"}},
		{"[REDACTED:GITHUB_TOKEN]", []string{"GITHUB_TOKEN"}},
		{"before [REDACTED:KEY1] after", []string{"KEY1"}},
		{"[REDACTED:A] and [REDACTED:B]", []string{"A", "B"}},
		{"[REDACTED:KEY1] and [REDACTED:KEY2] and [REDACTED:KEY1]", []string{"KEY1", "KEY2", "KEY1"}},
		{"no redactions here", nil},
		{"[REDACTED:lowercase]", nil},       // lowercase not matched
		{"[NOT_REDACTED:FOO]", nil},         // wrong prefix
		{"REDACTED:TOKEN", nil},             // no brackets
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			matches := redactionPattern.FindAllStringSubmatch(tt.input, -1)
			var got []string
			for _, m := range matches {
				if len(m) >= 2 {
					got = append(got, m[1])
				}
			}
			if len(got) != len(tt.expected) {
				t.Errorf("got %v, want %v", got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestRedactionsAnalyzer_NoRedactions(t *testing.T) {
	content := []byte(`{"type":"user","message":{"content":"Hello world"}}
{"type":"assistant","message":{"content":"Hi there!"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 0 {
		t.Errorf("TotalRedactions = %d, want 0", result.TotalRedactions)
	}
	if len(result.RedactionCounts) != 0 {
		t.Errorf("RedactionCounts = %v, want empty", result.RedactionCounts)
	}
}

func TestRedactionsAnalyzer_SingleType(t *testing.T) {
	content := []byte(`{"type":"user","message":{"content":"My token is [REDACTED:GITHUB_TOKEN]"}}
{"type":"assistant","message":{"content":"I see your [REDACTED:GITHUB_TOKEN] token"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", result.TotalRedactions)
	}
	if result.RedactionCounts["GITHUB_TOKEN"] != 2 {
		t.Errorf("GITHUB_TOKEN count = %d, want 2", result.RedactionCounts["GITHUB_TOKEN"])
	}
}

func TestRedactionsAnalyzer_MultipleTypes(t *testing.T) {
	content := []byte(`{"type":"user","message":{"content":"Token: [REDACTED:GITHUB_TOKEN], Key: [REDACTED:AWS_KEY]"}}
{"type":"assistant","message":{"content":"Found [REDACTED:PASSWORD] and [REDACTED:AWS_KEY]"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 4 {
		t.Errorf("TotalRedactions = %d, want 4", result.TotalRedactions)
	}
	if result.RedactionCounts["GITHUB_TOKEN"] != 1 {
		t.Errorf("GITHUB_TOKEN count = %d, want 1", result.RedactionCounts["GITHUB_TOKEN"])
	}
	if result.RedactionCounts["AWS_KEY"] != 2 {
		t.Errorf("AWS_KEY count = %d, want 2", result.RedactionCounts["AWS_KEY"])
	}
	if result.RedactionCounts["PASSWORD"] != 1 {
		t.Errorf("PASSWORD count = %d, want 1", result.RedactionCounts["PASSWORD"])
	}
}

func TestRedactionsAnalyzer_NestedJSON(t *testing.T) {
	// Redactions in nested structures (tool inputs, arrays, etc.)
	content := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"export TOKEN=[REDACTED:API_KEY]"}}]}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 1 {
		t.Errorf("TotalRedactions = %d, want 1", result.TotalRedactions)
	}
	if result.RedactionCounts["API_KEY"] != 1 {
		t.Errorf("API_KEY count = %d, want 1", result.RedactionCounts["API_KEY"])
	}
}

func TestRedactionsAnalyzer_DeeplyNested(t *testing.T) {
	// Redactions buried deep in nested objects and arrays
	content := []byte(`{"type":"user","data":{"level1":{"level2":{"level3":[{"value":"[REDACTED:DEEP_SECRET]"}]}}}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 1 {
		t.Errorf("TotalRedactions = %d, want 1", result.TotalRedactions)
	}
	if result.RedactionCounts["DEEP_SECRET"] != 1 {
		t.Errorf("DEEP_SECRET count = %d, want 1", result.RedactionCounts["DEEP_SECRET"])
	}
}

func TestRedactionsAnalyzer_MultipleInSameString(t *testing.T) {
	// Multiple redactions in the same string value
	content := []byte(`{"type":"user","message":{"content":"Keys: [REDACTED:KEY1] and [REDACTED:KEY2] and [REDACTED:KEY1]"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 3 {
		t.Errorf("TotalRedactions = %d, want 3", result.TotalRedactions)
	}
	if result.RedactionCounts["KEY1"] != 2 {
		t.Errorf("KEY1 count = %d, want 2", result.RedactionCounts["KEY1"])
	}
	if result.RedactionCounts["KEY2"] != 1 {
		t.Errorf("KEY2 count = %d, want 1", result.RedactionCounts["KEY2"])
	}
}

func TestRedactionsAnalyzer_WithAgentFiles(t *testing.T) {
	mainContent := []byte(`{"type":"user","message":{"content":"Main: [REDACTED:MAIN_TOKEN]"}}`)
	agentContents := map[string][]byte{
		"agent-123": []byte(`{"type":"assistant","message":{"content":"Agent: [REDACTED:AGENT_SECRET]"}}
{"type":"user","message":{"content":"More: [REDACTED:AGENT_SECRET]"}}`),
	}

	fc, err := NewFileCollectionWithAgents(mainContent, agentContents)
	if err != nil {
		t.Fatalf("NewFileCollectionWithAgents failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 3 {
		t.Errorf("TotalRedactions = %d, want 3", result.TotalRedactions)
	}
	if result.RedactionCounts["MAIN_TOKEN"] != 1 {
		t.Errorf("MAIN_TOKEN count = %d, want 1", result.RedactionCounts["MAIN_TOKEN"])
	}
	if result.RedactionCounts["AGENT_SECRET"] != 2 {
		t.Errorf("AGENT_SECRET count = %d, want 2", result.RedactionCounts["AGENT_SECRET"])
	}
}

func TestRedactionsAnalyzer_FieldNameRedaction(t *testing.T) {
	// When an entire field value is redacted (field-based redaction)
	content := []byte(`{"type":"user","password":"[REDACTED:PASSWORD]","api_key":"[REDACTED:API_KEY]"}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", result.TotalRedactions)
	}
	if result.RedactionCounts["PASSWORD"] != 1 {
		t.Errorf("PASSWORD count = %d, want 1", result.RedactionCounts["PASSWORD"])
	}
	if result.RedactionCounts["API_KEY"] != 1 {
		t.Errorf("API_KEY count = %d, want 1", result.RedactionCounts["API_KEY"])
	}
}

func TestRedactionsAnalyzer_UnderscoreInType(t *testing.T) {
	// Type names with underscores
	content := []byte(`{"type":"user","message":{"content":"[REDACTED:SOME_LONG_SECRET_NAME]"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 1 {
		t.Errorf("TotalRedactions = %d, want 1", result.TotalRedactions)
	}
	if result.RedactionCounts["SOME_LONG_SECRET_NAME"] != 1 {
		t.Errorf("SOME_LONG_SECRET_NAME count = %d, want 1", result.RedactionCounts["SOME_LONG_SECRET_NAME"])
	}
}

func TestRedactionsAnalyzer_NotARedaction(t *testing.T) {
	// Things that look like redactions but aren't
	content := []byte(`{"type":"user","message":{"content":"[REDACTED:lowercase] [NOT_REDACTED:FOO] [REDACTED:123] REDACTED:TOKEN"}}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Only [REDACTED:FOO] would match if FOO were uppercase, but none of these should match:
	// - [REDACTED:lowercase] - lowercase type
	// - [NOT_REDACTED:FOO] - wrong prefix
	// - [REDACTED:123] - numbers only
	// - REDACTED:TOKEN - no brackets
	if result.TotalRedactions != 0 {
		t.Errorf("TotalRedactions = %d, want 0 (found: %v)", result.TotalRedactions, result.RedactionCounts)
	}
}

func TestRedactionsAnalyzer_EmptyContent(t *testing.T) {
	content := []byte(``)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 0 {
		t.Errorf("TotalRedactions = %d, want 0", result.TotalRedactions)
	}
}

func TestRedactionsAnalyzer_ArrayOfStrings(t *testing.T) {
	// Redactions in arrays of strings
	content := []byte(`{"type":"user","tokens":["[REDACTED:TOKEN_A]","normal","[REDACTED:TOKEN_B]"]}`)

	fc, err := NewFileCollection(content)
	if err != nil {
		t.Fatalf("NewFileCollection failed: %v", err)
	}

	result, err := (&RedactionsAnalyzer{}).Analyze(fc)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.TotalRedactions != 2 {
		t.Errorf("TotalRedactions = %d, want 2", result.TotalRedactions)
	}
	if result.RedactionCounts["TOKEN_A"] != 1 {
		t.Errorf("TOKEN_A count = %d, want 1", result.RedactionCounts["TOKEN_A"])
	}
	if result.RedactionCounts["TOKEN_B"] != 1 {
		t.Errorf("TOKEN_B count = %d, want 1", result.RedactionCounts["TOKEN_B"])
	}
}
