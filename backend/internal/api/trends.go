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

// dateRangeParams holds parsed and validated date range query parameters.
type dateRangeParams struct {
	StartTS  int64
	EndTS    int64
	TZOffset int
}

// parseDateRangeParams parses start_ts, end_ts, and tz_offset from query parameters.
// Returns nil and writes an error response if parsing or validation fails.
func parseDateRangeParams(w http.ResponseWriter, r *http.Request) *dateRangeParams {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	p := &dateRangeParams{
		StartTS: today.Add(-7 * 24 * time.Hour).Unix(),
		EndTS:   today.Add(24 * time.Hour).Unix(),
	}

	if tsStr := r.URL.Query().Get("start_ts"); tsStr != "" {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid start_ts")
			return nil
		}
		p.StartTS = ts
	}

	if tsStr := r.URL.Query().Get("end_ts"); tsStr != "" {
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid end_ts")
			return nil
		}
		p.EndTS = ts
	}

	if offsetStr := r.URL.Query().Get("tz_offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid tz_offset")
			return nil
		}
		p.TZOffset = offset
	}

	if p.EndTS <= p.StartTS {
		respondError(w, http.StatusBadRequest, "end_ts must be after start_ts")
		return nil
	}
	if p.EndTS-p.StartTS > maxTrendsRangeSeconds {
		respondError(w, http.StatusBadRequest, "Date range cannot exceed 90 days")
		return nil
	}

	return p
}

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

		userID, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		dr := parseDateRangeParams(w, r)
		if dr == nil {
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

		req := analytics.TrendsRequest{
			StartTS:       dr.StartTS,
			EndTS:         dr.EndTS,
			TZOffset:      dr.TZOffset,
			Repos:         repos,
			IncludeNoRepo: includeNoRepo,
		}

		response, err := analyticsStore.GetTrends(r.Context(), userID, req)
		if err != nil {
			log.Error("Failed to get trends", "error", err, "user_id", userID)
			respondError(w, http.StatusInternalServerError, "Failed to compute trends")
			return
		}

		respondJSON(w, http.StatusOK, response)
	}
}
