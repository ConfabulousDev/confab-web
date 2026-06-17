package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

// =============================================================================
// Conversion helpers
// =============================================================================

// ToCards converts a ComputeResult to Cards for storage.
// Cards with computation errors are left nil, and errors are propagated via CardErrors.
func (r *ComputeResult) ToCards(sessionID string, lineCount int64) *Cards {
	now := time.Now().UTC()

	cards := &Cards{
		CardErrors: r.CardErrors,
	}

	// tokens_v2 is always written (empty data for a token-less session), so it
	// participates in AllValid and the staleness gate exactly like the other
	// cards — mirroring the Workflows card's "always written, empty for N/A
	// sessions" pattern. As of pjnz it is the sole tokens card; the flat v1
	// tokens card is no longer written.
	if _, hasErr := r.CardErrors["tokens_v2"]; !hasErr {
		data := TokensV2Data{TotalCostUSD: "0", ByProvider: map[string]TokensV2Provider{}}
		if r.TokensV2 != nil {
			data = *r.TokensV2
		}
		cards.TokensV2 = &TokensV2CardRecord{
			SessionID:  sessionID,
			Version:    TokensV2CardVersion,
			ComputedAt: now,
			UpToLine:   lineCount,
			Data:       data,
		}
	}

	if _, hasErr := r.CardErrors["session"]; !hasErr {
		cards.Session = &SessionCardRecord{
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
		}
	}

	if _, hasErr := r.CardErrors["tools"]; !hasErr {
		cards.Tools = &ToolsCardRecord{
			SessionID:  sessionID,
			Version:    ToolsCardVersion,
			ComputedAt: now,
			UpToLine:   lineCount,
			TotalCalls: r.TotalToolCalls,
			ToolStats:  r.ToolStats,
			ErrorCount: r.ToolErrorCount,
		}
	}

	if _, hasErr := r.CardErrors["code_activity"]; !hasErr {
		cards.CodeActivity = &CodeActivityCardRecord{
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
		}
	}

	if _, hasErr := r.CardErrors["conversation"]; !hasErr {
		cards.Conversation = &ConversationCardRecord{
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
			AssistantUtilizationPct:  r.AssistantUtilizationPct,
		}
	}

	if _, hasErr := r.CardErrors["agents_and_skills"]; !hasErr {
		cards.AgentsAndSkills = &AgentsAndSkillsCardRecord{
			SessionID:        sessionID,
			Version:          AgentsAndSkillsCardVersion,
			ComputedAt:       now,
			UpToLine:         lineCount,
			AgentInvocations: r.TotalAgentInvocations,
			SkillInvocations: r.TotalSkillInvocations,
			AgentStats:       r.AgentStats,
			SkillStats:       r.SkillStats,
		}
	}

	if _, hasErr := r.CardErrors["redactions"]; !hasErr {
		cards.Redactions = &RedactionsCardRecord{
			SessionID:       sessionID,
			Version:         RedactionsCardVersion,
			ComputedAt:      now,
			UpToLine:        lineCount,
			TotalRedactions: r.TotalRedactions,
			RedactionCounts: r.RedactionCounts,
		}
	}

	// Workflows is always written (empty runs for non-workflow sessions), so the
	// card participates in the all-cards-exist staleness gate like the others.
	if _, hasErr := r.CardErrors["workflows"]; !hasErr {
		runs := r.Workflows
		if runs == nil {
			runs = []WorkflowRun{}
		}
		cards.Workflows = &WorkflowsCardRecord{
			SessionID:  sessionID,
			Version:    WorkflowsCardVersion,
			ComputedAt: now,
			UpToLine:   lineCount,
			Runs:       runs,
		}
	}

	return cards
}

