package storage

import (
	"bytes"
	"errors"
	"io"
	"runtime"
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
		// Codex-path fixtures (CF-351). The parser is opaque to the provider
		// segment and must accept any value there identically.
		{"123/codex/abc/chunks/transcript.jsonl/chunk_00000001_00000010.jsonl", 1, 10, true},
		{"456/codex/def/chunks/transcript.jsonl/chunk_00000050_00000099.jsonl", 50, 99, true},
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

// TestMergeChunks_ResultSizedExactly guards the 5m68 allocation fix: the merged
// buffer is allocated once at its exact size, even when overlapping chunks
// make the summed chunk sizes an overestimate.
func TestMergeChunks_ResultSizedExactly(t *testing.T) {
	chunks := []ChunkInfo{
		{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("old1\nold2\nold3\nold4\nold5\n")},
		{Key: "chunk_00000001_00000010.jsonl", FirstLine: 1, LastLine: 10, Data: []byte("new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n")},
	}
	result, err := MergeChunks(chunks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cap(result) != len(result) {
		t.Errorf("cap(result) = %d, len(result) = %d; the merged buffer must be sized exactly", cap(result), len(result))
	}

	empty, err := MergeChunks([]ChunkInfo{
		{Key: "chunk_00000001_00000001.jsonl", FirstLine: 1, LastLine: 1},
		{Key: "chunk_00000002_00000002.jsonl", FirstLine: 2, LastLine: 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty != nil {
		t.Errorf("merging chunks with no lines = %q, want nil", empty)
	}
}

// mergeParityCases pins the exact bytes MergeChunks produced before x4ry
// (captured from the pre-change implementation). WriteMergedLines with
// afterLine == 0 must reproduce them byte for byte.
var mergeParityCases = []struct {
	name   string
	chunks []ChunkInfo
	want   string
}{
	{
		name:   "single chunk returned verbatim",
		chunks: []ChunkInfo{{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("line1\nline2\nline3\n")}},
		want:   "line1\nline2\nline3\n",
	},
	{
		name:   "single chunk without trailing newline returned verbatim",
		chunks: []ChunkInfo{{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("line1\nline2")}},
		want:   "line1\nline2",
	},
	{
		name: "non-overlapping chunks",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("line1\nline2\n")},
			{Key: "chunk_00000003_00000004.jsonl", FirstLine: 3, LastLine: 4, Data: []byte("line3\nline4\n")},
		},
		want: "line1\nline2\nline3\nline4\n",
	},
	{
		name: "full overlap, last write wins",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("old1\nold2\nold3\nold4\nold5\n")},
			{Key: "chunk_00000001_00000010.jsonl", FirstLine: 1, LastLine: 10, Data: []byte("new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n")},
		},
		want: "new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n",
	},
	{
		name: "conflicting partial overlap",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("A1\nA2\nA3\nA4\nA5\n")},
			{Key: "chunk_00000003_00000010.jsonl", FirstLine: 3, LastLine: 10, Data: []byte("B3\nB4\nB5\nB6\nB7\nB8\nB9\nB10\n")},
		},
		want: "A1\nA2\nB3\nB4\nB5\nB6\nB7\nB8\nB9\nB10\n",
	},
	{
		name: "three overlapping chunks",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("X1\nX2\nX3\n")},
			{Key: "chunk_00000001_00000005.jsonl", FirstLine: 1, LastLine: 5, Data: []byte("Y1\nY2\nY3\nY4\nY5\n")},
			{Key: "chunk_00000004_00000010.jsonl", FirstLine: 4, LastLine: 10, Data: []byte("Z4\nZ5\nZ6\nZ7\nZ8\nZ9\nZ10\n")},
		},
		want: "Y1\nY2\nY3\nZ4\nZ5\nZ6\nZ7\nZ8\nZ9\nZ10\n",
	},
	{
		name: "gap in coverage is skipped",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("A\nB\nC\n")},
			{Key: "chunk_00000006_00000008.jsonl", FirstLine: 6, LastLine: 8, Data: []byte("F\nG\nH\n")},
		},
		want: "A\nB\nC\nF\nG\nH\n",
	},
	{
		name: "out-of-order chunks are written in line order",
		chunks: []ChunkInfo{
			{Key: "chunk_00000004_00000006.jsonl", FirstLine: 4, LastLine: 6, Data: []byte("D\nE\nF\n")},
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("A\nB\nC\n")},
		},
		want: "A\nB\nC\nD\nE\nF\n",
	},
	{
		name: "missing trailing newline is normalized across chunks",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("a\nb")},
			{Key: "chunk_00000003_00000003.jsonl", FirstLine: 3, LastLine: 3, Data: []byte("c\n")},
		},
		want: "a\nb\nc\n",
	},
	{
		name: "empty lines inside a chunk are preserved",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("a\n\nc\n")},
			{Key: "chunk_00000004_00000004.jsonl", FirstLine: 4, LastLine: 4, Data: []byte("d\n")},
		},
		want: "a\n\nc\nd\n",
	},
	{
		name: "chunks with no data produce nothing",
		chunks: []ChunkInfo{
			{Key: "chunk_00000001_00000001.jsonl", FirstLine: 1, LastLine: 1},
			{Key: "chunk_00000002_00000002.jsonl", FirstLine: 2, LastLine: 2},
		},
		want: "",
	},
	{
		name:   "no chunks produce nothing",
		chunks: nil,
		want:   "",
	},
}

