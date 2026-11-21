package admin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/httputil"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// session_insights.go: the admin Session Insights tab (c3ts). Two thin read-only
// handlers over analytics.Store's instance-wide session reads. Both take the same
// query parameters (parseSessionInsightsFilter).

// defaultSessionInsightsRange is the window used when no `from` is given.
const defaultSessionInsightsRange = 30 * 24 * time.Hour

// ProviderShareBucketJSON is one time bucket of the provider-share response.
type ProviderShareBucketJSON struct {
	Start  string         `json:"start"`
	Counts map[string]int `json:"counts"`
}

// ProviderShareResponse is the response for GET /admin/session-insights/provider-share.
// From/To are the effective range (RFC3339, UTC); Buckets holds only non-empty
// buckets, so the client zero-fills between From and To.
type ProviderShareResponse struct {
	From       string                    `json:"from"`
	To         string                    `json:"to"`
	BucketSize string                    `json:"bucket_size"`
	Buckets    []ProviderShareBucketJSON `json:"buckets"`
	Totals     map[string]int            `json:"totals"`
}

// SessionPointJSON is one session on the size scatter. Card-derived metrics are
// null when the session has no such card yet; Repo is the sanitized owner/repo.
type SessionPointJSON struct {
	ID         string   `json:"id"`
	FirstSeen  string   `json:"first_seen"`
	Provider   string   `json:"provider"`
	Title      string   `json:"title"`
	OwnerEmail string   `json:"owner_email"`
	Repo       *string  `json:"repo"`
	Branch     *string  `json:"branch"`
	Lines      int64    `json:"lines"`
	Tokens     *int64   `json:"tokens"`
	CostUSD    *float64 `json:"cost_usd"`
	UserTurns  *int64   `json:"user_turns"`
	DurationMs *int64   `json:"duration_ms"`
}

// SessionPointsResponse is the response for GET /admin/session-insights/points.
type SessionPointsResponse struct {
	Total    int                `json:"total"`
	Returned int                `json:"returned"`
	Limit    int                `json:"limit"`
	Points   []SessionPointJSON `json:"points"`
}

// HandleSessionInsightsProviderShare serves session counts per time bucket and
// provider over every listable session on the instance. Never capped.
func (h *Handlers) HandleSessionInsightsProviderShare(w http.ResponseWriter, r *http.Request) {
	f, err := parseSessionInsightsFilter(r.URL.Query(), time.Now())
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	share, err := h.analyticsStore.AdminProviderShare(ctx, f)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to compute session insights provider share", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to compute provider share")
		return
	}

	buckets := make([]ProviderShareBucketJSON, 0, len(share.Buckets))
	for _, b := range share.Buckets {
		buckets = append(buckets, ProviderShareBucketJSON{Start: b.Start.Format(time.RFC3339), Counts: b.Counts})
	}
	httputil.RespondJSON(w, http.StatusOK, ProviderShareResponse{
		From:       share.From.Format(time.RFC3339),
		To:         share.To.Format(time.RFC3339),
		BucketSize: share.BucketSize,
		Buckets:    buckets,
		Totals:     share.Totals,
	})
}

// HandleSessionInsightsPoints serves one point per matching session (every size
// metric included), capped at analytics.SessionInsightsPointLimit by a stable
// sample. Total always counts every matching session.
func (h *Handlers) HandleSessionInsightsPoints(w http.ResponseWriter, r *http.Request) {
	f, err := parseSessionInsightsFilter(r.URL.Query(), time.Now())
	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), DatabaseTimeout)
	defer cancel()

	res, err := h.analyticsStore.AdminSessionPoints(ctx, f, analytics.SessionInsightsPointLimit)
	if err != nil {
		logger.Ctx(r.Context()).Error("Failed to load session insights points", "error", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to load session points")
		return
	}

	points := make([]SessionPointJSON, 0, len(res.Points))
	for _, p := range res.Points {
		points = append(points, SessionPointJSON{
			ID:         p.ID,
			FirstSeen:  p.FirstSeen.Format(time.RFC3339),
			Provider:   p.Provider,
			Title:      p.Title,
			OwnerEmail: p.OwnerEmail,
			Repo:       p.Repo,
			Branch:     p.Branch,
			Lines:      p.Lines,
			Tokens:     p.Tokens,
			CostUSD:    p.CostUSD,
			UserTurns:  p.UserTurns,
			DurationMs: p.DurationMs,
		})
	}
	httputil.RespondJSON(w, http.StatusOK, SessionPointsResponse{
		Total:    res.Total,
		Returned: res.Returned,
		Limit:    analytics.SessionInsightsPointLimit,
		Points:   points,
	})
}

// parseSessionInsightsFilter reads the shared query parameters:
//   - from, to: RFC3339 with a timezone, or a YYYY-MM-DD date (UTC). A date `to`
//     is inclusive of that day. Defaults: to = now, from = to - 30 days.
//   - all=true: all time (no lower bound); cannot be combined with from.
//   - provider: canonical providers, repeatable and/or comma-separated,
//     case-insensitive. Empty means every provider.
//   - user_id: a positive user id to narrow to one owner.
func parseSessionInsightsFilter(q url.Values, now time.Time) (analytics.SessionInsightsFilter, error) {
	var f analytics.SessionInsightsFilter

	all := false
	if v := q.Get("all"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return f, errors.New("all must be true or false")
		}
		all = b
	}

	f.To = now.UTC()
	if v := q.Get("to"); v != "" {
		t, err := parseInsightsTime(v, true)
		if err != nil {
			return f, err
		}
		f.To = t
	}
	switch v := q.Get("from"); {
	case all && v != "":
		return f, errors.New("all cannot be combined with from")
	case all:
		// Zero From: all time.
	case v != "":
		t, err := parseInsightsTime(v, false)
		if err != nil {
			return f, err
		}
		f.From = t
	default:
		f.From = f.To.Add(-defaultSessionInsightsRange)
	}
	if !all && !f.From.Before(f.To) {
		return f, errors.New("from must be before to")
	}

	for _, raw := range q["provider"] {
		for p := range strings.SplitSeq(raw, ",") {
			p = strings.ToLower(strings.TrimSpace(p))
			if p == "" {
				continue
			}
			if err := validation.ValidateProvider(p); err != nil {
				return f, err
			}
			f.Providers = append(f.Providers, p)
		}
	}

	if v := q.Get("user_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			return f, errors.New("user_id must be a positive integer")
		}
		f.OwnerID = &id
	}
	return f, nil
}

// parseInsightsTime accepts RFC3339 (timezone required) or a YYYY-MM-DD date at
// UTC midnight. endOfDay moves a bare date to the next midnight so an inclusive
// end date becomes the exclusive bound.
func parseInsightsTime(s string, endOfDay bool) (time.Time, error) {
	if d, err := time.Parse(time.DateOnly, s); err == nil {
		if endOfDay {
			d = d.AddDate(0, 0, 1)
		}
		return d, nil
	}
	t, err := parseStrictTimestamp(s)
	if err != nil {
		return time.Time{}, errors.New("dates must be YYYY-MM-DD or RFC3339 with a timezone: " + s)
	}
	return t.UTC(), nil
}
