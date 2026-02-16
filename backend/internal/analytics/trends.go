package analytics

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GetTrends retrieves aggregated analytics across sessions for a user.
// All card aggregations run in parallel to minimize latency.
func (s *Store) GetTrends(ctx context.Context, userID int64, req TrendsRequest) (*TrendsResponse, error) {
	ctx, span := tracer.Start(ctx, "analytics.get_trends",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.Int64("start_ts", req.StartTS),
			attribute.Int64("end_ts", req.EndTS),
			attribute.Int("tz_offset", req.TZOffset),
		))
	defer span.End()

	// Derive local dates from epoch timestamps and timezone offset for the response
	tzDuration := time.Duration(req.TZOffset) * time.Minute
	startLocal := time.Unix(req.StartTS, 0).UTC().Add(-tzDuration)
	endLocal := time.Unix(req.EndTS, 0).UTC().Add(-tzDuration).Add(-24 * time.Hour) // EndTS is exclusive

	response := &TrendsResponse{
		ComputedAt: time.Now().UTC(),
		DateRange: DateRange{
			StartDate: startLocal.Format("2006-01-02"),
			EndDate:   endLocal.Format("2006-01-02"),
		},
		ReposIncluded: req.Repos,
		IncludeNoRepo: req.IncludeNoRepo,
		Cards:         TrendsCards{},
	}

	// Ensure ReposIncluded is an empty slice (not nil) for JSON serialization
	if req.Repos == nil {
		response.ReposIncluded = []string{}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 5)

	// Helper to run aggregation in parallel
	runAgg := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				errChan <- err
			}
		}()
	}

	// Run all card aggregations in parallel
	runAgg("overview_activity", func() error {
		overview, activity, utilization, count, err := s.aggregateOverviewAndActivity(ctx, userID, req)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Overview = overview
		response.Cards.Activity = activity
		response.Cards.Utilization = utilization
		response.SessionCount = count
		mu.Unlock()
		return nil
	})

	runAgg("tokens", func() error {
		tokens, err := s.aggregateTokens(ctx, userID, req)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tokens = tokens
		mu.Unlock()
		return nil
	})

	runAgg("tools", func() error {
		tools, err := s.aggregateTools(ctx, userID, req)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tools = tools
		mu.Unlock()
		return nil
	})

	runAgg("agents_and_skills", func() error {
		agentsAndSkills, err := s.aggregateAgentsAndSkills(ctx, userID, req)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.AgentsAndSkills = agentsAndSkills
		mu.Unlock()
		return nil
	})

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

