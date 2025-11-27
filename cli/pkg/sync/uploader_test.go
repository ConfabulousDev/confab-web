package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santaclaude2025/confab/pkg/redactor"
)

// TestRedactionPreservesJSONStructure verifies that redaction applied during
// upload never corrupts JSON structure. Field-based patterns match field names
// and redact the entire value, avoiding regex replacement across JSON structure.
func TestRedactionPreservesJSONStructure(t *testing.T) {
	// Create a field-based pattern: match fields named "password" and redact their values
	config := redactor.Config{
		Patterns: []redactor.Pattern{
			{
				Name:         "Password Field",
				Type:         "password",
				FieldPattern: `^password$`,
			},
		},
	}

	r, err := redactor.NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	// Create a temporary JSONL file with content that will trigger the pattern
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "transcript.jsonl")

	// This is valid JSONL - each line is a valid JSON object
	// Tests various scenarios: top-level field, nested field, and field in array of objects
	jsonlContent := `{"type":"message","password":"secret123","data":"test"}
{"type":"config","settings":{"password":"hunter2","timeout":30}}
{"type":"users","items":[{"name":"alice","password":"pass1"},{"name":"bob","password":"pass2"}]}`

	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Simulate what uploadFile does: read lines and apply redaction
	lines, err := readAndRedactFile(jsonlPath, r)
	if err != nil {
		t.Fatalf("Failed to read and redact file: %v", err)
	}

	// Verify each line is still valid JSON after redaction
	for i, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON after redaction:\n  Input line %d from file\n  Output: %s\n  Error: %v",
				i+1, i+1, line, err)
		}
	}

	// Also verify the secrets are actually redacted
	for i, line := range lines {
		if contains(line, "secret123") || contains(line, "hunter2") || contains(line, "pass1") || contains(line, "pass2") {
			t.Errorf("Line %d still contains unredacted secret: %s", i+1, line)
		}
	}
}

// TestRedactionFieldPatternWithArrays tests field-based patterns correctly handle
// arrays of strings and arrays of objects.
func TestRedactionFieldPatternWithArrays(t *testing.T) {
	config := redactor.Config{
		Patterns: []redactor.Pattern{
			{
				Name:         "Secrets Array",
				Type:         "secret",
				FieldPattern: `^secrets$`,
			},
			{
				Name:         "Password Field",
				Type:         "password",
				FieldPattern: `^password$`,
			},
		},
	}

	r, err := redactor.NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Test cases:
	// 1. Array of strings under a matching field name
	// 2. Array of objects where objects have matching field names
	// 3. Nested arrays
	jsonlContent := `{"secrets":["secret1","secret2","secret3"]}
{"users":[{"name":"alice","password":"pass1"},{"name":"bob","password":"pass2"}]}
{"config":{"nested":{"secrets":["deep1","deep2"]}}}`

	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	lines, err := readAndRedactFile(jsonlPath, r)
	if err != nil {
		t.Fatalf("Failed to read and redact file: %v", err)
	}

	// Verify each line is still valid JSON
	for i, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON after redaction:\n  Output: %s\n  Error: %v",
				i+1, line, err)
		}
	}

	// Verify secrets are redacted
	allSecrets := []string{"secret1", "secret2", "secret3", "pass1", "pass2", "deep1", "deep2"}
	for i, line := range lines {
		for _, secret := range allSecrets {
			if contains(line, secret) {
				t.Errorf("Line %d still contains unredacted secret '%s': %s", i+1, secret, line)
			}
		}
	}

	// Verify non-secret data is preserved
	for i, line := range lines {
		if i == 1 { // Second line should still have names
			if !contains(line, "alice") || !contains(line, "bob") {
				t.Errorf("Line %d should preserve non-secret data: %s", i+1, line)
			}
		}
	}
}

