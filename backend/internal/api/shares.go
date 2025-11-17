package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
)

// CreateShareRequest is the request body for creating a share
type CreateShareRequest struct {
	Visibility     string   `json:"visibility"`      // "public" or "private"
	InvitedEmails  []string `json:"invited_emails"`  // required for private
	ExpiresInDays  *int     `json:"expires_in_days"` // null = never expires
}

// CreateShareResponse is the response for creating a share
type CreateShareResponse struct {
	ShareToken    string    `json:"share_token"`
	ShareURL      string    `json:"share_url"`
	Visibility    string    `json:"visibility"`
	InvitedEmails []string  `json:"invited_emails,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// HandleCreateShare creates a new share for a session
func HandleCreateShare(database *db.DB, frontendURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Get user ID from context
		userID, ok := ctx.Value(auth.GetUserIDContextKey()).(int64)
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

		// Parse request body
		var req CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate visibility
		if req.Visibility != "public" && req.Visibility != "private" {
			http.Error(w, "Visibility must be 'public' or 'private'", http.StatusBadRequest)
			return
		}

		// Validate private shares have invited emails
		if req.Visibility == "private" {
			if len(req.InvitedEmails) == 0 {
				http.Error(w, "Private shares require at least one invited email", http.StatusBadRequest)
				return
			}
			if len(req.InvitedEmails) > 50 {
				http.Error(w, "Maximum 50 invited emails allowed", http.StatusBadRequest)
				return
			}
			// Validate email formats (basic)
			for _, email := range req.InvitedEmails {
				email = strings.TrimSpace(email)
				if !strings.Contains(email, "@") {
					http.Error(w, "Invalid email format", http.StatusBadRequest)
					return
				}
			}
		}

		// Generate share token (UUID-like)
		shareToken, err := generateShareToken()
		if err != nil {
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
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
			if strings.Contains(err.Error(), "session not found") {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "unauthorized") {
				http.Error(w, "Unauthorized", http.StatusForbidden)
				return
			}
			http.Error(w, "Failed to create share", http.StatusInternalServerError)
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
		userID, ok := ctx.Value(auth.GetUserIDContextKey()).(int64)
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

		// Get shares from database
		shares, err := database.ListShares(ctx, sessionID, userID)
		if err != nil {
			if strings.Contains(err.Error(), "session not found") {
				http.Error(w, "Session not found", http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "unauthorized") {
				http.Error(w, "Unauthorized", http.StatusForbidden)
				return
			}
			http.Error(w, "Failed to list shares", http.StatusInternalServerError)
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
		userID, ok := ctx.Value(auth.GetUserIDContextKey()).(int64)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Get share token from URL
		shareToken := chi.URLParam(r, "shareToken")
		if shareToken == "" {
			http.Error(w, "Missing share token", http.StatusBadRequest)
			return
		}

		// Revoke share
		err := database.RevokeShare(ctx, shareToken, userID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "unauthorized") {
				http.Error(w, "Share not found or unauthorized", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to revoke share", http.StatusInternalServerError)
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
			http.Error(w, "Missing session ID or share token", http.StatusBadRequest)
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
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Share not found", http.StatusNotFound)
				return
			}
			if strings.Contains(err.Error(), "expired") {
				http.Error(w, "Share expired", http.StatusGone)
				return
			}
			if strings.Contains(err.Error(), "unauthorized") {
				http.Error(w, "Please log in to view this private share", http.StatusUnauthorized)
				return
			}
			if strings.Contains(err.Error(), "forbidden") {
				http.Error(w, "You are not authorized to view this share", http.StatusForbidden)
				return
			}
			http.Error(w, "Failed to get shared session", http.StatusInternalServerError)
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
