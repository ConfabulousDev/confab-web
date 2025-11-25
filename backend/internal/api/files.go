package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
	"github.com/santaclaude2025/confab/backend/internal/storage"
	"github.com/santaclaude2025/confab/backend/internal/validation"
)

// HandleGetFileContent returns file content from S3
func HandleGetFileContent(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get file ID from URL
		fileIDStr := chi.URLParam(r, "fileId")
		if fileIDStr == "" {
			respondError(w, http.StatusBadRequest, "Missing file ID")
			return
		}

		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid file ID")
			return
		}

		// Create context with timeout for database operation
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		// Get file metadata and verify ownership
		file, err := database.GetFileByID(dbCtx, fileID, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusNotFound, "File not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to get file")
			return
		}

		// Check if file was uploaded to S3
		if file.S3Key == nil {
			respondError(w, http.StatusNotFound, "File not available")
			return
		}

		// Create context with timeout for storage operation
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		// Download file from S3
		content, err := store.Download(storageCtx, *file.S3Key)
		if err != nil {
			respondStorageError(w, err, "Failed to download file")
			return
		}

		// Audit log: File downloaded
		logger.Info("File downloaded",
			"user_id", userID,
			"file_id", fileID,
			"file_type", file.FileType,
			"file_path", file.FilePath,
			"size_bytes", len(content))

		// Set appropriate Content-Type based on file type
		contentType := "application/json"
		if file.FileType == "transcript" || file.FileType == "agent" {
			contentType = "application/x-ndjson" // JSON Lines
		}
		w.Header().Set("Content-Type", contentType)

		// Write content
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}
}

// HandleGetSharedFileContent returns file content for shared sessions (no auth required)
func HandleGetSharedFileContent(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get params from URL
		sessionID := chi.URLParam(r, "id")
		shareToken := chi.URLParam(r, "shareToken")
		fileIDStr := chi.URLParam(r, "fileId")

		// Validate session ID (UUID)
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Validate share token
		if err := validation.ValidateShareToken(shareToken); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate file ID
		if fileIDStr == "" {
			respondError(w, http.StatusBadRequest, "Missing file ID")
			return
		}

		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid file ID")
			return
		}

		// Create context with timeout for database operations
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		// Get viewer email if authenticated (for private shares)
		// Read cookie directly since this endpoint is outside auth middleware
		var viewerEmail *string
		cookie, err := r.Cookie("confab_session")
		if err == nil {
			webSession, err := database.GetWebSession(dbCtx, cookie.Value)
			if err == nil {
				user, err := database.GetUserByID(dbCtx, webSession.UserID)
				if err == nil && user != nil {
					viewerEmail = &user.Email
				}
			}
		}

		// Validate share token and get file
		file, err := database.GetSharedFileByID(dbCtx, sessionID, shareToken, fileID, viewerEmail)
		if err != nil {
			if errors.Is(err, db.ErrFileNotFound) || errors.Is(err, db.ErrShareNotFound) {
				respondError(w, http.StatusNotFound, "File not found")
				return
			}
			if errors.Is(err, db.ErrShareExpired) {
				respondError(w, http.StatusGone, "Share expired")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to get file")
			return
		}

		// Check if file was uploaded to S3
		if file.S3Key == nil {
			respondError(w, http.StatusNotFound, "File not available")
			return
		}

		// Create context with timeout for storage operation
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		// Download file from S3
		content, err := store.Download(storageCtx, *file.S3Key)
		if err != nil {
			respondStorageError(w, err, "Failed to download file")
			return
		}

		// Audit log: Shared file downloaded
		logger.Info("Shared file downloaded",
			"session_id", sessionID,
			"share_token", shareToken,
			"file_id", fileID,
			"file_type", file.FileType,
			"file_path", file.FilePath,
			"viewer_email", viewerEmail,
			"size_bytes", len(content))

		// Set appropriate Content-Type based on file type
		contentType := "application/json"
		if file.FileType == "transcript" || file.FileType == "agent" {
			contentType = "application/x-ndjson"
		}
		w.Header().Set("Content-Type", contentType)

		// Write content
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}
}
