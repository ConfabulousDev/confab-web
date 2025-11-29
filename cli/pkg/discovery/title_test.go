package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSessionTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty file",
			content:  "",
			expected: "",
		},
		{
			name:     "summary message",
			content:  `{"type":"summary","summary":"Fix authentication bug in login flow"}`,
			expected: "Fix authentication bug in login flow",
		},
		{
			name: "summary after user message",
			content: `{"type":"user","message":{"content":"Help me fix a bug"}}
{"type":"assistant","message":{"content":"Sure!"}}
{"type":"summary","summary":"Bug fix assistance"}`,
			expected: "Bug fix assistance",
		},
		{
			name:     "user message fallback",
			content:  `{"type":"user","message":{"content":"Can you help me refactor this function?"}}`,
			expected: "Can you help me refactor this function?",
		},
		{
			name:     "long user message truncated",
			content:  `{"type":"user","message":{"content":"This is a very long message that should be truncated because it exceeds the maximum title length of one hundred characters which we set as the limit"}}`,
			expected: "This is a very long message that should be truncated because it exceeds the maximum title length of ",
		},
		{
			name:     "HTML tags removed",
			content:  `{"type":"summary","summary":"Fix <code>auth</code> bug"}`,
			expected: "Fix auth bug",
		},
		{
			name:     "HTML entities decoded",
			content:  `{"type":"summary","summary":"Fix &lt;div&gt; rendering"}`,
			expected: "Fix <div> rendering",
		},
		{
			name:     "newlines collapsed",
			content:  `{"type":"user","message":{"content":"Line one\nLine two\nLine three"}}`,
			expected: "Line one Line two Line three",
		},
		{
			name:     "no user or summary messages",
			content:  `{"type":"assistant","message":{"content":"Hello!"}}`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with test content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.jsonl")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			result := ExtractSessionTitle(tmpFile)
			if result != tt.expected {
				t.Errorf("ExtractSessionTitle() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExtractSessionTitle_NonexistentFile(t *testing.T) {
	result := ExtractSessionTitle("/nonexistent/path/file.jsonl")
	if result != "" {
		t.Errorf("ExtractSessionTitle() for nonexistent file = %q, want empty string", result)
	}
}

func TestSanitizeTitleText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "HTML tags",
			input:    "<p>Hello</p> <strong>world</strong>",
			expected: "Hello world",
		},
		{
			name:     "HTML entities",
			input:    "&lt;div&gt; &amp; &quot;test&quot;",
			expected: "<div> & \"test\"",
		},
		{
			name:     "whitespace normalization",
			input:    "  multiple   spaces  ",
			expected: "multiple spaces",
		},
		{
			name:     "newlines",
			input:    "line1\nline2\r\nline3",
			expected: "line1 line2 line3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTitleText(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeTitleText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
