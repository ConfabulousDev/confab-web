package types

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewJSONLScanner(t *testing.T) {
	t.Run("handles normal sized lines", func(t *testing.T) {
		input := `{"type":"user","message":"hello"}`
		scanner := NewJSONLScanner(strings.NewReader(input))

		if !scanner.Scan() {
			t.Fatal("Failed to scan normal line")
		}

		got := scanner.Text()
		if got != input {
			t.Errorf("Got %q, want %q", got, input)
		}
	})

	t.Run("handles lines larger than default 64KB buffer", func(t *testing.T) {
		// Create a line that's 100KB (larger than default 64KB buffer)
		largeContent := strings.Repeat("x", 100*1024)
		input := `{"type":"assistant","content":"` + largeContent + `"}`

		scanner := NewJSONLScanner(strings.NewReader(input))

		if !scanner.Scan() {
			t.Fatalf("Failed to scan large line: %v", scanner.Err())
		}

		got := scanner.Text()
		if len(got) != len(input) {
			t.Errorf("Got %d bytes, want %d bytes", len(got), len(input))
		}
	})

	t.Run("handles lines up to 10MB", func(t *testing.T) {
		// Create a line close to the 10MB limit
		largeContent := strings.Repeat("a", 9*1024*1024) // 9MB
		input := `{"data":"` + largeContent + `"}`

		scanner := NewJSONLScanner(strings.NewReader(input))

		if !scanner.Scan() {
			t.Fatalf("Failed to scan 9MB line: %v", scanner.Err())
		}

		got := scanner.Text()
		if len(got) != len(input) {
			t.Errorf("Got %d bytes, want %d bytes", len(got), len(input))
		}
	})

	t.Run("handles multiple lines", func(t *testing.T) {
		input := "line1\nline2\nline3"
		scanner := NewJSONLScanner(strings.NewReader(input))

		lines := []string{}
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}

		if len(lines) != 3 {
			t.Fatalf("Got %d lines, want 3", len(lines))
		}

		expected := []string{"line1", "line2", "line3"}
		for i, line := range lines {
			if line != expected[i] {
				t.Errorf("Line %d: got %q, want %q", i, line, expected[i])
			}
		}
	})

	t.Run("handles empty input", func(t *testing.T) {
		scanner := NewJSONLScanner(strings.NewReader(""))

		if scanner.Scan() {
			t.Error("Expected no lines from empty input")
		}

		if scanner.Err() != nil {
			t.Errorf("Unexpected error: %v", scanner.Err())
		}
	})

	t.Run("returns error for lines exceeding 10MB", func(t *testing.T) {
		// Create a line that exceeds 10MB
		tooLargeContent := strings.Repeat("x", 11*1024*1024) // 11MB
		input := `{"data":"` + tooLargeContent + `"}`

		scanner := NewJSONLScanner(strings.NewReader(input))

		// Should fail to scan
		if scanner.Scan() {
			t.Error("Expected scan to fail for line > 10MB")
		}

		// Should have an error
		if scanner.Err() == nil {
			t.Error("Expected error for line > 10MB, got nil")
		}
	})
}

func TestMaxJSONLLineSize(t *testing.T) {
	// Verify the constant is set to 10MB
	expected := 10 * 1024 * 1024
	if MaxJSONLLineSize != expected {
		t.Errorf("MaxJSONLLineSize = %d, want %d", MaxJSONLLineSize, expected)
	}
}

func TestNewJSONLScanner_RealWorldScenarios(t *testing.T) {
	t.Run("handles JSONL with thinking blocks", func(t *testing.T) {
		// Simulate a realistic transcript line with a large thinking block
		thinkingBlock := strings.Repeat("This is a long thinking process. ", 10000) // ~330KB
		jsonLine := `{"type":"assistant","message":{"thinking":"` + thinkingBlock + `"}}`

		scanner := NewJSONLScanner(bytes.NewReader([]byte(jsonLine)))

		if !scanner.Scan() {
			t.Fatalf("Failed to scan realistic thinking block: %v", scanner.Err())
		}

		if scanner.Err() != nil {
			t.Errorf("Unexpected error: %v", scanner.Err())
		}
	})

	t.Run("handles JSONL with large tool results", func(t *testing.T) {
		// Simulate a tool result with lots of output
		toolOutput := strings.Repeat("output line\n", 50000) // ~550KB
		jsonLine := `{"type":"tool_result","content":"` + toolOutput + `"}`

		scanner := NewJSONLScanner(bytes.NewReader([]byte(jsonLine)))

		if !scanner.Scan() {
			t.Fatalf("Failed to scan large tool result: %v", scanner.Err())
		}

		if scanner.Err() != nil {
			t.Errorf("Unexpected error: %v", scanner.Err())
		}
	})
}
