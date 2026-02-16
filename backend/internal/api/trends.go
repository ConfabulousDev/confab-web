package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// maxTrendsRangeSeconds is the maximum allowed date range for trends queries (90 days).
const maxTrendsRangeSeconds = 90 * 24 * 60 * 60

// HandleGetTrends returns aggregated analytics across sessions for the authenticated user.
// Supports filtering by date range and repos.
//
// Query parameters:
//   - start_ts: Start of date range as epoch seconds (inclusive, typically local midnight)
//   - end_ts: End of date range as epoch seconds (exclusive, typically local midnight of day after last day)
//   - tz_offset: Client timezone offset in minutes (from JS getTimezoneOffset(); positive=behind UTC)
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

		// Default: last 7 days in UTC
		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		startTS := today.Add(-7 * 24 * time.Hour).Unix()
		endTS := today.Add(24 * time.Hour).Unix()
		tzOffset := 0

		// Parse start_ts
		if tsStr := r.URL.Query().Get("start_ts"); tsStr != "" {
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid start_ts")
				return
			}
			startTS = ts
		}

		// Parse end_ts
		if tsStr := r.URL.Query().Get("end_ts"); tsStr != "" {
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid end_ts")
				return
			}
			endTS = ts
		}

		// Parse tz_offset (minutes, matching JS getTimezoneOffset convention)
		if offsetStr := r.URL.Query().Get("tz_offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid tz_offset")
				return
			}
			tzOffset = offset
		}

		// Validate
		if endTS <= startTS {
			respondError(w, http.StatusBadRequest, "end_ts must be after start_ts")
			return
		}
		if endTS-startTS > maxTrendsRangeSeconds {
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
			StartTS:       startTS,
			EndTS:         endTS,
			TZOffset:      tzOffset,
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
