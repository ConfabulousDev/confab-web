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

	// Get code activity card
	codeActivity, err := s.getCodeActivityCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting code activity card: %w", err)
	}
	cards.CodeActivity = codeActivity

	// Get conversation card
	conversation, err := s.getConversationCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting conversation card: %w", err)
	}
	cards.Conversation = conversation

	// Get agents and skills card
	agentsAndSkills, err := s.getAgentsAndSkillsCard(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("getting agents and skills card: %w", err)
	}
	cards.AgentsAndSkills = agentsAndSkills

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

	if cards.CodeActivity != nil {
		if err := s.upsertCodeActivityCard(ctx, cards.CodeActivity); err != nil {
			return fmt.Errorf("upserting code activity card: %w", err)
		}
	}

	if cards.Conversation != nil {
		if err := s.upsertConversationCard(ctx, cards.Conversation); err != nil {
			return fmt.Errorf("upserting conversation card: %w", err)
		}
	}

	if cards.AgentsAndSkills != nil {
		if err := s.upsertAgentsAndSkillsCard(ctx, cards.AgentsAndSkills); err != nil {
			return fmt.Errorf("upserting agents and skills card: %w", err)
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
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms
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
			user_turns, assistant_turns, avg_assistant_turn_ms, avg_user_thinking_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			computed_at = EXCLUDED.computed_at,
			up_to_line = EXCLUDED.up_to_line,
			user_turns = EXCLUDED.user_turns,
			assistant_turns = EXCLUDED.assistant_turns,
			avg_assistant_turn_ms = EXCLUDED.avg_assistant_turn_ms,
			avg_user_thinking_ms = EXCLUDED.avg_user_thinking_ms
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
			SessionID:          sessionID,
			Version:            ConversationCardVersion,
			ComputedAt:         now,
			UpToLine:           lineCount,
			UserTurns:          r.UserTurns,
			AssistantTurns:     r.AssistantTurns,
			AvgAssistantTurnMs: r.AvgAssistantTurnMs,
			AvgUserThinkingMs:  r.AvgUserThinkingMs,
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
			UserTurns:          c.Conversation.UserTurns,
			AssistantTurns:     c.Conversation.AssistantTurns,
			AvgAssistantTurnMs: c.Conversation.AvgAssistantTurnMs,
			AvgUserThinkingMs:  c.Conversation.AvgUserThinkingMs,
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

	return response
}
