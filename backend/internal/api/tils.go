package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	dbtil "github.com/ConfabulousDev/confab-web/internal/db/til"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// parseTILID extracts and validates the TIL ID from the URL path.
func parseTILID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid TIL ID")
	}
	return id, nil
}

// createTILRequest is the request body for creating a TIL
type createTILRequest struct {
	Title       string  `json:"title"`
	Summary     string  `json:"summary"`
	SessionID   string  `json:"session_id"`
	MessageUUID *string `json:"message_uuid,omitempty"`
}

// HandleCreateTIL creates a new TIL (CLI only, API key auth).
// Access: requires session ownership (matches POST /sessions/{id}/github-links).
func HandleCreateTIL(database *db.DB) http.HandlerFunc {
	sessionStore := &dbsession.Store{DB: database}
	tilStore := &dbtil.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		var req createTILRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		req.Title = strings.TrimSpace(req.Title)
		if req.Title == "" {
			respondError(w, http.StatusBadRequest, "title is required")
			return
		}
		if len(req.Title) > 500 {
			respondError(w, http.StatusBadRequest, "title must be at most 500 characters")
			return
		}

		req.Summary = strings.TrimSpace(req.Summary)
		if req.Summary == "" {
			respondError(w, http.StatusBadRequest, "summary is required")
			return
		}
		if len(req.Summary) > 10000 {
			respondError(w, http.StatusBadRequest, "summary must be at most 10000 characters")
			return
		}

		if req.SessionID == "" {
			respondError(w, http.StatusBadRequest, "session_id is required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Verify user owns the session (WHERE user_id = $2)
		_, err := sessionStore.GetSessionDetail(ctx, req.SessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to verify session ownership", "error", err, "session_id", req.SessionID)
			respondError(w, http.StatusInternalServerError, "Failed to verify session")
			return
		}

		til := &models.TIL{
			Title:       req.Title,
			Summary:     req.Summary,
			SessionID:   req.SessionID,
			MessageUUID: req.MessageUUID,
			OwnerID:     userID,
		}

		createdTIL, err := tilStore.Create(ctx, til)
		if err != nil {
			log.Error("Failed to create TIL", "error", err, "session_id", req.SessionID)
			respondError(w, http.StatusInternalServerError, "Failed to create TIL")
			return
		}

		log.Info("TIL created", "til_id", createdTIL.ID, "session_id", req.SessionID)
		respondJSON(w, http.StatusCreated, createdTIL)
	}
}

// HandleListTILs lists TILs visible to the authenticated user with filtering.
// Access: three-CTE visibility model (matches GET /sessions).
func HandleListTILs(database *db.DB) http.HandlerFunc {
	tilStore := &dbtil.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		params := dbtil.ListParams{
			Query:  r.URL.Query().Get("q"),
			Cursor: r.URL.Query().Get("cursor"),
		}

		if owners := r.URL.Query().Get("owner"); owners != "" {
			params.Owners = strings.Split(owners, ",")
		}
		if repos := r.URL.Query().Get("repo"); repos != "" {
			params.Repos = strings.Split(repos, ",")
		}
		if branches := r.URL.Query().Get("branch"); branches != "" {
			params.Branches = strings.Split(branches, ",")
		}
		if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
			if ps, err := strconv.Atoi(pageSizeStr); err == nil {
				params.PageSize = ps
			}
		}

		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		result, err := tilStore.List(ctx, userID, params)
		if err != nil {
			log.Error("Failed to list TILs", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to list TILs")
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// HandleGetTIL returns a single TIL by ID.
// Access: canonical session read (matches GET /sessions/{id}).
// Route: OptionalAuth group — supports unauthenticated access for public shares.
// Returns uniform "TIL not found" for both not-found and no-access to prevent existence leaks.
func HandleGetTIL(database *db.DB) http.HandlerFunc {
	tilStore := &dbtil.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		tilID, err := parseTILID(r)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid TIL ID")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		til, err := tilStore.GetByID(ctx, tilID)
		if err != nil {
			if errors.Is(err, db.ErrTILNotFound) {
				respondError(w, http.StatusNotFound, "TIL not found")
				return
			}
			log.Error("Failed to get TIL", "error", err, "til_id", tilID)
			respondError(w, http.StatusInternalServerError, "Failed to get TIL")
			return
		}

		// Same access check as GET /sessions/{id} — but with uniform "TIL not found"
		// to prevent leaking whether the TIL exists vs session is inaccessible.
		result, err := CheckCanonicalAccess(ctx, database, til.SessionID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) || errors.Is(err, db.ErrOwnerInactive) {
				respondError(w, http.StatusNotFound, "TIL not found")
				return
			}
			log.Error("Failed to check session access", "error", err, "til_id", tilID)
			respondError(w, http.StatusInternalServerError, "Failed to get TIL")
			return
		}
		if result.AccessInfo.AccessType == db.SessionAccessNone {
			respondError(w, http.StatusNotFound, "TIL not found")
			return
		}

		respondJSON(w, http.StatusOK, til)
	}
}

// HandleDeleteTIL deletes a TIL.
// Access: requires session ownership (matches DELETE /sessions/{id}).
// Returns uniform 404 for both "not found" and "not owner" to avoid existence leaks.
func HandleDeleteTIL(database *db.DB) http.HandlerFunc {
	sessionStore := &dbsession.Store{DB: database}
	tilStore := &dbtil.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		tilID, err := parseTILID(r)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid TIL ID")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		til, err := tilStore.GetByID(ctx, tilID)
		if err != nil {
			if errors.Is(err, db.ErrTILNotFound) {
				respondError(w, http.StatusNotFound, "TIL not found")
				return
			}
			log.Error("Failed to get TIL", "error", err, "til_id", tilID)
			respondError(w, http.StatusInternalServerError, "Failed to get TIL")
			return
		}

		// Verify caller owns the session — same check as DELETE /sessions/{id}
		// Uses GetSessionDetail(sessionID, userID) which enforces WHERE user_id = $2
		_, err = sessionStore.GetSessionDetail(ctx, til.SessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				// Session not owned or not found — uniform 404
				respondError(w, http.StatusNotFound, "TIL not found")
				return
			}
			log.Error("Failed to verify session ownership", "error", err, "til_id", tilID)
			respondError(w, http.StatusInternalServerError, "Failed to delete TIL")
			return
		}

		err = tilStore.Delete(ctx, tilID)
		if err != nil {
			log.Error("Failed to delete TIL", "error", err, "til_id", tilID)
			respondError(w, http.StatusInternalServerError, "Failed to delete TIL")
			return
		}

		log.Info("TIL deleted", "til_id", tilID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleListSessionTILs lists TILs for a session.
// Access: canonical session read (matches GET /sessions/{id}/github-links).
func HandleListSessionTILs(database *db.DB) http.HandlerFunc {
	tilStore := &dbtil.Store{DB: database}

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Same access check as GET /sessions/{id}
		if RequireCanonicalRead(ctx, w, database, sessionID) == nil {
			return
		}

		tils, err := tilStore.ListForSession(ctx, sessionID)
		if err != nil {
			log.Error("Failed to list session TILs", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to list session TILs")
			return
		}

		if tils == nil {
			tils = []models.TIL{}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"tils": tils,
		})
	}
}
