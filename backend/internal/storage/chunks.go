package storage

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	index    int
	chunk    ChunkInfo
	err      error
	duration time.Duration
}

// ParseChunkKey extracts line numbers from a chunk S3 key.
// Key format: .../chunk_00000001_00000100.jsonl
// Returns (firstLine, lastLine, ok).
func ParseChunkKey(key string) (int, int, bool) {
	parts := strings.Split(key, "/")
	if len(parts) == 0 {
		return 0, 0, false
	}
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
func (s *S3Storage) DownloadAndMergeChunks(ctx context.Context, userID int64, externalID, fileName string) ([]byte, error) {
	ctx, span := tracer.Start(ctx, "storage.download_and_merge_chunks",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("session.external_id", externalID),
			attribute.String("file.name", fileName),
		))
	defer span.End()

	chunkKeys, err := s.ListChunks(ctx, userID, externalID, fileName)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	chunks, err := s.DownloadChunks(ctx, chunkKeys)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	merged, err := MergeChunks(chunks)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("chunks.count", len(chunks)),
		attribute.Int("merged.bytes", len(merged)),
	)

	return merged, nil
}

// DownloadChunks downloads all chunks for the given keys in parallel and returns them as ChunkInfo slices.
// Keys with unparseable names are skipped with a warning.
// Downloads are limited to maxParallelDownloads concurrent operations.
func (s *S3Storage) DownloadChunks(ctx context.Context, chunkKeys []string) ([]ChunkInfo, error) {
	ctx, span := tracer.Start(ctx, "storage.download_chunks",
		trace.WithAttributes(attribute.Int("keys.count", len(chunkKeys))))
	defer span.End()

	if len(chunkKeys) == 0 {
		return nil, nil
	}

	// Parse all keys first to filter out invalid ones
	type keyInfo struct {
		key       string
		firstLine int
		lastLine  int
		index     int
	}
	validKeys := make([]keyInfo, 0, len(chunkKeys))
	for i, key := range chunkKeys {
		firstLine, lastLine, ok := ParseChunkKey(key)
		if !ok {
			span.AddEvent("skipped_unparseable_key", trace.WithAttributes(attribute.String("key", key)))
			continue
		}
		validKeys = append(validKeys, keyInfo{key: key, firstLine: firstLine, lastLine: lastLine, index: i})
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

			start := time.Now()
			data, err := s.Download(ctx, ki.key)
			elapsed := time.Since(start)

			if err != nil {
				results <- chunkResult{index: idx, err: err, duration: elapsed}
				return
			}

			results <- chunkResult{
				index:    idx,
				duration: elapsed,
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
	var maxDuration time.Duration
	var sumDuration time.Duration

	for range validKeys {
		result := <-results
		sumDuration += result.duration
		if result.duration > maxDuration {
			maxDuration = result.duration
		}
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		chunks[result.index] = result.chunk
	}

	span.SetAttributes(
		attribute.Int("valid_keys.count", len(validKeys)),
		attribute.Int64("max_duration_ms", maxDuration.Milliseconds()),
		attribute.Int64("sum_duration_ms", sumDuration.Milliseconds()),
	)

	if firstErr != nil {
		span.RecordError(firstErr)
		span.SetStatus(codes.Error, firstErr.Error())
		return nil, firstErr
	}

	return chunks, nil
}

// MergeChunks takes downloaded chunks and merges them, handling overlaps.
// Uses a simple array indexed by line number - each chunk's lines are written
// to the array, and later chunks overwrite earlier ones for the same line.
// The final array is then concatenated into the result.
//
// Returns an error if maxLine exceeds MaxMergeLines to prevent memory exhaustion.
func MergeChunks(chunks []ChunkInfo) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	if len(chunks) == 1 {
		return chunks[0].Data, nil
	}

	// Find max line number
	maxLine := 0
	for _, c := range chunks {
		if c.LastLine > maxLine {
			maxLine = c.LastLine
		}
	}

	// Safety check: prevent memory exhaustion from corrupted data
	if maxLine > MaxMergeLines {
		return nil, fmt.Errorf("maxLine %d exceeds safety limit %d", maxLine, MaxMergeLines)
	}

	// Log warning for unusually large merges
	if maxLine > LargeMergeWarningThreshold {
		slog.Warn("Large chunk merge operation",
			"max_line", maxLine,
			"chunk_count", len(chunks),
			"threshold", LargeMergeWarningThreshold)
	}

	// Build array indexed by line number (0-indexed, so line 1 is at index 0)
	lines := make([][]byte, maxLine)

	// Populate array from each chunk (last write wins)
	for _, c := range chunks {
		chunkLines := splitLines(c.Data)
		for i, line := range chunkLines {
			lineNum := c.FirstLine + i // 1-based line number
			if lineNum >= 1 && lineNum <= maxLine {
				idx := lineNum - 1
				// Check for conflicting content on overlap
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
	}

	// Build result from array
	var result []byte
	for _, line := range lines {
		if line != nil {
			result = append(result, line...)
			result = append(result, '\n')
		}
	}

	return result, nil
}

// splitLines splits data into lines, preserving each line's content without the newline.
func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
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

// MergeChunksSimple merges chunks by simple concatenation (no line-based deduplication).
// Use this only when chunks are guaranteed to be sequential and non-overlapping.
// For general use, prefer MergeChunks which handles overlaps correctly.
func MergeChunksSimple(chunks [][]byte) []byte {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return chunks[0]
	}

	totalSize := 0
	for _, chunk := range chunks {
		totalSize += len(chunk)
	}

	result := make([]byte, 0, totalSize)
	for _, chunk := range chunks {
		result = append(result, chunk...)
	}

	return result
}

// ChunksOverlap checks if any chunks have overlapping line ranges.
// This can be used to decide whether to use MergeChunks (handles overlaps) or MergeChunksSimple.
func ChunksOverlap(chunks []ChunkInfo) bool {
	if len(chunks) <= 1 {
		return false
	}

	// Check each pair for overlap
	for i := 0; i < len(chunks); i++ {
		for j := i + 1; j < len(chunks); j++ {
			// Chunks overlap if one starts before the other ends
			if chunks[i].FirstLine <= chunks[j].LastLine && chunks[j].FirstLine <= chunks[i].LastLine {
				return true
			}
		}
	}
	return false
}
