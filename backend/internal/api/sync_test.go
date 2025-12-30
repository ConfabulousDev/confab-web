package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMergeChunks(t *testing.T) {
	t.Run("single chunk returns as-is", func(t *testing.T) {
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("line1\nline2\nline3\n")},
		}

		result, err := mergeChunks(chunks)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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

		result, err := mergeChunks(chunks)
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
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("old1\nold2\nold3\nold4\nold5\n")},
			{key: "chunk_00000001_00000010.jsonl", firstLine: 1, lastLine: 10, data: []byte("new1\nnew2\nnew3\nnew4\nnew5\nnew6\nnew7\nnew8\nnew9\nnew10\n")},
		}

		result, err := mergeChunks(chunks)
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
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("A1\nA2\nA3\nA4\nA5\n")},
			{key: "chunk_00000003_00000010.jsonl", firstLine: 3, lastLine: 10, data: []byte("B3\nB4\nB5\nB6\nB7\nB8\nB9\nB10\n")},
		}

		result, err := mergeChunks(chunks)
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
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("X1\nX2\nX3\n")},
			{key: "chunk_00000001_00000005.jsonl", firstLine: 1, lastLine: 5, data: []byte("Y1\nY2\nY3\nY4\nY5\n")},
			{key: "chunk_00000004_00000010.jsonl", firstLine: 4, lastLine: 10, data: []byte("Z4\nZ5\nZ6\nZ7\nZ8\nZ9\nZ10\n")},
		}

		result, err := mergeChunks(chunks)
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
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000003.jsonl", firstLine: 1, lastLine: 3, data: []byte("A\nB\nC\n")},
			{key: "chunk_00000006_00000008.jsonl", firstLine: 6, lastLine: 8, data: []byte("F\nG\nH\n")},
		}

		result, err := mergeChunks(chunks)
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
		result, err := mergeChunks(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil, got %q", string(result))
		}
	})

	t.Run("chunk with no trailing newline", func(t *testing.T) {
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000002.jsonl", firstLine: 1, lastLine: 2, data: []byte("line1\nline2")}, // no trailing newline
		}

		result, err := mergeChunks(chunks)
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
		chunks := []chunkInfo{
			{key: "chunk_00000001_00000010.jsonl", firstLine: 1, lastLine: 10, data: []byte("a\n")},
			{key: "chunk_99999990_99999999.jsonl", firstLine: MaxMergeLines + 1, lastLine: MaxMergeLines + 10, data: []byte("b\n")},
		}

		_, err := mergeChunks(chunks)
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

// mockStorage implements chunkDownloader for testing
type mockStorage struct {
	chunks       map[string][]byte
	downloadFunc func(key string) ([]byte, error) // optional custom behavior
	callCount    atomic.Int32
}

func (m *mockStorage) Download(ctx context.Context, key string) ([]byte, error) {
	m.callCount.Add(1)
	if m.downloadFunc != nil {
		return m.downloadFunc(key)
	}
	if data, ok := m.chunks[key]; ok {
		return data, nil
	}
	return nil, errors.New("not found")
}

func TestDownloadChunks(t *testing.T) {
	t.Run("downloads chunks in parallel and preserves order", func(t *testing.T) {
		storage := &mockStorage{
			chunks: map[string][]byte{
				"chunk_00000001_00000002.jsonl": []byte("line1\nline2\n"),
				"chunk_00000003_00000004.jsonl": []byte("line3\nline4\n"),
				"chunk_00000005_00000006.jsonl": []byte("line5\nline6\n"),
			},
		}

		keys := []string{
			"chunk_00000001_00000002.jsonl",
			"chunk_00000003_00000004.jsonl",
			"chunk_00000005_00000006.jsonl",
		}

		chunks, err := downloadChunks(context.Background(), storage, keys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(chunks) != 3 {
			t.Fatalf("expected 3 chunks, got %d", len(chunks))
		}

		// Verify order is preserved
		if chunks[0].firstLine != 1 || chunks[0].lastLine != 2 {
			t.Errorf("chunk 0: expected lines 1-2, got %d-%d", chunks[0].firstLine, chunks[0].lastLine)
		}
		if chunks[1].firstLine != 3 || chunks[1].lastLine != 4 {
			t.Errorf("chunk 1: expected lines 3-4, got %d-%d", chunks[1].firstLine, chunks[1].lastLine)
		}
		if chunks[2].firstLine != 5 || chunks[2].lastLine != 6 {
			t.Errorf("chunk 2: expected lines 5-6, got %d-%d", chunks[2].firstLine, chunks[2].lastLine)
		}
	})

	t.Run("skips unparseable keys", func(t *testing.T) {
		storage := &mockStorage{
			chunks: map[string][]byte{
				"chunk_00000001_00000002.jsonl": []byte("line1\nline2\n"),
			},
		}

		keys := []string{
			"invalid_key.jsonl",
			"chunk_00000001_00000002.jsonl",
			"another_bad_key",
		}

		chunks, err := downloadChunks(context.Background(), storage, keys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(chunks) != 1 {
			t.Fatalf("expected 1 chunk, got %d", len(chunks))
		}
	})

	t.Run("returns error on download failure", func(t *testing.T) {
		storage := &mockStorage{
			downloadFunc: func(key string) ([]byte, error) {
				return nil, errors.New("download failed")
			},
		}

		keys := []string{"chunk_00000001_00000002.jsonl"}

		_, err := downloadChunks(context.Background(), storage, keys)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("returns nil for empty keys", func(t *testing.T) {
		storage := &mockStorage{}

		chunks, err := downloadChunks(context.Background(), storage, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chunks != nil {
			t.Errorf("expected nil, got %v", chunks)
		}
	})

	t.Run("returns nil for all unparseable keys", func(t *testing.T) {
		storage := &mockStorage{}

		keys := []string{"bad1.jsonl", "bad2.jsonl"}

		chunks, err := downloadChunks(context.Background(), storage, keys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chunks != nil {
			t.Errorf("expected nil, got %v", chunks)
		}
	})

	t.Run("large batch exceeding maxParallelDownloads", func(t *testing.T) {
		// Create 12 chunks (more than maxParallelDownloads=5)
		chunkData := make(map[string][]byte)
		keys := make([]string, 12)
		for i := 0; i < 12; i++ {
			first := i*10 + 1
			last := first + 9
			key := fmt.Sprintf("chunk_%08d_%08d.jsonl", first, last)
			keys[i] = key
			chunkData[key] = []byte(fmt.Sprintf("data for chunk %d\n", i))
		}

		storage := &mockStorage{chunks: chunkData}

		chunks, err := downloadChunks(context.Background(), storage, keys)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(chunks) != 12 {
			t.Fatalf("expected 12 chunks, got %d", len(chunks))
		}

		// Verify all chunks downloaded in correct order
		for i, chunk := range chunks {
			expectedFirst := i*10 + 1
			expectedLast := expectedFirst + 9
			if chunk.firstLine != expectedFirst || chunk.lastLine != expectedLast {
				t.Errorf("chunk %d: expected lines %d-%d, got %d-%d",
					i, expectedFirst, expectedLast, chunk.firstLine, chunk.lastLine)
			}
		}

		// Verify all downloads were called
		if storage.callCount.Load() != 12 {
			t.Errorf("expected 12 download calls, got %d", storage.callCount.Load())
		}
	})

	t.Run("partial failures - returns first error", func(t *testing.T) {
		downloadErr := errors.New("chunk 2 failed")
		callCount := atomic.Int32{}

		storage := &mockStorage{
			downloadFunc: func(key string) ([]byte, error) {
				callCount.Add(1)
				// Fail on the second chunk
				if strings.Contains(key, "00000003_00000004") {
					return nil, downloadErr
				}
				return []byte("data\n"), nil
			},
		}

		keys := []string{
			"chunk_00000001_00000002.jsonl",
			"chunk_00000003_00000004.jsonl",
			"chunk_00000005_00000006.jsonl",
		}

		_, err := downloadChunks(context.Background(), storage, keys)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != downloadErr.Error() {
			t.Errorf("expected error %q, got %q", downloadErr.Error(), err.Error())
		}

		// All downloads should still be attempted (we don't cancel on first error)
		if callCount.Load() != 3 {
			t.Errorf("expected 3 download attempts, got %d", callCount.Load())
		}
	})
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
