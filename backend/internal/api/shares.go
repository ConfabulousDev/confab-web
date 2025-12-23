package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// CreateShareRequest is the request body for creating a share
type CreateShareRequest struct {
	IsPublic          bool     `json:"is_public"`          // true for public (anyone with link), false for recipients only
	Recipients        []string `json:"recipients"`         // email addresses (required if not public)
	ExpiresInDays     *int     `json:"expires_in_days"`    // null = never expires
	SkipNotifications bool     `json:"skip_notifications"` // skip sending invitation emails (default: false)
}

// CreateShareResponse is the response for creating a share
type CreateShareResponse struct {
	ShareURL      string     `json:"share_url"`
	IsPublic      bool       `json:"is_public"`
	Recipients    []string   `json:"recipients,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	EmailsSent    bool       `json:"emails_sent"`              // True if all invitation emails were sent successfully
	EmailFailures []string   `json:"email_failures,omitempty"` // List of emails that failed to send
}

// HandleCreateShare creates a new share for a session
func HandleCreateShare(database *db.DB, frontendURL string, emailService *email.RateLimitedService) http.HandlerFunc {
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
		var req CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Get sharer info early so we can validate against self-invite
		sharerCtx, sharerCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer sharerCancel()
		sharer, err := database.GetUserByID(sharerCtx, userID)
		if err != nil {
			logger.Error("Failed to get sharer info", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to get user info")
			return
		}

		// Validate recipient shares have recipients
		if !req.IsPublic {
			if len(req.Recipients) == 0 {
				respondError(w, http.StatusBadRequest, "Non-public shares require at least one recipient email")
				return
			}
			if len(req.Recipients) > 50 {
				respondError(w, http.StatusBadRequest, "Maximum 50 recipients allowed")
				return
			}
			// Validate email formats and check for self-invite
			sharerEmailLower := strings.ToLower(sharer.Email)
			for _, recipientEmail := range req.Recipients {
				if !validation.IsValidEmail(recipientEmail) {
					respondError(w, http.StatusBadRequest, "Invalid email format")
					return
				}
				if strings.ToLower(recipientEmail) == sharerEmailLower {
					respondError(w, http.StatusBadRequest, "You cannot share with yourself")
					return
				}
			}
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

		// Get session info for email (title)
		session, err := database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			logger.Error("Failed to get session info", "error", err, "user_id", userID, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session info")
			return
		}

		// Create share in database
		share, err := database.CreateShare(ctx, sessionID, userID, req.IsPublic, expiresAt, req.Recipients)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			logger.Error("Failed to create share", "error", err, "user_id", userID, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to create share")
			return
		}

		// Build canonical share URL (CF-132: no token in URL)
		shareURL := frontendURL + "/sessions/" + sessionID

		// Send invitation emails for recipient shares (unless skipped)
		var emailsSent bool
		var emailFailures []string
		if !req.IsPublic && !req.SkipNotifications && emailService != nil && len(req.Recipients) > 0 {
			emailsSent = true
			sharerName := sharer.Email // Default to email
			if sharer.Name != nil && *sharer.Name != "" {
				sharerName = *sharer.Name
			}

			// Get session title (use summary, then first_user_message, then external ID as fallback)
			sessionTitle := session.ExternalID
			if session.Summary != nil && *session.Summary != "" {
				sessionTitle = *session.Summary
			} else if session.FirstUserMessage != nil && *session.FirstUserMessage != "" {
				sessionTitle = *session.FirstUserMessage
			}

			for _, toEmail := range req.Recipients {
				// Include recipient email in URL so login flow can guide them to the right account
				recipientShareURL := shareURL + "?email=" + url.QueryEscape(toEmail)
				emailParams := email.ShareInvitationParams{
					ToEmail:      toEmail,
					SharerName:   sharerName,
					SharerEmail:  sharer.Email,
					SessionTitle: sessionTitle,
					ShareURL:     recipientShareURL,
					ExpiresAt:    expiresAt,
				}

				if err := emailService.SendShareInvitation(r.Context(), userID, emailParams); err != nil {
					logger.Error("Failed to send share invitation email",
						"error", err,
						"to_email", toEmail,
						"share_id", share.ID)
					emailFailures = append(emailFailures, toEmail)
					emailsSent = false
				} else {
					logger.Info("Share invitation email sent",
						"to_email", toEmail,
						"share_id", share.ID)
				}
			}
		}

		// Audit log: Share created
		logger.Info("Share created",
			"user_id", userID,
			"session_id", sessionID,
			"share_id", share.ID,
			"is_public", share.IsPublic,
			"recipients_count", len(share.Recipients),
			"expires_at", share.ExpiresAt,
			"skip_notifications", req.SkipNotifications,
			"emails_sent", emailsSent,
			"email_failures_count", len(emailFailures))

		// Return response
		response := CreateShareResponse{
			ShareURL:      shareURL,
			IsPublic:      share.IsPublic,
			Recipients:    share.Recipients,
			ExpiresAt:     share.ExpiresAt,
			EmailsSent:    emailsSent && len(emailFailures) == 0,
			EmailFailures: emailFailures,
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

		// Get session ID from URL (UUID)
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

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
			logger.Error("Failed to list shares", "error", err, "user_id", userID, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Success log
		logger.Info("Shares listed", "user_id", userID, "session_id", sessionID, "count", len(shares))

		respondJSON(w, http.StatusOK, shares)
	}
}

// HandleRevokeShare revokes a share by ID
func HandleRevokeShare(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get share ID from URL
		shareIDStr := chi.URLParam(r, "shareID")
		shareID, err := strconv.ParseInt(shareIDStr, 10, 64)
		if err != nil || shareID <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid share ID")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Revoke share
		err = database.RevokeShare(ctx, shareID, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusNotFound, "Share not found or unauthorized")
				return
			}
			logger.Error("Failed to revoke share", "error", err, "user_id", userID, "share_id", shareID)
			respondError(w, http.StatusInternalServerError, "Failed to revoke share")
			return
		}

		// Audit log: Share revoked
		logger.Info("Share revoked", "user_id", userID, "share_id", shareID)

		w.WriteHeader(http.StatusNoContent)
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

		respondJSON(w, http.StatusOK, shares)
	}
}

// getViewerUserIDFromSession extracts the viewer's user ID from their session cookie if authenticated.
// Returns nil if no valid session cookie or any lookup fails.
func getViewerUserIDFromSession(ctx context.Context, r *http.Request, database *db.DB) *int64 {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return nil
	}
	webSession, err := database.GetWebSession(ctx, cookie.Value)
	if err != nil {
		return nil
	}
	return &webSession.UserID
}
