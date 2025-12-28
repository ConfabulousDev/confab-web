package api

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// HandleGetSessionAnalytics returns computed analytics for a session.
// Uses the same canonical access model as HandleGetSession (CF-132):
// - Owner access: authenticated user who owns the session
// - Public share: anyone (no auth required)
// - System share: any authenticated user
// - Recipient share: authenticated user who is a share recipient
//
// Analytics are cached in the database and recomputed when stale.
func HandleGetSessionAnalytics(database *db.DB, store *storage.S3Storage) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer dbCancel()

		// Check canonical access (CF-132 unified access model)
		result, err := CheckCanonicalAccess(dbCtx, database, sessionID)
		if RespondCanonicalAccessError(dbCtx, w, err, sessionID) {
			return
		}

		// Handle no access - check AuthMayHelp to decide 401 vs 404
		if result.AccessInfo.AccessType == db.SessionAccessNone {
			if result.AccessInfo.AuthMayHelp {
				respondError(w, http.StatusUnauthorized, "Sign in to view this session")
				return
			}
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}

		session := result.Session

		// Find the main transcript file
		var fileInfo *db.SyncFileDetail
		for i := range session.Files {
			if session.Files[i].FileType == "transcript" {
				fileInfo = &session.Files[i]
				break
			}
		}
		if fileInfo == nil {
			// No transcript file - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Current state for cache validation
		currentLineCount := int64(fileInfo.LastSyncedLine)

		// Check if we have valid cached analytics
		cached, err := analyticsStore.Get(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get cached analytics", "error", err, "session_id", sessionID)
			// Continue to compute fresh analytics
		}

		if analytics.IsCacheValid(cached, analytics.CurrentAnalyticsVersion, currentLineCount) {
			// Cache hit - return cached data
			respondJSON(w, http.StatusOK, cached.ToResponse())
			return
		}

		// Cache miss or stale - need to recompute
		// Get the session's user_id and external_id for S3 path
		sessionUserID, externalID, err := database.GetSessionOwnerAndExternalID(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get session info", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session info")
			return
		}

		// List all chunks for this file
		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		chunkKeys, err := store.ListChunks(storageCtx, sessionUserID, externalID, fileInfo.FileName)
		if err != nil {
			log.Error("Failed to list chunks", "error", err, "session_id", sessionID)
			respondStorageError(w, err, "Failed to list chunks")
			return
		}

		if len(chunkKeys) == 0 {
			// No chunks - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Download and merge all chunks
		// TODO: Extract common getFullFileContent() helper shared with handleCanonicalSyncFileRead
		chunks, err := downloadChunks(storageCtx, store, chunkKeys)
		if err != nil {
			log.Error("Failed to download chunks", "error", err, "session_id", sessionID)
			respondStorageError(w, err, "Failed to download file chunks")
			return
		}

		if len(chunks) == 0 {
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Merge chunks to get complete JSONL content
		content := mergeChunks(chunks)

		// Compute analytics from JSONL
		computed, err := analytics.ComputeFromJSONL(content)
		if err != nil {
			log.Error("Failed to compute analytics", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to compute analytics")
			return
		}

		// Convert to SessionAnalytics and cache
		sessionAnalytics := computed.ToSessionAnalytics(sessionID, analytics.CurrentAnalyticsVersion, currentLineCount)

		// Store in cache (errors logged but not returned - we can still return computed result)
		if err := analyticsStore.Upsert(dbCtx, sessionAnalytics); err != nil {
			log.Error("Failed to cache analytics", "error", err, "session_id", sessionID)
		}

		respondJSON(w, http.StatusOK, sessionAnalytics.ToResponse())
	}
}
