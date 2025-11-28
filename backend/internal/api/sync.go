package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
)

// ============================================================================
// Request/Response Types
// ============================================================================

// SyncInitRequest is the request body for POST /api/v1/sync/init
type SyncInitRequest struct {
	ExternalID     string          `json:"external_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	GitInfo        json.RawMessage `json:"git_info,omitempty"` // Optional git metadata
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

// SyncChunkRequest is the request body for POST /api/v1/sync/chunk
type SyncChunkRequest struct {
	SessionID string   `json:"session_id"`
	FileName  string   `json:"file_name"`
	FileType  string   `json:"file_type"`
	FirstLine int      `json:"first_line"`
	Lines     []string `json:"lines"`
}

// SyncChunkResponse is the response for POST /api/v1/sync/chunk
type SyncChunkResponse struct {
	LastSyncedLine int `json:"last_synced_line"`
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

	// Find or create session
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	params := db.SyncSessionParams{
		ExternalID:     req.ExternalID,
		TranscriptPath: req.TranscriptPath,
		CWD:            req.CWD,
		GitInfo:        req.GitInfo,
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
	if req.FirstLine < 1 {
		respondError(w, http.StatusBadRequest, "first_line must be >= 1")
		return
	}
	if len(req.Lines) == 0 {
		respondError(w, http.StatusBadRequest, "lines array cannot be empty")
		return
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

	// Build chunk content (lines joined by newlines, with trailing newline)
	// Also extract metadata from transcript lines (timestamp, title)
	var content bytes.Buffer
	var latestTimestamp *time.Time
	var extractedTitle string
	for _, line := range req.Lines {
		content.WriteString(line)
		content.WriteString("\n")

		// Try to extract metadata from transcript lines
		if req.FileType == "transcript" {
			if ts := extractTimestampFromLine(line); ts != nil {
				if latestTimestamp == nil || ts.After(*latestTimestamp) {
					latestTimestamp = ts
				}
			}
			// Extract title from summary or first user message
			if extractedTitle == "" {
				if title := extractTitleFromLine(line); title != "" {
					extractedTitle = title
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

	if err := s.db.UpdateSyncFileState(updateCtx, req.SessionID, req.FileName, req.FileType, lastLine, latestTimestamp, extractedTitle); err != nil {
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

// handleSyncFileRead reads and concatenates all chunks for a file
// GET /api/v1/sync/file?session_id=...&file_name=...
func (s *Server) handleSyncFileRead(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse query params
	sessionID := r.URL.Query().Get("session_id")
	fileName := r.URL.Query().Get("file_name")

	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if fileName == "" {
		respondError(w, http.StatusBadRequest, "file_name is required")
		return
	}

	// Verify session ownership and get external_id
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	externalID, err := s.db.VerifySessionOwnership(dbCtx, sessionID, userID)
	if err != nil {
		if errors.Is(err, db.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		if errors.Is(err, db.ErrForbidden) {
			respondError(w, http.StatusForbidden, "Access denied")
			return
		}
		logger.Error("Failed to verify session ownership", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to verify session")
		return
	}

	// List all chunks for this file
	storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
	defer storageCancel()

	chunkKeys, err := s.storage.ListChunks(storageCtx, userID, externalID, fileName)
	if err != nil {
		logger.Error("Failed to list chunks", "error", err, "session_id", sessionID, "file_name", fileName)
		respondStorageError(w, err, "Failed to list chunks")
		return
	}

	if len(chunkKeys) == 0 {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Download all chunks and parse their line ranges
	chunks := make([]chunkInfo, 0, len(chunkKeys))
	for _, key := range chunkKeys {
		firstLine, lastLine, ok := parseChunkKey(key)
		if !ok {
			logger.Warn("Skipping chunk with unparseable key", "key", key)
			continue
		}

		data, err := s.storage.Download(storageCtx, key)
		if err != nil {
			logger.Error("Failed to download chunk", "error", err, "key", key)
			respondStorageError(w, err, "Failed to download file chunk")
			return
		}

		chunks = append(chunks, chunkInfo{
			key:       key,
			firstLine: firstLine,
			lastLine:  lastLine,
			data:      data,
		})
	}

	if len(chunks) == 0 {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Merge chunks, handling any overlaps from partial upload failures
	merged := mergeChunks(chunks)

	// Write response
	// Use text/plain for JSONL files (multiple JSON objects, one per line)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(merged)
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
				if lines[idx] != nil && !bytesEqual(lines[idx], line) {
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

// bytesEqual compares two byte slices for equality
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// handleSharedSyncFileRead reads and concatenates all chunks for a file via share token
// GET /api/v1/sessions/{id}/shared/{shareToken}/sync/file?file_name=...
func (s *Server) handleSharedSyncFileRead(w http.ResponseWriter, r *http.Request) {
	// Get params from URL
	sessionID := chi.URLParam(r, "id")
	shareToken := chi.URLParam(r, "shareToken")
	fileName := r.URL.Query().Get("file_name")

	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if shareToken == "" {
		respondError(w, http.StatusBadRequest, "share_token is required")
		return
	}
	if fileName == "" {
		respondError(w, http.StatusBadRequest, "file_name is required")
		return
	}

	// Verify share access
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	// Get viewer email if authenticated (for private shares)
	var viewerEmail *string
	cookie, err := r.Cookie("confab_session")
	if err == nil {
		webSession, err := s.db.GetWebSession(dbCtx, cookie.Value)
		if err == nil {
			user, err := s.db.GetUserByID(dbCtx, webSession.UserID)
			if err == nil && user != nil {
				viewerEmail = &user.Email
			}
		}
	}

	// Verify share and get session (this validates share token, expiration, and private access)
	session, err := s.db.GetSharedSession(dbCtx, sessionID, shareToken, viewerEmail)
	if err != nil {
		if errors.Is(err, db.ErrShareNotFound) || errors.Is(err, db.ErrSessionNotFound) {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		if errors.Is(err, db.ErrShareExpired) {
			respondError(w, http.StatusGone, "Share expired")
			return
		}
		if errors.Is(err, db.ErrUnauthorized) || errors.Is(err, db.ErrForbidden) {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		logger.Error("Failed to verify share access", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to verify share")
		return
	}

	// Get the session's user_id and external_id for S3 path
	sessionUserID, externalID, err := s.db.GetSessionOwnerAndExternalID(dbCtx, sessionID)
	if err != nil {
		logger.Error("Failed to get session info", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to get session info")
		return
	}

	// Verify file exists in this session
	fileExists := false
	for _, file := range session.Files {
		if file.FileName == fileName {
			fileExists = true
			break
		}
	}
	if !fileExists {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// List and download all chunks for this file
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

	// Download all chunks and parse their line ranges
	chunks := make([]chunkInfo, 0, len(chunkKeys))
	for _, key := range chunkKeys {
		firstLine, lastLine, ok := parseChunkKey(key)
		if !ok {
			logger.Warn("Skipping chunk with unparseable key", "key", key)
			continue
		}

		data, err := s.storage.Download(storageCtx, key)
		if err != nil {
			logger.Error("Failed to download chunk", "error", err, "key", key)
			respondStorageError(w, err, "Failed to download file chunk")
			return
		}

		chunks = append(chunks, chunkInfo{
			key:       key,
			firstLine: firstLine,
			lastLine:  lastLine,
			data:      data,
		})
	}

	if len(chunks) == 0 {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}

	// Merge chunks, handling any overlaps from partial upload failures
	merged := mergeChunks(chunks)

	logger.Info("Shared sync file read",
		"session_id", sessionID,
		"share_token", shareToken,
		"file_name", fileName,
		"chunk_count", len(chunks),
		"viewer_email", viewerEmail)

	// Write response
	// Use text/plain for JSONL files (multiple JSON objects, one per line)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(merged)
}

// extractTitleFromLine extracts a title from a JSONL line
// Looks for summary messages (type: "summary") or first user message
// Returns empty string if no title found
func extractTitleFromLine(line string) string {
	// Quick check - must have "type" field
	if !strings.Contains(line, `"type"`) {
		return ""
	}

	// Parse enough to determine message type and extract title
	var entry struct {
		Type    string `json:"type"`
		Summary string `json:"summary"` // For summary messages
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"` // For regular messages
	}
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return ""
	}

	// Priority 1: Summary messages have explicit summaries
	if entry.Type == "summary" && entry.Summary != "" {
		// Truncate long summaries
		if len(entry.Summary) > 100 {
			return entry.Summary[:100] + "..."
		}
		return entry.Summary
	}

	// Priority 2: First user message content (first line)
	if entry.Type == "user" && entry.Message.Role == "user" && entry.Message.Content != "" {
		// Take first line and truncate
		content := entry.Message.Content
		if idx := strings.Index(content, "\n"); idx > 0 {
			content = content[:idx]
		}
		if len(content) > 100 {
			return content[:100] + "..."
		}
		return content
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
