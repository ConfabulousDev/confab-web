package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Store provides database operations for session analytics cards.
type Store struct {
	db *sql.DB
}

// NewStore creates a new analytics store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// GetCards retrieves all cached card data for a session.
// Returns a Cards struct with nil fields for cards that don't exist.
func (s *Store) GetCards(ctx context.Context, sessionID string) (*Cards, error) {
	cards := &Cards{}

	// Get tokens card
	tokens, err := s.getTokensCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting tokens card: %w", err)
	}
	cards.Tokens = tokens

	// Get cost card
	cost, err := s.getCostCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting cost card: %w", err)
	}
	cards.Cost = cost

	// Get compaction card
	compaction, err := s.getCompactionCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting compaction card: %w", err)
	}
	cards.Compaction = compaction

	return cards, nil
}

// UpsertCards inserts or updates all cards for a session.
func (s *Store) UpsertCards(ctx context.Context, cards *Cards) error {
	if cards.Tokens != nil {
		if err := s.upsertTokensCard(ctx, cards.Tokens); err != nil {
			return fmt.Errorf("upserting tokens card: %w", err)
		}
	}

	if cards.Cost != nil {
		if err := s.upsertCostCard(ctx, cards.Cost); err != nil {
			return fmt.Errorf("upserting cost card: %w", err)
		}
	}

	if cards.Compaction != nil {
		if err := s.upsertCompactionCard(ctx, cards.Compaction); err != nil {
			return fmt.Errorf("upserting compaction card: %w", err)
		}
	}

	return nil
}

// =============================================================================
// Tokens card operations
// =============================================================================

func (s *Store) getTokensCard(ctx context.Context, sessionID string) (*TokensCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens
		FROM session_card_tokens
		WHERE session_id = $1
	`

	var record TokensCardRecord
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.InputTokens,
		&record.OutputTokens,
		&record.CacheCreationTokens,
		&record.CacheReadTokens,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) upsertTokensCard(ctx context.Context, record *TokensCardRecord) error {
	query := `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens
	`

	_, err := s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.InputTokens,
		record.OutputTokens,
		record.CacheCreationTokens,
		record.CacheReadTokens,
	)
	return err
}

// =============================================================================
// Cost card operations
// =============================================================================

func (s *Store) getCostCard(ctx context.Context, sessionID string) (*CostCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line, estimated_cost_usd
		FROM session_card_cost
		WHERE session_id = $1
	`

	var record CostCardRecord
	var costStr string
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&costStr,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record.EstimatedCostUSD, err = decimal.NewFromString(costStr)
	if err != nil {
		return nil, fmt.Errorf("parsing cost: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertCostCard(ctx context.Context, record *CostCardRecord) error {
	query := `
		INSERT INTO session_card_cost (
			session_id, version, computed_at, up_to_line, estimated_cost_usd
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			estimated_cost_usd = EXCLUDED.estimated_cost_usd
	`

	_, err := s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.EstimatedCostUSD.String(),
	)
	return err
}

// =============================================================================
// Compaction card operations
// =============================================================================

func (s *Store) getCompactionCard(ctx context.Context, sessionID string) (*CompactionCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			auto_count, manual_count, avg_time_ms
		FROM session_card_compaction
		WHERE session_id = $1
	`

	var record CompactionCardRecord
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.AutoCount,
		&record.ManualCount,
		&record.AvgTimeMs,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *Store) upsertCompactionCard(ctx context.Context, record *CompactionCardRecord) error {
	query := `
		INSERT INTO session_card_compaction (
			session_id, version, computed_at, up_to_line,
			auto_count, manual_count, avg_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			auto_count = EXCLUDED.auto_count,
			manual_count = EXCLUDED.manual_count,
			avg_time_ms = EXCLUDED.avg_time_ms
	`

	_, err := s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.AutoCount,
		record.ManualCount,
		record.AvgTimeMs,
	)
	return err
}

// =============================================================================
// Conversion helpers
// =============================================================================

// ToCards converts a ComputeResult to Cards for storage.
func (r *ComputeResult) ToCards(sessionID string, lineCount int64) *Cards {
	now := time.Now()

	return &Cards{
		Tokens: &TokensCardRecord{
			SessionID:           sessionID,
			Version:             TokensCardVersion,
			ComputedAt:          now,
			UpToLine:            lineCount,
			InputTokens:         r.InputTokens,
			OutputTokens:        r.OutputTokens,
			CacheCreationTokens: r.CacheCreationTokens,
			CacheReadTokens:     r.CacheReadTokens,
		},
		Cost: &CostCardRecord{
			SessionID:        sessionID,
			Version:          CostCardVersion,
			ComputedAt:       now,
			UpToLine:         lineCount,
			EstimatedCostUSD: r.EstimatedCostUSD,
		},
		Compaction: &CompactionCardRecord{
			SessionID:   sessionID,
			Version:     CompactionCardVersion,
			ComputedAt:  now,
			UpToLine:    lineCount,
			AutoCount:   r.CompactionAuto,
			ManualCount: r.CompactionManual,
			AvgTimeMs:   r.CompactionAvgTimeMs,
		},
	}
}

// ToResponse converts Cards to an AnalyticsResponse for the API.
func (c *Cards) ToResponse() *AnalyticsResponse {
	response := &AnalyticsResponse{
		Cards: make(map[string]interface{}),
	}

	if c.Tokens != nil {
		response.ComputedAt = c.Tokens.ComputedAt
		response.ComputedLines = c.Tokens.UpToLine

		// Legacy flat format
		response.Tokens = TokenStats{
			Input:         c.Tokens.InputTokens,
			Output:        c.Tokens.OutputTokens,
			CacheCreation: c.Tokens.CacheCreationTokens,
			CacheRead:     c.Tokens.CacheReadTokens,
		}

		// Cards format
		response.Cards["tokens"] = TokensCardData{
			Input:         c.Tokens.InputTokens,
			Output:        c.Tokens.OutputTokens,
			CacheCreation: c.Tokens.CacheCreationTokens,
			CacheRead:     c.Tokens.CacheReadTokens,
		}
	}

	if c.Cost != nil {
		// Legacy flat format
		response.Cost = CostStats{
			EstimatedUSD: c.Cost.EstimatedCostUSD,
		}

		// Cards format
		response.Cards["cost"] = CostCardData{
			EstimatedUSD: c.Cost.EstimatedCostUSD.String(),
		}
	}

	if c.Compaction != nil {
		// Legacy flat format
		response.Compaction = CompactionInfo{
			Auto:      c.Compaction.AutoCount,
			Manual:    c.Compaction.ManualCount,
			AvgTimeMs: c.Compaction.AvgTimeMs,
		}

		// Cards format
		response.Cards["compaction"] = CompactionCardData{
			Auto:      c.Compaction.AutoCount,
			Manual:    c.Compaction.ManualCount,
			AvgTimeMs: c.Compaction.AvgTimeMs,
		}
	}

	return response
}
