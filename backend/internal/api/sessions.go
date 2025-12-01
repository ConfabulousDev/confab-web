package api

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"regexp"
	"strings"

	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// HandleCheckSessions checks which external IDs already exist for the user
func HandleCheckSessions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user ID
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Parse request body
		var req struct {
			ExternalIDs []string `json:"external_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate
		if len(req.ExternalIDs) == 0 {
			respondError(w, http.StatusBadRequest, "external_ids is required")
			return
		}
		if len(req.ExternalIDs) > 1000 {
			respondError(w, http.StatusBadRequest, "Too many external IDs (max 1000)")
			return
		}

		// Create context with timeout for database operation
		ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
		defer cancel()

		// Check which sessions exist
		existing, err := s.db.CheckSessionsExist(ctx, userID, req.ExternalIDs)
		if err != nil {
			logger.Error("Failed to check sessions", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to check sessions")
			return
		}

		// Success log
		logger.Info("Sessions checked",
			"user_id", userID,
			"requested_count", len(req.ExternalIDs),
			"existing_count", len(existing))

		// Build missing list
		existingSet := make(map[string]bool)
		for _, id := range existing {
			existingSet[id] = true
		}
		var missing []string
		for _, id := range req.ExternalIDs {
			if !existingSet[id] {
				missing = append(missing, id)
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"existing": existing,
			"missing":  missing,
		})
	}
}

// sanitizeTitleText removes HTML tags and decodes HTML entities
// Note: We don't HTML-escape here because React automatically escapes text content.
// Double-escaping would cause &gt; to display literally instead of as >
func sanitizeTitleText(input string) string {
	// Remove all HTML tags using regex
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlTagRegex.ReplaceAllString(input, "")

	// Decode HTML entities (e.g., &lt; -> <, &gt; -> >)
	decoded := html.UnescapeString(cleaned)

	// Trim whitespace
	return strings.TrimSpace(decoded)
}

// extractSessionTitle parses the first few lines of a transcript to extract a title
func extractSessionTitle(content []byte) string {
	if len(content) == 0 {
		return ""
	}

	// Parse JSONL to find summary or first text content
	lines := strings.Split(string(content), "\n")
	var firstTextContent string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		msgType, _ := entry["type"].(string)

		// Priority 1: Extract title from summary (best quality)
		if msgType == "summary" {
			if summary, ok := entry["summary"].(string); ok && summary != "" {
				return sanitizeTitleText(summary)
			}
		}

		// Collect first text content as fallback (from any user message)
		if firstTextContent == "" && msgType == "user" {
			if text := extractTextFromMessage(entry); text != "" {
				// Use first 100 characters as fallback title
				sanitized := sanitizeTitleText(text)
				if len(sanitized) > 100 {
					firstTextContent = sanitized[:100]
				} else {
					firstTextContent = sanitized
				}
			}
		}
	}

	return firstTextContent
}
