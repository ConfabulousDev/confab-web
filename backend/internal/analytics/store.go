package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
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

	// Get tokens card (includes cost)
	tokens, err := s.getTokensCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting tokens card: %w", err)
	}
	cards.Tokens = tokens

	// Get session card (includes compaction)
	session, err := s.getSessionCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting session card: %w", err)
	}
	cards.Session = session

	// Get tools card
	tools, err := s.getToolsCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting tools card: %w", err)
	}
	cards.Tools = tools

	return cards, nil
}

// UpsertCards inserts or updates all cards for a session.
func (s *Store) UpsertCards(ctx context.Context, cards *Cards) error {
	if cards.Tokens != nil {
		if err := s.upsertTokensCard(ctx, cards.Tokens); err != nil {
			return fmt.Errorf("upserting tokens card: %w", err)
		}
	}

	if cards.Session != nil {
		if err := s.upsertSessionCard(ctx, cards.Session); err != nil {
			return fmt.Errorf("upserting session card: %w", err)
		}
	}

	if cards.Tools != nil {
		if err := s.upsertToolsCard(ctx, cards.Tools); err != nil {
			return fmt.Errorf("upserting tools card: %w", err)
		}
	}

	return nil
}

// =============================================================================
// Tokens card operations (includes cost)
// =============================================================================

func (s *Store) getTokensCard(ctx context.Context, sessionID string) (*TokensCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		FROM session_card_tokens
		WHERE session_id = $1
	`

	var record TokensCardRecord
	var costStr string
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.InputTokens,
		&record.OutputTokens,
		&record.CacheCreationTokens,
		&record.CacheReadTokens,
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

func (s *Store) upsertTokensCard(ctx context.Context, record *TokensCardRecord) error {
	query := `
		INSERT INTO session_card_tokens (
			session_id, version, computed_at, up_to_line,
			input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
			estimated_cost_usd
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			estimated_cost_usd = EXCLUDED.estimated_cost_usd
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
		record.EstimatedCostUSD.String(),
	)
	return err
}

// =============================================================================
// Session card operations (includes compaction)
// =============================================================================

func (s *Store) getSessionCard(ctx context.Context, sessionID string) (*SessionCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			total_messages, user_messages, assistant_messages,
			human_prompts, tool_results, text_responses, tool_calls, thinking_blocks,
			user_turns, assistant_turns, duration_ms, models_used,
			compaction_auto, compaction_manual, compaction_avg_time_ms
		FROM session_card_session
		WHERE session_id = $1
	`

	var record SessionCardRecord
	var modelsJSON []byte
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.TotalMessages,
		&record.UserMessages,
		&record.AssistantMessages,
		&record.HumanPrompts,
		&record.ToolResults,
		&record.TextResponses,
		&record.ToolCalls,
		&record.ThinkingBlocks,
		&record.UserTurns,
		&record.AssistantTurns,
		&record.DurationMs,
		&modelsJSON,
		&record.CompactionAuto,
		&record.CompactionManual,
		&record.CompactionAvgTimeMs,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(modelsJSON, &record.ModelsUsed); err != nil {
		return nil, fmt.Errorf("parsing models_used: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertSessionCard(ctx context.Context, record *SessionCardRecord) error {
	modelsJSON, err := json.Marshal(record.ModelsUsed)
	if err != nil {
		return fmt.Errorf("marshaling models_used: %w", err)
	}

	query := `
		INSERT INTO session_card_session (
			session_id, version, computed_at, up_to_line,
			total_messages, user_messages, assistant_messages,
			human_prompts, tool_results, text_responses, tool_calls, thinking_blocks,
			user_turns, assistant_turns, duration_ms, models_used,
			compaction_auto, compaction_manual, compaction_avg_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			total_messages = EXCLUDED.total_messages,
			user_messages = EXCLUDED.user_messages,
			assistant_messages = EXCLUDED.assistant_messages,
			human_prompts = EXCLUDED.human_prompts,
			tool_results = EXCLUDED.tool_results,
			text_responses = EXCLUDED.text_responses,
			tool_calls = EXCLUDED.tool_calls,
			thinking_blocks = EXCLUDED.thinking_blocks,
			user_turns = EXCLUDED.user_turns,
			assistant_turns = EXCLUDED.assistant_turns,
			duration_ms = EXCLUDED.duration_ms,
			models_used = EXCLUDED.models_used,
			compaction_auto = EXCLUDED.compaction_auto,
			compaction_manual = EXCLUDED.compaction_manual,
			compaction_avg_time_ms = EXCLUDED.compaction_avg_time_ms
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.TotalMessages,
		record.UserMessages,
		record.AssistantMessages,
		record.HumanPrompts,
		record.ToolResults,
		record.TextResponses,
		record.ToolCalls,
		record.ThinkingBlocks,
		record.UserTurns,
		record.AssistantTurns,
		record.DurationMs,
		modelsJSON,
		record.CompactionAuto,
		record.CompactionManual,
		record.CompactionAvgTimeMs,
	)
	return err
}

// =============================================================================
// Tools card operations
// =============================================================================

