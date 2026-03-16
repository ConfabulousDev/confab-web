package api

import (
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/auth"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// HandleGetOrgAnalytics returns per-user aggregated analytics across all users.
// Requires ENABLE_ORG_ANALYTICS=true (route is only registered when enabled).
//
// Privacy: any authenticated user can see every other user's name, email,
// session count, cost, and time breakdowns. Intended for trusted-team
// deployments only. See API.md "Organization Analytics" for full details.
//
// Query parameters:
//   - start_ts: Start of date range as epoch seconds (inclusive, typically local midnight)
//   - end_ts: End of date range as epoch seconds (exclusive, typically local midnight of day after last day)
//   - tz_offset: Client timezone offset in minutes (from JS getTimezoneOffset(); positive=behind UTC)
func HandleGetOrgAnalytics(database *db.DB) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		_, ok := auth.GetUserID(r.Context())
		if !ok {
			respondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		dr := parseDateRangeParams(w, r)
		if dr == nil {
			return
		}

		req := analytics.OrgAnalyticsRequest{
			StartTS:  dr.StartTS,
			EndTS:    dr.EndTS,
			TZOffset: dr.TZOffset,
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
