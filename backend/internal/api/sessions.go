package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/logger"
	"github.com/santaclaude2025/confab/backend/internal/models"
)

// Validation limits for session uploads
const (
	MaxRequestBodySize = 200 * 1024 * 1024 // 200MB total request size
	MaxFileSize        = 50 * 1024 * 1024 // 50MB per file
	MaxFiles           = 100              // Maximum number of files per session
	MaxSessionIDLength = 256              // Max session ID length
	MaxPathLength      = 1024             // Max file path length
	MaxReasonLength    = 10000            // Max reason text length
	MaxCWDLength       = 4096             // Max current working directory length
	MinSessionIDLength = 1                // Min session ID length
)

// handleSaveSession processes session upload requests
func (s *Server) handleSaveSession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user ID
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Limit request body size to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Parse request body
	var req models.SaveSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Check if error is due to request too large (type-safe check)
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
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

	// Extract session metadata from transcript
	title, sessionType := extractSessionMetadata(req.Files)
	req.Title = title
	req.SessionType = sessionType

	logger.Info("Processing session save",
		"user_id", userID,
		"session_id", req.SessionID,
		"file_count", len(req.Files),
		"title", title,
		"session_type", sessionType)

	// Create context with timeout for storage operations (longer timeout for uploads)
	storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
	defer storageCancel()

	// Upload files to S3 and collect S3 keys
	s3Keys := make(map[string]string)
	for _, file := range req.Files {
		if len(file.Content) == 0 {
			logger.Warn("Empty file content", "file_path", file.Path, "session_id", req.SessionID)
			continue
		}

		s3Key, err := s.storage.Upload(storageCtx, userID, req.SessionID, file.Path, file.Content)
		if err != nil {
			logger.Error("File upload failed",
				"error", err,
				"user_id", userID,
				"session_id", req.SessionID,
				"file_path", file.Path)
			respondStorageError(w, err, "Failed to upload files to storage")
			return
		}

		s3Keys[file.Path] = s3Key
		logger.Debug("File uploaded", "file_path", file.Path, "s3_key", s3Key)
	}

	// Create context with timeout for database operation
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	// Save metadata to database
	runID, err := s.db.SaveSession(dbCtx, userID, &req, s3Keys, "hook")
	if err != nil {
		logger.Error("Failed to save session metadata",
			"error", err,
			"user_id", userID,
			"session_id", req.SessionID)
		respondError(w, http.StatusInternalServerError, "Failed to save session metadata")
		return
	}

	// Audit log: Session saved successfully
	logger.Info("Session saved successfully",
		"user_id", userID,
		"session_id", req.SessionID,
		"run_id", runID,
		"file_count", len(s3Keys))

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
			logger.Warn("File size mismatch",
				"file_index", i,
				"file_path", file.Path,
				"declared_size", file.SizeBytes,
				"actual_size", contentSize)
		}
	}

	return nil
}

// sanitizeTitleText removes HTML tags and escapes HTML entities to prevent XSS
func sanitizeTitleText(input string) string {
	// First, remove all HTML tags using regex
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlTagRegex.ReplaceAllString(input, "")

	// Unescape HTML entities (e.g., &lt; -> <) and then escape them again
	// This handles cases like "&lt;script&gt;" -> "<script>" -> "&lt;script&gt;"
	unescaped := html.UnescapeString(cleaned)
	escaped := html.EscapeString(unescaped)

	// Trim whitespace
	return strings.TrimSpace(escaped)
}

// extractSessionMetadata parses the transcript JSONL to extract title and session type
func extractSessionMetadata(files []models.FileUpload) (title string, sessionType string) {
	// Find the transcript file
	var transcriptContent []byte
	for _, file := range files {
		if file.Type == "transcript" {
			transcriptContent = file.Content
			break
		}
	}

	if len(transcriptContent) == 0 {
		return "", "Claude Code"
	}

	// Parse JSONL to find summary, user message, and version info
	lines := strings.Split(string(transcriptContent), "\n")
	var firstUserMessage string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Extract session type from version field (if present)
		if sessionType == "" {
			if version, ok := entry["version"].(string); ok && version != "" {
				// For now, all sessions with version field are Claude Code
				sessionType = "Claude Code"
			}
		}

		msgType, _ := entry["type"].(string)

		// Priority 1: Extract title from summary (best quality)
		if title == "" && msgType == "summary" {
			if summary, ok := entry["summary"].(string); ok && summary != "" {
				title = sanitizeTitleText(summary)
			}
		}

		// Collect first user message as fallback
		if firstUserMessage == "" && msgType == "user" {
			if message, ok := entry["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok && content != "" {
					// Use first 100 characters as fallback title
					sanitized := sanitizeTitleText(content)
					if len(sanitized) > 100 {
						firstUserMessage = sanitized[:100]
					} else {
						firstUserMessage = sanitized
					}
				}
			}
		}

		// Stop once we have both summary title and session type
		if title != "" && sessionType != "" {
			break
		}
	}

	// Fallback to first user message if no summary found
	if title == "" && firstUserMessage != "" {
		title = firstUserMessage
	}

	// Set defaults if not found
	if sessionType == "" {
		sessionType = "Claude Code"
	}

	return title, sessionType
}

// HandleCheckSessions checks which session IDs already exist for the user
func HandleCheckSessions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Parse request body
		var req struct {
			SessionIDs []string `json:"session_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate
		if len(req.SessionIDs) == 0 {
			respondError(w, http.StatusBadRequest, "session_ids is required")
			return
		}
		if len(req.SessionIDs) > 1000 {
			respondError(w, http.StatusBadRequest, "Too many session IDs (max 1000)")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Check which sessions exist
		existing, err := s.db.CheckSessionsExist(ctx, userID, req.SessionIDs)
		if err != nil {
			logger.Error("Failed to check sessions", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to check sessions")
			return
		}

		// Success log
		logger.Info("Sessions checked",
			"user_id", userID,
			"requested_count", len(req.SessionIDs),
			"existing_count", len(existing))

		// Build missing list
		existingSet := make(map[string]bool)
		for _, id := range existing {
			existingSet[id] = true
		}
		var missing []string
		for _, id := range req.SessionIDs {
			if !existingSet[id] {
				missing = append(missing, id)
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"existing": existing,
			"missing":  missing,
		})
	}
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