// aggregateOverviewAndActivity computes the overview, activity, and utilization cards.
// These are combined because they share the same session and code_activity queries.
func (s *Store) aggregateOverviewAndActivity(ctx context.Context, userID int64, req TrendsRequest) (*TrendsOverviewCard, *TrendsActivityCard, *TrendsUtilizationCard, int, error) {
	// Query sessions and their code activity data
	// Uses generate_series to ensure all dates in range are returned (with zeros for missing days)
	// $2/$3 are epoch seconds; $6 is the client TZ offset in minutes (JS getTimezoneOffset convention)
	query := `
		WITH date_range AS (
			SELECT generate_series(
				(to_timestamp($2) - make_interval(mins => $6))::date,
				(to_timestamp($3) - make_interval(mins => $6) - interval '1 day')::date,
				'1 day'
			)::date as d
		),
		filtered_sessions AS (
			SELECT
				s.id,
				(s.first_seen - make_interval(mins => $6))::date as session_date
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
		),
		daily_agg AS (
			SELECT
				fs.session_date,
				COUNT(DISTINCT fs.id) as session_count,
				COALESCE(SUM(sess.duration_ms), 0) as total_duration_ms,
				COALESCE(SUM(ca.files_read), 0) as files_read,
				COALESCE(SUM(ca.files_modified), 0) as files_modified,
				COALESCE(SUM(ca.lines_added), 0) as lines_added,
				COALESCE(SUM(ca.lines_removed), 0) as lines_removed,
				COALESCE(SUM(cv.total_assistant_duration_ms), 0) as assistant_duration_ms
			FROM filtered_sessions fs
			LEFT JOIN session_card_session sess ON fs.id = sess.session_id
			LEFT JOIN session_card_code_activity ca ON fs.id = ca.session_id
			LEFT JOIN session_card_conversation cv ON fs.id = cv.session_id
			GROUP BY fs.session_date
		)
		SELECT
			dr.d as session_date,
			COALESCE(da.session_count, 0) as session_count,
			COALESCE(da.total_duration_ms, 0) as total_duration_ms,
			COALESCE(da.files_read, 0) as files_read,
			COALESCE(da.files_modified, 0) as files_modified,
			COALESCE(da.lines_added, 0) as lines_added,
			COALESCE(da.lines_removed, 0) as lines_removed,
			COALESCE(da.assistant_duration_ms, 0) as assistant_duration_ms
		FROM date_range dr
		LEFT JOIN daily_agg da ON dr.d = da.session_date
		ORDER BY dr.d
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
		req.TZOffset,
	)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer rows.Close()

	var dailyData []DailyActivityAggregation
	var totalSessions int
	var totalDurationMs int64
	var totalAssistantDurationMs int64
	var totalFilesRead, totalFilesModified, totalLinesAdded, totalLinesRemoved int

	for rows.Next() {
		var d DailyActivityAggregation
		var sessionDate time.Time
		err := rows.Scan(
			&sessionDate,
			&d.SessionCount,
			&d.DurationMs,
			&d.FilesRead,
			&d.FilesModified,
			&d.LinesAdded,
			&d.LinesRemoved,
			&d.AssistantDurationMs,
		)
		if err != nil {
			return nil, nil, nil, 0, err
		}
		d.Date = sessionDate.Format("2006-01-02")
		dailyData = append(dailyData, d)

		totalSessions += d.SessionCount
		totalDurationMs += d.DurationMs
		totalAssistantDurationMs += d.AssistantDurationMs
		totalFilesRead += d.FilesRead
		totalFilesModified += d.FilesModified
		totalLinesAdded += d.LinesAdded
		totalLinesRemoved += d.LinesRemoved
	}

	if err := rows.Err(); err != nil {
		return nil, nil, nil, 0, err
	}

	// Build daily session counts and utilization for charts, count days with activity
	dailyCounts := make([]DailySessionCount, len(dailyData))
	dailyUtilization := make([]DailyUtilizationPoint, len(dailyData))
	daysWithActivity := 0
	for i, d := range dailyData {
		dailyCounts[i] = DailySessionCount{
			Date:         d.Date,
			SessionCount: d.SessionCount,
		}
		// Calculate daily utilization: only if there's duration data for that day
		point := DailyUtilizationPoint{Date: d.Date}
		if d.DurationMs > 0 {
			util := float64(d.AssistantDurationMs) / float64(d.DurationMs) * 100
			point.UtilizationPct = &util
		}
		dailyUtilization[i] = point
		if d.SessionCount > 0 {
			daysWithActivity++
		}
	}

	overview := &TrendsOverviewCard{
		SessionCount:             totalSessions,
		TotalDurationMs:          totalDurationMs,
		DaysCovered:              daysWithActivity,
		TotalAssistantDurationMs: totalAssistantDurationMs,
	}
	if totalSessions > 0 {
		avgMs := totalDurationMs / int64(totalSessions)
		overview.AvgDurationMs = &avgMs
	}
	// Calculate utilization percentage: (assistant time / total duration) * 100
	if totalDurationMs > 0 {
		utilization := float64(totalAssistantDurationMs) / float64(totalDurationMs) * 100
		overview.AssistantUtilizationPct = &utilization
	}

	activity := &TrendsActivityCard{
		TotalFilesRead:     totalFilesRead,
		TotalFilesModified: totalFilesModified,
		TotalLinesAdded:    totalLinesAdded,
		TotalLinesRemoved:  totalLinesRemoved,
		DailySessionCounts: dailyCounts,
	}

	utilizationCard := &TrendsUtilizationCard{
		DailyUtilization: dailyUtilization,
	}

	return overview, activity, utilizationCard, totalSessions, nil
}

// aggregateTokens computes the tokens card with daily cost breakdown.
// Uses generate_series to ensure all dates in range are returned (with zeros for missing days)
func (s *Store) aggregateTokens(ctx context.Context, userID int64, req TrendsRequest) (*TrendsTokensCard, error) {
	query := `
		WITH date_range AS (
			SELECT generate_series(
				(to_timestamp($2) - make_interval(mins => $6))::date,
				(to_timestamp($3) - make_interval(mins => $6) - interval '1 day')::date,
				'1 day'
			)::date as d
		),
		filtered_sessions AS (
			SELECT
				s.id,
				(s.first_seen - make_interval(mins => $6))::date as session_date
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
		),
		daily_agg AS (
			SELECT
				fs.session_date,
				COALESCE(SUM(t.input_tokens), 0) as input_tokens,
				COALESCE(SUM(t.output_tokens), 0) as output_tokens,
				COALESCE(SUM(t.cache_creation_tokens), 0) as cache_creation_tokens,
				COALESCE(SUM(t.cache_read_tokens), 0) as cache_read_tokens,
				COALESCE(SUM(t.estimated_cost_usd::numeric), 0) as cost_usd
			FROM filtered_sessions fs
			LEFT JOIN session_card_tokens t ON fs.id = t.session_id
			GROUP BY fs.session_date
		)
		SELECT
			dr.d as session_date,
			COALESCE(da.input_tokens, 0) as input_tokens,
			COALESCE(da.output_tokens, 0) as output_tokens,
			COALESCE(da.cache_creation_tokens, 0) as cache_creation_tokens,
			COALESCE(da.cache_read_tokens, 0) as cache_read_tokens,
			COALESCE(da.cost_usd, 0) as cost_usd
		FROM date_range dr
		LEFT JOIN daily_agg da ON dr.d = da.session_date
		ORDER BY dr.d
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
		req.TZOffset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dailyCosts := []DailyCostPoint{}
	totalInput := int64(0)
	totalOutput := int64(0)
	totalCacheCreation := int64(0)
	totalCacheRead := int64(0)
	totalCost := decimal.Zero

	for rows.Next() {
		var sessionDate time.Time
		var input, output, cacheCreation, cacheRead int64
		var costStr string

		err := rows.Scan(
			&sessionDate,
			&input,
			&output,
			&cacheCreation,
			&cacheRead,
			&costStr,
		)
		if err != nil {
			return nil, err
		}

		cost, _ := decimal.NewFromString(costStr)
		dailyCosts = append(dailyCosts, DailyCostPoint{
			Date:    sessionDate.Format("2006-01-02"),
			CostUSD: cost.String(),
		})

		totalInput += input
		totalOutput += output
		totalCacheCreation += cacheCreation
		totalCacheRead += cacheRead
		totalCost = totalCost.Add(cost)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TrendsTokensCard{
		TotalInputTokens:         totalInput,
		TotalOutputTokens:        totalOutput,
		TotalCacheCreationTokens: totalCacheCreation,
		TotalCacheReadTokens:     totalCacheRead,
		TotalCostUSD:             totalCost.String(),
		DailyCosts:               dailyCosts,
	}, nil
}

// aggregateTools computes the tools card with per-tool breakdown.
func (s *Store) aggregateTools(ctx context.Context, userID int64, req TrendsRequest) (*TrendsToolsCard, error) {
	query := `
		WITH filtered_sessions AS (
			SELECT
				s.id
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
		)
		SELECT
			t.total_calls,
			t.error_count,
			t.tool_breakdown
		FROM filtered_sessions fs
		INNER JOIN session_card_tools t ON fs.id = t.session_id
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totalCalls := 0
	totalErrors := 0
	aggregatedStats := make(map[string]*ToolStats)

	for rows.Next() {
		var calls, errors int
		var breakdownJSON []byte

		err := rows.Scan(&calls, &errors, &breakdownJSON)
		if err != nil {
			return nil, err
		}

		totalCalls += calls
		totalErrors += errors

		// Parse tool breakdown and aggregate
		if len(breakdownJSON) > 0 {
			var breakdown map[string]*ToolStats
			if err := json.Unmarshal(breakdownJSON, &breakdown); err == nil {
				for tool, stats := range breakdown {
					if aggregatedStats[tool] == nil {
						aggregatedStats[tool] = &ToolStats{}
					}
					aggregatedStats[tool].Success += stats.Success
					aggregatedStats[tool].Errors += stats.Errors
				}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TrendsToolsCard{
		TotalCalls:  totalCalls,
		TotalErrors: totalErrors,
		ToolStats:   aggregatedStats,
	}, nil
}

// aggregateAgentsAndSkills computes the agents and skills card with per-name breakdown.
func (s *Store) aggregateAgentsAndSkills(ctx context.Context, userID int64, req TrendsRequest) (*TrendsAgentsAndSkillsCard, error) {
	query := `
		WITH filtered_sessions AS (
			SELECT
				s.id
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
		)
		SELECT
			a.agent_invocations,
			a.skill_invocations,
			a.agent_stats,
			a.skill_stats
		FROM filtered_sessions fs
		INNER JOIN session_card_agents_and_skills a ON fs.id = a.session_id
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totalAgentInvocations := 0
	totalSkillInvocations := 0
	aggregatedAgentStats := make(map[string]*AgentStats)
	aggregatedSkillStats := make(map[string]*SkillStats)

	for rows.Next() {
		var agentInvocations, skillInvocations int
		var agentStatsJSON, skillStatsJSON []byte

		err := rows.Scan(&agentInvocations, &skillInvocations, &agentStatsJSON, &skillStatsJSON)
		if err != nil {
			return nil, err
		}

		totalAgentInvocations += agentInvocations
		totalSkillInvocations += skillInvocations

		// Parse and aggregate agent stats
		if len(agentStatsJSON) > 0 {
			var agentStats map[string]*AgentStats
			if err := json.Unmarshal(agentStatsJSON, &agentStats); err == nil {
				for name, stats := range agentStats {
					if aggregatedAgentStats[name] == nil {
						aggregatedAgentStats[name] = &AgentStats{}
					}
					aggregatedAgentStats[name].Success += stats.Success
					aggregatedAgentStats[name].Errors += stats.Errors
				}
			}
		}

		// Parse and aggregate skill stats
		if len(skillStatsJSON) > 0 {
			var skillStats map[string]*SkillStats
			if err := json.Unmarshal(skillStatsJSON, &skillStats); err == nil {
				for name, stats := range skillStats {
					if aggregatedSkillStats[name] == nil {
						aggregatedSkillStats[name] = &SkillStats{}
					}
					aggregatedSkillStats[name].Success += stats.Success
					aggregatedSkillStats[name].Errors += stats.Errors
				}
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TrendsAgentsAndSkillsCard{
		TotalAgentInvocations: totalAgentInvocations,
		TotalSkillInvocations: totalSkillInvocations,
		AgentStats:            aggregatedAgentStats,
		SkillStats:            aggregatedSkillStats,
	}, nil
}
