package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// ============================================================================
// Request/Response Types
// ============================================================================

// SyncInitMetadata contains optional metadata for session initialization.
// This groups session metadata consistently with the chunk API's metadata field.
type SyncInitMetadata struct {
	CWD      string          `json:"cwd,omitempty"`
	GitInfo  json.RawMessage `json:"git_info,omitempty"`
	Hostname string          `json:"hostname,omitempty"`
	Username string          `json:"username,omitempty"`
}

// SyncInitRequest is the request body for POST /api/v1/sync/init
type SyncInitRequest struct {
	ExternalID     string            `json:"external_id"`
	TranscriptPath string            `json:"transcript_path"`
	Metadata       *SyncInitMetadata `json:"metadata,omitempty"`

	// ===========================================================================
	// DEPRECATED: The following top-level fields are deprecated.
	// Use the nested Metadata struct instead for consistency with the chunk API.
	// These fields are kept for backward compatibility with older CLI versions.
	// When both are provided, Metadata takes precedence.
	// ===========================================================================

	// Deprecated: Use Metadata.CWD instead.
	CWD string `json:"cwd,omitempty"`
	// Deprecated: Use Metadata.GitInfo instead.
	GitInfo json.RawMessage `json:"git_info,omitempty"`
}

// SyncInitResponse is the response for POST /api/v1/sync/init
type SyncInitResponse struct {
	SessionID string                        `json:"session_id"`
	Files     map[string]SyncFileStateResp `json:"files"`
}

// SyncFileStateResp represents the sync state for a single file in API responses
type SyncFileStateResp struct {
	LastSyncedLine int `json:"last_synced_line"`
}

// SyncChunkMetadata contains optional mutable metadata that can be updated with each chunk
// This allows metadata like git info to be updated throughout the session lifecycle,
// rather than only at init time.
type SyncChunkMetadata struct {
	GitInfo          json.RawMessage `json:"git_info,omitempty"`           // Git metadata (repo_url, branch, etc.)
	Summary          *string         `json:"summary,omitempty"`            // First summary from transcript
	FirstUserMessage *string         `json:"first_user_message,omitempty"` // First user message
}

// SyncChunkRequest is the request body for POST /api/v1/sync/chunk
type SyncChunkRequest struct {
	SessionID string             `json:"session_id"`
	FileName  string             `json:"file_name"`
	FileType  string             `json:"file_type"`
	FirstLine int                `json:"first_line"`
	Lines     []string           `json:"lines"`
	Metadata  *SyncChunkMetadata `json:"metadata,omitempty"` // Optional: mutable session metadata (git_info, summary, first_user_message)
}

// SyncChunkResponse is the response for POST /api/v1/sync/chunk
type SyncChunkResponse struct {
	LastSyncedLine int `json:"last_synced_line"`
}

// SyncEventRequest is the request body for POST /api/v1/sync/event
type SyncEventRequest struct {
	SessionID string          `json:"session_id"`
	EventType string          `json:"event_type"` // "session_end"
	Timestamp time.Time       `json:"timestamp"`  // When the event occurred
	Payload   json.RawMessage `json:"payload"`    // Full event payload (e.g., HookInput)
}

// SyncEventResponse is the response for POST /api/v1/sync/event
type SyncEventResponse struct {
	Success bool `json:"success"`
}

// ============================================================================
// Handlers
// ============================================================================

