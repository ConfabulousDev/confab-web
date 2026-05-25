package api

import (
	"strings"
	"testing"
)

// TestSanitizeContentDispositionFilename pins the CF-425 behavior that the
// download endpoint emits a safe filename in the Content-Disposition header —
// stripping characters that could break header syntax (CR/LF/quotes) or be
// misinterpreted as a path by client tools (/, \, ..).
func TestSanitizeContentDispositionFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"transcript.jsonl", "transcript.jsonl"},
		{"file with spaces.jsonl", "file_with_spaces.jsonl"},
		{"../../etc/passwd", ".._.._etc_passwd"},
		{"name\r\nInjected: yes", "name__Injected__yes"},
		{`evil";X-Injected: 1`, "evil__X-Injected__1"},
		{"", "download.txt"},
		{"中文.txt", "__.txt"},
	}
	for _, tc := range cases {
		got := sanitizeContentDispositionFilename(tc.in)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/org/repo.git", "org/repo"},
		{"https://github.com/org/repo", "org/repo"},
		// SSH URLs use colon separator — extractRepoName splits on "/" only
		{"git@github.com:org/repo.git", "git@github.com:org/repo"},
		{"repo", "repo"},
	}

	for _, tc := range tests {
		result := extractRepoName(tc.input)
		if result != tc.expected {
			t.Errorf("extractRepoName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTruncateTranscriptFromStart(t *testing.T) {
	transcript := `<transcript>
<user id="1">Hello world</user>
<assistant id="2">Hi there! How can I help?</assistant>
<user id="3">What is 2+2?</user>
<assistant id="4">The answer is 4.</assistant>
</transcript>`

	t.Run("no truncation when under limit", func(t *testing.T) {
		result := truncateTranscriptFromStart(transcript, len(transcript)+100)
		if result != transcript {
			t.Error("expected no truncation")
		}
	})

	t.Run("truncates from beginning preserving element boundaries", func(t *testing.T) {
		// Request only ~100 chars — should find a clean element boundary
		result := truncateTranscriptFromStart(transcript, 100)
		if !strings.Contains(result, "[Transcript truncated") {
			t.Error("expected truncation header")
		}
		// Should start at an element boundary
		if !strings.Contains(result, "<user ") && !strings.Contains(result, "<assistant ") {
			t.Error("expected result to contain a complete element start")
		}
	})

	t.Run("exact length returns unchanged", func(t *testing.T) {
		result := truncateTranscriptFromStart(transcript, len(transcript))
		if result != transcript {
			t.Error("expected no truncation at exact length")
		}
	})
}
