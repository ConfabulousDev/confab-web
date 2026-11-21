package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// admin_session_insights.go: the admin "Session Insights" tab (c3ts), an
// instance-wide view of session metadata. Unlike Trends and org analytics it is
// NOT scoped to what the viewer can see: it covers every listable session on the
// instance (db.ListableSessionPredicate), sits behind the admin middleware, and
// ignores share visibility entirely (SHARE_ALL makes no difference).
//
// Two reads share one filter builder:
//   - AdminProviderShare: session counts per (time bucket, provider), plus
//     whole-range totals. Never capped.
//   - AdminSessionPoints: one point per session for the size scatter, capped at a
//     deterministic pseudo-random sample (ORDER BY md5(id)), so the same filter
//     set always returns the same points.
//
// Sessions are bucketed and range-filtered by first_seen (idx_sessions_first_seen,
// migration 000059). Buckets are UTC.

// Bucket sizes for AdminProviderShare, as date_trunc units.
const (
	BucketDay   = "day"
	BucketWeek  = "week"
	BucketMonth = "month"
)

// SessionInsightsPointLimit caps AdminSessionPoints for the HTTP surface.
const SessionInsightsPointLimit = 5000

// SessionInsightsFilter narrows both Session Insights reads. From is inclusive
// and To exclusive; a zero From means all time. Providers are canonical values (legacy aliases are expanded
// here); empty means every provider. OwnerID nil means every user.
type SessionInsightsFilter struct {
	From      time.Time
	To        time.Time
	Providers []string
	OwnerID   *int64
}

// SessionInsightsBucketSize picks the bucket for a range: day up to 31 days,
// week up to 366 days (a year, leap-safe), month beyond.
func SessionInsightsBucketSize(from, to time.Time) string {
	span := to.Sub(from)
	switch {
	case span <= 31*24*time.Hour:
		return BucketDay
	case span <= 366*24*time.Hour:
		return BucketWeek
	default:
		return BucketMonth
	}
}