// handleSyncInit initializes or resumes a sync session
// POST /api/v1/sync/init
func (s *Server) handleSyncInit(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse request
	var req SyncInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.ExternalID == "" {
		respondError(w, http.StatusBadRequest, "external_id is required")
		return
	}
	if req.TranscriptPath == "" {
		respondError(w, http.StatusBadRequest, "transcript_path is required")
		return
	}

	// Validate field lengths
	if err := validation.ValidateExternalID(req.ExternalID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validation.ValidateTranscriptPath(req.TranscriptPath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Extract metadata, preferring new nested format over deprecated top-level fields
	cwd := req.CWD
	gitInfo := req.GitInfo
	var hostname, username string
	if req.Metadata != nil {
		if req.Metadata.CWD != "" {
			cwd = req.Metadata.CWD
		}
		if req.Metadata.GitInfo != nil {
			gitInfo = req.Metadata.GitInfo
		}
		hostname = req.Metadata.Hostname
		username = req.Metadata.Username
	}

	// Validate cwd regardless of which field it came from
	if err := validation.ValidateCWD(cwd); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate hostname and username
	if err := validation.ValidateHostname(hostname); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validation.ValidateUsername(username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Find or create session
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	params := db.SyncSessionParams{
		ExternalID:     req.ExternalID,
		TranscriptPath: req.TranscriptPath,
		CWD:            cwd,
		GitInfo:        gitInfo,
		Hostname:       hostname,
		Username:       username,
	}
	sessionID, files, err := s.db.FindOrCreateSyncSession(ctx, userID, params)
	if err != nil {
		logger.Error("Failed to find/create sync session", "error", err, "user_id", userID, "external_id", req.ExternalID)
		respondError(w, http.StatusInternalServerError, "Failed to initialize sync session")
		return
	}

	// Convert to response format
	respFiles := make(map[string]SyncFileStateResp)
	for fileName, state := range files {
		respFiles[fileName] = SyncFileStateResp{
			LastSyncedLine: state.LastSyncedLine,
		}
	}

	logger.Info("Sync session initialized",
		"user_id", userID,
		"session_id", sessionID,
		"external_id", req.ExternalID,
		"file_count", len(files))

	respondJSON(w, http.StatusOK, SyncInitResponse{
		SessionID: sessionID,
		Files:     respFiles,
	})
}

// handleSyncChunk uploads a chunk of lines for a file
// POST /api/v1/sync/chunk
func (s *Server) handleSyncChunk(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse request
	var req SyncChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.FileName == "" {
		respondError(w, http.StatusBadRequest, "file_name is required")
		return
	}
	if req.FileType == "" {
		respondError(w, http.StatusBadRequest, "file_type is required")
		return
	}
	if req.FileType == "todo" {
		respondError(w, http.StatusBadRequest, "todo file sync is no longer supported")
		return
	}
	if req.FirstLine < 1 {
		respondError(w, http.StatusBadRequest, "first_line must be >= 1")
		return
	}
	if len(req.Lines) == 0 {
		respondError(w, http.StatusBadRequest, "lines array cannot be empty")
		return
	}

	// Validate field lengths
	// TODO(2026-Q2): Remove grace logic for user 9 - truncate instead of reject
	if userID == 9 {
		req.FileName = validation.TruncateSyncFileName(req.FileName)
		if req.Metadata != nil {
			if req.Metadata.Summary != nil {
				*req.Metadata.Summary = validation.TruncateSummary(*req.Metadata.Summary)
			}
			if req.Metadata.FirstUserMessage != nil {
				*req.Metadata.FirstUserMessage = validation.TruncateFirstUserMessage(*req.Metadata.FirstUserMessage)
			}
		}
	} else {
		if err := validation.ValidateSyncFileName(req.FileName); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Metadata != nil {
			if req.Metadata.Summary != nil {
				if err := validation.ValidateSummary(*req.Metadata.Summary); err != nil {
					respondError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
			if req.Metadata.FirstUserMessage != nil {
				if err := validation.ValidateFirstUserMessage(*req.Metadata.FirstUserMessage); err != nil {
					respondError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
		}
	}

	// Verify session ownership and get external_id (needed for S3 key)
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	externalID, err := s.db.VerifySessionOwnership(dbCtx, req.SessionID, userID)
	if err != nil {
		if errors.Is(err, db.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		if errors.Is(err, db.ErrForbidden) {
			respondError(w, http.StatusForbidden, "Access denied")
			return
		}
		logger.Error("Failed to verify session ownership", "error", err, "session_id", req.SessionID)
		respondError(w, http.StatusInternalServerError, "Failed to verify session")
		return
	}

	// Get current sync state to validate chunk continuity
	syncState, err := s.db.GetSyncFileState(dbCtx, req.SessionID, req.FileName)
	expectedFirstLine := 1
	if err == nil {
		// File exists - next chunk must continue from where we left off
		expectedFirstLine = syncState.LastSyncedLine + 1
	} else if !errors.Is(err, db.ErrFileNotFound) {
		logger.Error("Failed to get sync state", "error", err, "session_id", req.SessionID, "file_name", req.FileName)
		respondError(w, http.StatusInternalServerError, "Failed to get sync state")
		return
	}
	// ErrFileNotFound is fine - it's a new file, expectedFirstLine stays 1

	// Validate chunk continuity (no gaps, no overlaps)
	if req.FirstLine != expectedFirstLine {
		logger.Warn("Chunk continuity error",
			"session_id", req.SessionID,
			"file_name", req.FileName,
			"expected_first_line", expectedFirstLine,
			"actual_first_line", req.FirstLine)
		respondError(w, http.StatusBadRequest,
			fmt.Sprintf("first_line must be %d (got %d) - chunks must be contiguous", expectedFirstLine, req.FirstLine))
		return
	}

	// Soft limit check on chunk count (if known)
	// This is a soft limit - races may allow slightly exceeding it, but reads will self-heal
	if syncState != nil && syncState.ChunkCount != nil && *syncState.ChunkCount >= storage.MaxChunksPerFile {
		logger.Warn("Chunk limit exceeded",
			"session_id", req.SessionID,
			"file_name", req.FileName,
			"chunk_count", *syncState.ChunkCount,
			"limit", storage.MaxChunksPerFile)
		respondError(w, http.StatusBadRequest,
			fmt.Sprintf("File has too many chunks (limit: %d). Consider starting a new session.", storage.MaxChunksPerFile))
		return
	}

	// Build chunk content (lines joined by newlines, with trailing newline)
	// Also extract timestamp metadata from transcript lines
	var content bytes.Buffer
	var latestTimestamp *time.Time
	for _, line := range req.Lines {
		content.WriteString(line)
		content.WriteString("\n")

		// Try to extract timestamp from transcript lines
		if req.FileType == "transcript" {
			if ts := extractTimestampFromLine(line); ts != nil {
				if latestTimestamp == nil || ts.After(*latestTimestamp) {
					latestTimestamp = ts
				}
			}
		}
	}

	// Calculate last line number
	lastLine := req.FirstLine + len(req.Lines) - 1

	// Upload chunk to S3
	storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
	defer storageCancel()

	s3Key, err := s.storage.UploadChunk(storageCtx, userID, externalID, req.FileName, req.FirstLine, lastLine, content.Bytes())
	if err != nil {
		logger.Error("Failed to upload chunk",
			"error", err,
			"user_id", userID,
			"session_id", req.SessionID,
			"file_name", req.FileName,
			"first_line", req.FirstLine,
			"last_line", lastLine)
		respondStorageError(w, err, "Failed to upload chunk")
		return
	}

	// Update sync state in DB (includes session's last_message_at if we found timestamps)
	updateCtx, updateCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer updateCancel()

	// Extract metadata fields if present and this is a transcript file
	// Only process metadata for transcript files, not agent/todo files
	var gitInfo json.RawMessage
	var summary, firstUserMessage *string
	if req.Metadata != nil && req.FileType == "transcript" {
		gitInfo = req.Metadata.GitInfo
		summary = req.Metadata.Summary
		firstUserMessage = req.Metadata.FirstUserMessage
	}

	if err := s.db.UpdateSyncFileState(updateCtx, req.SessionID, req.FileName, req.FileType, lastLine, latestTimestamp, summary, firstUserMessage, gitInfo); err != nil {
		logger.Error("Failed to update sync state",
			"error", err,
			"session_id", req.SessionID,
			"file_name", req.FileName,
			"last_line", lastLine)
		// Note: S3 chunk was already uploaded - consider this a partial success
		// The next sync will detect the mismatch and can retry
		respondError(w, http.StatusInternalServerError, "Failed to update sync state")
		return
	}

	logger.Debug("Chunk uploaded",
		"user_id", userID,
		"session_id", req.SessionID,
		"file_name", req.FileName,
		"first_line", req.FirstLine,
		"last_line", lastLine,
		"s3_key", s3Key)

	respondJSON(w, http.StatusOK, SyncChunkResponse{
		LastSyncedLine: lastLine,
	})
}

// filterLinesAfterOffset removes lines at or before the given offset.
// The firstLineNum parameter indicates what transcript line the content starts at.
// For example, if content is lines 4,5,6 of the transcript (firstLineNum=4) and
// offset=3, all lines are kept (4,5,6 are all > 3). If offset=5, only line 6 is kept.
func filterLinesAfterOffset(content []byte, offset int, firstLineNum int) []byte {
	if offset <= 0 {
		return content
	}

	// If all content is after offset, return everything
	if offset < firstLineNum {
		return content
	}

	lines := bytes.Split(content, []byte("\n"))

	// Calculate how many lines to skip from the beginning
	// Line at index i corresponds to transcript line (firstLineNum + i)
	// We want lines where (firstLineNum + i) > offset
	// So: i > offset - firstLineNum
	// So: i >= offset - firstLineNum + 1
	startIndex := offset - firstLineNum + 1

	if startIndex >= len(lines) {
		return nil
	}

	// Skip lines before startIndex
	remaining := lines[startIndex:]

	// Filter out empty trailing lines that result from split
	for len(remaining) > 0 && len(remaining[len(remaining)-1]) == 0 {
		remaining = remaining[:len(remaining)-1]
	}

	if len(remaining) == 0 {
		return nil
	}

	return bytes.Join(remaining, []byte("\n"))
}

// handleSyncEvent records a session lifecycle event
// POST /api/v1/sync/event
func (s *Server) handleSyncEvent(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse request
	var req SyncEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	// Validate required fields
	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.EventType == "" {
		respondError(w, http.StatusBadRequest, "event_type is required")
		return
	}
	if req.EventType != "session_end" {
		respondError(w, http.StatusBadRequest, "invalid event_type: must be 'session_end'")
		return
	}
	if req.Timestamp.IsZero() {
		respondError(w, http.StatusBadRequest, "timestamp is required")
		return
	}

	// Verify session ownership
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	_, err := s.db.VerifySessionOwnership(dbCtx, req.SessionID, userID)
	if err != nil {
		if errors.Is(err, db.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		if errors.Is(err, db.ErrForbidden) {
			respondError(w, http.StatusForbidden, "Access denied")
			return
		}
		logger.Error("Failed to verify session ownership", "error", err, "session_id", req.SessionID)
		respondError(w, http.StatusInternalServerError, "Failed to verify session")
		return
	}

	// Insert event
	err = s.db.InsertSessionEvent(dbCtx, db.SessionEventParams{
		SessionID:      req.SessionID,
		EventType:      req.EventType,
		EventTimestamp: req.Timestamp,
		Payload:        req.Payload,
	})
	if err != nil {
		logger.Error("Failed to insert session event", "error", err, "session_id", req.SessionID, "event_type", req.EventType)
		respondError(w, http.StatusInternalServerError, "Failed to record event")
		return
	}

	logger.Info("Session event recorded",
		"user_id", userID,
		"session_id", req.SessionID,
		"event_type", req.EventType,
		"timestamp", req.Timestamp)

	respondJSON(w, http.StatusOK, SyncEventResponse{Success: true})
}

// ============================================================================
// Helpers
// ============================================================================

// buildChunkS3Key constructs the S3 key for a chunk file
// Format: {user_id}/claude-code/{external_id}/chunks/{file_name}/chunk_{first:08d}_{last:08d}.jsonl
func buildChunkS3Key(userID int64, externalID, fileName string, firstLine, lastLine int) string {
	return fmt.Sprintf("%d/claude-code/%s/chunks/%s/chunk_%08d_%08d.jsonl",
		userID, externalID, fileName, firstLine, lastLine)
}

// parseChunkKey extracts line numbers from a chunk S3 key
// Returns (firstLine, lastLine, ok)
func parseChunkKey(key string) (int, int, bool) {
	// Key format: .../chunk_00000001_00000100.jsonl
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

// chunkInfo holds parsed chunk metadata
type chunkInfo struct {
	key       string
	firstLine int
	lastLine  int
	data      []byte
}

// chunkDownloader is an interface for downloading chunks from storage
type chunkDownloader interface {
	Download(ctx context.Context, key string) ([]byte, error)
}

// maxParallelDownloads limits concurrent chunk downloads to avoid overwhelming S3
const maxParallelDownloads = 10

// chunkResult holds the result of a parallel chunk download
type chunkResult struct {
	index    int
	chunk    chunkInfo
	err      error
	duration time.Duration // time taken to download this chunk
}

// downloadChunks downloads all chunks for the given keys in parallel and returns them as chunkInfo slices.
// Keys with unparseable names are skipped with a warning log.
// Downloads are limited to maxParallelDownloads concurrent operations.
func downloadChunks(ctx context.Context, storage chunkDownloader, chunkKeys []string) ([]chunkInfo, error) {
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	// Parse all keys first to filter out invalid ones
	type keyInfo struct {
		key       string
		firstLine int
		lastLine  int
		index     int // original index for ordering
	}
	validKeys := make([]keyInfo, 0, len(chunkKeys))
	for i, key := range chunkKeys {
		firstLine, lastLine, ok := parseChunkKey(key)
		if !ok {
			logger.Warn("Skipping chunk with unparseable key", "key", key)
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
			data, err := storage.Download(ctx, ki.key)
			elapsed := time.Since(start)

			if err != nil {
				logger.Error("Failed to download chunk", "error", err, "key", ki.key)
				results <- chunkResult{index: idx, err: err, duration: elapsed}
				return
			}

			results <- chunkResult{
				index:    idx,
				duration: elapsed,
				chunk: chunkInfo{
					key:       ki.key,
					firstLine: ki.firstLine,
					lastLine:  ki.lastLine,
					data:      data,
				},
			}
		}(i, ki)
	}

	// Collect results and track timing metrics
	chunks := make([]chunkInfo, len(validKeys))
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

	// Log parallel download performance metrics
	// maxDuration = wall-clock time if downloads were fully parallel (limited by slowest)
	// sumDuration = total time if downloads were sequential
	// Improvement ratio = sumDuration / maxDuration (higher = better parallelization)
	if len(validKeys) > 1 {
		var ratio float64
		if maxDuration > 0 {
			ratio = float64(sumDuration) / float64(maxDuration)
		}
		logger.Info("Parallel chunk download metrics",
			"chunk_count", len(validKeys),
			"max_duration_ms", maxDuration.Milliseconds(),
			"sum_duration_ms", sumDuration.Milliseconds(),
			"speedup_ratio", fmt.Sprintf("%.2f", ratio))
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return chunks, nil
}

// mergeChunks takes downloaded chunks and merges them, handling overlaps.
// Uses a simple array indexed by line number - each chunk's lines are written
// to the array, and later chunks overwrite earlier ones for the same line.
// The final array is then concatenated into the result.
//
// If overlapping lines have different content (shouldn't happen normally),
// a warning is logged since this may indicate data corruption.
func mergeChunks(chunks []chunkInfo) []byte {
	if len(chunks) == 0 {
		return nil
	}
	if len(chunks) == 1 {
		return chunks[0].data
	}

	// Find max line number
	maxLine := 0
	for _, c := range chunks {
		if c.lastLine > maxLine {
			maxLine = c.lastLine
		}
	}

	// Build array indexed by line number (0-indexed, so line 1 is at index 0)
	lines := make([][]byte, maxLine)

	// Populate array from each chunk (last write wins)
	for _, c := range chunks {
		chunkLines := splitLines(c.data)
		for i, line := range chunkLines {
			lineNum := c.firstLine + i // 1-based line number
			if lineNum >= 1 && lineNum <= maxLine {
				idx := lineNum - 1
				// Check for conflicting content on overlap
				if lines[idx] != nil && !bytes.Equal(lines[idx], line) {
					logger.Warn("Chunk overlap with differing content",
						"line_num", lineNum,
						"chunk", c.key,
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

	return result
}

// splitLines splits data into lines, preserving each line's content without the newline
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

// handleCanonicalSyncFileRead reads and concatenates all chunks for a file via canonical access (CF-132)
// GET /api/v1/sessions/{id}/sync/file?file_name=...&line_offset=...
// Supports: owner access, public shares, system shares, recipient shares
//
// The optional line_offset parameter enables incremental fetching:
// - If line_offset is 0 or omitted, returns all lines
// - If line_offset > 0, returns only lines after line_offset (lines N+1 onwards)
//
// Optimizations:
// - DB short-circuit: if line_offset >= last_synced_line, returns empty without S3 access
// - Chunk filtering: only downloads chunks containing lines > line_offset
// - Self-healing: corrects DB chunk_count if it differs from actual S3 count (owner only)
func (s *Server) handleCanonicalSyncFileRead(w http.ResponseWriter, r *http.Request) {
	// Get params from URL
	sessionID := chi.URLParam(r, "id")
	fileName := r.URL.Query().Get("file_name")
	lineOffsetStr := r.URL.Query().Get("line_offset")

	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if fileName == "" {
		respondError(w, http.StatusBadRequest, "file_name is required")
		return
	}

	// Parse line_offset (default 0 for backward compatibility)
	var lineOffset int
	if lineOffsetStr != "" {
		var err error
		lineOffset, err = strconv.Atoi(lineOffsetStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "line_offset must be a valid integer")
			return
		}
		if lineOffset < 0 {
			respondError(w, http.StatusBadRequest, "line_offset must be non-negative")
			return
		}
	}

	// Check canonical access (CF-132 unified access model)
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	result, err := CheckCanonicalAccess(dbCtx, s.db, sessionID)
	if RespondCanonicalAccessError(w, err, sessionID) {
		return
	}

	// Handle no access - sync endpoint always returns 404 (no AuthMayHelp prompt)
	if result.AccessInfo.AccessType == db.SessionAccessNone {
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	session := result.Session
	isOwner := result.AccessInfo.AccessType == db.SessionAccessOwner

	// Get the session's user_id and external_id for S3 path
	sessionUserID, externalID, err := s.db.GetSessionOwnerAndExternalID(dbCtx, sessionID)
	if err != nil {
		logger.Error("Failed to get session info", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to get session info")
		return
	}

	// Find file in session and get last_synced_line for short-circuit optimization
	var fileInfo *db.SyncFileDetail
	for i := range session.Files {
		if session.Files[i].FileName == fileName {
			fileInfo = &session.Files[i]
			break
		}
	}
	if fileInfo == nil {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Short-circuit: if line_offset >= last_synced_line, no new lines exist
	// Return empty response without touching S3
	if lineOffset >= fileInfo.LastSyncedLine {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}

	// List all chunks for this file
	storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
	defer storageCancel()

	chunkKeys, err := s.storage.ListChunks(storageCtx, sessionUserID, externalID, fileName)
	if err != nil {
		logger.Error("Failed to list chunks", "error", err, "session_id", sessionID, "file_name", fileName)
		respondStorageError(w, err, "Failed to list chunks")
		return
	}

	if len(chunkKeys) == 0 {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Self-healing: update DB chunk_count if it differs from actual S3 count (owner only)
	// This corrects any drift from races or failed uploads
	// Only do this when lineOffset == 0 (full read) to avoid extra DB calls on incremental fetches
	if isOwner && lineOffset == 0 {
		// Get current chunk_count from DB for comparison
		syncState, err := s.db.GetSyncFileState(dbCtx, sessionID, fileName)
		if err == nil {
			actualChunkCount := len(chunkKeys)
			if syncState.ChunkCount == nil || *syncState.ChunkCount != actualChunkCount {
				if err := s.db.UpdateSyncFileChunkCount(dbCtx, sessionID, fileName, actualChunkCount); err != nil {
					// Log but don't fail the read - this is best-effort healing
					logger.Warn("Failed to self-heal chunk count",
						"error", err,
						"session_id", sessionID,
						"file_name", fileName,
						"actual_count", actualChunkCount)
				} else {
					logger.Debug("Self-healed chunk count",
						"session_id", sessionID,
						"file_name", fileName,
						"old_count", syncState.ChunkCount,
						"new_count", actualChunkCount)
				}
			}
		}
	}

	// Filter chunks to only those containing lines > lineOffset
	// A chunk with range [firstLine, lastLine] is relevant if lastLine > lineOffset
	if lineOffset > 0 {
		var relevantKeys []string
		for _, key := range chunkKeys {
			_, lastLine, ok := parseChunkKey(key)
			if !ok {
				// Include unparseable keys - they'll be skipped during download
				relevantKeys = append(relevantKeys, key)
				continue
			}
			// Only include chunks that have lines after lineOffset
			if lastLine > lineOffset {
				relevantKeys = append(relevantKeys, key)
			}
		}
		chunkKeys = relevantKeys
	}

	// If no relevant chunks after filtering, return empty response
	if len(chunkKeys) == 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Download chunks and parse their line ranges
	chunks, err := downloadChunks(storageCtx, s.storage, chunkKeys)
	if err != nil {
		respondStorageError(w, err, "Failed to download file chunk")
		return
	}

	if len(chunks) == 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Find the first line number among downloaded chunks (for correct filtering)
	minFirstLine := chunks[0].firstLine
	for _, c := range chunks[1:] {
		if c.firstLine < minFirstLine {
			minFirstLine = c.firstLine
		}
	}

	// Merge chunks, handling any overlaps from partial upload failures
	merged := mergeChunks(chunks)

	// If line_offset is specified, filter output to only lines after offset
	if lineOffset > 0 {
		merged = filterLinesAfterOffset(merged, lineOffset, minFirstLine)
	}

	logger.Info("Canonical sync file read",
		"session_id", sessionID,
		"file_name", fileName,
		"chunk_count", len(chunks),
		"line_offset", lineOffset,
		"access_type", result.AccessInfo.AccessType,
		"viewer_user_id", result.ViewerUserID)

	// Write response
	// Use text/plain for JSONL files (multiple JSON objects, one per line)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if merged != nil {
		w.Write(merged)
	}
}

// extractTextFromMessage extracts the first text content from a message entry
// Handles both string content and array content (multimodal messages)
func extractTextFromMessage(entry map[string]interface{}) string {
	message, ok := entry["message"].(map[string]interface{})
	if !ok {
		return ""
	}

	content := message["content"]
	if content == nil {
		return ""
	}

	// Case 1: content is a string
	if str, ok := content.(string); ok {
		return str
	}

	// Case 2: content is an array of content blocks (multimodal)
	if arr, ok := content.([]interface{}); ok {
		for _, block := range arr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockType, _ := blockMap["type"].(string); blockType == "text" {
					if text, ok := blockMap["text"].(string); ok && text != "" {
						return text
					}
				}
			}
		}
	}

	return ""
}

// extractTimestampFromLine parses a JSONL line and extracts the timestamp field if present
// Returns nil if no timestamp found or parsing fails
func extractTimestampFromLine(line string) *time.Time {
	// Quick check to avoid parsing lines without timestamp
	if !strings.Contains(line, `"timestamp"`) {
		return nil
	}

	// Parse just enough to get the timestamp
	var entry struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil
	}

	if entry.Timestamp == "" {
		return nil
	}

	// Parse ISO 8601 timestamp
	ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
	if err != nil {
		// Try alternative formats
		ts, err = time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return nil
		}
	}

	return &ts
}

// UpdateSummaryRequest is the request body for PATCH /api/v1/sessions/{external_id}/summary
type UpdateSummaryRequest struct {
	Summary string `json:"summary"`
}

// handleUpdateSessionSummary updates the summary for a session by external_id
// PATCH /api/v1/sessions/{external_id}/summary
func (s *Server) handleUpdateSessionSummary(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get external_id from URL
	externalID := chi.URLParam(r, "external_id")
	if externalID == "" {
		respondError(w, http.StatusBadRequest, "external_id is required")
		return
	}

	// Parse request body
	var req UpdateSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate summary length
	if err := validation.ValidateSummary(req.Summary); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Update summary in database
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	err := s.db.UpdateSessionSummary(ctx, externalID, userID, req.Summary)
	if err != nil {
		if errors.Is(err, db.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		if errors.Is(err, db.ErrForbidden) {
			respondError(w, http.StatusForbidden, "Access denied")
			return
		}
		logger.Error("Failed to update session summary", "error", err, "user_id", userID, "external_id", externalID)
		respondError(w, http.StatusInternalServerError, "Failed to update summary")
		return
	}

	logger.Info("Session summary updated", "user_id", userID, "external_id", externalID)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
