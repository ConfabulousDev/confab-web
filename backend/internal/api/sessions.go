package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/models"
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

	// Parse request body
	var req models.SaveSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.TranscriptPath == "" {
		respondError(w, http.StatusBadRequest, "transcript_path is required")
		return
	}
	if len(req.Files) == 0 {
		respondError(w, http.StatusBadRequest, "files array cannot be empty")
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
