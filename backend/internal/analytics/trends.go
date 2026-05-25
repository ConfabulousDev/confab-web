package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/db"
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

// trendsQuery captures the shared CTE prelude every trends aggregation
// splices into its own query. The prelude defines:
//
//	visible_sessions(id, user_id, owner_email, access_type, shared_by_email)
//	filtered_sessions(id, owner_email, session_type, session_date)
//
// args has a fixed positional layout so every aggregation can reference the
// same placeholders without recomputing:
//
//	$1 userID
//	$2 StartTS (epoch seconds, inclusive)
//	$3 EndTS   (epoch seconds, exclusive)
//	$4 Repos   (pq.Array text[])
//	$5 IncludeNoRepo (bool)
//	$6 TZOffset (int minutes; JS getTimezoneOffset convention)
//	$7 Providers (pq.Array text[])
//	$8 Owners (pq.Array text[], lowercased) — only present when len(req.Owners) > 0
//
// CF-495: every aggregation routes through this prelude so the visibility
// predicate and owner narrowing live in one place. providers_present
// inherits the owner narrowing automatically (multi-provider caveat reflects
// the data the user actually sees).
type trendsQuery struct {
	cteSQL string
	args   []interface{}
}

func buildTrendsQuery(userID int64, req TrendsRequest) trendsQuery {
	args := []interface{}{
		userID,              // $1
		req.StartTS,         // $2
		req.EndTS,           // $3
		pq.Array(req.Repos), // $4
		req.IncludeNoRepo,   // $5
		req.TZOffset,        // $6
		pq.Array(resolveProviderFilter(req.Providers)), // $7
	}
	ownerClause := ""
	if len(req.Owners) > 0 {
		args = append(args, pq.Array(lowercaseAll(req.Owners)))
		ownerClause = "\n\t\t\t\tAND LOWER(vs.owner_email) = ANY($8::text[])"
	}

	cte := `WITH ` + db.VisibleSessionsCTE(req.ShareAllSessions) + `,
		filtered_sessions AS (
			SELECT
				vs.id,
				vs.owner_email,
				s.session_type,
				(s.first_seen - make_interval(mins => $6))::date as session_date
			FROM (SELECT DISTINCT id, user_id, owner_email FROM visible_sessions) vs
			JOIN sessions s ON vs.id = s.id
			WHERE s.first_seen >= to_timestamp($2)
				AND s.first_seen < to_timestamp($3)
				AND (
					` + db.RepoMatchExpr("s", "$4::text[]") + `
					OR (COALESCE(cardinality($4::text[]), 0) = 0 AND COALESCE(s.git_info->>'repo_url', '') <> '')
					OR ($5 = true AND COALESCE(s.git_info->>'repo_url', '') = '')
				)
				AND s.session_type = ANY($7::text[])` + ownerClause + `
		)`

	return trendsQuery{cteSQL: cte, args: args}
}

func lowercaseAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.ToLower(s)
	}
	return out
}

