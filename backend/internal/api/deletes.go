package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
	"github.com/santaclaude2025/confab/backend/internal/storage"
)

// HandleDeleteRun deletes a single run (version) and all its associated S3 objects
// If this is the only run for the session, the entire session is deleted
func HandleDeleteRun(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Get run ID from URL
		runIDStr := chi.URLParam(r, "runId")
		runID, err := strconv.ParseInt(runIDStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid run ID")
			return
		}

		// Step 1: Get S3 keys (with ownership verification)
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		sessionID, runCount, s3Keys, err := database.GetRunS3Keys(dbCtx, runID, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Access denied")
				return
			}
			logger.Error("Failed to get S3 keys for run",
				"error", err,
				"user_id", userID,
				"run_id", runID)
			respondError(w, http.StatusInternalServerError, "Failed to retrieve run information")
			return
		}

		// Step 2: Delete from S3 first (critical: must succeed before DB deletion)
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		for _, s3Key := range s3Keys {
			if err := store.Delete(storageCtx, s3Key); err != nil {
				logger.Error("Failed to delete S3 object",
					"error", err,
					"user_id", userID,
					"run_id", runID,
					"s3_key", s3Key)
				respondError(w, http.StatusInternalServerError, "Failed to delete files from storage")
				return
			}
			logger.Debug("S3 object deleted", "s3_key", s3Key)
		}

		// Step 3: Delete from database (only after S3 deletion succeeds)
		dbCtx2, dbCancel2 := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel2()

		if err := database.DeleteRunFromDB(dbCtx2, runID, userID, sessionID, runCount); err != nil {
			if errors.Is(err, db.ErrRunNotFound) {
				respondError(w, http.StatusNotFound, "Run not found")
				return
			}
			logger.Error("Failed to delete run from database",
				"error", err,
				"user_id", userID,
				"run_id", runID)
			respondError(w, http.StatusInternalServerError, "Failed to delete run")
			return
		}

		// Audit log: Run deleted successfully
		logger.Info("Run deleted successfully",
			"user_id", userID,
			"run_id", runID,
			"session_id", sessionID,
			"s3_objects_deleted", len(s3Keys),
			"session_deleted", runCount == 1)

		// Return success response
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":         true,
			"run_id":          runID,
			"session_deleted": runCount == 1,
			"message":         "Run deleted successfully",
		})
	}
}

// HandleDeleteSession deletes an entire session and all its runs/versions and associated S3 objects
func HandleDeleteSession(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		// Step 1: Get all S3 keys and external_id (with ownership verification)
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		s3Keys, err := database.GetSessionS3Keys(dbCtx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Access denied")
				return
			}
			logger.Error("Failed to get S3 keys for session",
				"error", err,
				"user_id", userID,
				"session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to retrieve session information")
			return
		}

		// Get external_id for chunk deletion
		externalID, err := database.VerifySessionOwnership(dbCtx, sessionID, userID)
		if err != nil {
			// Ownership was already verified above, so this shouldn't fail
			logger.Error("Failed to get external_id for session",
				"error", err,
				"user_id", userID,
				"session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to retrieve session information")
			return
		}

		// Step 2: Delete from S3 first (critical: must succeed before DB deletion)
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		// Delete regular files
		for _, s3Key := range s3Keys {
			if err := store.Delete(storageCtx, s3Key); err != nil {
				logger.Error("Failed to delete S3 object",
					"error", err,
					"user_id", userID,
					"session_id", sessionID,
					"s3_key", s3Key)
				respondError(w, http.StatusInternalServerError, "Failed to delete files from storage")
				return
			}
			logger.Debug("S3 object deleted", "s3_key", s3Key)
		}

		// Delete incremental sync chunks (if any)
		if err := store.DeleteAllSessionChunks(storageCtx, userID, externalID); err != nil {
			logger.Error("Failed to delete session chunks",
				"error", err,
				"user_id", userID,
				"session_id", sessionID,
				"external_id", externalID)
			// Continue anyway - chunks will be orphaned but session deletion should proceed
		}

		// Step 3: Delete from database (only after S3 deletion succeeds)
		dbCtx2, dbCancel2 := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel2()

		if err := database.DeleteSessionFromDB(dbCtx2, sessionID, userID); err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			logger.Error("Failed to delete session from database",
				"error", err,
				"user_id", userID,
				"session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to delete session")
			return
		}

		// Audit log: Session deleted successfully
		logger.Info("Session deleted successfully",
			"user_id", userID,
			"session_id", sessionID,
			"s3_objects_deleted", len(s3Keys))

		// Return success response
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"session_id": sessionID,
			"message":    "Session deleted successfully",
		})
	}
}

// HandleDeleteSessionOrRun handles the request to delete either a specific version or the entire session
func HandleDeleteSessionOrRun(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Get session ID (UUID) from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Session ID is required")
			return
		}

		// Parse request body to determine what to delete
		var req struct {
			RunID *int64 `json:"run_id,omitempty"` // If provided, delete specific run; otherwise delete entire session
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// If run_id is provided, delete specific run; otherwise delete entire session
		if req.RunID != nil {
			// Verify the run belongs to this session before delegating to HandleDeleteRun
			dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
			defer dbCancel()

			sessionIDFromRun, _, _, err := database.GetRunS3Keys(dbCtx, *req.RunID, userID)
			if err != nil {
				if errors.Is(err, db.ErrUnauthorized) {
					respondError(w, http.StatusForbidden, "Access denied")
					return
				}
				logger.Error("Failed to verify run ownership",
					"error", err,
					"user_id", userID,
					"run_id", *req.RunID)
				respondError(w, http.StatusInternalServerError, "Failed to verify run")
				return
			}

			// Verify run belongs to the specified session
			if sessionIDFromRun != sessionID {
				respondError(w, http.StatusBadRequest, "Run does not belong to specified session")
				return
			}

			// Use chi context to inject runId for the handler
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("runId", strconv.FormatInt(*req.RunID, 10))
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			// Delegate to HandleDeleteRun
			HandleDeleteRun(database, store)(w, r)
		} else {
			// Delete entire session
			HandleDeleteSession(database, store)(w, r)
		}
	}
}
