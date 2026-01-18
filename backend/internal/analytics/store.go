package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("confab/analytics")

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
// All card queries run in parallel to minimize latency.
func (s *Store) GetCards(ctx context.Context, sessionID string) (*Cards, error) {
	ctx, span := tracer.Start(ctx, "analytics.get_cards",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	cards := &Cards{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, 7)

	// Helper to run a getter in parallel
	runGet := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				errs <- fmt.Errorf("%s: %w", name, err)
			}
		}()
	}

	runGet("tokens", func() error {
		result, err := s.getTokensCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.Tokens = result
		mu.Unlock()
		return nil
	})

	runGet("session", func() error {
		result, err := s.getSessionCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.Session = result
		mu.Unlock()
		return nil
	})

	runGet("tools", func() error {
		result, err := s.getToolsCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.Tools = result
		mu.Unlock()
		return nil
	})

	runGet("code_activity", func() error {
		result, err := s.getCodeActivityCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.CodeActivity = result
		mu.Unlock()
		return nil
	})

	runGet("conversation", func() error {
		result, err := s.getConversationCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.Conversation = result
		mu.Unlock()
		return nil
	})

	runGet("agents_and_skills", func() error {
		result, err := s.getAgentsAndSkillsCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.AgentsAndSkills = result
		mu.Unlock()
		return nil
	})

	runGet("redactions", func() error {
		result, err := s.getRedactionsCard(ctx, sessionID)
		if err != nil {
			return err
		}
		mu.Lock()
		cards.Redactions = result
		mu.Unlock()
		return nil
	})

	wg.Wait()
	close(errs)

	// Collect all errors
	var allErrs []error
	for err := range errs {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) > 0 {
		combined := errors.Join(allErrs...)
		span.RecordError(combined)
		span.SetStatus(codes.Error, combined.Error())
		return nil, combined
	}

	return cards, nil
}

