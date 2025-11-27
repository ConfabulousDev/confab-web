package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
)

// ============================================================================
// Request/Response Types
// ============================================================================

// SyncInitRequest is the request body for POST /api/v1/sync/init
type SyncInitRequest struct {
	ExternalID     string `json:"external_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
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

	sessionID, files, err := s.db.FindOrCreateSyncSession(ctx, userID, req.ExternalID, req.TranscriptPath, req.CWD)
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

	// Build chunk content (lines joined by newlines, with trailing newline)
	var content bytes.Buffer
	for _, line := range req.Lines {
		content.WriteString(line)
		content.WriteString("\n")
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

	// Update sync state in DB
	updateCtx, updateCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer updateCancel()

	if err := s.db.UpdateSyncFileState(updateCtx, req.SessionID, req.FileName, req.FileType, lastLine); err != nil {
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

	// Download all chunks first (fail fast if any chunk is missing)
	var allData [][]byte
	for _, key := range chunkKeys {
		data, err := s.storage.Download(storageCtx, key)
		if err != nil {
			logger.Error("Failed to download chunk", "error", err, "key", key)
			respondStorageError(w, err, "Failed to download file chunk")
			return
		}
		allData = append(allData, data)
	}

	// All chunks downloaded successfully - write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	for _, data := range allData {
		w.Write(data)
	}
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