// ToResponse converts Cards to an AnalyticsResponse for the API.
func (c *Cards) ToResponse() *AnalyticsResponse {
	response := &AnalyticsResponse{
		Cards: make(map[string]interface{}),
	}

	// Get ComputedAt and ComputedLines from the first available card
	// (tokens_v2 preferred, but fallback to others if it failed)
	switch {
	case c.TokensV2 != nil:
		response.ComputedAt = c.TokensV2.ComputedAt
		response.ComputedLines = c.TokensV2.UpToLine
	case c.Session != nil:
		response.ComputedAt = c.Session.ComputedAt
		response.ComputedLines = c.Session.UpToLine
	case c.Tools != nil:
		response.ComputedAt = c.Tools.ComputedAt
		response.ComputedLines = c.Tools.UpToLine
	case c.CodeActivity != nil:
		response.ComputedAt = c.CodeActivity.ComputedAt
		response.ComputedLines = c.CodeActivity.UpToLine
	case c.Conversation != nil:
		response.ComputedAt = c.Conversation.ComputedAt
		response.ComputedLines = c.Conversation.UpToLine
	case c.AgentsAndSkills != nil:
		response.ComputedAt = c.AgentsAndSkills.ComputedAt
		response.ComputedLines = c.AgentsAndSkills.UpToLine
	case c.Redactions != nil:
		response.ComputedAt = c.Redactions.ComputedAt
		response.ComputedLines = c.Redactions.UpToLine
	case c.Workflows != nil:
		response.ComputedAt = c.Workflows.ComputedAt
		response.ComputedLines = c.Workflows.UpToLine
	}

	// The flat tokens card (legacy top-level Tokens/Cost plus cards["tokens"]) is
	// derived from the tokens_v2 top-level scalars: pjnz retired the flat v1
	// session_card_tokens storage, so v2 is the sole source. The scalars
	// (total_input/total_output/total_cache_creation/total_cache_read/
	// total_cost_usd) reproduce the v1 flat columns exactly. The flat card is
	// served only when v2 carries provider data (i.e. a token-bearing session);
	// the frontend's dedup gate then suppresses it in favor of tokens_v2. The
	// fast-mode breakdown is not reconstructable from the v2 top-level scalars and
	// is dropped here — it surfaced only on this dedup-hidden flat card.
	if c.TokensV2 != nil && len(c.TokensV2.Data.ByProvider) > 0 {
		v2 := c.TokensV2.Data
		response.Tokens = TokenStats{
			Input:         v2.TotalInput,
			Output:        v2.TotalOutput,
			CacheCreation: v2.TotalCacheCreation,
			CacheRead:     v2.TotalCacheRead,
		}
		estimatedCost, _ := decimal.NewFromString(v2.TotalCostUSD)
		response.Cost = CostStats{EstimatedUSD: estimatedCost}

		response.Cards["tokens"] = TokensCardData{
			Input:         v2.TotalInput,
			Output:        v2.TotalOutput,
			CacheCreation: v2.TotalCacheCreation,
			CacheRead:     v2.TotalCacheRead,
			EstimatedUSD:  v2.TotalCostUSD,
		}

		// tokens_v2: hierarchical per-provider/per-model breakdown. The stored
		// Data is already the API wire shape.
		response.Cards["tokens_v2"] = v2
	}

	if c.Session != nil {
		// Legacy flat format (deprecated)
		response.Compaction = CompactionInfo{
			Auto:      c.Session.CompactionAuto,
			Manual:    c.Session.CompactionManual,
			AvgTimeMs: c.Session.CompactionAvgTimeMs,
		}

		// Defensive coerce: models_used is a required, non-nullable array on the
		// frontend (SessionCardDataSchema). A nil slice marshals to JSON null and
		// fails the whole Summary parse, so guarantee a non-nil slice on the wire
		// for any provider — including cards stored before a provider set it (y0kc).
		modelsUsed := c.Session.ModelsUsed
		if modelsUsed == nil {
			modelsUsed = []string{}
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
			ModelsUsed: modelsUsed,
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
			AssistantUtilizationPct:  c.Conversation.AssistantUtilizationPct,
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

	// Only include workflows card if the session has workflow runs (hide if empty)
	if c.Workflows != nil && len(c.Workflows.Runs) > 0 {
		response.Cards["workflows"] = WorkflowsCardData{
			Runs: c.Workflows.Runs,
		}
	}

	// Include per-card errors if any (graceful degradation)
	if len(c.CardErrors) > 0 {
		response.CardErrors = c.CardErrors
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
	}
	return err
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

// =============================================================================
// Search Index operations
// =============================================================================

// UpsertSearchIndex inserts or updates the search index for a session.
// The tsvector is built server-side with weighted components:
//   - Weight A: metadata (titles, summary, first message)
//   - Weight B: smart recap content
//   - Weight C: user messages from transcript
func (s *Store) UpsertSearchIndex(ctx context.Context, record *SearchIndexRecord, content *SearchIndexContent) error {
	ctx, span := tracer.Start(ctx, "analytics.upsert_search_index",
		trace.WithAttributes(attribute.String("session.id", record.SessionID)))
	defer span.End()

	query := `
		INSERT INTO session_search_index (
			session_id, version, content_text, search_vector,
			indexed_up_to_line, recap_indexed_at, metadata_hash, updated_at
		) VALUES (
			$1, $2, $3,
			setweight(to_tsvector('english', COALESCE($4, '')), 'A') ||
			setweight(to_tsvector('english', COALESCE($5, '')), 'B') ||
			setweight(to_tsvector('english', COALESCE($6, '')), 'C'),
			$7, $8, $9, NOW()
		)
		ON CONFLICT (session_id) DO UPDATE SET
			version = EXCLUDED.version,
			content_text = EXCLUDED.content_text,
			search_vector = EXCLUDED.search_vector,
			indexed_up_to_line = EXCLUDED.indexed_up_to_line,
			recap_indexed_at = EXCLUDED.recap_indexed_at,
			metadata_hash = EXCLUDED.metadata_hash,
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query,
		record.SessionID,         // $1
		record.Version,           // $2
		content.CombinedText(),   // $3
		content.MetadataText,     // $4
		content.RecapText,        // $5
		content.UserMessagesText, // $6
		record.IndexedUpToLine,   // $7
		record.RecapIndexedAt,    // $8
		record.MetadataHash,      // $9
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

// GetSearchIndex retrieves the search index record for a session.
// Returns nil if no index exists.
func (s *Store) GetSearchIndex(ctx context.Context, sessionID string) (*SearchIndexRecord, error) {
	query := `
		SELECT session_id, version, content_text, indexed_up_to_line,
			recap_indexed_at, metadata_hash, updated_at
		FROM session_search_index
		WHERE session_id = $1
	`

	var record SearchIndexRecord
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(
		&record.SessionID,
		&record.Version,
		&record.ContentText,
		&record.IndexedUpToLine,
		&record.RecapIndexedAt,
		&record.MetadataHash,
		&record.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &record, nil
}
