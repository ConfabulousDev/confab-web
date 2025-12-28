package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

// Store provides database operations for session analytics.
type Store struct {
	db *sql.DB
}

// NewStore creates a new analytics store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get retrieves cached analytics for a session.
// Returns nil, nil if no cached analytics exist.
func (s *Store) Get(ctx context.Context, sessionID string) (*SessionAnalytics, error) {
	query := `
		SELECT session_id, analytics_version, up_to_line, computed_at,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd, compaction_auto, compaction_manual, compaction_avg_time_ms,
			details
		FROM session_analytics
		WHERE session_id = $1
	`

	var analytics SessionAnalytics
	var detailsJSON []byte
	var costStr string

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&analytics.SessionID,
		&analytics.AnalyticsVersion,
		&analytics.UpToLine,
		&analytics.ComputedAt,
		&analytics.InputTokens,
		&analytics.OutputTokens,
		&analytics.CacheCreationTokens,
		&analytics.CacheReadTokens,
		&costStr,
		&analytics.CompactionAuto,
		&analytics.CompactionManual,
		&analytics.CompactionAvgTimeMs,
		&detailsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying analytics: %w", err)
	}

	// Parse decimal from string
	analytics.EstimatedCostUSD, err = decimal.NewFromString(costStr)
	if err != nil {
		return nil, fmt.Errorf("parsing cost: %w", err)
	}

	// Parse details JSON
	if len(detailsJSON) > 0 {
		if err := json.Unmarshal(detailsJSON, &analytics.Details); err != nil {
			return nil, fmt.Errorf("parsing details: %w", err)
		}
	}

	return &analytics, nil
}

// Upsert inserts or updates analytics for a session.
func (s *Store) Upsert(ctx context.Context, analytics *SessionAnalytics) error {
	detailsJSON, err := json.Marshal(analytics.Details)
	if err != nil {
		return fmt.Errorf("marshaling details: %w", err)
	}

	query := `
		INSERT INTO session_analytics (
			session_id, analytics_version, up_to_line, computed_at,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd, compaction_auto, compaction_manual, compaction_avg_time_ms,
			details
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (session_id) DO UPDATE SET
			analytics_version = EXCLUDED.analytics_version,
			up_to_line = EXCLUDED.up_to_line,
			computed_at = EXCLUDED.computed_at,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			estimated_cost_usd = EXCLUDED.estimated_cost_usd,
			compaction_auto = EXCLUDED.compaction_auto,
			compaction_manual = EXCLUDED.compaction_manual,
			compaction_avg_time_ms = EXCLUDED.compaction_avg_time_ms,
			details = EXCLUDED.details
	`

	_, err = s.db.ExecContext(ctx, query,
		analytics.SessionID,
		analytics.AnalyticsVersion,
		analytics.UpToLine,
		analytics.ComputedAt,
		analytics.InputTokens,
		analytics.OutputTokens,
		analytics.CacheCreationTokens,
		analytics.CacheReadTokens,
		analytics.EstimatedCostUSD.String(),
		analytics.CompactionAuto,
		analytics.CompactionManual,
		analytics.CompactionAvgTimeMs,
		detailsJSON,
	)
	if err != nil {
		return fmt.Errorf("upserting analytics: %w", err)
	}

	return nil
}

// CurrentAnalyticsVersion is incremented when analytics computation logic changes.
// This triggers recomputation of cached analytics.
const CurrentAnalyticsVersion = 1

// IsCacheValid checks if cached analytics are still valid.
// Cache is valid when analytics version matches and line count matches.
func IsCacheValid(cached *SessionAnalytics, currentVersion int, currentLineCount int64) bool {
	if cached == nil {
		return false
	}
	return cached.AnalyticsVersion == currentVersion && cached.UpToLine == currentLineCount
}
