package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	dbtil "github.com/ConfabulousDev/confab-web/internal/db/til"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

const (
	exportDefaultPageSize = 100
	exportMaxPageSize     = 500
)

// ExportTIL is a single TIL in the export response, enriched with session URLs.
type ExportTIL struct {
	ID                 int64     `json:"id"`
	Title              string    `json:"title"`
	Summary            string    `json:"summary"`
	CreatedAt          time.Time `json:"created_at"`
	SessionID          string    `json:"session_id"`
	SessionTitle       *string   `json:"session_title,omitempty"`
	SessionURL         string    `json:"session_url"`
	TranscriptDeepLink string    `json:"transcript_deep_link"`
	GitRepo            *string   `json:"git_repo,omitempty"`
	GitBranch          *string   `json:"git_branch,omitempty"`
	OwnerEmail         string    `json:"owner_email"`
}

// ExportTILsResponse is the JSON response for the TIL export endpoint.
type ExportTILsResponse struct {
	TILs       []ExportTIL `json:"tils"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor"`
	PageSize   int         `json:"page_size"`
	Count      int         `json:"count"`
}

// handleExportTILs returns TILs visible to the authenticated user for external consumption.
// GET /api/v1/tils/export?owner=...&from=...&to=...&page_size=...&cursor=...
func (s *Server) handleExportTILs(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	q := r.URL.Query()

	params := dbtil.ListParams{
		Cursor:      q.Get("cursor"),
		MaxPageSize: exportMaxPageSize,
	}

	// Owner filter (optional)
	if owner := q.Get("owner"); owner != "" {
		params.Owners = []string{owner}
	}

	// Date range: from (inclusive), to (exclusive). Must be valid RFC 3339.
	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid 'from' parameter: must be RFC 3339 (e.g. 2026-03-01T00:00:00Z)")
			return
		}
		params.From = &t
	}
	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid 'to' parameter: must be RFC 3339 (e.g. 2026-03-16T00:00:00Z)")
			return
		}
		params.To = &t
	}

	// Page size
	params.PageSize = exportDefaultPageSize
	if psStr := q.Get("page_size"); psStr != "" {
		ps, err := strconv.Atoi(psStr)
		if err != nil || ps < 1 || ps > exportMaxPageSize {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid 'page_size': must be 1-%d", exportMaxPageSize))
			return
		}
		params.PageSize = ps
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	tilStore := &dbtil.Store{DB: s.db}
	result, err := tilStore.List(ctx, userID, params)
	if err != nil {
		log.Error("Failed to export TILs", "error", err)
		respondError(w, http.StatusInternalServerError, "Failed to export TILs")
		return
	}

	// Map to export response with URLs
	exportTILs := make([]ExportTIL, len(result.TILs))
	for i, t := range result.TILs {
		sessionURL := s.frontendURL + "/sessions/" + t.SessionID
		deepLink := sessionURL
		if t.MessageUUID != nil && *t.MessageUUID != "" {
			deepLink = sessionURL + "?msg=" + *t.MessageUUID
		}

		exportTILs[i] = ExportTIL{
			ID:                 t.ID,
			Title:              t.Title,
			Summary:            t.Summary,
			CreatedAt:          t.CreatedAt,
			SessionID:          t.SessionID,
			SessionTitle:       t.SessionTitle,
			SessionURL:         sessionURL,
			TranscriptDeepLink: deepLink,
			GitRepo:            t.GitRepo,
			GitBranch:          t.GitBranch,
			OwnerEmail:         t.OwnerEmail,
		}
	}

	respondJSON(w, http.StatusOK, ExportTILsResponse{
		TILs:       exportTILs,
		HasMore:    result.HasMore,
		NextCursor: result.NextCursor,
		PageSize:   result.PageSize,
		Count:      len(exportTILs),
	})
}