func TestMergeChunks_ByteParity(t *testing.T) {
	for _, tc := range mergeParityCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MergeChunks(tc.chunks)
			if err != nil {
				t.Fatalf("MergeChunks: unexpected error: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("MergeChunks = %q, want %q", got, tc.want)
			}
			if tc.want == "" && got != nil {
				t.Errorf("MergeChunks with no lines = %q, want nil", got)
			}
		})
	}
}

func TestWriteMergedLines_MatchesPreChangeMergeChunks(t *testing.T) {
	for _, tc := range mergeParityCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := WriteMergedLines(&buf, tc.chunks, 0)
			if err != nil {
				t.Fatalf("WriteMergedLines: unexpected error: %v", err)
			}
			if buf.String() != tc.want {
				t.Errorf("WriteMergedLines wrote %q, want the pre-change MergeChunks bytes %q", buf.String(), tc.want)
			}
			if n != int64(buf.Len()) {
				t.Errorf("WriteMergedLines returned n = %d, but wrote %d bytes", n, buf.Len())
			}
		})
	}
}

func TestWriteMergedLines_AfterLine(t *testing.T) {
	twoChunks := []ChunkInfo{
		{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("L1\nL2\nL3\n")},
		{Key: "chunk_00000004_00000006.jsonl", FirstLine: 4, LastLine: 6, Data: []byte("L4\nL5\nL6\n")},
	}
	gapped := []ChunkInfo{
		{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("A\nB\nC\n")},
		{Key: "chunk_00000006_00000008.jsonl", FirstLine: 6, LastLine: 8, Data: []byte("F\nG\nH\n")},
	}
	tests := []struct {
		name      string
		chunks    []ChunkInfo
		afterLine int
		want      string
	}{
		{"zero writes every line", twoChunks, 0, "L1\nL2\nL3\nL4\nL5\nL6\n"},
		{"negative behaves like zero", twoChunks, -1, "L1\nL2\nL3\nL4\nL5\nL6\n"},
		{"mid-chunk offset keeps later lines", twoChunks, 2, "L3\nL4\nL5\nL6\n"},
		{"offset exactly at a chunk boundary", twoChunks, 3, "L4\nL5\nL6\n"},
		{"offset at the last line writes nothing", twoChunks, 6, ""},
		{"offset past the end writes nothing", twoChunks, 100, ""},
		{"offset before the first downloaded chunk keeps all lines", twoChunks[1:2], 2, "L4\nL5\nL6\n"},
		{"offset inside a gap filters by absolute line number", gapped, 4, "F\nG\nH\n"},
		{"offset at the end of a gap", gapped, 5, "F\nG\nH\n"},
		{"offset after the gap", gapped, 6, "G\nH\n"},
		{
			"single chunk with an offset is newline-terminated",
			[]ChunkInfo{{Key: "chunk_00000001_00000003.jsonl", FirstLine: 1, LastLine: 3, Data: []byte("l1\nl2\nl3")}},
			1,
			"l2\nl3\n",
		},
		{"no chunks writes nothing", nil, 3, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := WriteMergedLines(&buf, tc.chunks, tc.afterLine)
			if err != nil {
				t.Fatalf("WriteMergedLines: unexpected error: %v", err)
			}
			if buf.String() != tc.want {
				t.Errorf("WriteMergedLines(afterLine=%d) wrote %q, want %q", tc.afterLine, buf.String(), tc.want)
			}
			if n != int64(len(tc.want)) {
				t.Errorf("WriteMergedLines returned n = %d, want %d", n, len(tc.want))
			}
		})
	}
}

