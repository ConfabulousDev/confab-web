package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// Validation limits for session uploads
const (
	MaxRequestBodySize = 50 * 1024 * 1024 // 50MB total request size
	MaxFileSize        = 10 * 1024 * 1024 // 10MB per file
	MaxFiles           = 100              // Maximum number of files per session
	MaxSessionIDLength = 256              // Max session ID length
	MaxPathLength      = 1024             // Max file path length
	MaxReasonLength    = 10000            // Max reason text length
	MaxCWDLength       = 4096             // Max current working directory length
	MinSessionIDLength = 1                // Min session ID length
)

// handleSaveSession processes session upload requests
func (s *Server) handleSaveSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get authenticated user ID
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Limit request body size to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Parse request body
	var req models.SaveSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Check if error is due to request too large
		if strings.Contains(err.Error(), "request body too large") {
			respondError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("Request body too large (max %d MB)", MaxRequestBodySize/(1024*1024)))
			return
		}
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request with detailed error messages
	if err := validateSaveSessionRequest(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("Processing session save for user %d, session %s with %d files", userID, req.SessionID, len(req.Files))

	// Upload files to S3 and collect S3 keys
	s3Keys := make(map[string]string)
	for _, file := range req.Files {
		if len(file.Content) == 0 {
			log.Printf("Warning: Empty content for file %s", file.Path)
			continue
		}

		s3Key, err := s.storage.Upload(ctx, userID, req.SessionID, file.Path, file.Content)
		if err != nil {
			log.Printf("Error uploading file %s to S3: %v", file.Path, err)
			respondError(w, http.StatusInternalServerError, "Failed to upload files to storage")
			return
		}

		s3Keys[file.Path] = s3Key
		log.Printf("Uploaded %s to S3 key: %s", file.Path, s3Key)
	}

	// Save metadata to database
	runID, err := s.db.SaveSession(ctx, userID, &req, s3Keys)
	if err != nil {
		log.Printf("Error saving session to database: %v", err)
		respondError(w, http.StatusInternalServerError, "Failed to save session metadata")
		return
	}

	log.Printf("Successfully saved session %s, run ID: %d", req.SessionID, runID)

	// Return success response
	respondJSON(w, http.StatusOK, models.SaveSessionResponse{
		Success:   true,
		SessionID: req.SessionID,
		RunID:     runID,
		Message:   "Session saved successfully",
	})
}

// validateSaveSessionRequest validates session upload request
func validateSaveSessionRequest(req *models.SaveSessionRequest) error {
	// Validate session ID
	if req.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if len(req.SessionID) < MinSessionIDLength || len(req.SessionID) > MaxSessionIDLength {
		return fmt.Errorf("session_id must be between %d and %d characters", MinSessionIDLength, MaxSessionIDLength)
	}
	if !utf8.ValidString(req.SessionID) {
		return fmt.Errorf("session_id must be valid UTF-8")
	}

	// Validate transcript path
	if req.TranscriptPath == "" {
		return fmt.Errorf("transcript_path is required")
	}
	if len(req.TranscriptPath) > MaxPathLength {
		return fmt.Errorf("transcript_path too long (max %d characters)", MaxPathLength)
	}
	if !utf8.ValidString(req.TranscriptPath) {
		return fmt.Errorf("transcript_path must be valid UTF-8")
	}

	// Validate CWD (optional but if provided must be valid)
	if req.CWD != "" {
		if len(req.CWD) > MaxCWDLength {
			return fmt.Errorf("cwd too long (max %d characters)", MaxCWDLength)
		}
		if !utf8.ValidString(req.CWD) {
			return fmt.Errorf("cwd must be valid UTF-8")
		}
	}

	// Validate reason (optional but if provided must be valid)
	if req.Reason != "" {
		if len(req.Reason) > MaxReasonLength {
			return fmt.Errorf("reason too long (max %d characters)", MaxReasonLength)
		}
		if !utf8.ValidString(req.Reason) {
			return fmt.Errorf("reason must be valid UTF-8")
		}
	}

	// Validate files array
	if len(req.Files) == 0 {
		return fmt.Errorf("files array cannot be empty")
	}
	if len(req.Files) > MaxFiles {
		return fmt.Errorf("too many files (max %d, got %d)", MaxFiles, len(req.Files))
	}

	// Validate each file
	var totalSize int64
	for i, file := range req.Files {
		// Validate path
		if file.Path == "" {
			return fmt.Errorf("file[%d]: path is required", i)
		}
		if len(file.Path) > MaxPathLength {
			return fmt.Errorf("file[%d]: path too long (max %d characters)", i, MaxPathLength)
		}
		if !utf8.ValidString(file.Path) {
			return fmt.Errorf("file[%d]: path must be valid UTF-8", i)
		}

		// Check for path traversal attempts
		if strings.Contains(file.Path, "..") {
			return fmt.Errorf("file[%d]: path contains invalid sequence '..'", i)
		}

		// Validate content size
		contentSize := int64(len(file.Content))
		if contentSize > MaxFileSize {
			return fmt.Errorf("file[%d]: content too large (max %d MB, got %d MB)",
				i, MaxFileSize/(1024*1024), contentSize/(1024*1024))
		}

		// Track total size
		totalSize += contentSize
		if totalSize > MaxRequestBodySize {
			return fmt.Errorf("total file content too large (max %d MB)", MaxRequestBodySize/(1024*1024))
		}

		// Validate SizeBytes matches actual content (if provided)
		if file.SizeBytes > 0 && file.SizeBytes != contentSize {
			log.Printf("Warning: file[%d] (%s) size_bytes mismatch: declared=%d, actual=%d",
				i, file.Path, file.SizeBytes, contentSize)
		}
	}

	return nil
}

// sanitizeString sanitizes user input strings
// Removes null bytes and non-printable characters that could cause issues
func sanitizeString(s string) string {
	// Remove null bytes (can cause issues in logs and database)
	s = strings.ReplaceAll(s, "\x00", "")

	// Keep valid UTF-8 runes only
	if !utf8.ValidString(s) {
		// Convert to valid UTF-8 by replacing invalid sequences
		v := make([]rune, 0, len(s))
		for _, r := range s {
			if r != utf8.RuneError {
				v = append(v, r)
			}
		}
		s = string(v)
	}

	return s
}
