// ABOUTME: HTTP handlers for learnings CRUD endpoints.
// ABOUTME: Provides create, list, get, update, and delete handlers for learning artifacts.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// handleCreateLearning creates a new learning artifact.
// POST /api/v1/learnings
func (s *Server) handleCreateLearning(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req models.CreateLearningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate required fields
	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "Title is required")
		return
	}

	// Default source to manual_review if not provided
	if req.Source == "" {
		req.Source = models.LearningSourceManualReview
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	learning, err := s.db.CreateLearning(ctx, userID, &req)
	if err != nil {
		log.Error("Failed to create learning", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to create learning")
		return
	}

	log.Info("Learning created", "learning_id", learning.ID)
	respondJSON(w, http.StatusCreated, learning)
}

// handleListLearnings lists learnings for the authenticated user with optional filters.
// GET /api/v1/learnings?status=draft&source=manual_review&q=search+term
func (s *Server) handleListLearnings(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse optional query params into filters
	filters := &db.LearningFilters{}

	if status := r.URL.Query().Get("status"); status != "" {
		s := models.LearningStatus(status)
		filters.Status = &s
	}
	if source := r.URL.Query().Get("source"); source != "" {
		s := models.LearningSource(source)
		filters.Source = &s
	}
	if q := r.URL.Query().Get("q"); q != "" {
		filters.Query = &q
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	learnings, err := s.db.ListLearnings(ctx, userID, filters)
	if err != nil {
		log.Error("Failed to list learnings", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to list learnings")
		return
	}

	counts, err := s.db.CountLearningsByStatus(ctx, userID)
	if err != nil {
		log.Error("Failed to count learnings by status", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to count learnings")
		return
	}

	log.Info("Learnings listed", "count", len(learnings))
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"learnings": learnings,
		"counts":    counts,
	})
}

// handleGetLearning returns a single learning by ID.
// GET /api/v1/learnings/{id}
func (s *Server) handleGetLearning(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	learningID := chi.URLParam(r, "id")
	if learningID == "" {
		respondError(w, http.StatusBadRequest, "Invalid learning ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	learning, err := s.db.GetLearning(ctx, learningID, userID)
	if err != nil {
		if errors.Is(err, db.ErrLearningNotFound) {
			respondError(w, http.StatusNotFound, "Learning not found")
			return
		}
		log.Error("Failed to get learning", "error", err, "learning_id", learningID)
		respondError(w, http.StatusInternalServerError, "Failed to get learning")
		return
	}

	respondJSON(w, http.StatusOK, learning)
}

// handleUpdateLearning updates a learning's mutable fields.
// PATCH /api/v1/learnings/{id}
func (s *Server) handleUpdateLearning(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	learningID := chi.URLParam(r, "id")
	if learningID == "" {
		respondError(w, http.StatusBadRequest, "Invalid learning ID")
		return
	}

	var req models.UpdateLearningRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	learning, err := s.db.UpdateLearning(ctx, learningID, userID, &req)
	if err != nil {
		if errors.Is(err, db.ErrLearningNotFound) {
			respondError(w, http.StatusNotFound, "Learning not found")
			return
		}
		log.Error("Failed to update learning", "error", err, "learning_id", learningID)
		respondError(w, http.StatusInternalServerError, "Failed to update learning")
		return
	}

	log.Info("Learning updated", "learning_id", learning.ID)
	respondJSON(w, http.StatusOK, learning)
}

// handleDeleteLearning removes a learning by ID.
// DELETE /api/v1/learnings/{id}
func (s *Server) handleDeleteLearning(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	learningID := chi.URLParam(r, "id")
	if learningID == "" {
		respondError(w, http.StatusBadRequest, "Invalid learning ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	err := s.db.DeleteLearning(ctx, learningID, userID)
	if err != nil {
		if errors.Is(err, db.ErrLearningNotFound) {
			respondError(w, http.StatusNotFound, "Learning not found")
			return
		}
		log.Error("Failed to delete learning", "error", err, "learning_id", learningID)
		respondError(w, http.StatusInternalServerError, "Failed to delete learning")
		return
	}

	log.Info("Learning deleted", "learning_id", learningID)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Learning deleted"})
}
