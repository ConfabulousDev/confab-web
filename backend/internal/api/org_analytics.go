package api

import (
	"net/http"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/validation"
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
//   - provider: Comma-separated canonical providers (claude-code, codex). Case-insensitive.
//     Omitted/empty = aggregate across all AllowedProviders.
//   - repos: Comma-separated repo names (owner/name form) to include.
//   - include_no_repo: Include sessions without a repo_url. Defaults to true.
func HandleGetOrgAnalytics(database *db.DB) http.HandlerFunc {
	analyticsStore := analytics.NewStore(database.Conn())

	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Ctx(r.Context())

		_, ok := requireUserID(w, r)
		if !ok {
			return
		}

		dr := parseDateRangeParams(w, r)
		if dr == nil {
			return
		}

		providers, perr := parseProviders(r.URL.Query().Get("provider"))
		if perr != nil {
			respondError(w, http.StatusBadRequest, perr.Error())
			return
		}
		if err := validation.ValidateFilterValues("provider", providers); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		repos := parseCommaSeparated(r.URL.Query().Get("repos"))
		if err := validation.ValidateFilterValues("repo", repos); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// include_no_repo defaults to true; explicit "false"/"0" disables.
		includeNoRepo := true
		if includeStr := r.URL.Query().Get("include_no_repo"); includeStr != "" {
			includeNoRepo = includeStr == "true" || includeStr == "1"
		}

		req := analytics.OrgAnalyticsRequest{
			StartTS:       dr.StartTS,
			EndTS:         dr.EndTS,
			TZOffset:      dr.TZOffset,
			Providers:     providers,
			Repos:         repos,
			IncludeNoRepo: includeNoRepo,
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
