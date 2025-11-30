package api

import (
	"strings"
	"testing"
)

func TestMergeChunks(t *testing.T) {
	t.Run("single chunk returns as-is", func(t *testing.T) {
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("line1\nline2\nline3\n")},
		}

		result := mergeChunks(chunks)
		expected := "line1\nline2\nline3\n"

		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("non-overlapping chunks concatenate correctly", func(t *testing.T) {
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000002.jsonl", firstLine: 1, lastLine: 2, data: []byte("line1\nline2\n")},
			{key: "chunk_00000003_00000004.jsonl", firstLine: 3, lastLine: 4, data: []byte("line3\nline4\n")},
		}

		result := mergeChunks(chunks)
		expected := "line1\nline2\nline3\nline4\n"

		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("overlapping chunks - last write wins", func(t *testing.T) {
		// Scenario: chunk 1-5 uploaded, then 1-10 (DB update failed on first, client retried)
		// Chunks are in lexicographic order, so second chunk overwrites first
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("old1\nold2\nold3\nold4\nold5\n")},
			{key: "chunk_00000001_00000010.jsonl", firstLine: 1, lastLine: 10, data: []byte("new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n")},
		}

		result := mergeChunks(chunks)
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// All lines should come from the second chunk (processed last, overwrites)
		for i, line := range lines {
			expected := "new" + string('1'+byte(i))
			if i >= 9 {
				expected = "new10"
			} else {
				expected = "new" + string(rune('1'+i))
			}
			if line != expected {
				t.Errorf("line %d: expected %q, got %q", i+1, expected, line)
			}
		}
	})

	t.Run("overlapping chunks - partial overlap", func(t *testing.T) {
		// Scenario: chunk 1-5 uploaded, then 3-10 (overlap on lines 3-5)
		// Last write wins: lines 3-5 come from second chunk
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("A1\nA2\nA3\nA4\nA5\n")},
			{key: "chunk_00000003_00000010.jsonl", firstLine: 3, lastLine: 10, data: []byte("B3\nB4\nB5\nB6\nB7\nB8\nB9\nB10\n")},
		}

		result := mergeChunks(chunks)
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// Lines 1-2 from chunk A (not overwritten), lines 3-10 from chunk B (last write)
		expectedLines := []string{"A1", "A2", "B3", "B4", "B5", "B6", "B7", "B8", "B9", "B10"}
		for i, expected := range expectedLines {
			if lines[i] != expected {
				t.Errorf("line %d: expected %q, got %q", i+1, expected, lines[i])
			}
		}
	})

	t.Run("three overlapping chunks", func(t *testing.T) {
		// Scenario: multiple partial failures, chunks in lexicographic order
		// chunk 1-3, then 1-5, then 4-10
		// Last write wins for each line number
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("X1\nX2\nX3\n")},
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("Y1\nY2\nY3\nY4\nY5\n")},
			{key: "chunk_00000004_00000010.jsonl", firstLine: 4, lastLine: 10, data: []byte("Z4\nZ5\nZ6\nZ7\nZ8\nZ9\nZ10\n")},
		}

		result := mergeChunks(chunks)
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// Lines 1-3: Y overwrites X, then Z doesn't cover these
		// Lines 4-5: Y writes, then Z overwrites
		// Lines 6-10: only Z covers
		expectedLines := []string{"Y1", "Y2", "Y3", "Z4", "Z5", "Z6", "Z7", "Z8", "Z9", "Z10"}
		for i, expected := range expectedLines {
			if lines[i] != expected {
				t.Errorf("line %d: expected %q, got %q", i+1, expected, lines[i])
			}
		}
	})

	t.Run("gap in coverage", func(t *testing.T) {
		// Scenario: chunks 1-3 and 6-8, missing 4-5
		// Should output lines 1-3 and 6-8, skipping the gap
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("A\nB\nC\n")},
			{key: "chunk_00000006_00000008.jsonl", firstLine: 6, lastLine: 8, data: []byte("F\nG\nH\n")},
		}

		result := mergeChunks(chunks)
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		// Should have 6 lines (gap is skipped)
		if len(lines) != 6 {
			t.Errorf("expected 6 lines, got %d: %v", len(lines), lines)
		}

		expectedLines := []string{"A", "B", "C", "F", "G", "H"}
		for i, expected := range expectedLines {
			if lines[i] != expected {
				t.Errorf("line %d: expected %q, got %q", i+1, expected, lines[i])
			}
		}
	})

	t.Run("empty chunks slice", func(t *testing.T) {
		result := mergeChunks(nil)
		if result != nil {
			t.Errorf("expected nil, got %q", string(result))
		}
	})

	t.Run("chunk with no trailing newline", func(t *testing.T) {
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000002.jsonl", firstLine: 1, lastLine: 2, data: []byte("line1\nline2")}, // no trailing newline
		}

		result := mergeChunks(chunks)
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
		}
		if lines[0] != "line1" || lines[1] != "line2" {
			t.Errorf("unexpected lines: %v", lines)
		}
	})
}

