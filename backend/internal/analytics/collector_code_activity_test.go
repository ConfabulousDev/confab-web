package analytics

import (
	"os"
	"testing"
)

// TestCodeActivityCollector_RealSession tests the collector against a real session transcript.
// The test fixture is a copy of an actual Claude Code session with comprehensive tool usage.
//
// Expected values derived from testdata/session_comprehensive.jsonl:
//   - Read: 10 unique file paths
//   - Write: 1 unique file path (367 lines)
//   - Edit: 6 unique file paths (34 lines added, 26 lines removed)
//   - Glob: 3 calls
//   - Grep: 1 call
//   - FilesModified: 7 (Write + Edit, no overlap)
//   - Extensions: css=2, ts=5, tsx=16
func TestCodeActivityCollector_RealSession(t *testing.T) {
	content, err := os.ReadFile("testdata/session_comprehensive.jsonl")
	if err != nil {
		t.Fatalf("Failed to read test fixture: %v", err)
	}

	result, err := ComputeFromJSONL(content)
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	// Test file counts
	t.Run("FilesRead", func(t *testing.T) {
		expected := 10
		if result.FilesRead != expected {
			t.Errorf("FilesRead = %d, want %d", result.FilesRead, expected)
		}
	})

	t.Run("FilesModified", func(t *testing.T) {
		expected := 7 // 1 Write + 6 Edit (no overlap)
		if result.FilesModified != expected {
			t.Errorf("FilesModified = %d, want %d", result.FilesModified, expected)
		}
	})

	// Test line counts (matches GitHub diff behavior: old lines removed, new lines added)
	t.Run("LinesAdded", func(t *testing.T) {
		expected := 401 // 367 from Write + 34 from Edit new_strings
		if result.LinesAdded != expected {
			t.Errorf("LinesAdded = %d, want %d", result.LinesAdded, expected)
		}
	})

	t.Run("LinesRemoved", func(t *testing.T) {
		expected := 26 // From Edit old_strings
		if result.LinesRemoved != expected {
			t.Errorf("LinesRemoved = %d, want %d", result.LinesRemoved, expected)
		}
	})

	// Test search counts
	t.Run("SearchCount", func(t *testing.T) {
		expected := 4 // 3 Glob + 1 Grep
		if result.SearchCount != expected {
			t.Errorf("SearchCount = %d, want %d", result.SearchCount, expected)
		}
	})

	// Test language breakdown
	t.Run("LanguageBreakdown", func(t *testing.T) {
		expectedBreakdown := map[string]int{
			"css": 2,
			"ts":  5,
			"tsx": 16,
		}

		if len(result.LanguageBreakdown) != len(expectedBreakdown) {
			t.Errorf("LanguageBreakdown has %d entries, want %d",
				len(result.LanguageBreakdown), len(expectedBreakdown))
		}

		for ext, expected := range expectedBreakdown {
			if got := result.LanguageBreakdown[ext]; got != expected {
				t.Errorf("LanguageBreakdown[%q] = %d, want %d", ext, got, expected)
			}
		}
	})
}

// TestCountLines verifies the line counting logic.
func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single line no newline", "hello", 1},
		{"single line with newline", "hello\n", 1}, // Trailing newline ignored
		{"two lines", "hello\nworld", 2},
		{"two lines with trailing", "hello\nworld\n", 2}, // Trailing newline ignored
		{"multiple lines", "a\nb\nc\nd", 4},
		{"multiple lines with trailing", "a\nb\nc\n", 3}, // Trailing newline ignored
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.input)
			if got != tt.expected {
				t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// TestCodeActivityCollector_EmptySession tests behavior with no tool calls.
func TestCodeActivityCollector_EmptySession(t *testing.T) {
	content := []byte(`{"type":"user","message":{"role":"user","content":"hello"}}
{"type":"assistant","message":{"role":"assistant","content":"hi","usage":{"input_tokens":10,"output_tokens":5}}}`)

	result, err := ComputeFromJSONL(content)
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	if result.FilesRead != 0 {
		t.Errorf("FilesRead = %d, want 0", result.FilesRead)
	}
	if result.FilesModified != 0 {
		t.Errorf("FilesModified = %d, want 0", result.FilesModified)
	}
	if result.LinesAdded != 0 {
		t.Errorf("LinesAdded = %d, want 0", result.LinesAdded)
	}
	if result.LinesRemoved != 0 {
		t.Errorf("LinesRemoved = %d, want 0", result.LinesRemoved)
	}
	if result.SearchCount != 0 {
		t.Errorf("SearchCount = %d, want 0", result.SearchCount)
	}
	if len(result.LanguageBreakdown) != 0 {
		t.Errorf("LanguageBreakdown has %d entries, want 0", len(result.LanguageBreakdown))
	}
}

// TestCodeActivityCollector_Deduplication verifies that duplicate file paths are counted once.
func TestCodeActivityCollector_Deduplication(t *testing.T) {
	// Two Read calls to the same file should count as 1 unique file
	content := []byte(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo/bar.go"}}],"usage":{"input_tokens":10,"output_tokens":5}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/foo/bar.go"}}],"usage":{"input_tokens":10,"output_tokens":5}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/foo/bar.go","old_string":"a","new_string":"b"}}],"usage":{"input_tokens":10,"output_tokens":5}}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/foo/bar.go","old_string":"c","new_string":"d"}}],"usage":{"input_tokens":10,"output_tokens":5}}}`)

	result, err := ComputeFromJSONL(content)
	if err != nil {
		t.Fatalf("ComputeFromJSONL failed: %v", err)
	}

	if result.FilesRead != 1 {
		t.Errorf("FilesRead = %d, want 1 (should dedupe)", result.FilesRead)
	}
	if result.FilesModified != 1 {
		t.Errorf("FilesModified = %d, want 1 (should dedupe)", result.FilesModified)
	}
	// Extension should be counted per operation, not per unique file
	if result.LanguageBreakdown["go"] != 4 {
		t.Errorf("LanguageBreakdown[go] = %d, want 4 (2 reads + 2 edits)", result.LanguageBreakdown["go"])
	}
}
