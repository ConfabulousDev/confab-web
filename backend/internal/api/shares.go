package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/db"
	"github.com/santaclaude2025/confab/backend/internal/logger"
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
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session PK from URL (UUID)
		sessionPK := chi.URLParam(r, "sessionId")
		if sessionPK == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
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
			expires := time.Now().UTC().AddDate(0, 0, *req.ExpiresInDays)
			expiresAt = &expires
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Create share in database
		share, err := database.CreateShare(ctx, sessionPK, userID, shareToken, req.Visibility, expiresAt, req.InvitedEmails)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			logger.Error("Failed to create share", "error", err, "user_id", userID, "session_pk", sessionPK)
			respondError(w, http.StatusInternalServerError, "Failed to create share")
			return
		}

		// Build share URL (uses session PK in URL)
		shareURL := frontendURL + "/sessions/" + sessionPK + "/shared/" + shareToken

		// Audit log: Share created
		logger.Info("Share created",
			"user_id", userID,
			"session_pk", sessionPK,
			"share_token", shareToken,
			"visibility", share.Visibility,
			"invited_emails_count", len(share.InvitedEmails),
			"expires_at", share.ExpiresAt)

		// Return response
		response := CreateShareResponse{
			ShareToken:    share.ShareToken,
			ShareURL:      shareURL,
			Visibility:    share.Visibility,
			InvitedEmails: share.InvitedEmails,
			ExpiresAt:     share.ExpiresAt,
		}

		respondJSON(w, http.StatusOK, response)
	}
}

// HandleListShares lists all shares for a session
func HandleListShares(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session PK from URL (UUID)
		sessionPK := chi.URLParam(r, "sessionId")
		if sessionPK == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get shares from database
		shares, err := database.ListShares(ctx, sessionPK, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			logger.Error("Failed to list shares", "error", err, "user_id", userID, "session_pk", sessionPK)
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Success log
		logger.Info("Shares listed", "user_id", userID, "session_pk", sessionPK, "count", len(shares))

		// Return empty array if no shares
		if shares == nil {
			shares = []db.SessionShare{}
		}

		respondJSON(w, http.StatusOK, shares)
	}
}

// HandleRevokeShare revokes a share
func HandleRevokeShare(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get share token from URL
		shareToken := chi.URLParam(r, "shareToken")
		if err := validation.ValidateShareToken(shareToken); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Revoke share
		err := database.RevokeShare(ctx, shareToken, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusNotFound, "Share not found or unauthorized")
				return
			}
			logger.Error("Failed to revoke share", "error", err, "user_id", userID, "share_token", shareToken)
			respondError(w, http.StatusInternalServerError, "Failed to revoke share")
			return
		}

		// Audit log: Share revoked
		logger.Info("Share revoked", "user_id", userID, "share_token", shareToken)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleGetSharedSession returns a session accessed via share link
func HandleGetSharedSession(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get session PK (UUID) and share token from URL
		sessionPK := chi.URLParam(r, "sessionId")
		shareToken := chi.URLParam(r, "shareToken")

		// Validate session PK
		if sessionPK == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Validate share token
		if err := validation.ValidateShareToken(shareToken); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Create context with timeout for database operations
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Try to get viewer email from session (for private shares)
		var viewerEmail *string
		cookie, err := r.Cookie("confab_session")
		if err == nil {
			// User is logged in, get their email
			webSession, err := database.GetWebSession(ctx, cookie.Value)
			if err == nil {
				user, err := database.GetUserByID(ctx, webSession.UserID)
				if err == nil {
					viewerEmail = &user.Email
				}
			}
		}

		// Get shared session
		session, err := database.GetSharedSession(ctx, sessionPK, shareToken, viewerEmail)
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
			logger.Error("Failed to get shared session", "error", err, "session_pk", sessionPK, "share_token", shareToken)
			respondError(w, http.StatusInternalServerError, "Failed to get shared session")
			return
		}

		// Audit log: Shared session accessed
		logger.Info("Shared session accessed",
			"session_pk", sessionPK,
			"share_token", shareToken,
			"viewer_email", viewerEmail)

		// If user is logged in, record their access for later retrieval in session list
		if viewerEmail != nil {
			// Get user ID from session cookie (we already validated it above)
			cookie, _ := r.Cookie("confab_session")
			webSession, err := database.GetWebSession(ctx, cookie.Value)
			if err == nil {
				// Record that this user accessed this share
				// This allows the share to appear in their session list later
				recordCtx, recordCancel := context.WithTimeout(context.Background(), DatabaseTimeout)
				defer recordCancel()
				if err := database.RecordShareAccess(recordCtx, shareToken, webSession.UserID); err != nil {
					// Log but don't fail the request
					logger.Error("Failed to record share access", "error", err, "share_token", shareToken, "user_id", webSession.UserID)
				}
			}
		}

		respondJSON(w, http.StatusOK, session)
	}
}

// HandleListAllUserShares lists all shares for the authenticated user across all sessions
func HandleListAllUserShares(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get all shares from database
		shares, err := database.ListAllUserShares(ctx, userID)
		if err != nil {
			logger.Error("Failed to list all user shares", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Success log
		logger.Info("All user shares listed", "user_id", userID, "count", len(shares))

		// Return empty array if no shares
		if shares == nil {
			shares = []db.ShareWithSessionInfo{}
		}

		respondJSON(w, http.StatusOK, shares)
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
