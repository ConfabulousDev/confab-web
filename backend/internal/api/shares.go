package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbaccess "github.com/ConfabulousDev/confab-web/internal/db/access"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/email"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// MaxShareRecipients is the maximum number of recipients allowed per share
const MaxShareRecipients = 50

// defaultShareDailyQuota is the default per-user cap on shares created in a
// rolling 24h window (CF-429 / H2). Overridable via SHARE_DAILY_QUOTA. Guards
// storage exhaustion (unbounded share + recipient rows) from a scripted abuser.
const defaultShareDailyQuota = 100

// shareQuotaWindow is the rolling window the daily share-creation quota counts
// over.
const shareQuotaWindow = 24 * time.Hour

// shareDailyQuotaFromEnv resolves the per-user daily share-creation quota from
// SHARE_DAILY_QUOTA, falling back to defaultShareDailyQuota. A value of 0
// disables the cap; a negative or non-numeric value is rejected at startup so a
// misconfiguration fails loud instead of silently disabling the guard.
func shareDailyQuotaFromEnv() int {
	raw := os.Getenv("SHARE_DAILY_QUOTA")
	if raw == "" {
		return defaultShareDailyQuota
	}
	quota, err := strconv.Atoi(raw)
	if err != nil || quota < 0 {
		logger.Fatal("invalid SHARE_DAILY_QUOTA", "value", raw)
	}
	return quota
}

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

// HandleCreateShare creates a new share for a session. shareDailyQuota caps the
// number of shares a single user may create in a rolling 24h window (CF-429 /
// H2); a value <= 0 disables the cap.
func HandleCreateShare(database *db.DB, frontendURL string, emailService *email.RateLimitedService, sharesEnabled bool, shareDailyQuota int) http.HandlerFunc {
	userStore := &dbuser.Store{DB: database}
	sessionStore := &dbsession.Store{DB: database}
	accessStore := &dbaccess.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		if !sharesEnabled {
			respondError(w, http.StatusForbidden, "Share creation is disabled by the administrator")
			return
		}

		log := logger.Ctx(r.Context())

		// Get user ID from context
		userID, ok := requireUserID(w, r)
		if !ok {
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
		sharer, err := userStore.GetUserByID(sharerCtx, userID)
		if err != nil {
			log.Error("Failed to get sharer info", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to get user info")
			return
		}

		// Validate recipient shares have recipients
		if !req.IsPublic {
			if len(req.Recipients) == 0 {
				respondError(w, http.StatusBadRequest, "Non-public shares require at least one recipient email")
				return
			}
			if len(req.Recipients) > MaxShareRecipients {
				respondError(w, http.StatusBadRequest, fmt.Sprintf("Maximum %d recipients allowed", MaxShareRecipients))
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

		// Abuse guards (CF-429). Both run BEFORE CreateShare so a rejected
		// request leaves no share/recipient rows behind and sends no email.

		// H2: per-user daily share-creation quota. Counting live rows is
		// durable across restarts and multi-instance-safe.
		if shareDailyQuota > 0 {
			quotaCtx, quotaCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
			count, err := accessStore.CountUserSharesSince(quotaCtx, userID, time.Now().Add(-shareQuotaWindow))
			quotaCancel()
			if err != nil {
				log.Error("Failed to check share creation quota", "error", err, "user_id", userID)
				respondError(w, http.StatusInternalServerError, "Failed to check share quota")
				return
			}
			if count >= shareDailyQuota {
				log.Warn("Share creation quota exceeded",
					"user_id", userID,
					"reason", "share_daily_quota",
					"count", count,
					"quota", shareDailyQuota)
				respondError(w, http.StatusTooManyRequests, "Daily share creation limit reached. Please try again later.")
				return
			}
		}

		// H1: reject a whole multi-recipient batch up front if it would exceed
		// the sender's remaining email quota, so a 50-recipient share can't
		// partially drain the shared Resend quota. CheckRateLimit only checks
		// (no record), so this does not double-count against the per-send loop.
		if !req.IsPublic && !req.SkipNotifications && emailService != nil && len(req.Recipients) > 0 {
			if err := emailService.CheckRateLimit(userID, len(req.Recipients)); err != nil {
				log.Warn("Share invitation email quota exceeded",
					"user_id", userID,
					"reason", "email_rate_limit",
					"recipients", len(req.Recipients))
				respondError(w, http.StatusTooManyRequests, "Sending these invitations would exceed your email rate limit. Please try again later.")
				return
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
		session, err := sessionStore.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to get session info", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get session info")
			return
		}

		// Create share in database
		share, err := accessStore.CreateShare(ctx, sessionID, userID, req.IsPublic, expiresAt, req.Recipients)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			log.Error("Failed to create share", "error", err, "session_id", sessionID)
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

			// Resolve provider once: GetSessionDetail should already return
			// the canonical form (per CLAUDE.md, every Scan site normalises),
			// but applying NormalizeProvider here is defensive and matches
			// the project convention for boundary-layer code.
			provider := models.NormalizeProvider(session.Provider)
			shareIDStr := strconv.FormatInt(share.ID, 10)
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
					Provider:     provider,
					ShareID:      shareIDStr,
				}

				if err := emailService.SendShareInvitation(r.Context(), userID, emailParams); err != nil {
					log.Error("Failed to send share invitation email",
						"error", err,
						"to_email", toEmail,
						"share_id", share.ID)
					emailFailures = append(emailFailures, toEmail)
					emailsSent = false
				} else {
					log.Info("Share invitation email sent",
						"to_email", toEmail,
						"share_id", share.ID)
				}
			}
		}

		// Audit log: Share created
		log.Info("Share created",
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
	accessStore := &dbaccess.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context
		userID, ok := requireUserID(w, r)
		if !ok {
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
		shares, err := accessStore.ListShares(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusForbidden, "Unauthorized")
				return
			}
			log.Error("Failed to list shares", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Success log
		log.Info("Shares listed", "session_id", sessionID, "count", len(shares))

		respondJSON(w, http.StatusOK, shares)
	}
}

// HandleRevokeShare revokes a share by ID
func HandleRevokeShare(database *db.DB) http.HandlerFunc {
	accessStore := &dbaccess.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context
		userID, ok := requireUserID(w, r)
		if !ok {
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
		err = accessStore.RevokeShare(ctx, shareID, userID)
		if err != nil {
			if errors.Is(err, db.ErrUnauthorized) {
				respondError(w, http.StatusNotFound, "Share not found or unauthorized")
				return
			}
			log.Error("Failed to revoke share", "error", err, "share_id", shareID)
			respondError(w, http.StatusInternalServerError, "Failed to revoke share")
			return
		}

		// Audit log: Share revoked
		log.Info("Share revoked", "share_id", shareID)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleListAllUserShares lists all shares for the authenticated user across all sessions
func HandleListAllUserShares(database *db.DB) http.HandlerFunc {
	accessStore := &dbaccess.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Get all shares from database
		shares, err := accessStore.ListAllUserShares(ctx, userID)
		if err != nil {
			log.Error("Failed to list all user shares", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to list shares")
			return
		}

		// Success log
		log.Info("All user shares listed", "count", len(shares))

		respondJSON(w, http.StatusOK, shares)
	}
}

