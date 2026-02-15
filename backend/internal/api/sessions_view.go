package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// HandleListSessions lists all sessions visible to the authenticated user.
// Returns owned sessions and shared sessions (private + system shares) with deduplication.
func HandleListSessions(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context (set by SessionMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get sessions from database
		sessions, err := database.ListUserSessions(ctx, userID)
		if err != nil {
			log.Error("Failed to list sessions", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to list sessions")
			return
		}

		// Ensure non-nil slice for JSON encoding
		if sessions == nil {
			sessions = make([]db.SessionListItem, 0)
		}

		respondJSON(w, http.StatusOK, sessions)
	}
}

// HandleGetSession returns detailed information about a specific session.
// Supports unified canonical access (CF-132):
// - Owner access: authenticated user who owns the session
// - Public share: anyone (no auth required)
// - System share: any authenticated user
// - Recipient share: authenticated user who is a share recipient
//
// This handler supports optional authentication - it extracts user ID from
// the session cookie if present, but doesn't require it.
func HandleGetSession(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Note: No log := here since this handler doesn't have logging calls
		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Check canonical access (CF-132 unified access model)
		result, err := CheckCanonicalAccess(ctx, database, sessionID)
		if RespondCanonicalAccessError(ctx, w, err, sessionID) {
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

		respondJSON(w, http.StatusOK, result.Session)
	}
}

// SessionLookupResponse is the response for looking up a session by external_id
type SessionLookupResponse struct {
	SessionID string `json:"session_id"`
}

// HandleLookupSessionByExternalID looks up a session's internal ID by external_id.
// This is an authenticated endpoint - users can only look up their own sessions.
// Supports both session cookie auth and API key auth (via SessionOrAPIKeyMiddleware).
func HandleLookupSessionByExternalID(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context (set by SessionOrAPIKeyMiddleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get external_id from URL
		externalID := chi.URLParam(r, "external_id")
		if externalID == "" {
			respondError(w, http.StatusBadRequest, "Missing external_id")
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Look up session
		sessionID, err := database.GetSessionIDByExternalID(ctx, externalID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to lookup session by external_id", "error", err, "external_id", externalID)
			respondError(w, http.StatusInternalServerError, "Failed to lookup session")
			return
		}

		respondJSON(w, http.StatusOK, SessionLookupResponse{SessionID: sessionID})
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
		log := logger.Ctx(r.Context())

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
			log.Error("Failed to update session title", "error", err, "session_id", sessionID)
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
