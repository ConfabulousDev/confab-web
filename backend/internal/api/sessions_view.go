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
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get sessions from database
		sessions, err := database.ListUserSessions(ctx, userID)
		if err != nil {
			http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
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
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get session ID from URL
		sessionID := chi.URLParam(r, "sessionId")
		if sessionID == "" {
			http.Error(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		// Get session detail (includes ownership check)
		session, err := database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to get session", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	}
}
