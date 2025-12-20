package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// HandleListSessions lists all sessions for the authenticated user
// Query parameters:
//   - view: "owned" (default) or "shared" to select which sessions to list
//
// Supports conditional requests via ETag/If-None-Match for efficient polling.
func HandleListSessions(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Parse view parameter
		var view db.SessionListView
		switch r.URL.Query().Get("view") {
		case "shared":
			view = db.SessionListViewSharedWithMe
		case "owned", "":
			view = db.SessionListViewOwned
		default:
			respondError(w, http.StatusBadRequest, "Invalid view parameter, must be 'owned' or 'shared'")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get last modified timestamp for ETag
		lastModified, err := database.GetSessionsLastModified(ctx, userID, view)
		if err != nil {
			logger.Error("Failed to get sessions last modified", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to list sessions")
			return
		}

		// Generate ETag from last modified timestamp
		etag := fmt.Sprintf(`"%d"`, lastModified.UnixNano())

		// Check If-None-Match header for conditional request
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Get sessions from database
		sessions, err := database.ListUserSessions(ctx, userID, view)
		if err != nil {
			logger.Error("Failed to list sessions", "error", err, "user_id", userID, "view", view)
			respondError(w, http.StatusInternalServerError, "Failed to list sessions")
			return
		}

		// Ensure non-nil slice for JSON encoding
		if sessions == nil {
			sessions = make([]db.SessionListItem, 0)
		}

		w.Header().Set("ETag", etag)
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

// UpdateSessionTitleRequest is the request body for updating a session's custom title
type UpdateSessionTitleRequest struct {
	// CustomTitle is the new title. Use nil/null to clear and revert to auto-derived title.
	CustomTitle *string `json:"custom_title"`
}

// HandleUpdateSessionTitle updates the custom title for a session
func HandleUpdateSessionTitle(database *db.DB) http.HandlerFunc {
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

		// Parse request body
		var req UpdateSessionTitleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate custom title length if provided
		if req.CustomTitle != nil && len(*req.CustomTitle) > db.MaxCustomTitleLength {
			respondError(w, http.StatusBadRequest, "Custom title exceeds maximum length of 255 characters")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Update the custom title
		err := database.UpdateSessionCustomTitle(ctx, sessionID, userID, req.CustomTitle)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrForbidden) {
				respondError(w, http.StatusForbidden, "You don't have permission to modify this session")
				return
			}
			logger.Error("Failed to update session title", "error", err, "session_id", sessionID, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to update session title")
			return
		}

		// Return the updated session
		session, err := database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			// Title was updated but failed to fetch - return success without body
			w.WriteHeader(http.StatusNoContent)
			return
		}

		respondJSON(w, http.StatusOK, session)
	}
}