// UpsertCards inserts or updates all cards for a session.
// All card upserts run in parallel to minimize latency.
func (s *Store) UpsertCards(ctx context.Context, cards *Cards) error {
	// Get session ID from the first available card for tracing
	var sessionID string
	if cards.Tokens != nil {
		sessionID = cards.Tokens.SessionID
	} else if cards.Session != nil {
		sessionID = cards.Session.SessionID
	}

	ctx, span := tracer.Start(ctx, "analytics.upsert_cards",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	var wg sync.WaitGroup
	errs := make(chan error, 7)

	// Helper to run an upsert in parallel
	runUpsert := func(name string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				errs <- fmt.Errorf("%s: %w", name, err)
			}
		}()
	}

	if cards.Tokens != nil {
		runUpsert("tokens", func() error {
			return s.upsertTokensCard(ctx, cards.Tokens)
		})
	}

	if cards.Session != nil {
		runUpsert("session", func() error {
			return s.upsertSessionCard(ctx, cards.Session)
		})
	}

	if cards.Tools != nil {
		runUpsert("tools", func() error {
			return s.upsertToolsCard(ctx, cards.Tools)
		})
	}

	if cards.CodeActivity != nil {
		runUpsert("code_activity", func() error {
			return s.upsertCodeActivityCard(ctx, cards.CodeActivity)
		})
	}

	if cards.Conversation != nil {
		runUpsert("conversation", func() error {
			return s.upsertConversationCard(ctx, cards.Conversation)
		})
	}

	if cards.AgentsAndSkills != nil {
		runUpsert("agents_and_skills", func() error {
			return s.upsertAgentsAndSkillsCard(ctx, cards.AgentsAndSkills)
		})
	}

	if cards.Redactions != nil {
		runUpsert("redactions", func() error {
			return s.upsertRedactionsCard(ctx, cards.Redactions)
		})
	}

	wg.Wait()
	close(errs)

	// Collect all errors
	var allErrs []error
	for err := range errs {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) > 0 {
		combined := errors.Join(allErrs...)
		span.RecordError(combined)
		span.SetStatus(codes.Error, combined.Error())
		return combined
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
			duration_ms, models_used,
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
			duration_ms, models_used,
			compaction_auto, compaction_manual, compaction_avg_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
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
// Code Activity card operations
// =============================================================================

func (s *Store) getCodeActivityCard(ctx context.Context, sessionID string) (*CodeActivityCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			files_read, files_modified, lines_added, lines_removed, search_count,
			language_breakdown
		FROM session_card_code_activity
		WHERE session_id = $1
	`

	var record CodeActivityCardRecord
	var breakdownJSON []byte
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.FilesRead,
		&record.FilesModified,
		&record.LinesAdded,
		&record.LinesRemoved,
		&record.SearchCount,
		&breakdownJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(breakdownJSON, &record.LanguageBreakdown); err != nil {
		return nil, fmt.Errorf("parsing language_breakdown: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertCodeActivityCard(ctx context.Context, record *CodeActivityCardRecord) error {
	breakdownJSON, err := json.Marshal(record.LanguageBreakdown)
	if err != nil {
		return fmt.Errorf("marshaling language_breakdown: %w", err)
	}

	query := `
		INSERT INTO session_card_code_activity (
			session_id, version, computed_at, up_to_line,
			files_read, files_modified, lines_added, lines_removed, search_count,
			language_breakdown
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			files_read = EXCLUDED.files_read,
			files_modified = EXCLUDED.files_modified,
			lines_added = EXCLUDED.lines_added,
			lines_removed = EXCLUDED.lines_removed,
			search_count = EXCLUDED.search_count,
			language_breakdown = EXCLUDED.language_breakdown
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.FilesRead,
		record.FilesModified,
		record.LinesAdded,
		record.LinesRemoved,
		record.SearchCount,
		breakdownJSON,
	)
	return err
}

// =============================================================================
// Conversation card operations
// =============================================================================

func (s *Store) getConversationCard(ctx context.Context, sessionID string) (*ConversationCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms,
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization
		FROM session_card_conversation
		WHERE session_id = $1
	`

	var record ConversationCardRecord
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.UserTurns,
		&record.AssistantTurns,
		&record.AvgAssistantTurnMs,
		&record.AvgUserThinkingMs,
		&record.TotalAssistantDurationMs,
		&record.TotalUserDurationMs,
		&record.AssistantUtilization,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}

func (s *Store) upsertConversationCard(ctx context.Context, record *ConversationCardRecord) error {
	query := `
		INSERT INTO session_card_conversation (
			session_id, version, computed_at, up_to_line,
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms,
			total_assistant_duration_ms, total_user_duration_ms, assistant_utilization
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			user_turns = EXCLUDED.user_turns,
			assistant_turns = EXCLUDED.assistant_turns,
			avg_assistant_turn_ms = EXCLUDED.avg_assistant_turn_ms,
			avg_user_thinking_ms = EXCLUDED.avg_user_thinking_ms,
			total_assistant_duration_ms = EXCLUDED.total_assistant_duration_ms,
			total_user_duration_ms = EXCLUDED.total_user_duration_ms,
			assistant_utilization = EXCLUDED.assistant_utilization
	`

	_, err := s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.UserTurns,
		record.AssistantTurns,
		record.AvgAssistantTurnMs,
		record.AvgUserThinkingMs,
		record.TotalAssistantDurationMs,
		record.TotalUserDurationMs,
		record.AssistantUtilization,
	)
	return err
}

// =============================================================================
// Agents and Skills card operations
// =============================================================================

func (s *Store) getAgentsAndSkillsCard(ctx context.Context, sessionID string) (*AgentsAndSkillsCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			agent_invocations, skill_invocations, agent_stats, skill_stats
		FROM session_card_agents_and_skills
		WHERE session_id = $1
	`

	var record AgentsAndSkillsCardRecord
	var agentStatsJSON, skillStatsJSON []byte
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.AgentInvocations,
		&record.SkillInvocations,
		&agentStatsJSON,
		&skillStatsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(agentStatsJSON, &record.AgentStats); err != nil {
		return nil, fmt.Errorf("parsing agent_stats: %w", err)
	}
	if err := json.Unmarshal(skillStatsJSON, &record.SkillStats); err != nil {
		return nil, fmt.Errorf("parsing skill_stats: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertAgentsAndSkillsCard(ctx context.Context, record *AgentsAndSkillsCardRecord) error {
	agentStatsJSON, err := json.Marshal(record.AgentStats)
	if err != nil {
		return fmt.Errorf("marshaling agent_stats: %w", err)
	}
	skillStatsJSON, err := json.Marshal(record.SkillStats)
	if err != nil {
		return fmt.Errorf("marshaling skill_stats: %w", err)
	}

	query := `
		INSERT INTO session_card_agents_and_skills (
			session_id, version, computed_at, up_to_line,
			agent_invocations, skill_invocations, agent_stats, skill_stats
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			agent_invocations = EXCLUDED.agent_invocations,
			skill_invocations = EXCLUDED.skill_invocations,
			agent_stats = EXCLUDED.agent_stats,
			skill_stats = EXCLUDED.skill_stats
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.AgentInvocations,
		record.SkillInvocations,
		agentStatsJSON,
		skillStatsJSON,
	)
	return err
}

// =============================================================================
// Redactions card operations
// =============================================================================

func (s *Store) getRedactionsCard(ctx context.Context, sessionID string) (*RedactionsCardRecord, error) {
	query := `
		SELECT session_id, version, computed_at, up_to_line,
			total_redactions, redaction_counts
		FROM session_card_redactions
		WHERE session_id = $1
	`

	var record RedactionsCardRecord
	var countsJSON []byte
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.TotalRedactions,
		&countsJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(countsJSON, &record.RedactionCounts); err != nil {
		return nil, fmt.Errorf("parsing redaction_counts: %w", err)
	}

	return &record, nil
}

func (s *Store) upsertRedactionsCard(ctx context.Context, record *RedactionsCardRecord) error {
	countsJSON, err := json.Marshal(record.RedactionCounts)
	if err != nil {
		return fmt.Errorf("marshaling redaction_counts: %w", err)
	}

	query := `
		INSERT INTO session_card_redactions (
			session_id, version, computed_at, up_to_line,
			total_redactions, redaction_counts
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			total_redactions = EXCLUDED.total_redactions,
			redaction_counts = EXCLUDED.redaction_counts
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.TotalRedactions,
		countsJSON,
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
		CodeActivity: &CodeActivityCardRecord{
			SessionID:         sessionID,
			Version:           CodeActivityCardVersion,
			ComputedAt:        now,
			UpToLine:          lineCount,
			FilesRead:         r.FilesRead,
			FilesModified:     r.FilesModified,
			LinesAdded:        r.LinesAdded,
			LinesRemoved:      r.LinesRemoved,
			SearchCount:       r.SearchCount,
			LanguageBreakdown: r.LanguageBreakdown,
		},
		Conversation: &ConversationCardRecord{
			SessionID:                sessionID,
			Version:                  ConversationCardVersion,
			ComputedAt:               now,
			UpToLine:                 lineCount,
			UserTurns:                r.UserTurns,
			AssistantTurns:           r.AssistantTurns,
			AvgAssistantTurnMs:       r.AvgAssistantTurnMs,
			AvgUserThinkingMs:        r.AvgUserThinkingMs,
			TotalAssistantDurationMs: r.TotalAssistantDurationMs,
			TotalUserDurationMs:      r.TotalUserDurationMs,
			AssistantUtilization:     r.AssistantUtilization,
		},
		AgentsAndSkills: &AgentsAndSkillsCardRecord{
			SessionID:        sessionID,
			Version:          AgentsAndSkillsCardVersion,
			ComputedAt:       now,
			UpToLine:         lineCount,
			AgentInvocations: r.TotalAgentInvocations,
			SkillInvocations: r.TotalSkillInvocations,
			AgentStats:       r.AgentStats,
			SkillStats:       r.SkillStats,
		},
		Redactions: &RedactionsCardRecord{
			SessionID:       sessionID,
			Version:         RedactionsCardVersion,
			ComputedAt:      now,
			UpToLine:        lineCount,
			TotalRedactions: r.TotalRedactions,
			RedactionCounts: r.RedactionCounts,
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

	if c.CodeActivity != nil {
		// Cards format only (no legacy format for code activity)
		response.Cards["code_activity"] = CodeActivityCardData{
			FilesRead:         c.CodeActivity.FilesRead,
			FilesModified:     c.CodeActivity.FilesModified,
			LinesAdded:        c.CodeActivity.LinesAdded,
			LinesRemoved:      c.CodeActivity.LinesRemoved,
			SearchCount:       c.CodeActivity.SearchCount,
			LanguageBreakdown: c.CodeActivity.LanguageBreakdown,
		}
	}

	if c.Conversation != nil {
		// Cards format only (no legacy format for conversation)
		response.Cards["conversation"] = ConversationCardData{
			UserTurns:                c.Conversation.UserTurns,
			AssistantTurns:           c.Conversation.AssistantTurns,
			AvgAssistantTurnMs:       c.Conversation.AvgAssistantTurnMs,
			AvgUserThinkingMs:        c.Conversation.AvgUserThinkingMs,
			TotalAssistantDurationMs: c.Conversation.TotalAssistantDurationMs,
			TotalUserDurationMs:      c.Conversation.TotalUserDurationMs,
			AssistantUtilization:     c.Conversation.AssistantUtilization,
		}
	}

	if c.AgentsAndSkills != nil {
		response.Cards["agents_and_skills"] = AgentsAndSkillsCardData{
			AgentInvocations: c.AgentsAndSkills.AgentInvocations,
			SkillInvocations: c.AgentsAndSkills.SkillInvocations,
			AgentStats:       c.AgentsAndSkills.AgentStats,
			SkillStats:       c.AgentsAndSkills.SkillStats,
		}
	}

	// Only include redactions card if there are redactions (hide if empty)
	if c.Redactions != nil && c.Redactions.TotalRedactions > 0 {
		response.Cards["redactions"] = RedactionsCardData{
			TotalRedactions: c.Redactions.TotalRedactions,
			RedactionCounts: c.Redactions.RedactionCounts,
		}
	}

	return response
}

// =============================================================================
// Smart Recap card operations (separate from GetCards/UpsertCards due to
// time-based invalidation and background generation)
// =============================================================================

// GetSmartRecapCard retrieves the smart recap card for a session.
func (s *Store) GetSmartRecapCard(ctx context.Context, sessionID string) (*SmartRecapCardRecord, error) {
	ctx, span := tracer.Start(ctx, "analytics.get_smart_recap_card",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `
		SELECT session_id, version, computed_at, up_to_line,
			recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions,
			model_used, input_tokens, output_tokens, generation_time_ms,
			computing_started_at
		FROM session_card_smart_recap
		WHERE session_id = $1
	`

	var record SmartRecapCardRecord
	var wentWellJSON, wentBadJSON, humanSuggestionsJSON, envSuggestionsJSON, contextSuggestionsJSON []byte

	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ComputedAt,
		&record.UpToLine,
		&record.Recap,
		&wentWellJSON,
		&wentBadJSON,
		&humanSuggestionsJSON,
		&envSuggestionsJSON,
		&contextSuggestionsJSON,
		&record.ModelUsed,
		&record.InputTokens,
		&record.OutputTokens,
		&record.GenerationTimeMs,
		&record.ComputingStartedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Unmarshal JSONB arrays
	if err := json.Unmarshal(wentWellJSON, &record.WentWell); err != nil {
		return nil, fmt.Errorf("parsing went_well: %w", err)
	}
	if err := json.Unmarshal(wentBadJSON, &record.WentBad); err != nil {
		return nil, fmt.Errorf("parsing went_bad: %w", err)
	}
	if err := json.Unmarshal(humanSuggestionsJSON, &record.HumanSuggestions); err != nil {
		return nil, fmt.Errorf("parsing human_suggestions: %w", err)
	}
	if err := json.Unmarshal(envSuggestionsJSON, &record.EnvironmentSuggestions); err != nil {
		return nil, fmt.Errorf("parsing environment_suggestions: %w", err)
	}
	if err := json.Unmarshal(contextSuggestionsJSON, &record.DefaultContextSuggestions); err != nil {
		return nil, fmt.Errorf("parsing default_context_suggestions: %w", err)
	}

	return &record, nil
}

// UpsertSmartRecapCard inserts or updates a smart recap card, clearing the computing lock.
func (s *Store) UpsertSmartRecapCard(ctx context.Context, record *SmartRecapCardRecord) error {
	ctx, span := tracer.Start(ctx, "analytics.upsert_smart_recap_card",
		trace.WithAttributes(attribute.String("session.id", record.SessionID)))
	defer span.End()

	wentWellJSON, err := json.Marshal(record.WentWell)
	if err != nil {
		return fmt.Errorf("marshaling went_well: %w", err)
	}
	wentBadJSON, err := json.Marshal(record.WentBad)
	if err != nil {
		return fmt.Errorf("marshaling went_bad: %w", err)
	}
	humanSuggestionsJSON, err := json.Marshal(record.HumanSuggestions)
	if err != nil {
		return fmt.Errorf("marshaling human_suggestions: %w", err)
	}
	envSuggestionsJSON, err := json.Marshal(record.EnvironmentSuggestions)
	if err != nil {
		return fmt.Errorf("marshaling environment_suggestions: %w", err)
	}
	contextSuggestionsJSON, err := json.Marshal(record.DefaultContextSuggestions)
	if err != nil {
		return fmt.Errorf("marshaling default_context_suggestions: %w", err)
	}

	query := `
		INSERT INTO session_card_smart_recap (
			session_id, version, computed_at, up_to_line,
			recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions,
			model_used, input_tokens, output_tokens, generation_time_ms,
			computing_started_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NULL)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			recap = EXCLUDED.recap,
			went_well = EXCLUDED.went_well,
			went_bad = EXCLUDED.went_bad,
			human_suggestions = EXCLUDED.human_suggestions,
			environment_suggestions = EXCLUDED.environment_suggestions,
			default_context_suggestions = EXCLUDED.default_context_suggestions,
			model_used = EXCLUDED.model_used,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			generation_time_ms = EXCLUDED.generation_time_ms,
			computing_started_at = NULL
	`

	_, err = s.db.ExecContext(ctx, query,
		record.SessionID,
		record.Version,
		record.ComputedAt,
		record.UpToLine,
		record.Recap,
		wentWellJSON,
		wentBadJSON,
		humanSuggestionsJSON,
		envSuggestionsJSON,
		contextSuggestionsJSON,
		record.ModelUsed,
		record.InputTokens,
		record.OutputTokens,
		record.GenerationTimeMs,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	return nil
}

// AcquireSmartRecapLock attempts to acquire the computing lock for a smart recap.
// Returns true if the lock was acquired, false if another process is already computing.
func (s *Store) AcquireSmartRecapLock(ctx context.Context, sessionID string, lockTimeoutSeconds int) (bool, error) {
	ctx, span := tracer.Start(ctx, "analytics.acquire_smart_recap_lock",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	// Atomically set the lock if it doesn't exist or is stale
	query := `
		INSERT INTO session_card_smart_recap (
			session_id, version, computed_at, up_to_line,
			recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions,
			model_used, input_tokens, output_tokens,
			computing_started_at
		) VALUES ($1, 0, NOW(), 0, '', '[]', '[]', '[]', '[]', '[]', '', 0, 0, NOW())
		ON CONFLICT (session_id) DO UPDATE SET
			computing_started_at = NOW()
		WHERE session_card_smart_recap.computing_started_at IS NULL
		   OR session_card_smart_recap.computing_started_at < NOW() - INTERVAL '1 second' * $2
		RETURNING session_id
	`

	var returnedID string
	err := s.db.QueryRowContext(ctx, query, sessionID, lockTimeoutSeconds).Scan(&returnedID)
	if err == sql.ErrNoRows {
		// Lock not acquired - another process has it
		span.SetAttributes(attribute.Bool("lock.acquired", false))
		return false, nil
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return false, err
	}

	span.SetAttributes(attribute.Bool("lock.acquired", true))
	return true, nil
}

// ClearSmartRecapLock clears the computing lock (e.g., on error).
func (s *Store) ClearSmartRecapLock(ctx context.Context, sessionID string) error {
	ctx, span := tracer.Start(ctx, "analytics.clear_smart_recap_lock",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	query := `
		UPDATE session_card_smart_recap
		SET computing_started_at = NULL
		WHERE session_id = $1
	`

	_, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
