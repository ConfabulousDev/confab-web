package api

import (
	"context"
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/admin"
	dbuser "github.com/ConfabulousDev/confab-web/internal/db/user"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// meResponse extends the User model with onboarding status fields
type meResponse struct {
	models.User
	HasOwnSessions bool `json:"has_own_sessions"`
	HasAPIKeys     bool `json:"has_api_keys"`
	IsAdmin        bool `json:"is_admin"`
}

// handleGetMe returns the current authenticated user's info
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	userStore := &dbuser.Store{DB: s.db}
	user, err := userStore.GetUserByID(ctx, userID)
	if err != nil {
		log.Error("Failed to get user", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

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
		IsAdmin:        admin.IsSuperAdmin(user.Email),
	})
}
