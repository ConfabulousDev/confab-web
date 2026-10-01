package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// ChunkInfo holds parsed chunk metadata and content.
type ChunkInfo struct {
	Key       string
	FirstLine int
	LastLine  int
	Data      []byte
}

// MaxMergeLines is the maximum number of lines allowed in a merge operation.
// This prevents memory exhaustion from corrupted chunk filenames.
// Normal operation limit: 30,000 chunks × 100 lines = 3M lines.
// We set 10M as a generous safety margin.
const MaxMergeLines = 10_000_000

// LargeMergeWarningThreshold is the line count at which we log a warning.
const LargeMergeWarningThreshold = 1_000_000

// maxParallelDownloads limits concurrent chunk downloads to avoid overwhelming S3.
const maxParallelDownloads = 10

// chunkResult holds the result of a parallel chunk download.
type chunkResult struct {
	index int
	chunk ChunkInfo
	err   error
}

// ParseChunkKey extracts line numbers from a chunk S3 key.
// Key format: .../chunk_00000001_00000100.jsonl
// Returns (firstLine, lastLine, ok).
func ParseChunkKey(key string) (int, int, bool) {
	parts := strings.Split(key, "/")
	filename := parts[len(parts)-1]
	if !strings.HasPrefix(filename, "chunk_") || !strings.HasSuffix(filename, ".jsonl") {
		return 0, 0, false
	}

	// Extract line numbers
	// chunk_00000001_00000100.jsonl -> 00000001_00000100
	middle := strings.TrimPrefix(filename, "chunk_")
	middle = strings.TrimSuffix(middle, ".jsonl")

	var first, last int
	_, err := fmt.Sscanf(middle, "%08d_%08d", &first, &last)
	if err != nil {
		return 0, 0, false
	}

	return first, last, true
}

// DownloadAndMergeChunks downloads all chunks for a file and merges them into a single byte slice.
// This is a convenience method that combines ListChunks, DownloadChunks, and MergeChunks.
// Returns nil if no chunks exist (not an error).
func (s *S3Storage) DownloadAndMergeChunks(ctx context.Context, userID int64, provider string, externalID, fileName string) ([]byte, error) {
	chunkKeys, err := s.ListChunks(ctx, userID, provider, externalID, fileName)
	if err != nil {
		return nil, err
	}
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	chunks, err := s.DownloadChunks(ctx, chunkKeys)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	return MergeChunks(chunks)
}

// DownloadChunks downloads all chunks for the given keys in parallel and returns them as ChunkInfo slices.
// Keys with unparseable names are skipped with a warning.
// Downloads are limited to maxParallelDownloads concurrent operations.
func (s *S3Storage) DownloadChunks(ctx context.Context, chunkKeys []string) ([]ChunkInfo, error) {
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	// Parse all keys first to filter out invalid ones
	type keyInfo struct {
		key       string
		firstLine int
		lastLine  int
	}
	validKeys := make([]keyInfo, 0, len(chunkKeys))
	for _, key := range chunkKeys {
		firstLine, lastLine, ok := ParseChunkKey(key)
		if !ok {
			slog.Warn("Skipping unparseable chunk key", "key", key)
			continue
		}
		validKeys = append(validKeys, keyInfo{key: key, firstLine: firstLine, lastLine: lastLine})
	}

	if len(validKeys) == 0 {
		return nil, nil
	}

	// Use a semaphore pattern for bounded parallelism
	results := make(chan chunkResult, len(validKeys))
	sem := make(chan struct{}, maxParallelDownloads)

	// Launch download goroutines
	for i, ki := range validKeys {
		go func(idx int, ki keyInfo) {
			sem <- struct{}{}        // acquire semaphore
			defer func() { <-sem }() // release semaphore

			data, err := s.Download(ctx, ki.key)
			if err != nil {
				results <- chunkResult{index: idx, err: err}
				return
			}

			results <- chunkResult{
				index: idx,
				chunk: ChunkInfo{
					Key:       ki.key,
					FirstLine: ki.firstLine,
					LastLine:  ki.lastLine,
					Data:      data,
				},
			}
		}(i, ki)
	}

	// Collect results
	chunks := make([]ChunkInfo, len(validKeys))
	var firstErr error

	for range validKeys {
		result := <-results
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		chunks[result.index] = result.chunk
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return chunks, nil
}

// MergeChunks takes downloaded chunks and merges them into one byte slice,
// handling overlaps (last write wins, see buildLineIndex). A single chunk is
// returned as-is; no chunks or no lines return nil. Use WriteMergedLines
// instead when the result is only written out, to avoid the merged copy.
//
// Returns an error if maxLine exceeds MaxMergeLines to prevent memory exhaustion.
func MergeChunks(chunks []ChunkInfo) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0].Data, nil
	}

	lines, err := buildLineIndex(chunks)
	if err != nil {
		return nil, err
	}

	// Size the result exactly before building it, so the buffer never regrows
	// and copies (overlapping chunks make summed chunk sizes an overestimate).
	size := 0
	for _, line := range lines {
		if line != nil {
			size += len(line) + 1
		}
	}
	if size == 0 {
		return nil, nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, size))
	if _, err := writeLines(buf, lines, 0); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteMergedLines streams the merge of chunks to w without materializing it:
