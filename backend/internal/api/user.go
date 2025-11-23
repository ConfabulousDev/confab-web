package api

import (
	"context"
	"net/http"

	"github.com/santaclaude2025/confab/backend/internal/auth"
	"github.com/santaclaude2025/confab/backend/internal/logger"
)

// handleGetMe returns the current authenticated user's info
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
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
	user, err := s.db.GetUserByID(ctx, userID)
	if err != nil {
		logger.Error("Failed to get user", "error", err, "user_id", userID)
		respondError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// handleGetWeeklyUsage returns the current user's weekly upload usage statistics
func (s *Server) handleGetWeeklyUsage(w http.ResponseWriter, r *http.Request) {
	// Get user ID from session middleware
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Create context with timeout for database operation
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Get weekly usage stats from database
	usage, err := s.db.GetUserWeeklyUsage(ctx, userID, MaxRunsPerWeek)
	if err != nil {
		logger.Error("Failed to get weekly usage", "error", err, "user_id", userID)
		respondError(w, http.StatusInternalServerError, "Failed to get usage statistics")
		return
	}

	logger.Debug("Weekly usage retrieved",
		"user_id", userID,
		"current_count", usage.CurrentCount,
		"limit", usage.Limit,
		"remaining", usage.Remaining)

	respondJSON(w, http.StatusOK, usage)
}
