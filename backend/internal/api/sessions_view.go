package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
)

// HandleListSessions lists all sessions for the authenticated user
// Query parameters:
//   - include_shared: "true" to include sessions shared with the user (default: false)
func HandleListSessions(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Parse include_shared query parameter (default: false)
		includeShared := r.URL.Query().Get("include_shared") == "true"

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get sessions from database
		sessions, err := database.ListUserSessions(ctx, userID, includeShared)
		if err != nil {
			logger.Error("Failed to list sessions", "error", err, "user_id", userID, "include_shared", includeShared)
			respondError(w, http.StatusInternalServerError, "Failed to list sessions")
			return
		}

		// Return empty array if no sessions
		if sessions == nil {
			sessions = []db.SessionListItem{}
		}

		respondJSON(w, http.StatusOK, sessions)
	}
}

// HandleGetSession returns detailed information about a specific session
func HandleGetSession(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get session detail (includes ownership check)
		session, err := database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to get session")
			return
		}

		respondJSON(w, http.StatusOK, session)
	}
}
