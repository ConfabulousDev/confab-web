package redactor

import (
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
