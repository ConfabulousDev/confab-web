package analytics

import (
	"context"
	"database/sql"
	"sort"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GetOrgAnalytics retrieves per-user aggregated analytics across all active users.
// Only sessions with both tokens AND conversation cards are counted, ensuring
// every counted session contributes to all metrics. All active users appear
// in the result, even those with zero qualifying sessions in the range.
func (s *Store) GetOrgAnalytics(ctx context.Context, req OrgAnalyticsRequest) (*OrgAnalyticsResponse, error) {
	ctx, span := tracer.Start(ctx, "analytics.get_org_analytics",
		trace.WithAttributes(
			attribute.Int64("start_ts", req.StartTS),
			attribute.Int64("end_ts", req.EndTS),
			attribute.Int("tz_offset", req.TZOffset),
		))
	defer span.End()

	tzDuration := time.Duration(req.TZOffset) * time.Minute
	startLocal := time.Unix(req.StartTS, 0).UTC().Add(-tzDuration)
	endLocal := time.Unix(req.EndTS, 0).UTC().Add(-tzDuration).Add(-24 * time.Hour) // EndTS is exclusive

	providerArg := pq.Array(resolveProviderFilter(req.Providers))
	repoArg := pq.Array(req.Repos)
	// providers_present must drive the frontend's provider dropdown options, so
	// it has to be independent of the *current* provider selection — otherwise
	// once a user pins `?provider=claude-code`, the dropdown narrows to just
	// claude-code and they can never widen it from the UI. Compute it over the
	// full canonical+legacy set, scoped only by date range + repo filter.
	allProvidersArg := pq.Array(models.AllowedProviders)

	// Run the per-user aggregate and the providers-present DISTINCT in parallel —
	// they share filter args and don't depend on each other. Mirrors the trends
	// pattern of overlapping unrelated card queries.
	var (
		users            []OrgUserAnalytics
		providersPresent []string
		userErr          error
		provErr          error
		wg               sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		providersPresent, provErr = s.orgProvidersPresent(ctx, req, allProvidersArg, repoArg)
	}()
	go func() {
		defer wg.Done()
		users, userErr = s.orgUserAggregates(ctx, req, providerArg, repoArg)
	}()
	wg.Wait()
	if userErr != nil {
		return nil, userErr
	}
	if provErr != nil {
		return nil, provErr
	}

	return &OrgAnalyticsResponse{
		ComputedAt: time.Now().UTC(),
		DateRange: DateRange{
			StartDate: startLocal.Format("2006-01-02"),
			EndDate:   endLocal.Format("2006-01-02"),
		},
		ProvidersPresent: providersPresent,
		Users:            users,
	}, nil
}

func (s *Store) orgUserAggregates(ctx context.Context, req OrgAnalyticsRequest, providerArg, repoArg any) ([]OrgUserAnalytics, error) {
	query := `
		SELECT
			u.id,
			u.email,
			u.name,
			COUNT(DISTINCT qs.session_id) as session_count,
			COALESCE(SUM(qs.cost), 0) as total_cost_usd,
			COALESCE(SUM(qs.duration_ms), 0) as total_duration_ms,
			COALESCE(SUM(qs.assistant_time_ms), 0) as total_assistant_time_ms,
			COALESCE(SUM(qs.user_time_ms), 0) as total_user_time_ms
		FROM users u
		LEFT JOIN LATERAL (
			SELECT
				s.id as session_id,
				t.estimated_cost_usd::numeric as cost,
				COALESCE(sess.duration_ms, 0) as duration_ms,
				cv.total_assistant_duration_ms as assistant_time_ms,
				cv.total_user_duration_ms as user_time_ms
			FROM sessions s
			INNER JOIN session_card_tokens t ON s.id = t.session_id
			INNER JOIN session_card_conversation cv ON s.id = cv.session_id
			LEFT JOIN session_card_session sess ON s.id = sess.session_id
			WHERE s.user_id = u.id
				AND s.first_seen >= to_timestamp($1)
				AND s.first_seen < to_timestamp($2)
				AND s.session_type = ANY($3::text[])
				AND (
					` + db.RepoMatchExpr("s", "$4::text[]") + `
					OR (COALESCE(cardinality($4::text[]), 0) = 0 AND COALESCE(s.git_info->>'repo_url', '') <> '')
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
		) qs ON true
		WHERE u.status = 'active'
		GROUP BY u.id, u.email, u.name
		ORDER BY u.name ASC NULLS LAST, u.email ASC
	`

	rows, err := s.db.QueryContext(ctx, query, req.StartTS, req.EndTS, providerArg, repoArg, req.IncludeNoRepo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []OrgUserAnalytics
	for rows.Next() {
		var (
			userID         int64
			email          string
			name           sql.NullString
			sessionCount   int
			totalCost      decimal.Decimal
			totalDurMs     int64
			totalAssistant int64
			totalUser      int64
		)

		if err := rows.Scan(&userID, &email, &name, &sessionCount, &totalCost, &totalDurMs, &totalAssistant, &totalUser); err != nil {
			return nil, err
		}

		ua := OrgUserAnalytics{
			User: OrgUserInfo{
				ID:    userID,
				Email: email,
			},
			SessionCount:         sessionCount,
			TotalCostUSD:         totalCost.StringFixed(2),
			TotalDurationMs:      totalDurMs,
			TotalAssistantTimeMs: totalAssistant,
			TotalUserTimeMs:      totalUser,
			AvgCostUSD:           "0.00",
		}

		if name.Valid {
			ua.User.Name = &name.String
		}

		if sessionCount > 0 {
			avgCost := totalCost.Div(decimal.NewFromInt(int64(sessionCount)))
			ua.AvgCostUSD = avgCost.StringFixed(2)

			avgDur := totalDurMs / int64(sessionCount)
			ua.AvgDurationMs = &avgDur

			avgAssistant := totalAssistant / int64(sessionCount)
			ua.AvgAssistantTimeMs = &avgAssistant

			avgUser := totalUser / int64(sessionCount)
			ua.AvgUserTimeMs = &avgUser
		}

		users = append(users, ua)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if users == nil {
		users = []OrgUserAnalytics{} // Non-nil for `[]` JSON serialization.
	}
	return users, nil
}

// orgProvidersPresent returns the canonical providers with at least one
// qualifying session (tokens + conversation cards) in the date range × repo
// filter — independent of the request's provider filter. The frontend uses
// this to populate the provider-filter dropdown; if we narrowed by the current
// provider selection, a user with `?provider=claude-code` could never widen
// back from the UI. Legacy 'Claude Code' session_type rows fold into
// 'claude-code' via models.NormalizeProvider so the API surface only exposes
// canonical values. Always returns a non-nil slice (empty when no sessions
// match).
func (s *Store) orgProvidersPresent(ctx context.Context, req OrgAnalyticsRequest, providerArg, repoArg any) ([]string, error) {
	query := `
		SELECT DISTINCT s.session_type
		FROM sessions s
		INNER JOIN session_card_tokens t ON s.id = t.session_id
		INNER JOIN session_card_conversation cv ON s.id = cv.session_id
		INNER JOIN users u ON s.user_id = u.id AND u.status = 'active'
		WHERE s.first_seen >= to_timestamp($1)
			AND s.first_seen < to_timestamp($2)
			AND s.session_type = ANY($3::text[])
			AND (
				` + db.RepoMatchExpr("s", "$4::text[]") + `
				OR (COALESCE(cardinality($4::text[]), 0) = 0 AND COALESCE(s.git_info->>'repo_url', '') <> '')
				OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
			)
	`

	rows, err := s.db.QueryContext(ctx, query, req.StartTS, req.EndTS, providerArg, repoArg, req.IncludeNoRepo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		seen[models.NormalizeProvider(raw)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}