func (s *Store) getToolsCard(ctx context.Context, sessionID string) (*ToolsCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			total_calls, tool_breakdown, error_count
		FROM session_card_tools
		WHERE session_id = $1
	`

	var record ToolsCardRecord
	var breakdownJSON []byte
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.TotalCalls,
		&breakdownJSON,
		&record.ErrorCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(breakdownJSON, &record.ToolStats); err != nil {
		return nil, fmt.Errorf("parsing tool_stats: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertToolsCard(ctx context.Context, record *ToolsCardRecord) error {
	statsJSON, err := json.Marshal(record.ToolStats)
	if err != nil {
		return fmt.Errorf("marshaling tool_stats: %w", err)
	}

	query := `
		INSERT INTO session_card_tools (
			session_id, version, computed_at, up_to_line,
			total_calls, tool_breakdown, error_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			total_calls = EXCLUDED.total_calls,
			tool_breakdown = EXCLUDED.tool_breakdown,
			error_count = EXCLUDED.error_count
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.TotalCalls,
		statsJSON,
		record.ErrorCount,
	)
	return err
}

// =============================================================================
// Conversion helpers
// =============================================================================

// ToCards converts a ComputeResult to Cards for storage.
func (r *ComputeResult) ToCards(sessionID string, lineCount int64) *Cards {
	now := time.Now().UTC()

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
			EstimatedCostUSD:    r.EstimatedCostUSD,
		},
		Session: &SessionCardRecord{
			SessionID:  sessionID,
			Version:    SessionCardVersion,
			ComputedAt: now,
			UpToLine:   lineCount,
			// Message counts
			TotalMessages:     r.TotalMessages,
			UserMessages:      r.UserMessages,
			AssistantMessages: r.AssistantMessages,
			// Message type breakdown
			HumanPrompts:   r.HumanPrompts,
			ToolResults:    r.ToolResults,
			TextResponses:  r.TextResponses,
			ToolCalls:      r.ToolCalls,
			ThinkingBlocks: r.ThinkingBlocks,
			// Turns
			UserTurns:      r.UserTurns,
			AssistantTurns: r.AssistantTurns,
			// Metadata
			DurationMs: r.DurationMs,
			ModelsUsed: r.ModelsUsed,
			// Compaction
			CompactionAuto:      r.CompactionAuto,
			CompactionManual:    r.CompactionManual,
			CompactionAvgTimeMs: r.CompactionAvgTimeMs,
		},
		Tools: &ToolsCardRecord{
			SessionID:  sessionID,
			Version:    ToolsCardVersion,
			ComputedAt: now,
			UpToLine:   lineCount,
			TotalCalls: r.TotalToolCalls,
			ToolStats:  r.ToolStats,
			ErrorCount: r.ToolErrorCount,
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

		// Legacy flat format (deprecated)
		response.Tokens = TokenStats{
			Input:         c.Tokens.InputTokens,
			Output:        c.Tokens.OutputTokens,
			CacheCreation: c.Tokens.CacheCreationTokens,
			CacheRead:     c.Tokens.CacheReadTokens,
		}
		response.Cost = CostStats{
			EstimatedUSD: c.Tokens.EstimatedCostUSD,
		}

		// Cards format - tokens includes cost
		response.Cards["tokens"] = TokensCardData{
			Input:         c.Tokens.InputTokens,
			Output:        c.Tokens.OutputTokens,
			CacheCreation: c.Tokens.CacheCreationTokens,
			CacheRead:     c.Tokens.CacheReadTokens,
			EstimatedUSD:  c.Tokens.EstimatedCostUSD.String(),
		}
	}

	if c.Session != nil {
		// Legacy flat format (deprecated)
		response.Compaction = CompactionInfo{
			Auto:      c.Session.CompactionAuto,
			Manual:    c.Session.CompactionManual,
			AvgTimeMs: c.Session.CompactionAvgTimeMs,
		}

		// Cards format - session includes message breakdown and compaction
		response.Cards["session"] = SessionCardData{
			// Message counts
			TotalMessages:     c.Session.TotalMessages,
			UserMessages:      c.Session.UserMessages,
			AssistantMessages: c.Session.AssistantMessages,
			// Message type breakdown
			HumanPrompts:   c.Session.HumanPrompts,
			ToolResults:    c.Session.ToolResults,
			TextResponses:  c.Session.TextResponses,
			ToolCalls:      c.Session.ToolCalls,
			ThinkingBlocks: c.Session.ThinkingBlocks,
			// Turns
			UserTurns:      c.Session.UserTurns,
			AssistantTurns: c.Session.AssistantTurns,
			// Metadata
			DurationMs: c.Session.DurationMs,
			ModelsUsed: c.Session.ModelsUsed,
			// Compaction
			CompactionAuto:      c.Session.CompactionAuto,
			CompactionManual:    c.Session.CompactionManual,
			CompactionAvgTimeMs: c.Session.CompactionAvgTimeMs,
		}
	}

	if c.Tools != nil {
		// Cards format only (no legacy format for tools)
		response.Cards["tools"] = ToolsCardData{
			TotalCalls: c.Tools.TotalCalls,
			ToolStats:  c.Tools.ToolStats,
			ErrorCount: c.Tools.ErrorCount,
		}
	}

	return response
}
