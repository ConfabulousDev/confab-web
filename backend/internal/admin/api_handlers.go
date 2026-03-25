package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/ConfabulousDev/confab-web/internal/db"
	dbaccess "github.com/ConfabulousDev/confab-web/internal/db/access"
	"github.com/ConfabulousDev/confab-web/internal/db/dbauth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/httputil"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/recapquota"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// AdminUserListResponse is the response for GET /api/v1/admin/users
type AdminUserListResponse struct {
	Users  []AdminUserJSON `json:"users"`
	Totals AdminTotals     `json:"totals"`
}

// AdminUserJSON represents a user in the admin user list
type AdminUserJSON struct {
	ID              int64   `json:"id"`
	Email           string  `json:"email"`
	Name            *string `json:"name"`
	Status          string  `json:"status"`
	SessionCount    int     `json:"session_count"`
	RecapCacheCount int     `json:"recap_cache_count"`
	RecapsThisMonth int     `json:"recaps_this_month"`
	LastAPIKeyUsed  *string `json:"last_api_key_used"`
	LastLoggedIn    *string `json:"last_logged_in"`
	CreatedAt       string  `json:"created_at"`
}

// AdminTotals are system-wide aggregate stats
type AdminTotals struct {
	TotalSessions         int `json:"total_sessions"`
	NonEmptySessions      int `json:"non_empty_sessions"`
	SessionsWithCache     int `json:"sessions_with_cache"`
	ComputationsThisMonth int `json:"computations_this_month"`
}

// CreateUserRequest is the request body for POST /api/v1/admin/users
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateUserResponse is the response for POST /api/v1/admin/users
type CreateUserResponse struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// StatusChangeResponse is the response for activate/deactivate endpoints
type StatusChangeResponse struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// SystemShareJSON represents a system share in the admin list
type SystemShareJSON struct {
	ID             int64   `json:"id"`
	SessionID      string  `json:"session_id"`
	ExternalID     string  `json:"external_id"`
	ShareURL       string  `json:"share_url"`
	ExpiresAt      *string `json:"expires_at"`
	CreatedAt      string  `json:"created_at"`
	LastAccessedAt *string `json:"last_accessed_at"`
}

// SystemSharesResponse is the response for GET /api/v1/admin/system-shares
type SystemSharesResponse struct {
	Shares []SystemShareJSON `json:"shares"`
}

// CreateSystemShareRequest is the request body for POST /api/v1/admin/system-shares
type CreateSystemShareRequest struct {
	SessionID string `json:"session_id"`
}

// CreateSystemShareResponse is the response for POST /api/v1/admin/system-shares
type CreateSystemShareResponse struct {
	ShareID    int64  `json:"share_id"`
	ExternalID string `json:"external_id"`
	ShareURL   string `json:"share_url"`
}

