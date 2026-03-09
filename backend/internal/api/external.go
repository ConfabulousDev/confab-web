package api

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	dbsession "github.com/ConfabulousDev/confab-web/internal/db/session"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// CondensedTranscriptResponse is the JSON response for the condensed transcript endpoint.
type CondensedTranscriptResponse struct {
	Metadata   CondensedTranscriptMetadata `json:"metadata"`
	Transcript string                      `json:"transcript"`
}

// CondensedTranscriptMetadata contains session metadata alongside the condensed transcript.
type CondensedTranscriptMetadata struct {
	SessionID        string                 `json:"session_id"`
	ExternalID       string                 `json:"external_id"`
	Title            string                 `json:"title"`
	Repo             *string                `json:"repo,omitempty"`
	Branch           *string                `json:"branch,omitempty"`
	FirstSeen        time.Time              `json:"first_seen"`
	LastSyncAt       *time.Time             `json:"last_sync_at,omitempty"`
	TotalLines       int64                  `json:"total_lines"`
	EstimatedCostUSD *float64               `json:"estimated_cost_usd,omitempty"`
	SmartRecap       *SmartRecapExport      `json:"smart_recap,omitempty"`
	Analytics        map[string]interface{} `json:"analytics,omitempty"`
}

// SmartRecapExport is a simplified smart recap for external consumption.
type SmartRecapExport struct {
	Recap                     string   `json:"recap"`
	WentWell                  []string `json:"went_well"`
	WentBad                   []string `json:"went_bad"`
	HumanSuggestions          []string `json:"human_suggestions,omitempty"`
	EnvironmentSuggestions    []string `json:"environment_suggestions,omitempty"`
	DefaultContextSuggestions []string `json:"default_context_suggestions,omitempty"`
	ComputedAt                string   `json:"computed_at"`
}

// handleCondensedTranscript returns a condensed, AI-readable transcript for a session.
// Uses canonical access model (CF-132) — owner, recipient, system, and public shares.
// GET /api/v1/sessions/{id}/condensed-transcript
func (s *Server) handleCondensedTranscript(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}
	s.serveCondensedTranscript(w, r, sessionID)
}

// handleCondensedTranscriptByExternalID looks up a session by external_id, then returns
// the condensed transcript. Requires the caller to own the session.
// GET /api/v1/sessions/condensed-transcript?external_id=xxx
func (s *Server) handleCondensedTranscriptByExternalID(w http.ResponseWriter, r *http.Request) {
	log := logger.Ctx(r.Context())

	externalID := r.URL.Query().Get("external_id")
	if externalID == "" {
		respondError(w, http.StatusBadRequest, "Missing external_id query parameter")
		return
	}

	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	sessionStore := &dbsession.Store{DB: s.db}
	sessionID, err := sessionStore.GetSessionIDByExternalID(ctx, externalID, userID)
	if err != nil {
		if err == db.ErrSessionNotFound {
			respondError(w, http.StatusNotFound, "Session not found")
			return
		}
		log.Error("Failed to lookup session by external_id", "error", err, "external_id", externalID)
		respondError(w, http.StatusInternalServerError, "Failed to lookup session")
		return
	}

	s.serveCondensedTranscript(w, r, sessionID)
}

