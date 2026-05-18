package analytics

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// resolveProviderFilter expands canonical wire values to the canonical+legacy
// forms used for SQL filtering on `sessions.session_type`. Returns the full
// AllowedProviders list when input is empty so the `session_type = ANY` clause
// is always present and a missing provider param can never silently exclude
// rows (CF-352-style failure mode).
func resolveProviderFilter(providers []string) []string {
	if len(providers) == 0 {
		return models.AllowedProviders
	}
	return models.ExpandWithAliases(providers)
}

// GetTrends retrieves aggregated analytics across sessions for a user.
// All card aggregations run in parallel to minimize latency.
//
// The provider filter is resolved once here and threaded as pq.Array through
// each aggregation, so the five queries can never disagree on which session_type
// values they accept.
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
		ReposIncluded:    req.Repos,
		IncludeNoRepo:    req.IncludeNoRepo,
		ProvidersPresent: []string{},
		Cards:            TrendsCards{},
	}

	// Ensure ReposIncluded is an empty slice (not nil) for JSON serialization
	if req.Repos == nil {
		response.ReposIncluded = []string{}
	}

	providerArg := pq.Array(resolveProviderFilter(req.Providers))

	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 7)

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
		overview, activity, utilization, count, err := s.aggregateOverviewAndActivity(ctx, userID, req, providerArg)
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
		tokens, err := s.aggregateTokens(ctx, userID, req, providerArg)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tokens = tokens
		mu.Unlock()
		return nil
	})

	runAgg("tools", func() error {
		tools, err := s.aggregateTools(ctx, userID, req, providerArg)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tools = tools
		mu.Unlock()
		return nil
	})

	runAgg("agents_and_skills", func() error {
		agentsAndSkills, err := s.aggregateAgentsAndSkills(ctx, userID, req, providerArg)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.AgentsAndSkills = agentsAndSkills
		mu.Unlock()
		return nil
	})

	runAgg("top_sessions", func() error {
		topSessions, err := s.aggregateTopSessions(ctx, userID, req, providerArg)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.TopSessions = topSessions
		mu.Unlock()
		return nil
	})

	runAgg("providers_present", func() error {
		providersPresent, err := s.aggregateProvidersPresent(ctx, userID, req, providerArg)
		if err != nil {
			return err
		}
		mu.Lock()
		response.ProvidersPresent = providersPresent
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
func (s *Store) aggregateOverviewAndActivity(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (*TrendsOverviewCard, *TrendsActivityCard, *TrendsUtilizationCard, int, error) {
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
				AND s.session_type = ANY($7::text[])
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
		providerArg,
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

// aggregateTokens computes the tokens card with daily cost breakdown plus the
// per-provider breakdown introduced in CF-435. Two queries share the same
// filtered_sessions CTE shape: the first drives daily costs and cross-provider
// totals; the second groups by s.session_type for the per_provider map.
// Uses generate_series so all dates in range are returned (zeros for missing
// days). Per-provider keys are normalized via models.NormalizeProvider at the
// Scan site so legacy 'Claude Code' rows fold into 'claude-code'.
func (s *Store) aggregateTokens(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (*TrendsTokensCard, error) {
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
				AND s.session_type = ANY($7::text[])
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
		providerArg,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		dailyCosts         = []DailyCostPoint{}
		totalInput         int64
		totalOutput        int64
		totalCacheCreation int64
		totalCacheRead     int64
		totalCost          = decimal.Zero
	)

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

	perProvider, err := s.aggregateTokensPerProvider(ctx, userID, req, providerArg)
	if err != nil {
		return nil, err
	}

	return &TrendsTokensCard{
		TotalInputTokens:         totalInput,
		TotalOutputTokens:        totalOutput,
		TotalCacheCreationTokens: totalCacheCreation,
		TotalCacheReadTokens:     totalCacheRead,
		TotalCostUSD:             totalCost.String(),
		DailyCosts:               dailyCosts,
		PerProvider:              perProvider,
	}, nil
}

// aggregateTokensPerProvider returns the per-provider tokens/cost breakdown
// for the trends Tokens card (CF-435). GROUP BY s.session_type captures every
// provider present in the filtered range; the Scan loop normalizes legacy
// 'Claude Code' rows into 'claude-code' so the wire surface only exposes
// canonical provider ids. Returns a non-nil empty map when the range is
// empty so JSON serialization yields {} rather than null.
func (s *Store) aggregateTokensPerProvider(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (map[string]*TrendsTokensPerProvider, error) {
	query := `
		WITH filtered_sessions AS (
			SELECT s.id, s.session_type
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
				AND s.session_type = ANY($6::text[])
		)
		SELECT
			fs.session_type,
			COALESCE(SUM(t.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(t.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(t.cache_creation_tokens), 0) AS cache_creation_tokens,
			COALESCE(SUM(t.cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(t.estimated_cost_usd::numeric), 0) AS cost_usd
		FROM filtered_sessions fs
		LEFT JOIN session_card_tokens t ON fs.id = t.session_id
		GROUP BY fs.session_type
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
		providerArg,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Accumulate cost as decimal.Decimal and stringify once after the loop so
	// the legacy-alias fold doesn't round-trip strings through NewFromString
	// for every merged row.
	out := map[string]*TrendsTokensPerProvider{}
	costs := map[string]decimal.Decimal{}
	for rows.Next() {
		var rawProvider, costStr string
		var input, output, cacheCreation, cacheRead int64
		if err := rows.Scan(&rawProvider, &input, &output, &cacheCreation, &cacheRead, &costStr); err != nil {
			return nil, err
		}
		canonical := models.NormalizeProvider(rawProvider)
		cost, _ := decimal.NewFromString(costStr)

		// Get-or-create the canonical bucket, then accumulate. When two SQL
		// rows share a canonical key (e.g. 'Claude Code' + 'claude-code'),
		// the second row folds into the first.
		entry, ok := out[canonical]
		if !ok {
			entry = &TrendsTokensPerProvider{}
			out[canonical] = entry
		}
		entry.TotalInputTokens += input
		entry.TotalOutputTokens += output
		entry.TotalCacheCreationTokens += cacheCreation
		entry.TotalCacheReadTokens += cacheRead
		costs[canonical] = costs[canonical].Add(cost)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for canonical, entry := range out {
		entry.TotalCostUSD = costs[canonical].String()
	}
	return out, nil
}

// aggregateTools computes the tools card with per-tool breakdown.
func (s *Store) aggregateTools(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (*TrendsToolsCard, error) {
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
				AND s.session_type = ANY($6::text[])
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
		providerArg,
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

// aggregateTopSessions returns the top 10 most expensive sessions ranked by cost.
func (s *Store) aggregateTopSessions(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (*TrendsTopSessionsCard, error) {
	query := `
		WITH filtered_sessions AS (
			SELECT
				s.id,
				s.external_id,
				s.session_type,
				COALESCE(s.custom_title, s.suggested_session_title, s.summary, s.first_user_message) AS title,
				NULLIF(regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1'), '') AS git_repo
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
				AND s.session_type = ANY($6::text[])
		)
		SELECT fs.id, fs.external_id, fs.session_type, fs.title, fs.git_repo,
			   t.estimated_cost_usd, sess.duration_ms
		FROM filtered_sessions fs
		INNER JOIN session_card_tokens t ON fs.id = t.session_id
		LEFT JOIN session_card_session sess ON fs.id = sess.session_id
		WHERE t.estimated_cost_usd > 0
		ORDER BY t.estimated_cost_usd DESC
		LIMIT 10
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
		providerArg,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []TopSessionItem{}
	for rows.Next() {
		var item TopSessionItem
		var externalID string
		var providerRaw string
		var title *string
		var costStr string

		err := rows.Scan(
			&item.ID,
			&externalID,
			&providerRaw,
			&title,
			&item.GitRepo,
			&costStr,
			&item.DurationMs,
		)
		if err != nil {
			return nil, err
		}

		// Normalize legacy 'Claude Code' → canonical 'claude-code' (CLAUDE.md
		// invariant: every Scan site reading sessions.session_type must call
		// models.NormalizeProvider so the API surface only exposes canonical values).
		item.Provider = models.NormalizeProvider(providerRaw)

		if title != nil {
			item.Title = *title
		} else {
			// Fallback: "Untitled session - <first 8 chars of external_id>"
			truncID := externalID
			if len(truncID) > 8 {
				truncID = truncID[:8]
			}
			item.Title = "Untitled session - " + truncID
		}

		cost, _ := decimal.NewFromString(costStr)
		item.EstimatedCostUSD = cost.String()

		sessions = append(sessions, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TrendsTopSessionsCard{Sessions: sessions}, nil
}

// aggregateAgentsAndSkills computes the agents and skills card with per-name breakdown.
func (s *Store) aggregateAgentsAndSkills(ctx context.Context, userID int64, req TrendsRequest, providerArg any) (*TrendsAgentsAndSkillsCard, error) {
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
				AND s.session_type = ANY($6::text[])
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
		providerArg,
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

// aggregateProvidersPresent returns the distinct canonical providers in the
// filtered session set, sorted alphabetically. Drives the Tokens card's
// multi-provider caveat (CF-424): when len >= 2, the frontend renders a
// notice that totals span multiple AI providers. Legacy 'Claude Code' rows
// are collapsed into 'claude-code' via models.NormalizeProvider so the API
// surface only exposes canonical values.
func (s *Store) aggregateProvidersPresent(ctx context.Context, userID int64, req TrendsRequest, providerArg any) ([]string, error) {
	query := `
		WITH filtered_sessions AS (
			SELECT s.session_type
			FROM sessions s
			WHERE s.user_id = $1
				AND s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1') = ANY($4::text[])
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
				AND s.session_type = ANY($6::text[])
		)
		SELECT DISTINCT session_type FROM filtered_sessions
	`

	rows, err := s.db.QueryContext(ctx, query,
		userID,
		req.StartTS,
		req.EndTS,
		pq.Array(req.Repos),
		req.IncludeNoRepo,
		providerArg,
	)
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