func TestSplitLines(t *testing.T) {
	t.Run("normal lines with trailing newline", func(t *testing.T) {
		data := []byte("a\nb\nc\n")
		lines := splitLines(data)

		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}
		if string(lines[0]) != "a" || string(lines[1]) != "b" || string(lines[2]) != "c" {
			t.Errorf("unexpected lines: %v", lines)
		}
	})

	t.Run("lines without trailing newline", func(t *testing.T) {
		data := []byte("a\nb\nc")
		lines := splitLines(data)

		if len(lines) != 3 {
			t.Errorf("expected 3 lines, got %d", len(lines))
		}
		if string(lines[2]) != "c" {
			t.Errorf("expected last line 'c', got %q", string(lines[2]))
		}
	})

	t.Run("empty data", func(t *testing.T) {
		lines := splitLines(nil)
		if lines != nil {
			t.Errorf("expected nil, got %v", lines)
		}
	})

	t.Run("single line no newline", func(t *testing.T) {
		data := []byte("only")
		lines := splitLines(data)

		if len(lines) != 1 {
			t.Errorf("expected 1 line, got %d", len(lines))
		}
		if string(lines[0]) != "only" {
			t.Errorf("expected 'only', got %q", string(lines[0]))
		}
	})
}

func TestParseChunkKey(t *testing.T) {
	tests := []struct {
		key       string
		wantFirst int
		wantLast  int
		wantOK    bool
	}{
		{"123/claude-code/abc/chunks/transcript.jsonl/chunk_00000001_00000010.jsonl", 1, 10, true},
		{"123/claude-code/abc/chunks/agent.jsonl/chunk_00000100_00000200.jsonl", 100, 200, true},
		{"chunk_00000001_00000005.jsonl", 1, 5, true},
		{"invalid.jsonl", 0, 0, false},
		{"chunk_abc_def.jsonl", 0, 0, false},
		{"", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			first, last, ok := parseChunkKey(tt.key)
			if ok != tt.wantOK {
				t.Errorf("parseChunkKey(%q): ok = %v, want %v", tt.key, ok, tt.wantOK)
			}
			if ok && (first != tt.wantFirst || last != tt.wantLast) {
				t.Errorf("parseChunkKey(%q) = (%d, %d), want (%d, %d)", tt.key, first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

func TestExtractTitleFromLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "summary message",
			line: `{"type":"summary","summary":"This is a session summary"}`,
			want: "This is a session summary",
		},
		{
			name: "summary message truncated",
			line: `{"type":"summary","summary":"` + strings.Repeat("a", 150) + `"}`,
			want: strings.Repeat("a", 100) + "...",
		},
		{
			name: "user message with string content",
			line: `{"type":"user","message":{"role":"user","content":"Hello, how are you?"}}`,
			want: "Hello, how are you?",
		},
		{
			name: "user message with string content truncated",
			line: `{"type":"user","message":{"role":"user","content":"` + strings.Repeat("b", 150) + `"}}`,
			want: strings.Repeat("b", 100) + "...",
		},
		{
			name: "user message with multiline string content",
			line: `{"type":"user","message":{"role":"user","content":"First line\nSecond line\nThird line"}}`,
			want: "First line",
		},
		{
			name: "user message with array content containing text",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Hello from array"}]}}`,
			want: "Hello from array",
		},
		{
			name: "user message with array content - image then text",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{"type":"base64"}},{"type":"text","text":"Description after image"}]}}`,
			want: "Description after image",
		},
		{
			name: "user message with array content - only image",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"abc"}}]}}`,
			want: "",
		},
		{
			name: "user message with array content - multiple text blocks",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"First text"},{"type":"text","text":"Second text"}]}}`,
			want: "First text",
		},
		{
			name: "user message with array content - text truncated",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"` + strings.Repeat("c", 150) + `"}]}}`,
			want: strings.Repeat("c", 100) + "...",
		},
		{
			name: "user message with array content - multiline text",
			line: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Line one\nLine two"}]}}`,
			want: "Line one",
		},
		{
			name: "assistant message ignored",
			line: `{"type":"assistant","message":{"role":"assistant","content":"I can help with that"}}`,
			want: "",
		},
		{
			name: "empty summary",
			line: `{"type":"summary","summary":""}`,
			want: "",
		},
		{
			name: "no type field",
			line: `{"foo":"bar"}`,
			want: "",
		},
		{
			name: "invalid json",
			line: `not json`,
			want: "",
		},
		{
			name: "empty line",
			line: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTitleFromLine(tt.line)
			if got != tt.want {
				t.Errorf("extractTitleFromLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTextFromMessage(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]interface{}
		want  string
	}{
		{
			name: "string content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": "Hello world",
				},
			},
			want: "Hello world",
		},
		{
			name: "array content with text block",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "text", "text": "Array text"},
					},
				},
			},
			want: "Array text",
		},
		{
			name: "array content with image then text",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "image", "source": map[string]interface{}{}},
						map[string]interface{}{"type": "text", "text": "After image"},
					},
				},
			},
			want: "After image",
		},
		{
			name: "array content with only image",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{
						map[string]interface{}{"type": "image", "source": map[string]interface{}{}},
					},
				},
			},
			want: "",
		},
		{
			name: "no message field",
			entry: map[string]interface{}{
				"type": "user",
			},
			want: "",
		},
		{
			name: "nil content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": nil,
				},
			},
			want: "",
		},
		{
			name: "empty string content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": "",
				},
			},
			want: "",
		},
		{
			name: "empty array content",
			entry: map[string]interface{}{
				"message": map[string]interface{}{
					"content": []interface{}{},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromMessage(tt.entry)
			if got != tt.want {
				t.Errorf("extractTextFromMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSessionTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "summary takes priority",
			content: `{"type":"user","message":{"role":"user","content":"First question"}}` + "\n" + `{"type":"summary","summary":"Session about questions"}`,
			want:    "Session about questions",
		},
		{
			name:    "falls back to first user text",
			content: `{"type":"user","message":{"role":"user","content":"What is Go?"}}` + "\n" + `{"type":"assistant","message":{"role":"assistant","content":"Go is a language"}}`,
			want:    "What is Go?",
		},
		{
			name:    "skips image-only message finds later text",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}}]}}` + "\n" + `{"type":"user","message":{"role":"user","content":"Here is my question"}}`,
			want:    "Here is my question",
		},
		{
			name:    "multimodal message with text after image",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}},{"type":"text","text":"Describe this image"}]}}`,
			want:    "Describe this image",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "only image messages",
			content: `{"type":"user","message":{"role":"user","content":[{"type":"image","source":{}}]}}`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSessionTitle([]byte(tt.content))
			if got != tt.want {
				t.Errorf("extractSessionTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