// serveCondensedTranscript is the shared implementation for both condensed transcript routes.
func (s *Server) serveCondensedTranscript(w http.ResponseWriter, r *http.Request, sessionID string) {
	log := logger.Ctx(r.Context())

	// Parse optional max_chars query param
	var maxChars int
	if maxCharsStr := r.URL.Query().Get("max_chars"); maxCharsStr != "" {
		var err error
		maxChars, err = strconv.Atoi(maxCharsStr)
		if err != nil || maxChars < 1 {
			respondError(w, http.StatusBadRequest, "max_chars must be a positive integer")
			return
		}
	}

	// Check canonical access (CF-132 unified access model)
	dbCtx, dbCancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer dbCancel()

	result, err := CheckCanonicalAccess(dbCtx, s.db, sessionID)
	if RespondCanonicalAccessError(dbCtx, w, err, sessionID) {
		return
	}

	if result.AccessInfo.AccessType == db.SessionAccessNone {
		if result.AccessInfo.AuthMayHelp {
			respondError(w, http.StatusUnauthorized, "Sign in to view this session")
			return
		}
		respondError(w, http.StatusNotFound, "Session not found")
		return
	}

	session := result.Session

	// Classify session files to find transcript + agents
	files := classifySessionFiles(session.Files)
	if files == nil {
		respondError(w, http.StatusNotFound, "No transcript available for this session")
		return
	}

	// Get session owner info for S3 path
	sessionStore := &dbsession.Store{DB: s.db}
	sessionUserID, externalID, err := sessionStore.GetSessionOwnerAndExternalID(dbCtx, sessionID)
	if err != nil {
		log.Error("Failed to get session owner info", "error", err, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to get session info")
		return
	}

	// Download and build condensed transcript (no per-message truncation)
	mainTF, dlErr := downloadMainFromFiles(r.Context(), s.storage, files, sessionUserID, externalID)
	if dlErr != nil || mainTF == nil {
		log.Error("Failed to download transcript", "error", dlErr, "session_id", sessionID)
		respondError(w, http.StatusInternalServerError, "Failed to download transcript")
		return
	}

	tb := analytics.NewTranscriptBuilder(analytics.UnlimitedFormatConfig())
	tb.ProcessFile(mainTF)

	// Stream agent files one at a time (same pattern as analytics handler)
	agentInfos := agentInfosFromFiles(files)
	download := newAPIAgentDownloader(s.storage, sessionUserID, externalID)
	provider := analytics.NewAgentProvider(agentInfos, download, storage.MaxAgentFiles)
	for {
		agent, err := provider(r.Context())
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		tb.ProcessFile(agent)
	}

	transcript, _ := tb.Finish()

	// Apply max_chars truncation if specified (truncate from beginning, keep end)
	if maxChars > 0 && len(transcript) > maxChars {
		transcript = truncateTranscriptFromStart(transcript, maxChars)
	}

	// Build metadata
	metadata := buildCondensedMetadata(session, files.lineCount)

	// Fetch cached smart recap (no generation triggered)
	analyticsStore := analytics.NewStore(s.db.Conn())
	smartCard, err := analyticsStore.GetSmartRecapCard(dbCtx, sessionID)
	if err == nil && smartCard != nil && smartCard.HasValidVersion() {
		metadata.SmartRecap = convertSmartRecap(smartCard)
	}

	// Fetch cached analytics cards (no computation triggered)
	cached, err := analyticsStore.GetCards(dbCtx, sessionID)
	if err == nil && cached != nil {
		resp := cached.ToResponse()
		if len(resp.Cards) > 0 {
			metadata.Analytics = resp.Cards
		}
	}

	// Fetch estimated cost from tokens card for numeric conversion
	if metadata.Analytics != nil {
		if tokensCard, ok := metadata.Analytics["tokens"].(analytics.TokensCardData); ok {
			if cost, err := strconv.ParseFloat(tokensCard.EstimatedUSD, 64); err == nil {
				metadata.EstimatedCostUSD = &cost
			}
		}
	}

	respondJSON(w, http.StatusOK, CondensedTranscriptResponse{
		Metadata:   metadata,
		Transcript: transcript,
	})
}

// buildCondensedMetadata constructs the metadata portion of the condensed transcript response.
func buildCondensedMetadata(session *db.SessionDetail, totalLines int64) CondensedTranscriptMetadata {
	// Derive title: custom > suggested > summary > first user message
	title := ""
	switch {
	case session.CustomTitle != nil:
		title = *session.CustomTitle
	case session.SuggestedSessionTitle != nil:
		title = *session.SuggestedSessionTitle
	case session.Summary != nil:
		title = *session.Summary
	case session.FirstUserMessage != nil:
		title = *session.FirstUserMessage
	}

	meta := CondensedTranscriptMetadata{
		SessionID:  session.ID,
		ExternalID: session.ExternalID,
		Title:      title,
		FirstSeen:  session.FirstSeen,
		LastSyncAt: session.LastSyncAt,
		TotalLines: totalLines,
	}

	// Extract repo and branch from git_info
	if gitInfo, ok := session.GitInfo.(map[string]interface{}); ok {
		if repoURL, ok := gitInfo["repo_url"].(string); ok && repoURL != "" {
			repo := extractRepoName(repoURL)
			meta.Repo = &repo
		}
		if branch, ok := gitInfo["branch"].(string); ok && branch != "" {
			meta.Branch = &branch
		}
	}

	return meta
}

// extractRepoName extracts "org/repo" from a full repo URL.
func extractRepoName(repoURL string) string {
	// Strip .git suffix
	repoURL = strings.TrimSuffix(repoURL, ".git")
	// Find the last two path segments (org/repo)
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return repoURL
}

// convertSmartRecap converts a SmartRecapCardRecord to a SmartRecapExport.
func convertSmartRecap(card *analytics.SmartRecapCardRecord) *SmartRecapExport {
	return &SmartRecapExport{
		Recap:                     card.Recap,
		WentWell:                  extractTexts(card.WentWell),
		WentBad:                   extractTexts(card.WentBad),
		HumanSuggestions:          extractTexts(card.HumanSuggestions),
		EnvironmentSuggestions:    extractTexts(card.EnvironmentSuggestions),
		DefaultContextSuggestions: extractTexts(card.DefaultContextSuggestions),
		ComputedAt:                card.ComputedAt.Format(time.RFC3339),
	}
}

// extractTexts returns the Text field from each AnnotatedItem.
func extractTexts(items []analytics.AnnotatedItem) []string {
	if len(items) == 0 {
		return nil
	}
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.Text
	}
	return texts
}

// truncateTranscriptFromStart truncates a transcript XML string from the beginning,
// keeping complete XML elements at the cut point.
func truncateTranscriptFromStart(transcript string, maxChars int) string {
	if len(transcript) <= maxChars {
		return transcript
	}

	totalLen := len(transcript)
	cutPoint := totalLen - maxChars

	// Find the start of the next complete top-level element after the cut point.
	// Top-level elements are: <user, <assistant, <tool_results, <skill
	// We search for the first '<' that starts one of these element names.
	best := -1
	for i := cutPoint; i < totalLen; i++ {
		if transcript[i] == '<' && i+1 < totalLen && transcript[i+1] != '/' {
			rest := transcript[i:]
			if strings.HasPrefix(rest, "<user ") ||
				strings.HasPrefix(rest, "<assistant ") ||
				strings.HasPrefix(rest, "<tool_results ") ||
				strings.HasPrefix(rest, "<skill ") {
				best = i
				break
			}
		}
	}

	if best < 0 {
		// No element boundary found — return as much as we can
		return transcript[cutPoint:]
	}

	kept := transcript[best:]
	return "[Transcript truncated — showing last " +
		strconv.Itoa(len(kept)) + " of " + strconv.Itoa(totalLen) +
		" characters]\n<transcript>\n" + kept
}
