package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// MaxTrendsDateRange is the maximum allowed date range for trends queries (90 days).
const MaxTrendsDateRange = 90 * 24 * time.Hour

// DefaultTrendsDateRange is the default date range when no dates are specified (7 days).
const DefaultTrendsDateRange = 7 * 24 * time.Hour

// HandleGetTrends returns aggregated analytics across sessions for the authenticated user.
// Supports filtering by date range and repos.
//
// Query parameters:
//   - start_date: Start of date range (YYYY-MM-DD), default: 7 days ago
//   - end_date: End of date range (YYYY-MM-DD), default: today
//   - repos: Comma-separated repo names to filter by
//   - include_no_repo: Include sessions without a repo (default: true)
func HandleGetTrends(database *db.DB) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Get authenticated user ID (already validated by RequireSession middleware)
		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Parse query parameters
		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Parse start_date (default: 7 days ago)
		startDate := today.Add(-DefaultTrendsDateRange)
		if startStr := r.URL.Query().Get("start_date"); startStr != "" {
			parsed, err := time.Parse("2006-01-02", startStr)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid start_date format. Use YYYY-MM-DD")
				return
			}
			startDate = parsed
		}

		// Parse end_date (default: today, but we add 1 day to make it exclusive)
		endDate := today.Add(24 * time.Hour) // Make end_date exclusive
		if endStr := r.URL.Query().Get("end_date"); endStr != "" {
			parsed, err := time.Parse("2006-01-02", endStr)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid end_date format. Use YYYY-MM-DD")
				return
			}
			endDate = parsed.Add(24 * time.Hour) // Make end_date exclusive
		}

		// Validate date range
		if endDate.Before(startDate) || endDate.Equal(startDate) {
			respondError(w, http.StatusBadRequest, "end_date must be after start_date")
			return
		}
		if endDate.Sub(startDate) > MaxTrendsDateRange {
			respondError(w, http.StatusBadRequest, "Date range cannot exceed 90 days")
			return
		}

		// Parse repos filter (use empty slice, not nil, for correct JSON serialization)
		repos := []string{}
		if reposStr := r.URL.Query().Get("repos"); reposStr != "" {
			for _, repo := range strings.Split(reposStr, ",") {
				if trimmed := strings.TrimSpace(repo); trimmed != "" {
					repos = append(repos, trimmed)
				}
			}
		}

		// Parse include_no_repo (default: true)
		includeNoRepo := true
		if includeStr := r.URL.Query().Get("include_no_repo"); includeStr != "" {
			includeNoRepo = includeStr == "true" || includeStr == "1"
		}

		// Build request
		req := analytics.TrendsRequest{
			StartDate:     startDate,
			EndDate:       endDate,
			Repos:         repos,
			IncludeNoRepo: includeNoRepo,
		}

		// Get trends data
		response, err := analyticsStore.GetTrends(r.Context(), userID, req)
		if err != nil {
			log.Error("Failed to get trends", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to compute trends")
			return
		}

		respondJSON(w, http.StatusOK, response)
	}
}
