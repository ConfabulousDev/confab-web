package analytics

import (
	"context"
	"database/sql"
	"time"

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

	// Derive local dates from epoch timestamps and timezone offset for the response
	tzDuration := time.Duration(req.TZOffset) * time.Minute
	startLocal := time.Unix(req.StartTS, 0).UTC().Add(-tzDuration)
	endLocal := time.Unix(req.EndTS, 0).UTC().Add(-tzDuration).Add(-24 * time.Hour) // EndTS is exclusive

	query := `
		SELECT
			u.id,
			u.email,
			u.name,
			COUNT(DISTINCT qs.session_id) as session_count,
			COALESCE(SUM(qs.cost), 0) as total_cost_usd,
			COALESCE(SUM(qs.duration_ms), 0) as total_duration_ms,
			COALESCE(SUM(qs.claude_time_ms), 0) as total_claude_time_ms,
			COALESCE(SUM(qs.user_time_ms), 0) as total_user_time_ms
		FROM users u
		LEFT JOIN LATERAL (
			SELECT
				s.id as session_id,
				t.estimated_cost_usd::numeric as cost,
				COALESCE(sess.duration_ms, 0) as duration_ms,
				cv.total_assistant_duration_ms as claude_time_ms,
				cv.total_user_duration_ms as user_time_ms
			FROM sessions s
			INNER JOIN session_card_tokens t ON s.id = t.session_id
			INNER JOIN session_card_conversation cv ON s.id = cv.session_id
			LEFT JOIN session_card_session sess ON s.id = sess.session_id
			WHERE s.user_id = u.id
				AND s.first_seen >= to_timestamp($1)
				AND s.first_seen < to_timestamp($2)
		) qs ON true
		WHERE u.status = 'active'
		GROUP BY u.id, u.email, u.name
		ORDER BY u.name ASC NULLS LAST, u.email ASC
	`

	rows, err := s.db.QueryContext(ctx, query, req.StartTS, req.EndTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []OrgUserAnalytics
	for rows.Next() {
		var (
			userID       int64
			email        string
			name         sql.NullString
			sessionCount int
			totalCost    decimal.Decimal
			totalDurMs   int64
			totalClaude  int64
			totalUser    int64
		)

		if err := rows.Scan(&userID, &email, &name, &sessionCount, &totalCost, &totalDurMs, &totalClaude, &totalUser); err != nil {
			return nil, err
		}

		ua := OrgUserAnalytics{
			User: OrgUserInfo{
				ID:    userID,
				Email: email,
			},
			SessionCount:      sessionCount,
			TotalCostUSD:      totalCost.StringFixed(2),
			TotalDurationMs:   totalDurMs,
			TotalClaudeTimeMs: totalClaude,
			TotalUserTimeMs:   totalUser,
			AvgCostUSD:        "0.00",
		}

		if name.Valid {
			ua.User.Name = &name.String
		}

		// Compute per-session averages (only when sessions exist)
		if sessionCount > 0 {
			avgCost := totalCost.Div(decimal.NewFromInt(int64(sessionCount)))
			ua.AvgCostUSD = avgCost.StringFixed(2)

			avgDur := totalDurMs / int64(sessionCount)
			ua.AvgDurationMs = &avgDur

			avgClaude := totalClaude / int64(sessionCount)
			ua.AvgClaudeTimeMs = &avgClaude

			avgUser := totalUser / int64(sessionCount)
			ua.AvgUserTimeMs = &avgUser
		}

		users = append(users, ua)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Ensure non-nil slice for JSON serialization
	if users == nil {
		users = []OrgUserAnalytics{}
	}

	return &OrgAnalyticsResponse{
		ComputedAt: time.Now().UTC(),
		DateRange: DateRange{
			StartDate: startLocal.Format("2006-01-02"),
			EndDate:   endLocal.Format("2006-01-02"),
		},
		Users: users,
	}, nil
}
