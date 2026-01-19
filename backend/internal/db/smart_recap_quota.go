package db

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// SmartRecapQuota represents a user's smart recap quota status.
type SmartRecapQuota struct {
	UserID        int64
	ComputeCount  int
	LastComputeAt *time.Time
	QuotaResetAt  *time.Time
	CreatedAt     time.Time
}

// GetOrCreateSmartRecapQuota retrieves or creates a quota record for a user.
func (db *DB) GetOrCreateSmartRecapQuota(ctx context.Context, userID int64) (*SmartRecapQuota, error) {
	ctx, span := tracer.Start(ctx, "db.get_or_create_smart_recap_quota",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `
		INSERT INTO smart_recap_quota (user_id)
		VALUES ($1)
		ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING user_id, compute_count, last_compute_at, quota_reset_at, created_at
	`

	var quota SmartRecapQuota
	err := db.conn.QueryRowContext(ctx, query, userID).Scan(
		&quota.UserID,
		&quota.ComputeCount,
		&quota.LastComputeAt,
		&quota.QuotaResetAt,
		&quota.CreatedAt,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get or create quota: %w", err)
	}

	span.SetAttributes(attribute.Int("quota.compute_count", quota.ComputeCount))
	return &quota, nil
}

// IncrementSmartRecapQuota increments the compute count for a user.
func (db *DB) IncrementSmartRecapQuota(ctx context.Context, userID int64) error {
	ctx, span := tracer.Start(ctx, "db.increment_smart_recap_quota",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	query := `
		UPDATE smart_recap_quota
		SET compute_count = compute_count + 1,
		    last_compute_at = NOW()
		WHERE user_id = $1
	`

	result, err := db.conn.ExecContext(ctx, query, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to increment quota: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		span.SetStatus(codes.Error, "no quota record found")
		return fmt.Errorf("no quota record found for user %d", userID)
	}

	return nil
}

// ResetSmartRecapQuotaIfNeeded resets the quota if a new month has started.
// Returns true if the quota was reset, false otherwise.
func (db *DB) ResetSmartRecapQuotaIfNeeded(ctx context.Context, userID int64) (bool, error) {
	return db.ResetSmartRecapQuotaIfNeededAt(ctx, userID, time.Now().UTC())
}

// ResetSmartRecapQuotaIfNeededAt resets the quota if a new month has started relative to the given time.
// This variant allows passing a specific time for testing month boundary behavior.
// Returns true if the quota was reset, false otherwise.
func (db *DB) ResetSmartRecapQuotaIfNeededAt(ctx context.Context, userID int64, now time.Time) (bool, error) {
	ctx, span := tracer.Start(ctx, "db.reset_smart_recap_quota_if_needed",
		trace.WithAttributes(attribute.Int64("user.id", userID)))
	defer span.End()

	// Get current quota
	quota, err := db.GetOrCreateSmartRecapQuota(ctx, userID)
	if err != nil {
		return false, err
	}

	// Check if we need to reset (either never reset, or reset was in a previous month)
	now = now.UTC()
	needsReset := false

	if quota.QuotaResetAt == nil {
		// Never been reset - check if any usage exists from a previous month
		if quota.LastComputeAt != nil {
			lastCompute := quota.LastComputeAt.UTC()
			if lastCompute.Year() != now.Year() || lastCompute.Month() != now.Month() {
				needsReset = true
			}
		}
	} else {
		// Check if reset was in a previous month
		lastReset := quota.QuotaResetAt.UTC()
		if lastReset.Year() != now.Year() || lastReset.Month() != now.Month() {
			needsReset = true
		}
	}

	if !needsReset {
		span.SetAttributes(attribute.Bool("quota.reset", false))
		return false, nil
	}

	// Reset the quota
	query := `
		UPDATE smart_recap_quota
		SET compute_count = 0, quota_reset_at = $2
		WHERE user_id = $1
	`

	_, err = db.conn.ExecContext(ctx, query, userID, now)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, fmt.Errorf("failed to reset quota: %w", err)
	}

	span.SetAttributes(attribute.Bool("quota.reset", true))
	return true, nil
}

// UserSmartRecapStats contains per-user smart recap statistics.
type UserSmartRecapStats struct {
	UserID                int64
	Email                 string
	Name                  *string
	SessionsWithCache     int
	ComputationsThisMonth int
	LastComputeAt         *time.Time
}

// ListUserSmartRecapStats retrieves per-user smart recap statistics for all users
// who have either sessions with recap cache or computations this month.
func (db *DB) ListUserSmartRecapStats(ctx context.Context) ([]UserSmartRecapStats, error) {
	ctx, span := tracer.Start(ctx, "db.list_user_smart_recap_stats")
	defer span.End()

	// Join users with:
	// 1. Count of their sessions that have recap cache entries
	// 2. Their quota stats (computations this month)
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
			WHERE (
				quota_reset_at >= DATE_TRUNC('month', CURRENT_TIMESTAMP)
				OR (quota_reset_at IS NULL AND last_compute_at >= DATE_TRUNC('month', CURRENT_TIMESTAMP))
			)
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

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to list user smart recap stats: %w", err)
	}
	defer rows.Close()

	var stats []UserSmartRecapStats
	for rows.Next() {
		var s UserSmartRecapStats
		if err := rows.Scan(
			&s.UserID,
			&s.Email,
			&s.Name,
			&s.SessionsWithCache,
			&s.ComputationsThisMonth,
			&s.LastComputeAt,
		); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("failed to scan user smart recap stats: %w", err)
		}
		stats = append(stats, s)
	}

	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("error iterating user smart recap stats: %w", err)
	}

	span.SetAttributes(attribute.Int("stats.user_count", len(stats)))
	return stats, nil
}

// SmartRecapTotals contains aggregate totals for smart recap usage.
type SmartRecapTotals struct {
	TotalSessionsWithCache     int
	TotalComputationsThisMonth int
	TotalUsersWithActivity     int
}

// GetSmartRecapTotals retrieves aggregate totals for smart recap usage.
func (db *DB) GetSmartRecapTotals(ctx context.Context) (*SmartRecapTotals, error) {
	ctx, span := tracer.Start(ctx, "db.get_smart_recap_totals")
	defer span.End()

	totals := &SmartRecapTotals{}

	// Count total sessions with cache
	cacheQuery := `SELECT COUNT(*) FROM session_card_smart_recap`
	if err := db.conn.QueryRowContext(ctx, cacheQuery).Scan(&totals.TotalSessionsWithCache); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to count sessions with cache: %w", err)
	}

	// Count computations this month and users with activity
	quotaQuery := `
		SELECT
			COALESCE(SUM(compute_count), 0),
			COUNT(*)
		FROM smart_recap_quota
		WHERE (
			quota_reset_at >= DATE_TRUNC('month', CURRENT_TIMESTAMP)
			OR (quota_reset_at IS NULL AND last_compute_at >= DATE_TRUNC('month', CURRENT_TIMESTAMP))
		)
	`
	if err := db.conn.QueryRowContext(ctx, quotaQuery).Scan(
		&totals.TotalComputationsThisMonth,
		&totals.TotalUsersWithActivity,
	); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("failed to get quota totals: %w", err)
	}

	span.SetAttributes(
		attribute.Int("totals.sessions_with_cache", totals.TotalSessionsWithCache),
		attribute.Int("totals.computations_this_month", totals.TotalComputationsThisMonth),
		attribute.Int("totals.users_with_activity", totals.TotalUsersWithActivity),
	)

	return totals, nil
}
