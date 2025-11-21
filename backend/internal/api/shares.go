package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/validation"
)

// CreateShareRequest is the request body for creating a share
type CreateShareRequest struct {
	Visibility    string   `json:"visibility"`      // "public" or "private"
	InvitedEmails []string `json:"invited_emails"`  // required for private
	ExpiresInDays *int     `json:"expires_in_days"` // null = never expires
}

// CreateShareResponse is the response for creating a share
type CreateShareResponse struct {
	ShareToken    string     `json:"share_token"`
	ShareURL      string     `json:"share_url"`
	Visibility    string     `json:"visibility"`
	InvitedEmails []string   `json:"invited_emails,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// HandleCreateShare creates a new share for a session
func HandleCreateShare(database *db.DB, frontendURL string) http.HandlerFunc {
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

		// Parse request body
		var req CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate visibility
		if req.Visibility != "public" && req.Visibility != "private" {
			respondError(w, http.StatusBadRequest, "Visibility must be 'public' or 'private'")
			return
		}

		// Validate private shares have invited emails
		if req.Visibility == "private" {
			if len(req.InvitedEmails) == 0 {
				respondError(w, http.StatusBadRequest, "Private shares require at least one invited email")
				return
			}
			if len(req.InvitedEmails) > 50 {
				respondError(w, http.StatusBadRequest, "Maximum 50 invited emails allowed")
				return
			}
			// Validate email formats
			for _, email := range req.InvitedEmails {
				if !validation.IsValidEmail(email) {
					respondError(w, http.StatusBadRequest, "Invalid email format")
					return
				}
			}
		}

		// Generate share token (UUID-like)
		shareToken, err := generateShareToken()
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}

		// Calculate expiration
		var expiresAt *time.Time
		if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
			expires := time.Now().AddDate(0, 0, *req.ExpiresInDays)
			expiresAt = &expires
		}

		// Create share in database
		share, err := database.CreateShare(ctx, sessionID, userID, shareToken, req.Visibility, expiresAt, req.InvitedEmails)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to create share")
			return
		}

		// Build share URL
		shareURL := frontendURL + "/sessions/" + sessionID + "/shared/" + shareToken

		// Return response
		response := CreateShareResponse{
			ShareToken:    share.ShareToken,
			ShareURL:      shareURL,
			Visibility:    share.Visibility,
			InvitedEmails: share.InvitedEmails,
			ExpiresAt:     share.ExpiresAt,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleListShares lists all shares for a session
func HandleListShares(database *db.DB) http.HandlerFunc {
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

		// Get shares from database
		shares, err := database.ListShares(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Return empty array if no shares
		if shares == nil {
			shares = []db.SessionShare{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(shares)
	}
}

// HandleRevokeShare revokes a share
func HandleRevokeShare(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context
		userID, ok := auth.GetUserID(ctx)
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get share token from URL
		shareToken := chi.URLParam(r, "shareToken")
		if shareToken == "" {
			respondError(w, http.StatusBadRequest, "Missing share token")
			return
		}

		// Revoke share
		err := database.RevokeShare(ctx, shareToken, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusNotFound, "Share not found or unauthorized")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to revoke share")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleGetSharedSession returns a session accessed via share link
func HandleGetSharedSession(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get session ID and share token from URL
		sessionID := chi.URLParam(r, "sessionId")
		shareToken := chi.URLParam(r, "shareToken")

		if sessionID == "" || shareToken == "" {
			respondError(w, http.StatusBadRequest, "Missing session ID or share token")
			return
		}

		// Try to get viewer email from session (for private shares)
		var viewerEmail *string
		cookie, err := r.Cookie("confab_session")
		if err == nil {
			// User is logged in, get their email
			session, err := database.GetWebSession(ctx, cookie.Value)
			if err == nil {
				user, err := database.GetUserByID(ctx, session.UserID)
				if err == nil {
					viewerEmail = &user.Email
				}
			}
		}

		// Get shared session
		session, err := database.GetSharedSession(ctx, sessionID, shareToken, viewerEmail)
		if err != nil {
			if errors.Is(err, db.ErrShareNotFound) || errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Share not found")
				return
			}
			if errors.Is(err, db.ErrShareExpired) {
				respondError(w, http.StatusGone, "Share expired")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusUnauthorized, "Please log in to view this private share")
				return
			}
			if errors.Is(err, db.ErrForbidden) {
				respondError(w, http.StatusForbidden, "You are not authorized to view this share")
				return
			}
			respondError(w, http.StatusInternalServerError, "Failed to get shared session")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(session)
	}
}

// generateShareToken generates a random share token
func generateShareToken() (string, error) {
	bytes := make([]byte, 16) // 16 bytes = 32 hex chars
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