// HandleListUsersAPI returns the admin user list as JSON
func (h *Handlers) HandleListUsersAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	userStore := &dbuser.Store{DB: h.DB}

	users, err := userStore.ListAllUsers(ctx)
	if err != nil {
		log.Error("Failed to list users", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	recapStats, err := recapquota.ListUserStats(ctx, h.DB.Conn())
	if err != nil {
		log.Error("Failed to list smart recap stats", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to load smart recap stats")
		return
	}

	recapStatsByUser := make(map[int64]recapquota.UserStats)
	for _, stat := range recapStats {
		recapStatsByUser[stat.UserID] = stat
	}

	recapTotals, err := recapquota.GetTotals(ctx, h.DB.Conn())
	if err != nil {
		log.Error("Failed to get smart recap totals", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to load smart recap totals")
		return
	}

	var totalSessions int
	apiUsers := make([]AdminUserJSON, 0, len(users))
	for _, user := range users {
		totalSessions += user.SessionCount
		recapStat := recapStatsByUser[user.ID]

		apiUsers = append(apiUsers, AdminUserJSON{
			ID:              user.ID,
			Email:           user.Email,
			Name:            user.Name,
			Status:          string(user.Status),
			SessionCount:    user.SessionCount,
			RecapCacheCount: recapStat.SessionsWithCache,
			RecapsThisMonth: recapStat.ComputationsThisMonth,
			LastAPIKeyUsed:  formatTimePtr(user.LastAPIKeyUsed),
			LastLoggedIn:    formatTimePtr(user.LastLoggedIn),
			CreatedAt:       user.CreatedAt.Format(time.RFC3339),
		})
	}

	httputil.RespondJSON(w, http.StatusOK, AdminUserListResponse{
		Users: apiUsers,
		Totals: AdminTotals{
			TotalSessions:         totalSessions,
			NonEmptySessions:      recapTotals.TotalNonEmptySessions,
			SessionsWithCache:     recapTotals.TotalSessionsWithCache,
			ComputationsThisMonth: recapTotals.TotalComputationsThisMonth,
		},
	})
}

// HandleCreateUserAPI creates a new user with password authentication
func (h *Handlers) HandleCreateUserAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	email := validation.NormalizeEmail(req.Email)
	if !validation.IsValidEmail(email) {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid email address")
		return
	}

	if !validation.IsAllowedEmailDomain(email, h.AllowedEmailDomains) {
		httputil.RespondError(w, http.StatusBadRequest, "Email domain not permitted")
		return
	}

	if len(req.Password) < 8 {
		httputil.RespondError(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}
	if len(req.Password) > 1024 {
		httputil.RespondError(w, http.StatusBadRequest, "Password too long")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		log.Error("Failed to hash password", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	authStore := &dbauth.Store{DB: h.DB}
	user, err := authStore.CreatePasswordUser(ctx, email, string(passwordHash), false)
	if err != nil {
		log.Error("Failed to create user", "error", err, "email", email)
		if strings.Contains(err.Error(), "already exists") {
			httputil.RespondError(w, http.StatusConflict, "User with this email already exists")
		} else {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	AuditLogFromRequest(r, h.DB, ActionUserCreate, map[string]interface{}{
		"created_user_id":    user.ID,
		"created_user_email": email,
	})

	httputil.RespondJSON(w, http.StatusOK, CreateUserResponse{
		ID:    user.ID,
		Email: user.Email,
	})
}

// HandleDeactivateUserAPI sets a user's status to inactive
func (h *Handlers) HandleDeactivateUserAPI(w http.ResponseWriter, r *http.Request) {
	h.setUserStatusAPI(w, r, "inactive", ActionUserDeactivate)
}

// HandleActivateUserAPI sets a user's status to active
func (h *Handlers) HandleActivateUserAPI(w http.ResponseWriter, r *http.Request) {
	h.setUserStatusAPI(w, r, "active", ActionUserActivate)
}

func (h *Handlers) setUserStatusAPI(w http.ResponseWriter, r *http.Request, status string, action AdminAction) {
	log := logger.Ctx(r.Context())

	userID, err := parseUserID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	userStore := &dbuser.Store{DB: h.DB}

	var targetEmail string
	if targetUser, err := userStore.GetUserByID(ctx, userID); err == nil {
		targetEmail = targetUser.Email
	}

	modelStatus := models.UserStatus(status)
	if err := userStore.UpdateUserStatus(ctx, userID, modelStatus); err != nil {
		log.Error("Failed to update user status", "error", err, "user_id", userID, "status", status)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to update user status")
		return
	}

	AuditLogFromRequest(r, h.DB, action, map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": targetEmail,
	})

	httputil.RespondJSON(w, http.StatusOK, StatusChangeResponse{
		ID:     userID,
		Status: status,
	})
}

// HandleDeleteUserAPI permanently deletes a user and all their data.
func (h *Handlers) HandleDeleteUserAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, err := parseUserID(r)
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Longer timeout for S3 cleanup
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	userStore := &dbuser.Store{DB: h.DB}

	targetUser, err := userStore.GetUserByID(ctx, userID)
	if err != nil {
		httputil.RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	// Delete all S3 objects for this user (prefix: {userID}/) before DB deletion
	if err := h.Storage.DeleteAllUserData(ctx, userID); err != nil {
		log.Error("Failed to delete S3 data for user", "error", err, "user_id", userID)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete storage")
		return
	}

	if err := userStore.DeleteUser(ctx, userID); err != nil {
		log.Error("Failed to delete user from database", "error", err, "user_id", userID)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	AuditLogFromRequest(r, h.DB, ActionUserDelete, map[string]interface{}{
		"target_user_id":    userID,
		"target_user_email": targetUser.Email,
	})

	w.WriteHeader(http.StatusNoContent)
}

// HandleListSystemSharesAPI returns all system-wide shares
func (h *Handlers) HandleListSystemSharesAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	accessStore := &dbaccess.Store{DB: h.DB}
	shares, err := accessStore.ListSystemShares(ctx)
	if err != nil {
		log.Error("Failed to list system shares", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to list system shares")
		return
	}

	apiShares := make([]SystemShareJSON, 0, len(shares))
	for _, share := range shares {
		apiShares = append(apiShares, SystemShareJSON{
			ID:             share.ID,
			SessionID:      share.SessionID,
			ExternalID:     share.ExternalID,
			ShareURL:       h.FrontendURL + "/sessions/" + share.SessionID,
			ExpiresAt:      formatTimePtr(share.ExpiresAt),
			LastAccessedAt: formatTimePtr(share.LastAccessedAt),
			CreatedAt:      share.CreatedAt.Format(time.RFC3339),
		})
	}

	httputil.RespondJSON(w, http.StatusOK, SystemSharesResponse{
		Shares: apiShares,
	})
}

// HandleCreateSystemShareAPI creates a new system-wide share
func (h *Handlers) HandleCreateSystemShareAPI(w http.ResponseWriter, r *http.Request) {
	if !h.SharesEnabled {
		httputil.RespondError(w, http.StatusBadRequest, "Share creation is not enabled")
		return
	}

	log := logger.Ctx(r.Context())

	var req CreateSystemShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.SessionID == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Session ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	accessStore := &dbaccess.Store{DB: h.DB}
	share, err := accessStore.CreateSystemShare(ctx, req.SessionID, nil)
	if err != nil {
		if err == db.ErrSessionNotFound {
			httputil.RespondError(w, http.StatusNotFound, "Session not found")
			return
		}
		log.Error("Failed to create system share", "error", err, "session_id", req.SessionID)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to create system share")
		return
	}

	AuditLogFromRequest(r, h.DB, ActionSystemShareCreate, map[string]interface{}{
		"session_id":  req.SessionID,
		"share_id":    share.ID,
		"external_id": share.ExternalID,
	})

	httputil.RespondJSON(w, http.StatusOK, CreateSystemShareResponse{
		ShareID:    share.ID,
		ExternalID: share.ExternalID,
		ShareURL:   h.FrontendURL + "/sessions/" + req.SessionID,
	})
}

func parseUserID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
