// ABOUTME: HTTP handler for exporting a confirmed learning to Confluence as a draft page.
// ABOUTME: POST /api/v1/learnings/{id}/export triggers the export and updates learning status.
package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// handleExportLearning exports a confirmed learning to Confluence.
// POST /api/v1/learnings/{id}/export
func (s *Server) handleExportLearning(w http.ResponseWriter, r *http.Request) {
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

	// Check that Confluence is configured before doing any work
	if s.confluenceClient == nil || !s.confluenceClient.IsConfigured() {
		respondError(w, http.StatusServiceUnavailable, "Confluence integration not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	// Fetch the learning and verify ownership
	learning, err := s.db.GetLearning(ctx, learningID, userID)
	if err != nil {
		if errors.Is(err, db.ErrLearningNotFound) {
			respondError(w, http.StatusNotFound, "Learning not found")
			return
		}
		log.Error("Failed to get learning for export", "error", err, "learning_id", learningID)
		respondError(w, http.StatusInternalServerError, "Failed to get learning")
		return
	}

	// Only confirmed learnings can be exported
	if learning.Status != models.LearningStatusConfirmed {
		respondError(w, http.StatusConflict, "Only confirmed learnings can be exported")
		return
	}

	// Create the page in Confluence (use a longer timeout for the external call)
	exportCtx, exportCancel := context.WithTimeout(r.Context(), StorageTimeout)
	defer exportCancel()

	pageResult, err := s.confluenceClient.CreatePage(exportCtx, learning.Title, learning.Body, learning.Tags)
	if err != nil {
		log.Error("Failed to create Confluence page", "error", err, "learning_id", learningID)
		respondError(w, http.StatusBadGateway, "Failed to create Confluence page")
		return
	}

	log.Info("Confluence page created", "learning_id", learningID, "page_id", pageResult.PageID)

	// Update the learning: set status to exported and store the page ID
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	exportedStatus := models.LearningStatusExported
	updated, err := s.db.UpdateLearning(dbCtx, learningID, userID, &models.UpdateLearningRequest{
		Status: &exportedStatus,
	})
	if err != nil {
		// The page was created in Confluence but we failed to update locally.
		// Log the page ID so it can be reconciled manually.
		log.Error("Failed to update learning after Confluence export",
			"error", err, "learning_id", learningID, "confluence_page_id", pageResult.PageID)
		respondError(w, http.StatusInternalServerError, "Confluence page created but failed to update learning status")
		return
	}

	// Set the confluence_page_id on the learning record
	if err := s.db.SetLearningConfluencePageID(dbCtx, learningID, userID, pageResult.PageID); err != nil {
		log.Error("Failed to set confluence_page_id", "error", err, "learning_id", learningID)
		// Non-fatal: status is already exported, page was created
	}

	// Re-read to get the updated confluence_page_id in the response
	final, err := s.db.GetLearning(dbCtx, learningID, userID)
	if err != nil {
		// Fall back to the update result if re-read fails
		log.Warn("Failed to re-read learning after export, using update result", "error", err)
		final = updated
	}

	respondJSON(w, http.StatusOK, final)
}
