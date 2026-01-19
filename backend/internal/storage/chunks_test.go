package storage

import (
	"strings"
	"testing"
)

func TestMergeChunks(t *testing.T) {
	t.Run("single chunk returns as-is", func(t *testing.T) {
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("line1\nline2\nline3\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "line1\nline2\nline3\n"

		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("non-overlapping chunks concatenate correctly", func(t *testing.T) {
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("line1\nline2\n")},
			{Key: "chunk_00000003_00000004.jsonl", FirstLine: 3, LastLine: 4, Data: []byte("line3\nline4\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "line1\nline2\nline3\nline4\n"

		if string(result) != expected {
			t.Errorf("expected %q, got %q", expected, string(result))
		}
	})

	t.Run("overlapping chunks - last write wins", func(t *testing.T) {
		// Scenario: chunk 1-5 uploaded, then 1-10 (DB update failed on first, client retried)
		// Chunks are in lexicographic order, so second chunk overwrites first
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("old1\nold2\nold3\nold4\nold5\n")},
			{Key: "chunk_00000001_00000010.jsonl", FirstLine: 1, LastLine: 10, Data: []byte("new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 10 {
			t.Errorf("expected 10 lines, got %d: %v", len(lines), lines)
		}

		// All lines should come from the second chunk (processed last, overwrites)
		for i, line := range lines {
			var expected string
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
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("A1\nA2\nA3\nA4\nA5\n")},
			{Key: "chunk_00000003_00000010.jsonl", FirstLine: 3, LastLine: 10, Data: []byte("B3\nB4\nB5\nB6\nB7\nB8\nB9\nB10\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("X1\nX2\nX3\n")},
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("Y1\nY2\nY3\nY4\nY5\n")},
			{Key: "chunk_00000004_00000010.jsonl", FirstLine: 4, LastLine: 10, Data: []byte("Z4\nZ5\nZ6\nZ7\nZ8\nZ9\nZ10\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("A\nB\nC\n")},
			{Key: "chunk_00000006_00000008.jsonl", FirstLine: 6, LastLine: 8, Data: []byte("F\nG\nH\n")},
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		result, err := MergeChunks(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %q", string(result))
		}
	})

	t.Run("chunk with no trailing newline", func(t *testing.T) {
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("line1\nline2")}, // no trailing newline
		}

		result, err := MergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(string(result)), "\n")

		if len(lines) != 2 {
			t.Errorf("expected 2 lines, got %d: %v", len(lines), lines)
		}
		if lines[0] != "line1" || lines[1] != "line2" {
			t.Errorf("unexpected lines: %v", lines)
		}
	})

	t.Run("exceeds MaxMergeLines returns error", func(t *testing.T) {
		// Create chunks that would require more than MaxMergeLines
		chunks := []ChunkInfo{
			{Key: "chunk_00000001_00000010.jsonl", FirstLine: 1, LastLine: 10, Data: []byte("a\n")},
			{Key: "chunk_99999990_99999999.jsonl", FirstLine: MaxMergeLines + 1, LastLine: MaxMergeLines + 10, Data: []byte("b\n")},
		}

		_, err := MergeChunks(chunks)
		if err == nil {
			t.Error("expected error for exceeding MaxMergeLines, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds safety limit") {
			t.Errorf("expected error message about safety limit, got: %v", err)
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
			first, last, ok := ParseChunkKey(tt.key)
			if ok != tt.wantOK {
				t.Errorf("ParseChunkKey(%q): ok = %v, want %v", tt.key, ok, tt.wantOK)
			}
			if ok && (first != tt.wantFirst || last != tt.wantLast) {
				t.Errorf("ParseChunkKey(%q) = (%d, %d), want (%d, %d)", tt.key, first, last, tt.wantFirst, tt.wantLast)
			}
		})
	}
}

func TestChunksOverlap(t *testing.T) {
	tests := []struct {
		name   string
		chunks []ChunkInfo
		want   bool
	}{
		{
			name:   "empty slice",
			chunks: nil,
			want:   false,
		},
		{
			name:   "single chunk",
			chunks: []ChunkInfo{{FirstLine: 1, LastLine: 10}},
			want:   false,
		},
		{
			name: "non-overlapping",
			chunks: []ChunkInfo{
				{FirstLine: 1, LastLine: 5},
				{FirstLine: 6, LastLine: 10},
			},
			want: false,
		},
		{
			name: "adjacent (not overlapping)",
			chunks: []ChunkInfo{
				{FirstLine: 1, LastLine: 5},
				{FirstLine: 5, LastLine: 10},
			},
			want: true, // 5 is in both ranges
		},
		{
			name: "overlapping",
			chunks: []ChunkInfo{
				{FirstLine: 1, LastLine: 7},
				{FirstLine: 5, LastLine: 10},
			},
			want: true,
		},
		{
			name: "complete overlap",
			chunks: []ChunkInfo{
				{FirstLine: 1, LastLine: 10},
				{FirstLine: 3, LastLine: 7},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChunksOverlap(tt.chunks)
			if got != tt.want {
				t.Errorf("ChunksOverlap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeChunksSimple(t *testing.T) {
	tests := []struct {
		name   string
		chunks [][]byte
		want   string
	}{
		{
			name:   "empty chunks returns nil",
			chunks: [][]byte{},
			want:   "",
		},
		{
			name:   "single chunk",
			chunks: [][]byte{[]byte("hello")},
			want:   "hello",
		},
		{
			name:   "multiple chunks",
			chunks: [][]byte{[]byte("hello"), []byte(" "), []byte("world")},
			want:   "hello world",
		},
		{
			name:   "chunks with newlines",
			chunks: [][]byte{[]byte("line1\n"), []byte("line2\n")},
			want:   "line1\nline2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeChunksSimple(tt.chunks)
			if string(got) != tt.want {
				t.Errorf("MergeChunksSimple() = %q, want %q", string(got), tt.want)
			}
		})
	}
}
