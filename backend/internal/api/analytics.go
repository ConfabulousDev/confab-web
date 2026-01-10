package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

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

		// Collect transcript and agent files
		var mainFile *db.SyncFileDetail
		var agentFiles []db.SyncFileDetail
		for i := range session.Files {
			switch session.Files[i].FileType {
			case "transcript":
				mainFile = &session.Files[i]
			case "agent":
				agentFiles = append(agentFiles, session.Files[i])
			}
		}
		if mainFile == nil {
			// No transcript file - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Current state for cache validation (sum of all file line counts)
		totalLineCount := int64(mainFile.LastSyncedLine)
		for _, af := range agentFiles {
			totalLineCount += int64(af.LastSyncedLine)
		}

		// Parse optional as_of_line query parameter for conditional requests
		// If client already has analytics up to the current line count, return 304
		if asOfLineStr := r.URL.Query().Get("as_of_line"); asOfLineStr != "" {
			asOfLine, err := strconv.ParseInt(asOfLineStr, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "as_of_line must be a valid integer")
				return
			}
			if asOfLine < 0 {
				respondError(w, http.StatusBadRequest, "as_of_line must be non-negative")
				return
			}
			// Client already has analytics up to or past current line count - no new data
			if asOfLine >= totalLineCount {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		// Check if we have valid cached cards
		cached, err := analyticsStore.GetCards(dbCtx, sessionID)
		if err != nil {
			log.Error("Failed to get cached cards", "error", err, "session_id", sessionID)
			// Continue to compute fresh analytics
		}

		if cached.AllValid(totalLineCount) {
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

		storageCtx, storageCancel := context.WithTimeout(r.Context(), StorageTimeout)
		defer storageCancel()

		// Download main transcript
		mainContent, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, mainFile.FileName)
		if err != nil {
			respondStorageError(w, err, "Failed to download transcript")
			return
		}
		if mainContent == nil {
			// No chunks - return empty analytics
			respondJSON(w, http.StatusOK, &analytics.AnalyticsResponse{})
			return
		}

		// Download agent files
		agentContents := make(map[string][]byte)
		for _, af := range agentFiles {
			agentID := extractAgentID(af.FileName)
			if agentID == "" {
				continue
			}
			content, err := downloadAndMergeFile(storageCtx, store, sessionUserID, externalID, af.FileName)
			if err != nil {
				// Log but continue - graceful degradation
				log.Warn("Failed to download agent file", "error", err, "file", af.FileName)
				continue
			}
			if content != nil {
				agentContents[agentID] = content
			}
		}

		// Build FileCollection with agents
		fc, err := analytics.NewFileCollectionWithAgents(mainContent, agentContents)
		if err != nil {
			log.Error("Failed to parse transcript", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to process session data")
			return
		}

		// Compute analytics from FileCollection
		computed, err := analytics.ComputeFromFileCollection(storageCtx, fc)
		if err != nil {
			log.Error("Failed to compute analytics", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to compute analytics")
			return
		}

		// Log validation errors if any
		if computed.ValidationErrorCount > 0 {
			log.Warn("Transcript validation errors detected",
				"session_id", sessionID,
				"validation_error_count", computed.ValidationErrorCount,
			)
		}

		// Convert to Cards and cache
		cards := computed.ToCards(sessionID, totalLineCount)

		// Store in cache (errors logged but not returned - we can still return computed result)
		if err := analyticsStore.UpsertCards(dbCtx, cards); err != nil {
			log.Error("Failed to cache cards", "error", err, "session_id", sessionID)
		}

		// Build response with validation error count
		response := cards.ToResponse()
		response.ValidationErrorCount = computed.ValidationErrorCount

		respondJSON(w, http.StatusOK, response)
	}
}

// downloadAndMergeFile downloads and merges all chunks for a file.
// Returns nil content if no chunks exist (not an error).
func downloadAndMergeFile(ctx context.Context, store *storage.S3Storage, userID int64, externalID, fileName string) ([]byte, error) {
	chunkKeys, err := store.ListChunks(ctx, userID, externalID, fileName)
	if err != nil {
		return nil, err
	}
	if len(chunkKeys) == 0 {
		return nil, nil
	}

	chunks, err := downloadChunks(ctx, store, chunkKeys)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	return mergeChunks(chunks)
}

// extractAgentID extracts the agent ID from a filename like "agent-{id}.jsonl".
// Returns empty string if the filename doesn't match the expected pattern.
func extractAgentID(fileName string) string {
	if !strings.HasPrefix(fileName, "agent-") || !strings.HasSuffix(fileName, ".jsonl") {
		return ""
	}
	// Remove "agent-" prefix and ".jsonl" suffix
	return strings.TrimSuffix(strings.TrimPrefix(fileName, "agent-"), ".jsonl")
}