// failAfterWriter accepts up to limit bytes, then fails every write.
type failAfterWriter struct {
	limit   int
	written int
}

var errWriterClosed = errors.New("writer closed")

func (w *failAfterWriter) Write(p []byte) (int, error) {
	room := w.limit - w.written
	if room <= 0 {
		return 0, errWriterClosed
	}
	if len(p) > room {
		w.written += room
		return room, errWriterClosed
	}
	w.written += len(p)
	return len(p), nil
}

func TestWriteMergedLines_PropagatesWriteError(t *testing.T) {
	chunks := []ChunkInfo{
		{Key: "chunk_00000001_00000002.jsonl", FirstLine: 1, LastLine: 2, Data: []byte("aaaa\nbbbb\n")},
		{Key: "chunk_00000003_00000004.jsonl", FirstLine: 3, LastLine: 4, Data: []byte("cccc\ndddd\n")},
	}
	w := &failAfterWriter{limit: 7}
	n, err := WriteMergedLines(w, chunks, 0)
	if !errors.Is(err, errWriterClosed) {
		t.Fatalf("WriteMergedLines error = %v, want the writer's error", err)
	}
	if n != 7 {
		t.Errorf("WriteMergedLines returned n = %d, want the 7 bytes written before the failure", n)
	}

	single := &failAfterWriter{limit: 3}
	n, err = WriteMergedLines(single, chunks[:1], 0)
	if !errors.Is(err, errWriterClosed) {
		t.Fatalf("single-chunk WriteMergedLines error = %v, want the writer's error", err)
	}
	if n != 3 {
		t.Errorf("single-chunk WriteMergedLines returned n = %d, want 3", n)
	}
}

func TestWriteMergedLines_RejectsMaxMergeLinesBeforeWriting(t *testing.T) {
	chunks := []ChunkInfo{
		{Key: "chunk_00000001_00000010.jsonl", FirstLine: 1, LastLine: 10, Data: []byte("a\n")},
		{Key: "chunk_99999990_99999999.jsonl", FirstLine: MaxMergeLines + 1, LastLine: MaxMergeLines + 10, Data: []byte("b\n")},
	}
	if err := ValidateMerge(chunks); err == nil || !strings.Contains(err.Error(), "exceeds safety limit") {
		t.Errorf("ValidateMerge error = %v, want a safety-limit error", err)
	}
	var buf bytes.Buffer
	n, err := WriteMergedLines(&buf, chunks, 0)
	if err == nil || !strings.Contains(err.Error(), "exceeds safety limit") {
		t.Errorf("WriteMergedLines error = %v, want a safety-limit error", err)
	}
	if n != 0 || buf.Len() != 0 {
		t.Errorf("WriteMergedLines wrote %d bytes before rejecting the merge, want none", buf.Len())
	}

	atLimit := []ChunkInfo{{Key: "chunk_00000001_00000001.jsonl", FirstLine: 1, LastLine: MaxMergeLines}}
	if err := ValidateMerge(atLimit); err != nil {
		t.Errorf("ValidateMerge at exactly MaxMergeLines = %v, want nil", err)
	}
	if err := ValidateMerge(nil); err != nil {
		t.Errorf("ValidateMerge(nil) = %v, want nil", err)
	}
}

// TestWriteMergedLines_DoesNotCopyChunkData guards the x4ry memory fix:
// streaming the merge must not materialize a merged copy of the chunks.
// The only allocations allowed are the line index (a slice header per line).
func TestWriteMergedLines_DoesNotCopyChunkData(t *testing.T) {
	const (
		chunkCount    = 64
		linesPerChunk = 128
		lineLen       = 1024
	)
	line := bytes.Repeat([]byte("x"), lineLen-1)
	chunks := make([]ChunkInfo, 0, chunkCount)
	total := 0
	for c := range chunkCount {
		var data []byte
		for range linesPerChunk {
			data = append(data, line...)
			data = append(data, '\n')
		}
		first := c*linesPerChunk + 1
		chunks = append(chunks, ChunkInfo{FirstLine: first, LastLine: first + linesPerChunk - 1, Data: data})
		total += len(data)
	}

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	n, err := WriteMergedLines(io.Discard, chunks, 0)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("WriteMergedLines: unexpected error: %v", err)
	}
	if n != int64(total) {
		t.Fatalf("WriteMergedLines wrote %d bytes, want %d", n, total)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > uint64(total/4) {
		t.Errorf("WriteMergedLines allocated %d bytes for %d bytes of chunks; it must stream without a merged copy (limit %d)",
			allocated, total, total/4)
	}
}
