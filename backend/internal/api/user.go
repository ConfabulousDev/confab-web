package api

import (
	"net/http"

	"github.com/santaclaude2025/confab/backend/internal/auth"
)

// handleGetMe returns the current authenticated user's info
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from session middleware
	userID, ok := auth.GetUserID(ctx)
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Get user from database
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	respondJSON(w, http.StatusOK, user)
}
