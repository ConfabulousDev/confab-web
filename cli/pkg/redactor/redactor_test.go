package redactor

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestRedactSimplePattern tests redacting with a simple pattern (full match)
func TestRedactSimplePattern(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single API key",
			input:    "My API key is sk-1234567890",
			expected: "My API key is [REDACTED:API_KEY]",
		},
		{
			name:     "Multiple API keys",
			input:    "Keys: sk-abcdefghij and sk-0987654321",
			expected: "Keys: [REDACTED:API_KEY] and [REDACTED:API_KEY]",
		},
		{
			name:     "No match",
			input:    "This has no secrets",
			expected: "This has no secrets",
		},
		{
			name:     "Partial match should not redact",
			input:    "sk-short",
			expected: "sk-short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

// TestRedactWithCaptureGroup tests partial redaction using capture groups
func TestRedactWithCaptureGroup(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:         "PostgreSQL Password",
				Pattern:      `(postgres://[^:]+:)([^@\s]+)(@[^\s]+)`,
				Type:         "password",
				CaptureGroup: 2,
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PostgreSQL connection string",
			input:    "postgres://user:mypassword@localhost:5432/db",
			expected: "postgres://user:[REDACTED:PASSWORD]@localhost:5432/db",
		},
		{
			name:     "Multiple connection strings",
			input:    "DB1: postgres://admin:secret@db1.com DB2: postgres://user:pass123@db2.com",
			expected: "DB1: postgres://admin:[REDACTED:PASSWORD]@db1.com DB2: postgres://user:[REDACTED:PASSWORD]@db2.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

// TestRedactMultiplePatternTypes tests redacting with multiple pattern types
func TestRedactMultiplePatternTypes(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-ant-api\d{2}-[A-Za-z0-9_-]{20}`,
				Type:    "api_key",
			},
			{
				Name:    "AWS Key",
				Pattern: `AKIA[0-9A-Z]{16}`,
				Type:    "aws_key",
			},
			{
				Name:    "GitHub Token",
				Pattern: `ghp_[A-Za-z0-9]{10}`,
				Type:    "github_token",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	input := "API: sk-ant-api03-12345678901234567890 AWS: AKIAIOSFODNN7EXAMPLE GitHub: ghp_1234567890"
	result := redactor.Redact(input)

	// Verify all secrets are redacted
	if strings.Contains(result, "sk-ant-api03") {
		t.Error("API key should be redacted")
	}
	if strings.Contains(result, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("AWS key should be redacted")
	}
	if strings.Contains(result, "ghp_1234567890") {
		t.Error("GitHub token should be redacted")
	}

	// Verify redaction markers are present
	if !strings.Contains(result, "[REDACTED:API_KEY]") {
		t.Error("Expected API_KEY redaction marker")
	}
	if !strings.Contains(result, "[REDACTED:AWS_KEY]") {
		t.Error("Expected AWS_KEY redaction marker")
	}
	if !strings.Contains(result, "[REDACTED:GITHUB_TOKEN]") {
		t.Error("Expected GITHUB_TOKEN redaction marker")
	}
}

// TestRedactEmptyString tests redacting an empty string
func TestRedactEmptyString(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "Test",
				Pattern: `test`,
				Type:    "test",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	result := redactor.Redact("")
	if result != "" {
		t.Errorf("Expected empty string, got %s", result)
	}
}

// TestRedactMultilineText tests redacting across multiple lines
func TestRedactMultilineText(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	input := `Line 1: sk-1234567890
Line 2: Some text
Line 3: sk-abcdefghij`

	expected := `Line 1: [REDACTED:API_KEY]
Line 2: Some text
Line 3: [REDACTED:API_KEY]`

	result := redactor.Redact(input)
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// TestRedactWithInvalidPattern tests handling of invalid regex patterns
func TestRedactWithInvalidPattern(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "Invalid",
				Pattern: `[invalid(`,
				Type:    "test",
			},
		},
	}

	_, err := NewRedactor(config)
	if err == nil {
		t.Error("Expected error when creating redactor with invalid pattern")
	}
}

// TestRedactNoPatterns tests redactor with no patterns
func TestRedactNoPatterns(t *testing.T) {
	config := Config{
		Patterns: []Pattern{},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	input := "Some text with sk-1234567890"
	result := redactor.Redact(input)

	if result != input {
		t.Errorf("Expected no changes, got: %s", result)
	}
}

// TestRedactLargeText tests performance with large text
func TestRedactLargeText(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	// Create large text with secrets scattered throughout
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString("Some regular text here. ")
		if i%100 == 0 {
			builder.WriteString("sk-1234567890 ")
		}
	}

	input := builder.String()
	result := redactor.Redact(input)

	// Verify secrets are redacted
	if strings.Contains(result, "sk-1234567890") {
		t.Error("API key should be redacted in large text")
	}

	// Verify redaction markers are present
	count := strings.Count(result, "[REDACTED:API_KEY]")
	if count != 10 {
		t.Errorf("Expected 10 redactions, got %d", count)
	}
}

// TestRedactSpecialCharacters tests handling of special characters
func TestRedactSpecialCharacters(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9_-]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Key with dashes and underscores",
			input:    "Key: sk-abc_def-12",
			expected: "Key: [REDACTED:API_KEY]",
		},
		{
			name:     "Key in quotes",
			input:    `API_KEY="sk-1234567890"`,
			expected: `API_KEY="[REDACTED:API_KEY]"`,
		},
		{
			name:     "Key in JSON",
			input:    `{"key":"sk-abcdefghij"}`,
			expected: `{"key":"[REDACTED:API_KEY]"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

// TestRedactCaseSensitivity tests case-sensitive pattern matching
func TestRedactCaseSensitivity(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "AWS Key",
				Pattern: `AKIA[0-9A-Z]{16}`,
				Type:    "aws_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	// Should match uppercase
	result1 := redactor.Redact("AKIAIOSFODNN7EXAMPLE")
	if !strings.Contains(result1, "[REDACTED:AWS_KEY]") {
		t.Error("Should redact uppercase AWS key")
	}

	// Should NOT match lowercase
	result2 := redactor.Redact("akiaiosfodnn7example")
	if strings.Contains(result2, "[REDACTED:AWS_KEY]") {
		t.Error("Should not redact lowercase (pattern is case-sensitive)")
	}
}

// TestRedactOrderOfPatterns tests that pattern order doesn't affect results
func TestRedactOrderOfPatterns(t *testing.T) {
	config1 := Config{
		Patterns: []Pattern{
			{Name: "Pattern A", Pattern: `aaa`, Type: "type_a"},
			{Name: "Pattern B", Pattern: `bbb`, Type: "type_b"},
		},
	}

	config2 := Config{
		Patterns: []Pattern{
			{Name: "Pattern B", Pattern: `bbb`, Type: "type_b"},
			{Name: "Pattern A", Pattern: `aaa`, Type: "type_a"},
		},
	}

	redactor1, _ := NewRedactor(config1)
	redactor2, _ := NewRedactor(config2)

	input := "Text with aaa and bbb"

	result1 := redactor1.Redact(input)
	result2 := redactor2.Redact(input)

	// Results should be the same regardless of pattern order
	if result1 != result2 {
		t.Errorf("Pattern order should not affect results:\nResult1: %s\nResult2: %s", result1, result2)
	}
}

// TestRedactBytes tests redacting byte slices
func TestRedactBytes(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	input := []byte("My key is sk-1234567890")
	result := redactor.RedactBytes(input)

	expected := []byte("My key is [REDACTED:API_KEY]")
	if string(result) != string(expected) {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

// TestRedactJSONL tests JSON-aware redaction of JSONL content
func TestRedactJSONL(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple string field",
			input:    `{"message":"My key is sk-1234567890"}`,
			expected: `{"message":"My key is [REDACTED:API_KEY]"}`,
		},
		{
			name:     "Multiple lines",
			input:    "{\"a\":\"sk-1234567890\"}\n{\"b\":\"sk-abcdefghij\"}",
			expected: "{\"a\":\"[REDACTED:API_KEY]\"}\n{\"b\":\"[REDACTED:API_KEY]\"}",
		},
		{
			name:     "Nested object",
			input:    `{"outer":{"inner":"sk-1234567890"}}`,
			expected: `{"outer":{"inner":"[REDACTED:API_KEY]"}}`,
		},
		{
			name:     "Array of strings",
			input:    `{"keys":["sk-1234567890","sk-abcdefghij"]}`,
			expected: `{"keys":["[REDACTED:API_KEY]","[REDACTED:API_KEY]"]}`,
		},
		{
			name:     "Mixed types preserved",
			input:    `{"str":"sk-1234567890","num":42,"bool":true,"null":null}`,
			expected: `{"bool":true,"null":null,"num":42,"str":"[REDACTED:API_KEY]"}`,
		},
		{
			name:     "Empty lines preserved",
			input:    "{\"a\":\"test\"}\n\n{\"b\":\"sk-1234567890\"}",
			expected: "{\"a\":\"test\"}\n\n{\"b\":\"[REDACTED:API_KEY]\"}",
		},
		{
			name:     "No secrets - unchanged",
			input:    `{"message":"hello world"}`,
			expected: `{"message":"hello world"}`,
		},
		{
			name:     "Deeply nested",
			input:    `{"a":{"b":{"c":{"d":"sk-1234567890"}}}}`,
			expected: `{"a":{"b":{"c":{"d":"[REDACTED:API_KEY]"}}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.RedactJSONL([]byte(tt.input))
			if string(result) != tt.expected {
				t.Errorf("Expected:\n%s\nGot:\n%s", tt.expected, string(result))
			}
		})
	}
}

// TestRedactJSONLPreservesValidJSON verifies that output is always valid JSON
func TestRedactJSONLPreservesValidJSON(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	// Input with special characters that could break JSON if not handled properly
	input := `{"message":"Key: sk-1234567890\nWith newline","quote":"He said \"sk-abcdefghij\""}`
	result := redactor.RedactJSONL([]byte(input))

	// Verify result is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Errorf("Result is not valid JSON: %v\nResult: %s", err, string(result))
	}

	// Verify secrets are redacted
	if strings.Contains(string(result), "sk-1234567890") {
		t.Error("API key should be redacted")
	}
	if strings.Contains(string(result), "sk-abcdefghij") {
		t.Error("API key in quoted string should be redacted")
	}
}

// TestRedactJSONLInvalidJSON tests fallback behavior for invalid JSON lines
func TestRedactJSONLInvalidJSON(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	// Invalid JSON line should fall back to text-based redaction
	input := "not valid json with sk-1234567890"
	result := redactor.RedactJSONL([]byte(input))

	expected := "not valid json with [REDACTED:API_KEY]"
	if string(result) != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, string(result))
	}
}

// TestRedactJSONLMixedValidInvalid tests JSONL with mix of valid and invalid lines
func TestRedactJSONLMixedValidInvalid(t *testing.T) {
	config := Config{
		Patterns: []Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{10}`,
				Type:    "api_key",
			},
		},
	}

	redactor, err := NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	input := "{\"key\":\"sk-1234567890\"}\nnot json sk-abcdefghij\n{\"other\":\"sk-0987654321\"}"
	result := redactor.RedactJSONL([]byte(input))

	// First line: valid JSON, should be parsed and redacted
	// Second line: invalid JSON, should fall back to text redaction
	// Third line: valid JSON, should be parsed and redacted
	resultStr := string(result)

	if strings.Contains(resultStr, "sk-1234567890") {
		t.Error("First API key should be redacted")
	}
	if strings.Contains(resultStr, "sk-abcdefghij") {
		t.Error("Second API key should be redacted (text fallback)")
	}
	if strings.Contains(resultStr, "sk-0987654321") {
		t.Error("Third API key should be redacted")
	}
}

// BenchmarkRedactBytes benchmarks text-based redaction
func BenchmarkRedactBytes(b *testing.B) {
	config := Config{Patterns: GetDefaultPatterns()}
	redactor, _ := NewRedactor(config)

	// Generate realistic JSONL content (~1MB)
	input := generateTestJSONL(1000)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))
	for i := 0; i < b.N; i++ {
		redactor.RedactBytes(input)
	}
}

// BenchmarkRedactJSONL benchmarks JSON-aware redaction
func BenchmarkRedactJSONL(b *testing.B) {
	config := Config{Patterns: GetDefaultPatterns()}
	redactor, _ := NewRedactor(config)

	// Generate realistic JSONL content (~1MB)
	input := generateTestJSONL(1000)

	b.ResetTimer()
	b.SetBytes(int64(len(input)))
	for i := 0; i < b.N; i++ {
		redactor.RedactJSONL(input)
	}
}

// generateTestJSONL creates realistic transcript-like JSONL for benchmarking
func generateTestJSONL(lines int) []byte {
	var builder strings.Builder
	for i := 0; i < lines; i++ {
		// Mix of message types similar to real transcripts
		switch i % 4 {
		case 0:
			builder.WriteString(`{"type":"user","timestamp":"2024-01-15T10:00:00Z","message":{"role":"user","content":"Please help me with this code that uses sk-ant-api03-xxxxxxxxxxxxxxxxxxxxx for authentication"}}`)
		case 1:
			builder.WriteString(`{"type":"assistant","timestamp":"2024-01-15T10:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"I'll help you with that. Here's the updated code with proper error handling and validation."}]}}`)
		case 2:
			builder.WriteString(`{"type":"tool_use","timestamp":"2024-01-15T10:00:02Z","tool":"bash","input":{"command":"echo $OPENAI_KEY"},"output":"sk-1234567890abcdefghijklmnopqrstuvwxyz123456"}`)
		case 3:
			builder.WriteString(`{"type":"result","timestamp":"2024-01-15T10:00:03Z","data":{"nested":{"deeply":{"value":"Some text with postgres://user:secretpass@localhost:5432/db connection string"}}}}`)
		}
		if i < lines-1 {
			builder.WriteByte('\n')
		}
	}
	return []byte(builder.String())
}
