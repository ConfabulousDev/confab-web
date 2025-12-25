package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// GitHub URL patterns
var (
	// Matches: https://github.com/owner/repo/pull/123
	prURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/pull/(\d+)`)
	// Matches: https://github.com/owner/repo/commit/abc123
	commitURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/commit/([a-f0-9]+)`)
)

// ParsedGitHubURL contains the parsed components of a GitHub URL
type ParsedGitHubURL struct {
	LinkType models.GitHubLinkType
	Owner    string
	Repo     string
	Ref      string // PR number or commit SHA
}

// ParseGitHubURL extracts owner, repo, and ref from a GitHub PR or commit URL
func ParseGitHubURL(url string) (*ParsedGitHubURL, error) {
	// Try PR pattern first
	if matches := prURLPattern.FindStringSubmatch(url); matches != nil {
		return &ParsedGitHubURL{
			LinkType: models.GitHubLinkTypePullRequest,
			Owner:    matches[1],
			Repo:     matches[2],
			Ref:      matches[3],
		}, nil
	}

	// Try commit pattern
	if matches := commitURLPattern.FindStringSubmatch(url); matches != nil {
		return &ParsedGitHubURL{
			LinkType: models.GitHubLinkTypeCommit,
			Owner:    matches[1],
			Repo:     matches[2],
			Ref:      matches[3],
		}, nil
	}

	return nil, fmt.Errorf("invalid GitHub URL: must be a PR or commit URL")
}

// CreateGitHubLinkRequest is the request body for creating a GitHub link
type CreateGitHubLinkRequest struct {
	LinkType models.GitHubLinkType   `json:"link_type"`
	URL      string                  `json:"url"`
	Title    *string                 `json:"title,omitempty"`
	Source   models.GitHubLinkSource `json:"source"`
}

// HandleCreateGitHubLink creates a new GitHub link for a session
func HandleCreateGitHubLink(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session ID from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Parse request body
		var req CreateGitHubLinkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate URL
		if req.URL == "" {
			respondError(w, http.StatusBadRequest, "URL is required")
			return
		}

		// Parse GitHub URL to extract owner/repo/ref
		parsed, err := ParseGitHubURL(req.URL)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate link type matches URL (if provided)
		if req.LinkType != "" && req.LinkType != parsed.LinkType {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("link_type '%s' doesn't match URL type '%s'", req.LinkType, parsed.LinkType))
			return
		}

		// Validate source
		if req.Source != models.GitHubLinkSourceCLIHook && req.Source != models.GitHubLinkSourceManual {
			respondError(w, http.StatusBadRequest, "source must be 'cli_hook' or 'manual'")
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Verify user owns the session
		_, err = database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to verify session ownership", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to verify session")
			return
		}

		// Create the link
		link := &models.GitHubLink{
			SessionID: sessionID,
			LinkType:  parsed.LinkType,
			URL:       req.URL,
			Owner:     parsed.Owner,
			Repo:      parsed.Repo,
			Ref:       parsed.Ref,
			Title:     req.Title,
			Source:    req.Source,
		}

		createdLink, err := database.CreateGitHubLink(ctx, link)
		if err != nil {
			if errors.Is(err, db.ErrGitHubLinkDuplicate) {
				respondError(w, http.StatusConflict, "GitHub link already exists")
				return
			}
			log.Error("Failed to create GitHub link", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to create GitHub link")
			return
		}

		log.Info("GitHub link created",
			"session_id", sessionID,
			"link_id", createdLink.ID,
			"link_type", createdLink.LinkType,
			"source", createdLink.Source)

		respondJSON(w, http.StatusCreated, createdLink)
	}
}