// GetTrends retrieves aggregated analytics across sessions visible to the
// user. All card aggregations run in parallel to minimize latency. Every
// aggregation routes through buildTrendsQuery so the visibility predicate
// + owner narrowing live in exactly one place (CF-495).
func (s *Store) GetTrends(ctx context.Context, userID int64, req TrendsRequest) (*TrendsResponse, error) {
	ctx, span := tracer.Start(ctx, "analytics.get_trends",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.Int64("start_ts", req.StartTS),
			attribute.Int64("end_ts", req.EndTS),
			attribute.Int("tz_offset", req.TZOffset),
			attribute.Int("owners.count", len(req.Owners)),
			attribute.Bool("share_all_sessions", req.ShareAllSessions),
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
		FilterOptions:    TrendsFilterOptions{Owners: []string{}, Repos: []string{}},
	}

	if req.Repos == nil {
		response.ReposIncluded = []string{}
	}

	tq := buildTrendsQuery(userID, req)

	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 7)

	runAgg := func(_ string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				errChan <- err
			}
		}()
	}

	runAgg("overview_activity", func() error {
		overview, activity, utilization, count, err := s.aggregateOverviewAndActivity(ctx, tq)
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
		tokens, err := s.aggregateTokens(ctx, tq)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tokens = tokens
		mu.Unlock()
		return nil
	})

	runAgg("tools", func() error {
		tools, err := s.aggregateTools(ctx, tq)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.Tools = tools
		mu.Unlock()
		return nil
	})

	runAgg("agents_and_skills", func() error {
		agentsAndSkills, err := s.aggregateAgentsAndSkills(ctx, tq)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.AgentsAndSkills = agentsAndSkills
		mu.Unlock()
		return nil
	})

	runAgg("top_sessions", func() error {
		topSessions, err := s.aggregateTopSessions(ctx, tq)
		if err != nil {
			return err
		}
		mu.Lock()
		response.Cards.TopSessions = topSessions
		mu.Unlock()
		return nil
	})

	runAgg("providers_present", func() error {
		providersPresent, err := s.aggregateProvidersPresent(ctx, tq)
		if err != nil {
			return err
		}
		mu.Lock()
		response.ProvidersPresent = providersPresent
		mu.Unlock()
		return nil
	})

	runAgg("filter_options", func() error {
		opts, err := s.aggregateFilterOptions(ctx, userID, req.ShareAllSessions)
		if err != nil {
			return err
		}
		mu.Lock()
		response.FilterOptions = opts
		mu.Unlock()
		return nil
	})

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