// where returns the WHERE clause (over sessions alias s) and its positional args.
func (f SessionInsightsFilter) where() (string, []any) {
	args := []any{f.From, f.To, pq.Array(resolveProviderFilter(f.Providers))}
	clauses := []string{
		"s.first_seen >= $1",
		"s.first_seen < $2",
		"s.session_type = ANY($3)",
		db.ListableSessionPredicate("s"),
	}
	if f.OwnerID != nil {
		args = append(args, *f.OwnerID)
		clauses = append(clauses, "s.user_id = $"+strconv.Itoa(len(args)))
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// ProviderShareBucket is one time bucket's session counts by canonical provider.
type ProviderShareBucket struct {
	Start  time.Time
	Counts map[string]int
}

// ProviderShare is the AdminProviderShare result. From is the effective range
// start: the requested From, or for an all-time request (zero From) the earliest
// matching session, so all time buckets over the data's real span (and To when
// nothing matches). Buckets holds
// only non-empty buckets, ascending; the client zero-fills between From and To.
type ProviderShare struct {
	From       time.Time
	To         time.Time
	BucketSize string
	Buckets    []ProviderShareBucket
	Totals     map[string]int
}

// AdminProviderShare counts every matching session per (bucket, provider).
// Legacy session_type values fold into their canonical provider.
func (s *Store) AdminProviderShare(ctx context.Context, f SessionInsightsFilter) (*ProviderShare, error) {
	where, args := f.where()

	from := f.From
	if from.IsZero() {
		var earliest sql.NullTime
		if err := s.db.QueryRowContext(ctx, "SELECT MIN(s.first_seen) FROM sessions s "+where, args...).Scan(&earliest); err != nil {
			return nil, fmt.Errorf("provider share earliest: %w", err)
		}
		from = f.To
		if earliest.Valid {
			from = earliest.Time
		}
	}
	out := &ProviderShare{
		From:       from.UTC(),
		To:         f.To.UTC(),
		BucketSize: SessionInsightsBucketSize(from, f.To),
		Buckets:    []ProviderShareBucket{},
		Totals:     map[string]int{},
	}

	// The bucket unit is one of three constants, so formatting it in is safe.
	query := `
		SELECT date_trunc('` + out.BucketSize + `', s.first_seen AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket,
		       s.session_type, COUNT(*)
		FROM sessions s
		` + where + `
		GROUP BY 1, 2
		ORDER BY 1`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("provider share query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bucket time.Time
		var sessionType string
		var n int
		if err := rows.Scan(&bucket, &sessionType, &n); err != nil {
			return nil, fmt.Errorf("provider share scan: %w", err)
		}
		bucket = bucket.UTC()
		provider := models.NormalizeProvider(sessionType)
		if last := len(out.Buckets) - 1; last < 0 || !out.Buckets[last].Start.Equal(bucket) {
			out.Buckets = append(out.Buckets, ProviderShareBucket{Start: bucket, Counts: map[string]int{}})
		}
		out.Buckets[len(out.Buckets)-1].Counts[provider] += n
		out.Totals[provider] += n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("provider share rows: %w", err)
	}
	return out, nil
}

// SessionPoint is one session on the size scatter. Lines is always present
// (listable sessions have synced lines); the card-derived metrics are nil when
// the session has no such card yet. Repo is the sanitized "owner/repo" display
// form (db.SanitizeGitInfoForSharing), never a raw remote URL.
type SessionPoint struct {
	ID         string
	FirstSeen  time.Time
	Provider   string
	Title      string
	OwnerEmail string
	Repo       *string
	Branch     *string
	Lines      int64
	Tokens     *int64
	CostUSD    *float64
	UserTurns  *int64
	DurationMs *int64
}

// SessionPoints is the AdminSessionPoints result. Total counts every matching
// session; Returned is len(Points), at most the limit.
type SessionPoints struct {
	Total    int
	Returned int
	Points   []SessionPoint
}

// AdminSessionPoints returns up to limit matching sessions with every size
// metric. Above the limit it returns a deterministic sample (ordered by
// md5(id)), stable for a given filter set. Points come back ordered by
// first_seen.
func (s *Store) AdminSessionPoints(ctx context.Context, f SessionInsightsFilter, limit int) (*SessionPoints, error) {
	where, args := f.where()
	args = append(args, limit)
	limitArg := "$" + strconv.Itoa(len(args))

	// Tokens follow the Trends definition: input + output.
	tokensExpr := fmt.Sprintf(
		"CASE WHEN v.session_id IS NULL THEN NULL ELSE COALESCE(%s, '0')::bigint + COALESCE(%s, '0')::bigint END",
		db.V2TotalInputExpr("v"), db.V2TotalOutputExpr("v"))

	query := `
		WITH sample AS (
			SELECT s.id, COUNT(*) OVER () AS total
			FROM sessions s
			` + where + `
			ORDER BY md5(s.id::text)
			LIMIT ` + limitArg + `
		)
		SELECT s.id, s.first_seen, s.session_type,
		       COALESCE(` + db.SessionTitleExpr("s") + `, ''),
		       u.email, s.git_info,
		       COALESCE(sf.total_lines, 0),
		       ` + tokensExpr + `,
		       ` + db.V2TotalCostExpr("v") + `,
		       conv.user_turns, cs.duration_ms,
		       sample.total
		FROM sample
		JOIN sessions s ON s.id = sample.id
		JOIN users u ON u.id = s.user_id
		LEFT JOIN LATERAL (
			SELECT SUM(sf.last_synced_line) AS total_lines
			FROM sync_files sf WHERE sf.session_id = s.id
		) sf ON true
		LEFT JOIN session_card_tokens_v2 v ON v.session_id = s.id
		LEFT JOIN session_card_conversation conv ON conv.session_id = s.id
		LEFT JOIN session_card_session cs ON cs.session_id = s.id
		ORDER BY s.first_seen, s.id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("session points query: %w", err)
	}
	defer rows.Close()

	out := &SessionPoints{Points: []SessionPoint{}}
	for rows.Next() {
		var p SessionPoint
		var sessionType string
		var gitInfo []byte
		var tokens, userTurns, durationMs sql.NullInt64
		var cost sql.NullString
		if err := rows.Scan(&p.ID, &p.FirstSeen, &sessionType, &p.Title, &p.OwnerEmail, &gitInfo,
			&p.Lines, &tokens, &cost, &userTurns, &durationMs, &out.Total); err != nil {
			return nil, fmt.Errorf("session points scan: %w", err)
		}
		p.FirstSeen = p.FirstSeen.UTC()
		p.Provider = models.NormalizeProvider(sessionType)
		p.Repo, p.Branch = sanitizedRepoBranch(gitInfo)
		p.Tokens = nullInt64Ptr(tokens)
		p.UserTurns = nullInt64Ptr(userTurns)
		p.DurationMs = nullInt64Ptr(durationMs)
		if cost.Valid {
			if c, err := strconv.ParseFloat(cost.String, 64); err == nil {
				p.CostUSD = &c
			}
		}
		out.Points = append(out.Points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session points rows: %w", err)
	}
	out.Returned = len(out.Points)
	if out.Returned == 0 {
		// The sample CTE yields no rows (and so no total) when nothing matches.
		out.Total = 0
	}
	return out, nil
}

// sanitizedRepoBranch reduces a raw git_info blob to the redacted repo display
// name and branch, via the same helper non-owner viewers get.
func sanitizedRepoBranch(raw []byte) (repo, branch *string) {
	if len(raw) == 0 {
		return nil, nil
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, nil
	}
	clean, _ := db.SanitizeGitInfoForSharing(parsed).(map[string]any)
	if r, ok := clean["repo_url"].(string); ok {
		repo = &r
	}
	if b, ok := clean["branch"].(string); ok {
		branch = &b
	}
	return repo, branch
}

func nullInt64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
