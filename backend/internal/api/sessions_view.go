package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
)

// HandleListSessions lists all sessions for the authenticated user
func HandleListSessions(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get sessions from database
		sessions, err := database.ListUserSessions(ctx, userID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to list sessions")
			return
		}

		// Return empty array if no sessions
		if sessions == nil {
			sessions = []db.SessionListItem{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}

// HandleGetSession returns detailed information about a specific session
func HandleGetSession(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session ID from URL
		sessionID := chi.URLParam(r, "sessionId")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Missing session ID")
			return
		}

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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	}
}
