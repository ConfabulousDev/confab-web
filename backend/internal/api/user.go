package api

import (
	"context"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// meResponse extends the User model with onboarding status fields
type meResponse struct {
	models.User
	HasOwnSessions bool `json:"has_own_sessions"`
	HasAPIKeys     bool `json:"has_api_keys"`
}

// handleGetMe returns the current authenticated user's info
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	// Get user ID from session middleware
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Create context with timeout for database operation
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Get user from database
	userStore := &dbuser.Store{DB: s.db}
	user, err := userStore.GetUserByID(ctx, userID)
	if err != nil {
		log.Error("Failed to get user", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Check onboarding status
	hasOwnSessions, err := userStore.HasOwnSessions(ctx, userID)
	if err != nil {
		log.Error("Failed to check user sessions", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	hasAPIKeys, err := userStore.HasAPIKeys(ctx, userID)
	if err != nil {
		log.Error("Failed to check user API keys", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	respondJSON(w, http.StatusOK, meResponse{
		User:           *user,
		HasOwnSessions: hasOwnSessions,
		HasAPIKeys:     hasAPIKeys,
	})
}

// NOTE: handleGetWeeklyUsage removed - legacy runs-based rate limiting
