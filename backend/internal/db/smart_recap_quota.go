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