// aggregateOverviewAndActivity computes overview, activity, and utilization
// cards in one query (they share the same filtered_sessions / card joins).
//
// daily_agg groups by (session_date, session_type) so DailySessionCount can
// carry a per-provider session-count map for the stacked-bar chart. Empty
// days from the date_range LEFT JOIN surface as one row with session_type
// '' and zero numerics — used only to register the date in the output (no
// provider accumulation). Provider keys are folded server-side via
// models.NormalizeProvider so legacy 'Claude Code' rows collapse into
// 'claude-code'.
func (s *Store) aggregateOverviewAndActivity(ctx context.Context, tq trendsQuery) (*TrendsOverviewCard, *TrendsActivityCard, *TrendsUtilizationCard, int, error) {
	query := tq.cteSQL + `,
		date_range AS (
			SELECT generate_series(
				(to_timestamp($2) - make_interval(mins => $6))::date,
				(to_timestamp($3) - make_interval(mins => $6) - interval '1 day')::date,
				'1 day'
			)::date as d
		),
		daily_agg AS (
			SELECT
				fs.session_date,
				fs.session_type,
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
			GROUP BY fs.session_date, fs.session_type
		)
		SELECT
			dr.d as session_date,
			COALESCE(da.session_type, '') as session_type,
			COALESCE(da.session_count, 0) as session_count,
			COALESCE(da.total_duration_ms, 0) as total_duration_ms,
			COALESCE(da.files_read, 0) as files_read,
			COALESCE(da.files_modified, 0) as files_modified,
			COALESCE(da.lines_added, 0) as lines_added,
			COALESCE(da.lines_removed, 0) as lines_removed,
			COALESCE(da.assistant_duration_ms, 0) as assistant_duration_ms
		FROM date_range dr
		LEFT JOIN daily_agg da ON dr.d = da.session_date
		ORDER BY dr.d, da.session_type
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer rows.Close()

	type dayBucket struct {
		date                string
		sessionCount        int
		durationMs          int64
		assistantDurationMs int64
		perProvider         map[string]int
	}
	var (
		dateOrder                = []string{}
		dailyByDate              = map[string]*dayBucket{}
		totalSessions            int
		totalDurationMs          int64
		totalAssistantDurationMs int64
		totalFilesRead           int
		totalFilesModified       int
		totalLinesAdded          int
		totalLinesRemoved        int
	)

	for rows.Next() {
		var sessionDate time.Time
		var rawProvider string
		var sessionCount, filesRead, filesModified, linesAdded, linesRemoved int
		var durationMs, assistantDurationMs int64
		if err := rows.Scan(
			&sessionDate,
			&rawProvider,
			&sessionCount,
			&durationMs,
			&filesRead,
			&filesModified,
			&linesAdded,
			&linesRemoved,
			&assistantDurationMs,
		); err != nil {
			return nil, nil, nil, 0, err
		}

		dateKey := sessionDate.Format("2006-01-02")
		bucket, ok := dailyByDate[dateKey]
		if !ok {
			bucket = &dayBucket{date: dateKey, perProvider: map[string]int{}}
			dailyByDate[dateKey] = bucket
			dateOrder = append(dateOrder, dateKey)
		}

		if rawProvider == "" {
			continue
		}

		canonical := models.NormalizeProvider(rawProvider)
		bucket.sessionCount += sessionCount
		bucket.durationMs += durationMs
		bucket.assistantDurationMs += assistantDurationMs
		bucket.perProvider[canonical] += sessionCount

		totalSessions += sessionCount
		totalDurationMs += durationMs
		totalAssistantDurationMs += assistantDurationMs
		totalFilesRead += filesRead
		totalFilesModified += filesModified
		totalLinesAdded += linesAdded
		totalLinesRemoved += linesRemoved
	}

	if err := rows.Err(); err != nil {
		return nil, nil, nil, 0, err
	}

	dailyCounts := make([]DailySessionCount, 0, len(dateOrder))
	dailyUtilization := make([]DailyUtilizationPoint, 0, len(dateOrder))
	daysWithActivity := 0
	for _, dateKey := range dateOrder {
		bucket := dailyByDate[dateKey]
		dailyCounts = append(dailyCounts, DailySessionCount{
			Date:         bucket.date,
			SessionCount: bucket.sessionCount,
			PerProvider:  bucket.perProvider,
		})
		point := DailyUtilizationPoint{Date: bucket.date}
		if bucket.durationMs > 0 {
			util := float64(bucket.assistantDurationMs) / float64(bucket.durationMs) * 100
			point.UtilizationPct = &util
		}
		dailyUtilization = append(dailyUtilization, point)
		if bucket.sessionCount > 0 {
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

// aggregateTokens computes the tokens card in one SQL pass: per-day
// per-provider cost rows for the stacked-bar chart, per-provider grand
// totals, and cross-provider grand totals. A date_range LEFT JOIN backfills
// every day in the range so empty days still appear. Provider keys are
// normalized via models.NormalizeProvider at the Scan site so legacy
// 'Claude Code' rows fold into 'claude-code'.
func (s *Store) aggregateTokens(ctx context.Context, tq trendsQuery) (*TrendsTokensCard, error) {
	query := tq.cteSQL + `,
		date_range AS (
			SELECT generate_series(
				(to_timestamp($2) - make_interval(mins => $6))::date,
				(to_timestamp($3) - make_interval(mins => $6) - interval '1 day')::date,
				'1 day'
			)::date as d
		),
		per_day_per_provider AS (
			SELECT
				fs.session_date,
				fs.session_type,
				COALESCE(SUM(t.input_tokens), 0) as input_tokens,
				COALESCE(SUM(t.output_tokens), 0) as output_tokens,
				COALESCE(SUM(t.cache_creation_tokens), 0) as cache_creation_tokens,
				COALESCE(SUM(t.cache_read_tokens), 0) as cache_read_tokens,
				COALESCE(SUM(t.estimated_cost_usd::numeric), 0) as cost_usd
			FROM filtered_sessions fs
			LEFT JOIN session_card_tokens t ON fs.id = t.session_id
			GROUP BY fs.session_date, fs.session_type
		)
		SELECT
			dr.d as session_date,
			COALESCE(pdpp.session_type, '') as session_type,
			COALESCE(pdpp.input_tokens, 0),
			COALESCE(pdpp.output_tokens, 0),
			COALESCE(pdpp.cache_creation_tokens, 0),
			COALESCE(pdpp.cache_read_tokens, 0),
			COALESCE(pdpp.cost_usd, 0)
		FROM date_range dr
		LEFT JOIN per_day_per_provider pdpp ON dr.d = pdpp.session_date
		ORDER BY dr.d, pdpp.session_type
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type dailyAccum struct {
		total       decimal.Decimal
		perProvider map[string]decimal.Decimal
	}
	type providerAccum struct {
		entry *TrendsTokensPerProvider
		cost  decimal.Decimal
	}
	var (
		datesInOrder       []string
		dailyByDate        = map[string]*dailyAccum{}
		perProvider        = map[string]*providerAccum{}
		totalInput         int64
		totalOutput        int64
		totalCacheCreation int64
		totalCacheRead     int64
		totalCost          = decimal.Zero
	)

	for rows.Next() {
		var sessionDate time.Time
		var rawProvider, costStr string
		var input, output, cacheCreation, cacheRead int64
		if err := rows.Scan(&sessionDate, &rawProvider, &input, &output, &cacheCreation, &cacheRead, &costStr); err != nil {
			return nil, err
		}

		dateKey := sessionDate.Format("2006-01-02")
		day, ok := dailyByDate[dateKey]
		if !ok {
			day = &dailyAccum{total: decimal.Zero, perProvider: map[string]decimal.Decimal{}}
			dailyByDate[dateKey] = day
			datesInOrder = append(datesInOrder, dateKey)
		}

		if rawProvider == "" {
			continue
		}

		canonical := models.NormalizeProvider(rawProvider)
		cost, _ := decimal.NewFromString(costStr)

		day.perProvider[canonical] = day.perProvider[canonical].Add(cost)
		day.total = day.total.Add(cost)

		prov, ok := perProvider[canonical]
		if !ok {
			prov = &providerAccum{entry: &TrendsTokensPerProvider{}}
			perProvider[canonical] = prov
		}
		prov.entry.TotalInputTokens += input
		prov.entry.TotalOutputTokens += output
		prov.entry.TotalCacheCreationTokens += cacheCreation
		prov.entry.TotalCacheReadTokens += cacheRead
		prov.cost = prov.cost.Add(cost)

		totalInput += input
		totalOutput += output
		totalCacheCreation += cacheCreation
		totalCacheRead += cacheRead
		totalCost = totalCost.Add(cost)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	perProviderTotals := make(map[string]*TrendsTokensPerProvider, len(perProvider))
	for canonical, prov := range perProvider {
		prov.entry.TotalCostUSD = prov.cost.String()
		perProviderTotals[canonical] = prov.entry
	}

	dailyCosts := make([]DailyCostPoint, 0, len(datesInOrder))
	for _, dateKey := range datesInOrder {
		day := dailyByDate[dateKey]
		perDay := make(map[string]string, len(day.perProvider))
		for provider, cost := range day.perProvider {
			perDay[provider] = cost.String()
		}
		dailyCosts = append(dailyCosts, DailyCostPoint{
			Date:        dateKey,
			CostUSD:     day.total.String(),
			PerProvider: perDay,
		})
	}

	return &TrendsTokensCard{
		TotalInputTokens:         totalInput,
		TotalOutputTokens:        totalOutput,
		TotalCacheCreationTokens: totalCacheCreation,
		TotalCacheReadTokens:     totalCacheRead,
		TotalCostUSD:             totalCost.String(),
		DailyCosts:               dailyCosts,
		PerProvider:              perProviderTotals,
	}, nil
}

// aggregateTools computes the tools card with per-tool breakdown.
func (s *Store) aggregateTools(ctx context.Context, tq trendsQuery) (*TrendsToolsCard, error) {
	query := tq.cteSQL + `
		SELECT t.total_calls, t.error_count, t.tool_breakdown
		FROM filtered_sessions fs
		INNER JOIN session_card_tools t ON fs.id = t.session_id
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
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

		if err := rows.Scan(&calls, &errors, &breakdownJSON); err != nil {
			return nil, err
		}

		totalCalls += calls
		totalErrors += errors

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
func (s *Store) aggregateTopSessions(ctx context.Context, tq trendsQuery) (*TrendsTopSessionsCard, error) {
	query := tq.cteSQL + `
		SELECT
			fs.id,
			s.external_id,
			fs.session_type,
			COALESCE(s.custom_title, s.suggested_session_title, s.summary, s.first_user_message) AS title,
			NULLIF(regexp_replace(regexp_replace(COALESCE(s.git_info->>'repo_url', ''), '\.git$', ''), '^.*[/:]([^/:]+/[^/:]+)$', '\1'), '') AS git_repo,
			t.estimated_cost_usd,
			sess.duration_ms
		FROM filtered_sessions fs
		JOIN sessions s ON fs.id = s.id
		INNER JOIN session_card_tokens t ON fs.id = t.session_id
		LEFT JOIN session_card_session sess ON fs.id = sess.session_id
		WHERE t.estimated_cost_usd > 0
		ORDER BY t.estimated_cost_usd DESC
		LIMIT 10
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
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

		if err := rows.Scan(
			&item.ID,
			&externalID,
			&providerRaw,
			&title,
			&item.GitRepo,
			&costStr,
			&item.DurationMs,
		); err != nil {
			return nil, err
		}

		item.Provider = models.NormalizeProvider(providerRaw)

		if title != nil {
			item.Title = *title
		} else {
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
func (s *Store) aggregateAgentsAndSkills(ctx context.Context, tq trendsQuery) (*TrendsAgentsAndSkillsCard, error) {
	query := tq.cteSQL + `
		SELECT a.agent_invocations, a.skill_invocations, a.agent_stats, a.skill_stats
		FROM filtered_sessions fs
		INNER JOIN session_card_agents_and_skills a ON fs.id = a.session_id
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
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

		if err := rows.Scan(&agentInvocations, &skillInvocations, &agentStatsJSON, &skillStatsJSON); err != nil {
			return nil, err
		}

		totalAgentInvocations += agentInvocations
		totalSkillInvocations += skillInvocations

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
// multi-provider caveat (CF-424). Owner narrowing applies automatically
// since this query reads from filtered_sessions (CF-495).
func (s *Store) aggregateProvidersPresent(ctx context.Context, tq trendsQuery) ([]string, error) {
	query := tq.cteSQL + `
		SELECT DISTINCT session_type FROM filtered_sessions
	`

	rows, err := s.db.QueryContext(ctx, query, tq.args...)
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

// aggregateFilterOptions returns the owner + repo dropdown source for
// TrendsPage. CF-495: derives from visible_sessions (NOT filtered_sessions)
// so the lists are static across active filters — mirrors SessionFilterOptions.
//
// Owners are lowercased and alphabetical. Repos use db.RepoRootExpr (CF-491
// canonical) so fork→root collapsing is honored identically to Sessions/TILs.
func (s *Store) aggregateFilterOptions(ctx context.Context, userID int64, shareAllSessions bool) (TrendsFilterOptions, error) {
	query := `WITH ` + db.VisibleSessionsCTE(shareAllSessions) + `,
		visible_unique AS (
			SELECT DISTINCT id, user_id, owner_email FROM visible_sessions
		),
		owners_q AS (
			SELECT COALESCE(array_agg(DISTINCT LOWER(owner_email) ORDER BY LOWER(owner_email)), ARRAY[]::text[]) AS owners
			FROM visible_unique
		),
		repos_q AS (
			SELECT COALESCE(array_agg(DISTINCT root ORDER BY root), ARRAY[]::text[]) AS repos
			FROM (
				SELECT ` + db.RepoRootExpr("s") + ` AS root
				FROM visible_unique vs
				JOIN sessions s ON vs.id = s.id
				WHERE s.git_info->>'repo_url' IS NOT NULL
			) r
		)
		SELECT o.owners, r.repos FROM owners_q o, repos_q r
	`

	var owners, repos []string
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(pq.Array(&owners), pq.Array(&repos)); err != nil {
		return TrendsFilterOptions{}, fmt.Errorf("aggregate filter options: %w", err)
	}
	return TrendsFilterOptions{
		Owners: nonNilSlice(owners),
		Repos:  nonNilSlice(repos),
	}, nil
}

func nonNilSlice(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}
