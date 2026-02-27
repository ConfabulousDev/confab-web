package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// HandleGetOrgAnalytics returns per-user aggregated analytics across all users.
// Requires ENABLE_ORG_ANALYTICS=true (route is only registered when enabled).
//
// Query parameters:
//   - start_ts: Start of date range as epoch seconds (inclusive, typically local midnight)
//   - end_ts: End of date range as epoch seconds (exclusive, typically local midnight of day after last day)
//   - tz_offset: Client timezone offset in minutes (from JS getTimezoneOffset(); positive=behind UTC)
func HandleGetOrgAnalytics(database *db.DB) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		// Require authentication (any authenticated user can access org analytics)
		_, ok := auth.GetUserID(r.Context())
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

		req := analytics.OrgAnalyticsRequest{
			StartTS:  startTS,
			EndTS:    endTS,
			TZOffset: tzOffset,
		}

		response, err := analyticsStore.GetOrgAnalytics(r.Context(), req)
		if err != nil {
			log.Error("Failed to get org analytics", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to compute org analytics")
			return
		}

		respondJSON(w, http.StatusOK, response)
	}
}
