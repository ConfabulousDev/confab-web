package api

import (
	"net/http"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/logger"
)

// OrgReposResponse lists every repo with any session in the given date range
// across all active users in the organization.
type OrgReposResponse struct {
	ComputedAt time.Time          `json:"computed_at"`
	DateRange  analytics.DateRange `json:"date_range"`
	// Repos is an alphabetically sorted slice of canonical owner/name strings
	// extracted from sessions.git_info->>'repo_url'. Always non-nil (`[]` for
	// empty ranges) so the frontend dropdown can treat the field as authoritative.
	Repos []string `json:"repos"`
}

// HandleGetOrgRepos returns the org-wide repo list for the date range. Drives
// the repo filter dropdown on the Organization page. Mounted behind
// ENABLE_ORG_ANALYTICS and inherits the same privacy model as
// HandleGetOrgAnalytics: any authenticated user can see every repo name. The
// route is registered only when org analytics is enabled (see server.go).
//
// Query parameters:
//   - start_ts: Start of date range as epoch seconds (inclusive)
//   - end_ts: End of date range as epoch seconds (exclusive)
//   - tz_offset: Client timezone offset in minutes (from JS getTimezoneOffset)
func HandleGetOrgRepos(database *db.DB) http.HandlerFunc {
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

		tzDuration := time.Duration(dr.TZOffset) * time.Minute
		startLocal := time.Unix(dr.StartTS, 0).UTC().Add(-tzDuration)
		endLocal := time.Unix(dr.EndTS, 0).UTC().Add(-tzDuration).Add(-24 * time.Hour) // EndTS is exclusive

		// CF-491: collapse forks to their upstream root through session_repos.
		// Pure-COALESCE form keeps repos with NULL root_name passing through.
		query := `
			SELECT DISTINCT ` + db.RepoRootExpr("s") + ` AS repo
			FROM sessions s
			INNER JOIN users u ON s.user_id = u.id AND u.status = 'active'
			WHERE s.first_seen >= to_timestamp($1)
				AND s.first_seen < to_timestamp($2)
				AND s.git_info->>'repo_url' IS NOT NULL
				AND s.git_info->>'repo_url' <> ''
			ORDER BY repo ASC
		`

		rows, err := database.Conn().QueryContext(r.Context(), query, dr.StartTS, dr.EndTS)
		if err != nil {
			log.Error("Failed to list org repos", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to list org repos")
			return
		}
		defer rows.Close()

		repos := []string{}
		for rows.Next() {
			var repo string
			if err := rows.Scan(&repo); err != nil {
				log.Error("Failed to scan org repo row", "error", err)
				respondError(w, http.StatusInternalServerError, "Failed to list org repos")
				return
			}
			repos = append(repos, repo)
		}
		if err := rows.Err(); err != nil {
			log.Error("Failed iterating org repo rows", "error", err)
			respondError(w, http.StatusInternalServerError, "Failed to list org repos")
			return
		}

		respondJSON(w, http.StatusOK, OrgReposResponse{
			ComputedAt: time.Now().UTC(),
			DateRange: analytics.DateRange{
				StartDate: startLocal.Format("2006-01-02"),
				EndDate:   endLocal.Format("2006-01-02"),
			},
			Repos: repos,
		})
	}
}
