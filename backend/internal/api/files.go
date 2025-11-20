package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// HandleGetFileContent returns file content from S3
func HandleGetFileContent(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get file ID from URL
		fileIDStr := chi.URLParam(r, "fileId")
		if fileIDStr == "" {
			http.Error(w, "Missing file ID", http.StatusBadRequest)
			return
		}

		fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid file ID", http.StatusBadRequest)
			return
		}

		// Get file metadata and verify ownership
		file, err := database.GetFileByID(ctx, fileID, userID)
		if err != nil {
			if err.Error() == "file not found or unauthorized" {
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to get file", http.StatusInternalServerError)
			return
		}

		// Check if file was uploaded to S3
		if file.S3Key == nil {
			http.Error(w, "File not available", http.StatusNotFound)
			return
		}

		// Download file from S3
		content, err := store.Download(ctx, *file.S3Key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to download file: %v", err), http.StatusInternalServerError)
			return
		}

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