// every line whose 1-based line number is greater than afterLine, in line
// order, each followed by '\n'. Lines are written straight from chunk Data.
// For a single chunk with afterLine <= 0, its Data is written verbatim (the
// same bytes MergeChunks returns). Returns the bytes written and the first
// write error.
//
// It returns ValidateMerge's error before writing anything, so a caller can
// call ValidateMerge first to rule out errors other than write errors.
func WriteMergedLines(w io.Writer, chunks []ChunkInfo, afterLine int) (int64, error) {
	if len(chunks) == 1 && afterLine <= 0 {
		if err := ValidateMerge(chunks); err != nil {
			return 0, err
		}
		n, err := w.Write(chunks[0].Data)
		return int64(n), err
	}
	lines, err := buildLineIndex(chunks)
	if err != nil {
		return 0, err
	}
	return writeLines(w, lines, afterLine)
}

// ValidateMerge reports whether chunks exceed the MaxMergeLines safety limit,
// without reading their data.
func ValidateMerge(chunks []ChunkInfo) error {
	_, err := mergeMaxLine(chunks)
	return err
}

// mergeMaxLine returns the highest line number across chunks, or an error if
// it exceeds MaxMergeLines (corrupted chunk keys would otherwise size the line
// index arbitrarily large).
func mergeMaxLine(chunks []ChunkInfo) (int, error) {
	maxLine := 0
	for _, c := range chunks {
		maxLine = max(maxLine, c.LastLine)
	}
	if maxLine > MaxMergeLines {
		return 0, fmt.Errorf("maxLine %d exceeds safety limit %d", maxLine, MaxMergeLines)
	}
	return maxLine, nil
}

// buildLineIndex indexes chunk lines by line number (line 1 at index 0).
// Chunks are applied in slice order, so a later chunk overwrites an earlier
// one on overlap (last write wins). Entries are slices into chunk Data, not
// copies; nil marks a line no chunk covers.
func buildLineIndex(chunks []ChunkInfo) ([][]byte, error) {
	maxLine, err := mergeMaxLine(chunks)
	if err != nil {
		return nil, err
	}
	if maxLine > LargeMergeWarningThreshold {
		slog.Warn("Large chunk merge operation",
			"max_line", maxLine,
			"chunk_count", len(chunks),
			"threshold", LargeMergeWarningThreshold)
	}

	lines := make([][]byte, maxLine)
	for _, c := range chunks {
		for i, line := range splitLines(c.Data) {
			lineNum := c.FirstLine + i // 1-based line number
			if lineNum < 1 || lineNum > maxLine {
				continue
			}
			idx := lineNum - 1
			if lines[idx] != nil && !bytes.Equal(lines[idx], line) {
				slog.Warn("Chunk overlap with differing content",
					"line_num", lineNum,
					"chunk", c.Key,
					"old_len", len(lines[idx]),
					"new_len", len(line))
			}
			lines[idx] = line
		}
	}
	return lines, nil
}

var newline = []byte{'\n'}

// writeLines writes each indexed line numbered above afterLine, followed by
// '\n', and returns the bytes written and the first write error.
func writeLines(w io.Writer, lines [][]byte, afterLine int) (int64, error) {
	start := min(max(afterLine, 0), len(lines)) // index of line afterLine+1
	var total int64
	for _, line := range lines[start:] {
		if line == nil {
			continue
		}
		n, err := w.Write(line)
		total += int64(n)
		if err != nil {
			return total, err
		}
		n, err = w.Write(newline)
		total += int64(n)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// splitLines splits data into lines, preserving each line's content without the newline.
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var lines [][]byte
	start := 0
	for i := range data {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	// Handle last line if no trailing newline
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
