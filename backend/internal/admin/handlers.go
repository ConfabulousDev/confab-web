package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

const (
	// DatabaseTimeout is the maximum duration for database operations
	DatabaseTimeout = 5 * time.Second
)

// Handlers holds dependencies for admin handlers
type Handlers struct {
	DB      *db.DB
	Storage *storage.S3Storage
}

// NewHandlers creates admin handlers with dependencies
func NewHandlers(database *db.DB, store *storage.S3Storage) *Handlers {
	return &Handlers{
		DB:      database,
		Storage: store,
	}
}

// HandleListUsers renders the admin user list page
func (h *Handlers) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	users, err := h.DB.ListAllUsers(ctx)
	if err != nil {
		logger.Error("Failed to list users", "error", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	// Return JSON for now - HTML template will be added in Phase 6
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
	})
}

// HandleDeactivateUser sets a user's status to inactive
func (h *Handlers) HandleDeactivateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusInactive); err != nil {
		logger.Error("Failed to deactivate user", "error", err, "user_id", userID)
		http.Error(w, "Failed to deactivate user", http.StatusInternalServerError)
		return
	}

	logger.Info("User deactivated", "user_id", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("User %d deactivated", userID),
	})
}

// HandleActivateUser sets a user's status to active
func (h *Handlers) HandleActivateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	if err := h.DB.UpdateUserStatus(ctx, userID, models.UserStatusActive); err != nil {
		logger.Error("Failed to activate user", "error", err, "user_id", userID)
		http.Error(w, "Failed to activate user", http.StatusInternalServerError)
		return
	}

	logger.Info("User activated", "user_id", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("User %d activated", userID),
	})
}

// HandleDeleteUser permanently deletes a user and all their data
func (h *Handlers) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Use a longer timeout for deletion since it involves S3 operations
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Step 1: Get all session IDs for S3 cleanup
	sessionIDs, err := h.DB.GetUserSessionIDs(ctx, userID)
	if err != nil {
		logger.Error("Failed to get user sessions for deletion", "error", err, "user_id", userID)
		http.Error(w, "Failed to get user sessions", http.StatusInternalServerError)
		return
	}

	// Step 2: Delete S3 objects for each session (fail-fast: S3 before DB)
	for _, sessionID := range sessionIDs {
		if err := h.Storage.DeleteAllSessionChunks(ctx, userID, sessionID); err != nil {
			logger.Error("Failed to delete S3 objects for session", "error", err, "user_id", userID, "session_id", sessionID)
			http.Error(w, fmt.Sprintf("Failed to delete storage for session %s", sessionID), http.StatusInternalServerError)
			return
		}
	}

	// Step 3: Delete user from database (CASCADE handles related records)
	if err := h.DB.DeleteUser(ctx, userID); err != nil {
		logger.Error("Failed to delete user from database", "error", err, "user_id", userID)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	logger.Info("User permanently deleted", "user_id", userID, "sessions_deleted", len(sessionIDs))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":          true,
		"message":          fmt.Sprintf("User %d permanently deleted", userID),
		"sessions_deleted": len(sessionIDs),
	})
}

// parseUserID extracts and validates the user ID from the URL path
func parseUserID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}