// TestRedactionWithCaptureGroupPreservesJSON tests that capture group patterns
// (which redact only part of a match) also preserve JSON structure.
func TestRedactionWithCaptureGroupPreservesJSON(t *testing.T) {
	// This pattern uses a capture group to redact only the password value,
	// preserving the surrounding structure. However, when applied as text
	// replacement, it can still cause issues with JSON escaping.
	config := redactor.Config{
		Patterns: []redactor.Pattern{
			{
				Name:         "Connection String Password",
				Pattern:      `(postgres://[^:]+:)([^@]+)(@)`,
				Type:         "password",
				CaptureGroup: 2,
			},
		},
	}

	r, err := redactor.NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "transcript.jsonl")

	jsonlContent := `{"type":"config","database":"postgres://user:secretpass@localhost:5432/db"}
{"type":"log","message":"Connecting to postgres://admin:hunter2@db.example.com/prod"}`

	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	lines, err := readAndRedactFile(jsonlPath, r)
	if err != nil {
		t.Fatalf("Failed to read and redact file: %v", err)
	}

	for i, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON after redaction:\n  Output: %s\n  Error: %v",
				i+1, line, err)
		}
	}

	// Verify passwords are redacted
	for i, line := range lines {
		if contains(line, "secretpass") || contains(line, "hunter2") {
			t.Errorf("Line %d still contains unredacted password: %s", i+1, line)
		}
	}
}

// TestRedactionDoesNotCorruptNestedJSON tests redaction with deeply nested JSON
// structures that contain secrets at various levels.
func TestRedactionDoesNotCorruptNestedJSON(t *testing.T) {
	config := redactor.Config{
		Patterns: []redactor.Pattern{
			{
				Name:    "API Key",
				Pattern: `sk-[A-Za-z0-9]{20,}`,
				Type:    "api_key",
			},
			{
				Name:    "Bearer Token",
				Pattern: `Bearer [A-Za-z0-9._-]+`,
				Type:    "bearer_token",
			},
		},
	}

	r, err := redactor.NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "transcript.jsonl")

	// Complex nested JSON with secrets at various depths
	jsonlContent := `{"type":"request","headers":{"Authorization":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test"},"body":{"nested":{"deep":{"api_key":"sk-ant1234567890abcdefghij"}}}}
{"type":"response","data":{"items":[{"secret":"sk-proj9876543210zyxwvutsrq"},{"normal":"value"}],"meta":{"count":2}}}`

	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	lines, err := readAndRedactFile(jsonlPath, r)
	if err != nil {
		t.Fatalf("Failed to read and redact file: %v", err)
	}

	for i, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON after redaction:\n  Output: %s\n  Error: %v",
				i+1, line, err)
		}
	}
}

// TestRedactionWithSpecialJSONCharactersInSecrets tests that secrets containing
// characters that have special meaning in JSON (quotes, backslashes, etc.) are
// handled correctly.
func TestRedactionWithSpecialJSONCharactersInSecrets(t *testing.T) {
	config := redactor.Config{
		Patterns: []redactor.Pattern{
			{
				Name:    "Secret Token",
				Pattern: `secret_[A-Za-z0-9+/=]+`,
				Type:    "secret",
			},
		},
	}

	r, err := redactor.NewRedactor(config)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "transcript.jsonl")

	// JSON with escaped characters alongside secrets
	jsonlContent := `{"message":"Token is secret_abc123+/= in \"quotes\"","newline":"line1\nline2"}
{"data":"Path: C:\\Users\\test\\secret_xyz789","tab":"col1\tcol2"}`

	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	lines, err := readAndRedactFile(jsonlPath, r)
	if err != nil {
		t.Fatalf("Failed to read and redact file: %v", err)
	}

	for i, line := range lines {
		var parsed interface{}
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			t.Errorf("Line %d is not valid JSON after redaction:\n  Output: %s\n  Error: %v",
				i+1, line, err)
		}
	}

	// Verify secrets are redacted
	for i, line := range lines {
		if contains(line, "secret_abc123") || contains(line, "secret_xyz789") {
			t.Errorf("Line %d still contains unredacted secret: %s", i+1, line)
		}
	}
}

// readAndRedactFile simulates what uploadFile does: reads lines and applies redaction
func readAndRedactFile(path string, r *redactor.Redactor) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Split into lines (same as bufio.Scanner would do)
	var lines []string
	start := 0
	for i, b := range content {
		if b == '\n' {
			line := string(content[start:i])
			if r != nil {
				line = r.RedactJSONLine(line)
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	// Handle last line without trailing newline
	if start < len(content) {
		line := string(content[start:])
		if r != nil {
			line = r.RedactJSONLine(line)
		}
		lines = append(lines, line)
	}

	return lines, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