// HandleListGitHubLinks lists all GitHub links for a session
func HandleListGitHubLinks(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get session ID from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Check if user has access to the session
		// For GitHub links, we allow viewing if user has any access (owner, shared, public)
		viewerUserID, _ := auth.GetUserID(r.Context())
		var viewerPtr *int64
		if viewerUserID != 0 {
			viewerPtr = &viewerUserID
		}

		accessInfo, err := database.GetSessionAccessType(ctx, sessionID, viewerPtr)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to check session access", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to check access")
			return
		}

		if accessInfo.AccessType == db.SessionAccessNone {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}

		// Get GitHub links
		links, err := database.GetGitHubLinksForSession(ctx, sessionID)
		if err != nil {
			log.Error("Failed to get GitHub links", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to get GitHub links")
			return
		}

		// Return empty array instead of null
		if links == nil {
			links = []models.GitHubLink{}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"links": links,
		})
	}
}

// HandleDeleteGitHubLink deletes a GitHub link
func HandleDeleteGitHubLink(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context (web auth only, no API key)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session ID and link ID from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		linkIDStr := chi.URLParam(r, "linkID")
		linkID, err := strconv.ParseInt(linkIDStr, 10, 64)
		if err != nil || linkID <= 0 {
			respondError(w, http.StatusBadRequest, "Invalid link ID")
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Verify user owns the session
		_, err = database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to verify session ownership", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to verify session")
			return
		}

		// Verify the link belongs to this session
		link, err := database.GetGitHubLinkByID(ctx, linkID)
		if err != nil {
			if errors.Is(err, db.ErrGitHubLinkNotFound) {
				respondError(w, http.StatusNotFound, "GitHub link not found")
				return
			}
			log.Error("Failed to get GitHub link", "error", err, "link_id", linkID)
			respondError(w, http.StatusInternalServerError, "Failed to get GitHub link")
			return
		}

		if link.SessionID != sessionID {
			respondError(w, http.StatusNotFound, "GitHub link not found")
			return
		}

		// Delete the link
		err = database.DeleteGitHubLink(ctx, linkID)
		if err != nil {
			log.Error("Failed to delete GitHub link", "error", err, "link_id", linkID)
			respondError(w, http.StatusInternalServerError, "Failed to delete GitHub link")
			return
		}

		log.Info("GitHub link deleted",
			"session_id", sessionID,
			"link_id", linkID)

		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDeleteGitHubLinksByType deletes all GitHub links of a given type for a session
func HandleDeleteGitHubLinksByType(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get user ID from context (web auth only, no API key)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get session ID from URL
		sessionID := chi.URLParam(r, "id")
		if sessionID == "" {
			respondError(w, http.StatusBadRequest, "Invalid session ID")
			return
		}

		// Get link type from query parameter
		linkTypeStr := r.URL.Query().Get("type")
		if linkTypeStr == "" {
			respondError(w, http.StatusBadRequest, "type query parameter is required")
			return
		}

		var linkType models.GitHubLinkType
		switch linkTypeStr {
		case "commit":
			linkType = models.GitHubLinkTypeCommit
		case "pull_request":
			linkType = models.GitHubLinkTypePullRequest
		default:
			respondError(w, http.StatusBadRequest, "type must be 'commit' or 'pull_request'")
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Verify user owns the session
		_, err := database.GetSessionDetail(ctx, sessionID, userID)
		if err != nil {
			if errors.Is(err, db.ErrSessionNotFound) {
				respondError(w, http.StatusNotFound, "Session not found")
				return
			}
			log.Error("Failed to verify session ownership", "error", err, "session_id", sessionID)
			respondError(w, http.StatusInternalServerError, "Failed to verify session")
			return
		}

		// Delete all links of the given type
		deleted, err := database.DeleteGitHubLinksByType(ctx, sessionID, linkType)
		if err != nil {
			log.Error("Failed to delete GitHub links by type", "error", err, "session_id", sessionID, "link_type", linkType)
			respondError(w, http.StatusInternalServerError, "Failed to delete GitHub links")
			return
		}

		log.Info("GitHub links deleted by type",
			"session_id", sessionID,
			"link_type", linkType,
			"count", deleted)

		w.WriteHeader(http.StatusNoContent)
	}
}
