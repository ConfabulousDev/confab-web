package recapquota

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Quota represents a user's smart recap quota status for a given month.
type Quota struct {
	UserID        int64
	ComputeCount  int
	QuotaMonth    string // "2026-02"
	LastComputeAt *time.Time
	CreatedAt     time.Time
}

// CurrentMonth returns the current month string in "YYYY-MM" format (UTC).
func CurrentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// GetOrCreate retrieves or creates a quota record for a user, atomically resetting
// the count if the stored month is stale. Uses the current UTC month.
func GetOrCreate(ctx context.Context, conn *sql.DB, userID int64) (*Quota, error) {
	return GetOrCreateForMonth(ctx, conn, userID, CurrentMonth())
}

// GetOrCreateForMonth is the same as GetOrCreate but with an explicit month parameter (for tests).
func GetOrCreateForMonth(ctx context.Context, conn *sql.DB, userID int64, month string) (*Quota, error) {
	query := `
		INSERT INTO smart_recap_quota (user_id, quota_month)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET
			compute_count = CASE
				WHEN smart_recap_quota.quota_month = $2
				THEN smart_recap_quota.compute_count ELSE 0 END,
			quota_month = $2
		RETURNING user_id, compute_count, quota_month, last_compute_at, created_at
	`

	var q Quota
	err := conn.QueryRowContext(ctx, query, userID, month).Scan(
		&q.UserID,
		&q.ComputeCount,
		&q.QuotaMonth,
		&q.LastComputeAt,
		&q.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create quota: %w", err)
	}
	return &q, nil
}

// Increment bumps the compute count for a user. If the stored month is stale,
// it atomically resets the count to 1 and updates the month. Uses the current UTC month.
// Errors if no row exists for this user.
func Increment(ctx context.Context, conn *sql.DB, userID int64) error {
	return IncrementForMonth(ctx, conn, userID, CurrentMonth())
}

// IncrementForMonth is the same as Increment but with an explicit month parameter (for tests).
func IncrementForMonth(ctx context.Context, conn *sql.DB, userID int64, month string) error {
	query := `
		UPDATE smart_recap_quota
		SET compute_count = CASE
				WHEN quota_month = $2 THEN compute_count + 1
				ELSE 1 END,
			quota_month = $2,
			last_compute_at = NOW()
		WHERE user_id = $1
	`

	result, err := conn.ExecContext(ctx, query, userID, month)
	if err != nil {
		return fmt.Errorf("failed to increment quota: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no quota record found for user %d", userID)
	}

	return nil
}

// GetCount returns the compute count for the current month (0 if row missing or stale).
func GetCount(ctx context.Context, conn *sql.DB, userID int64) (int, error) {
	return GetCountForMonth(ctx, conn, userID, CurrentMonth())
}

// GetCountForMonth is the same as GetCount but with an explicit month parameter (for tests).
func GetCountForMonth(ctx context.Context, conn *sql.DB, userID int64, month string) (int, error) {
	query := `
		SELECT compute_count FROM smart_recap_quota
		WHERE user_id = $1 AND quota_month = $2
	`

	var count int
	err := conn.QueryRowContext(ctx, query, userID, month).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get quota count: %w", err)
	}
	return count, nil
}

// UserStats contains per-user smart recap statistics for admin display.
type UserStats struct {
	UserID                int64
	Email                 string
	Name                  *string
	SessionsWithCache     int
	ComputationsThisMonth int
	LastComputeAt         *time.Time
}

// ListUserStats retrieves per-user smart recap statistics for all users
// who have either sessions with recap cache or computations this month.
func ListUserStats(ctx context.Context, conn *sql.DB) ([]UserStats, error) {
	query := `
		WITH user_cache_counts AS (
			SELECT s.user_id, COUNT(sr.session_id) as sessions_with_cache
			FROM sessions s
			JOIN session_card_smart_recap sr ON s.id = sr.session_id
			GROUP BY s.user_id
		),
		current_month_quotas AS (
			SELECT user_id, compute_count, last_compute_at
			FROM smart_recap_quota
			WHERE quota_month = TO_CHAR(NOW() AT TIME ZONE 'UTC', 'YYYY-MM')
		)
		SELECT
			u.id,
			u.email,
			u.name,
			COALESCE(ucc.sessions_with_cache, 0) as sessions_with_cache,
			COALESCE(cmq.compute_count, 0) as computations_this_month,
			cmq.last_compute_at
		FROM users u
		LEFT JOIN user_cache_counts ucc ON u.id = ucc.user_id
		LEFT JOIN current_month_quotas cmq ON u.id = cmq.user_id
		WHERE ucc.sessions_with_cache > 0 OR cmq.compute_count > 0
		ORDER BY COALESCE(cmq.compute_count, 0) DESC, COALESCE(ucc.sessions_with_cache, 0) DESC
	`

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list user smart recap stats: %w", err)
	}
	defer rows.Close()

	var stats []UserStats
	for rows.Next() {
		var s UserStats
		if err := rows.Scan(
			&s.UserID,
			&s.Email,
			&s.Name,
			&s.SessionsWithCache,
			&s.ComputationsThisMonth,
			&s.LastComputeAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user smart recap stats: %w", err)
		}
		stats = append(stats, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user smart recap stats: %w", err)
	}

	return stats, nil
}

// Totals contains aggregate totals for smart recap usage.
type Totals struct {
	TotalNonEmptySessions      int
	TotalSessionsWithCache     int
	TotalComputationsThisMonth int
	TotalUsersWithActivity     int
}

// GetTotals retrieves aggregate totals for smart recap usage.
func GetTotals(ctx context.Context, conn *sql.DB) (*Totals, error) {
	totals := &Totals{}

	nonEmptyQuery := `
		SELECT COUNT(*) FROM (
			SELECT session_id
			FROM sync_files
			WHERE file_type IN ('transcript', 'agent')
			GROUP BY session_id
			HAVING SUM(last_synced_line) > 0
		) AS non_empty
	`
	if err := conn.QueryRowContext(ctx, nonEmptyQuery).Scan(&totals.TotalNonEmptySessions); err != nil {
		return nil, fmt.Errorf("failed to count non-empty sessions: %w", err)
	}

	cacheQuery := `SELECT COUNT(*) FROM session_card_smart_recap`
	if err := conn.QueryRowContext(ctx, cacheQuery).Scan(&totals.TotalSessionsWithCache); err != nil {
		return nil, fmt.Errorf("failed to count sessions with cache: %w", err)
	}

	quotaQuery := `
		SELECT
			COALESCE(SUM(compute_count), 0),
			COUNT(*)
		FROM smart_recap_quota
		WHERE quota_month = TO_CHAR(NOW() AT TIME ZONE 'UTC', 'YYYY-MM')
	`
	if err := conn.QueryRowContext(ctx, quotaQuery).Scan(
		&totals.TotalComputationsThisMonth,
		&totals.TotalUsersWithActivity,
	); err != nil {
		return nil, fmt.Errorf("failed to get quota totals: %w", err)
	}

	return totals, nil
}
