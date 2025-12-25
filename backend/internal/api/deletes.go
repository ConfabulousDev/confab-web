package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// HandleDeleteSession deletes an entire session and all associated S3 chunks
func HandleDeleteSession(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get authenticated user ID
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Step 1: Verify ownership and get external_id
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		externalID, err := database.VerifySessionOwnership(dbCtx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrForbidden) {
				respondError(w, http.StatusForbidden, "Access denied")
				return
			}
			log.Error("Failed to verify session ownership",
				"error", err,
				"session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to retrieve session information")
			return
		}

		// Step 2: Delete all sync chunks from S3
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		if err := store.DeleteAllSessionChunks(storageCtx, userID, externalID); err != nil {
			log.Error("Failed to delete session chunks",
				"error", err,
				"session_id", sessionID,
				"external_id", externalID)
			// Continue anyway - chunks will be orphaned but session deletion should proceed
		}

		// Step 3: Delete from database (CASCADE deletes sync_files, shares, etc.)
		dbCtx2, dbCancel2 := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel2()

		if err := database.DeleteSessionFromDB(dbCtx2, sessionID, userID); err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to delete session from database",
				"error", err,
				"session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to delete session")
			return
		}

		// Audit log: Session deleted successfully
		log.Info("Session deleted successfully",
			"session_id", sessionID,
			"external_id", externalID)

		// Return success response
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"session_id": sessionID,
			"message":    "Session deleted successfully",
		})
	}
}
