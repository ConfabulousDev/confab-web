package analytics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// =============================================================================
// Generic session_card_* get/upsert
//
// Every card table shares the same four-column header (session_id, version,
// computed_at, up_to_line) and an ON CONFLICT (session_id) DO UPDATE upsert.
// The machinery below generates the SQL from a per-card column list and binds
// rows through per-card scan/bind closures, collapsing 18 near-identical
// methods (and the two hand-written GetCards/UpsertCards fan-outs) into one
// small generic core plus a card registry (4thv).
//
// JSONB and DECIMAL columns are handled by sql.Scanner/driver.Valuer wrappers
// so a single closure works for both reads and writes.
// =============================================================================

// jsonCol adapts a pointer to a JSON-serializable value to a JSONB column,
// implementing both sql.Scanner (unmarshal) and driver.Valuer (marshal).
type jsonCol[V any] struct{ ptr *V }

func (c jsonCol[V]) Scan(src any) error {
	if src == nil {
		var zero V
		*c.ptr = zero
		return nil
	}
	var b []byte
	switch s := src.(type) {
	case []byte:
		b = s
	case string:
		b = []byte(s)
	default:
		return fmt.Errorf("jsonCol: unsupported source type %T", src)
	}
	return json.Unmarshal(b, c.ptr)
}

func (c jsonCol[V]) Value() (driver.Value, error) { return json.Marshal(*c.ptr) }

// jsonSliceCol is like jsonCol but marshals a nil slice as an empty JSON array
// ("[]") rather than null, preserving the workflows card's stored shape.
type jsonSliceCol[E any] struct{ ptr *[]E }

func (c jsonSliceCol[E]) Scan(src any) error { return jsonCol[[]E]{c.ptr}.Scan(src) }

func (c jsonSliceCol[E]) Value() (driver.Value, error) {
	s := *c.ptr
	if s == nil {
		s = []E{}
	}
	return json.Marshal(s)
}

// cardHeaderCols are the columns every session_card_* table shares, in order.
var cardHeaderCols = []string{"session_id", "version", "computed_at", "up_to_line"}

// cardTable describes one session_card_* table for SQL generation.
type cardTable struct {
	name     string   // table name, e.g. "session_card_tokens"
	dataCols []string // card-specific columns, after the shared header
}

func (ct cardTable) allCols() []string {
	cols := make([]string, 0, len(cardHeaderCols)+len(ct.dataCols))
	cols = append(cols, cardHeaderCols...)
	return append(cols, ct.dataCols...)
}

func (ct cardTable) selectSQL() string {
	return fmt.Sprintf("SELECT %s FROM %s WHERE session_id = $1",
		strings.Join(ct.allCols(), ", "), ct.name)
}

func (ct cardTable) upsertSQL() string {
	cols := ct.allCols()
	placeholders := make([]string, len(cols))
	sets := make([]string, 0, len(cols)-1)
	for i, c := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		if c != "session_id" {
			sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", c, c))
		}
	}
	return fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (session_id) DO UPDATE SET %s",
		ct.name, strings.Join(cols, ", "), strings.Join(placeholders, ", "),
		strings.Join(sets, ", "))
}

// getCard runs the table's SELECT and binds the row via scanTargets. Returns
// (nil, nil) when the session has no row in this table.
func getCard[T any](ctx context.Context, s *Store, ct cardTable, sessionID string,
	scanTargets func(*T) []any) (*T, error) {
	var record T
	err := s.db.QueryRowContext(ctx, ct.selectSQL(), sessionID).Scan(scanTargets(&record)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// upsertCard inserts or updates a single card row, binding values via bindValues.
func upsertCard[T any](ctx context.Context, s *Store, ct cardTable, record *T,
	bindValues func(*T) []any) error {
	_, err := s.db.ExecContext(ctx, ct.upsertSQL(), bindValues(record)...)
	return err
}

// =============================================================================
// Per-card tables and bindings
// =============================================================================

var tokensV2Table = cardTable{name: "session_card_tokens_v2", dataCols: []string{"data"}}

func tokensV2Scan(r *TokensV2CardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine, jsonCol[TokensV2Data]{&r.Data}}
}

func tokensV2Bind(r *TokensV2CardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine, jsonCol[TokensV2Data]{&r.Data}}
}

func (s *Store) getTokensV2Card(ctx context.Context, sessionID string) (*TokensV2CardRecord, error) {
	return getCard(ctx, s, tokensV2Table, sessionID, tokensV2Scan)
}

func (s *Store) upsertTokensV2Card(ctx context.Context, record *TokensV2CardRecord) error {
	return upsertCard(ctx, s, tokensV2Table, record, tokensV2Bind)
}

var sessionTable = cardTable{name: "session_card_session", dataCols: []string{
	"total_messages", "user_messages", "assistant_messages",
	"human_prompts", "tool_results", "text_responses", "tool_calls", "thinking_blocks",
	"duration_ms", "models_used",
	"compaction_auto", "compaction_manual", "compaction_avg_time_ms"}}

func sessionScan(r *SessionCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.TotalMessages, &r.UserMessages, &r.AssistantMessages,
		&r.HumanPrompts, &r.ToolResults, &r.TextResponses, &r.ToolCalls, &r.ThinkingBlocks,
		&r.DurationMs, jsonCol[[]string]{&r.ModelsUsed},
		&r.CompactionAuto, &r.CompactionManual, &r.CompactionAvgTimeMs}
}

func sessionBind(r *SessionCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.TotalMessages, r.UserMessages, r.AssistantMessages,
		r.HumanPrompts, r.ToolResults, r.TextResponses, r.ToolCalls, r.ThinkingBlocks,
		r.DurationMs, jsonCol[[]string]{&r.ModelsUsed},
		r.CompactionAuto, r.CompactionManual, r.CompactionAvgTimeMs}
}

func (s *Store) getSessionCard(ctx context.Context, sessionID string) (*SessionCardRecord, error) {
	return getCard(ctx, s, sessionTable, sessionID, sessionScan)
}

func (s *Store) upsertSessionCard(ctx context.Context, record *SessionCardRecord) error {
	return upsertCard(ctx, s, sessionTable, record, sessionBind)
}

var toolsTable = cardTable{name: "session_card_tools", dataCols: []string{
	"total_calls", "tool_breakdown", "error_count"}}

func toolsScan(r *ToolsCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.TotalCalls, jsonCol[map[string]*ToolStats]{&r.ToolStats}, &r.ErrorCount}
}

func toolsBind(r *ToolsCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.TotalCalls, jsonCol[map[string]*ToolStats]{&r.ToolStats}, r.ErrorCount}
}

func (s *Store) getToolsCard(ctx context.Context, sessionID string) (*ToolsCardRecord, error) {
	return getCard(ctx, s, toolsTable, sessionID, toolsScan)
}

func (s *Store) upsertToolsCard(ctx context.Context, record *ToolsCardRecord) error {
	return upsertCard(ctx, s, toolsTable, record, toolsBind)
}

var codeActivityTable = cardTable{name: "session_card_code_activity", dataCols: []string{
	"files_read", "files_modified", "lines_added", "lines_removed", "search_count",
	"language_breakdown"}}

func codeActivityScan(r *CodeActivityCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.FilesRead, &r.FilesModified, &r.LinesAdded, &r.LinesRemoved, &r.SearchCount,
		jsonCol[map[string]int]{&r.LanguageBreakdown}}
}

func codeActivityBind(r *CodeActivityCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.FilesRead, r.FilesModified, r.LinesAdded, r.LinesRemoved, r.SearchCount,
		jsonCol[map[string]int]{&r.LanguageBreakdown}}
}

func (s *Store) getCodeActivityCard(ctx context.Context, sessionID string) (*CodeActivityCardRecord, error) {
	return getCard(ctx, s, codeActivityTable, sessionID, codeActivityScan)
}

func (s *Store) upsertCodeActivityCard(ctx context.Context, record *CodeActivityCardRecord) error {
	return upsertCard(ctx, s, codeActivityTable, record, codeActivityBind)
}

var conversationTable = cardTable{name: "session_card_conversation", dataCols: []string{
	"user_turns", "assistant_turns", "avg_assistant_turn_ms", "avg_user_thinking_ms",
	"total_assistant_duration_ms", "total_user_duration_ms", "assistant_utilization_pct"}}

func conversationScan(r *ConversationCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.UserTurns, &r.AssistantTurns, &r.AvgAssistantTurnMs, &r.AvgUserThinkingMs,
		&r.TotalAssistantDurationMs, &r.TotalUserDurationMs, &r.AssistantUtilizationPct}
}

func conversationBind(r *ConversationCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.UserTurns, r.AssistantTurns, r.AvgAssistantTurnMs, r.AvgUserThinkingMs,
		r.TotalAssistantDurationMs, r.TotalUserDurationMs, r.AssistantUtilizationPct}
}

func (s *Store) getConversationCard(ctx context.Context, sessionID string) (*ConversationCardRecord, error) {
	return getCard(ctx, s, conversationTable, sessionID, conversationScan)
}

func (s *Store) upsertConversationCard(ctx context.Context, record *ConversationCardRecord) error {
	return upsertCard(ctx, s, conversationTable, record, conversationBind)
}

var agentsAndSkillsTable = cardTable{name: "session_card_agents_and_skills", dataCols: []string{
	"agent_invocations", "skill_invocations", "agent_stats", "skill_stats"}}

func agentsAndSkillsScan(r *AgentsAndSkillsCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.AgentInvocations, &r.SkillInvocations,
		jsonCol[map[string]*AgentStats]{&r.AgentStats}, jsonCol[map[string]*SkillStats]{&r.SkillStats}}
}

func agentsAndSkillsBind(r *AgentsAndSkillsCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.AgentInvocations, r.SkillInvocations,
		jsonCol[map[string]*AgentStats]{&r.AgentStats}, jsonCol[map[string]*SkillStats]{&r.SkillStats}}
}

func (s *Store) getAgentsAndSkillsCard(ctx context.Context, sessionID string) (*AgentsAndSkillsCardRecord, error) {
	return getCard(ctx, s, agentsAndSkillsTable, sessionID, agentsAndSkillsScan)
}

func (s *Store) upsertAgentsAndSkillsCard(ctx context.Context, record *AgentsAndSkillsCardRecord) error {
	return upsertCard(ctx, s, agentsAndSkillsTable, record, agentsAndSkillsBind)
}

var redactionsTable = cardTable{name: "session_card_redactions", dataCols: []string{
	"total_redactions", "redaction_counts"}}

func redactionsScan(r *RedactionsCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine,
		&r.TotalRedactions, jsonCol[map[string]int]{&r.RedactionCounts}}
}

func redactionsBind(r *RedactionsCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine,
		r.TotalRedactions, jsonCol[map[string]int]{&r.RedactionCounts}}
}

func (s *Store) getRedactionsCard(ctx context.Context, sessionID string) (*RedactionsCardRecord, error) {
	return getCard(ctx, s, redactionsTable, sessionID, redactionsScan)
}

func (s *Store) upsertRedactionsCard(ctx context.Context, record *RedactionsCardRecord) error {
	return upsertCard(ctx, s, redactionsTable, record, redactionsBind)
}

var workflowsTable = cardTable{name: "session_card_workflows", dataCols: []string{"runs"}}

func workflowsScan(r *WorkflowsCardRecord) []any {
	return []any{&r.SessionID, &r.Version, &r.ComputedAt, &r.UpToLine, jsonSliceCol[WorkflowRun]{&r.Runs}}
}

func workflowsBind(r *WorkflowsCardRecord) []any {
	return []any{r.SessionID, r.Version, r.ComputedAt, r.UpToLine, jsonSliceCol[WorkflowRun]{&r.Runs}}
}

func (s *Store) getWorkflowsCard(ctx context.Context, sessionID string) (*WorkflowsCardRecord, error) {
	return getCard(ctx, s, workflowsTable, sessionID, workflowsScan)
}

func (s *Store) upsertWorkflowsCard(ctx context.Context, record *WorkflowsCardRecord) error {
	return upsertCard(ctx, s, workflowsTable, record, workflowsBind)
}

// =============================================================================
// Card registry + parallel GetCards/UpsertCards
// =============================================================================

// cardOp wires one card into the parallel GetCards/UpsertCards fan-outs.
// fetch reads the card and returns a closure that assigns it into Cards (run
// under the shared mutex); present reports whether the card is set for upsert,
// and upsert writes it.
type cardOp struct {
	name    string
	fetch   func(ctx context.Context, s *Store, sessionID string) (func(*Cards), error)
	present func(*Cards) bool
	upsert  func(ctx context.Context, s *Store, c *Cards) error
}

var cardOps = []cardOp{
	{
		name: "tokens_v2",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getTokensV2Card(ctx, id)
			return func(c *Cards) { c.TokensV2 = r }, err
		},
		present: func(c *Cards) bool { return c.TokensV2 != nil },
		upsert:  func(ctx context.Context, s *Store, c *Cards) error { return s.upsertTokensV2Card(ctx, c.TokensV2) },
	},
	{
		name: "session",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getSessionCard(ctx, id)
			return func(c *Cards) { c.Session = r }, err
		},
		present: func(c *Cards) bool { return c.Session != nil },
		upsert:  func(ctx context.Context, s *Store, c *Cards) error { return s.upsertSessionCard(ctx, c.Session) },
	},
	{
		name: "tools",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getToolsCard(ctx, id)
			return func(c *Cards) { c.Tools = r }, err
		},
		present: func(c *Cards) bool { return c.Tools != nil },
		upsert:  func(ctx context.Context, s *Store, c *Cards) error { return s.upsertToolsCard(ctx, c.Tools) },
	},
	{
		name: "code_activity",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getCodeActivityCard(ctx, id)
			return func(c *Cards) { c.CodeActivity = r }, err
		},
		present: func(c *Cards) bool { return c.CodeActivity != nil },
		upsert: func(ctx context.Context, s *Store, c *Cards) error {
			return s.upsertCodeActivityCard(ctx, c.CodeActivity)
		},
	},
	{
		name: "conversation",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getConversationCard(ctx, id)
			return func(c *Cards) { c.Conversation = r }, err
		},
		present: func(c *Cards) bool { return c.Conversation != nil },
		upsert: func(ctx context.Context, s *Store, c *Cards) error {
			return s.upsertConversationCard(ctx, c.Conversation)
		},
	},
	{
		name: "agents_and_skills",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getAgentsAndSkillsCard(ctx, id)
			return func(c *Cards) { c.AgentsAndSkills = r }, err
		},
		present: func(c *Cards) bool { return c.AgentsAndSkills != nil },
		upsert: func(ctx context.Context, s *Store, c *Cards) error {
			return s.upsertAgentsAndSkillsCard(ctx, c.AgentsAndSkills)
		},
	},
	{
		name: "redactions",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getRedactionsCard(ctx, id)
			return func(c *Cards) { c.Redactions = r }, err
		},
		present: func(c *Cards) bool { return c.Redactions != nil },
		upsert:  func(ctx context.Context, s *Store, c *Cards) error { return s.upsertRedactionsCard(ctx, c.Redactions) },
	},
	{
		name: "workflows",
		fetch: func(ctx context.Context, s *Store, id string) (func(*Cards), error) {
			r, err := s.getWorkflowsCard(ctx, id)
			return func(c *Cards) { c.Workflows = r }, err
		},
		present: func(c *Cards) bool { return c.Workflows != nil },
		upsert:  func(ctx context.Context, s *Store, c *Cards) error { return s.upsertWorkflowsCard(ctx, c.Workflows) },
	},
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
	errs := make(chan error, len(cardOps))

	for _, op := range cardOps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assign, err := op.fetch(ctx, s, sessionID)
			if err != nil {
				errs <- fmt.Errorf("%s: %w", op.name, err)
				return
			}
			mu.Lock()
			assign(cards)
			mu.Unlock()
		}()
	}

	wg.Wait()
	close(errs)

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

// UpsertCards inserts or updates all set cards for a session.
// All card upserts run in parallel to minimize latency.
func (s *Store) UpsertCards(ctx context.Context, cards *Cards) error {
	// Get session ID from the first available card for tracing.
	var sessionID string
	if cards.TokensV2 != nil {
		sessionID = cards.TokensV2.SessionID
	} else if cards.Session != nil {
		sessionID = cards.Session.SessionID
	}

	ctx, span := tracer.Start(ctx, "analytics.upsert_cards",
		trace.WithAttributes(attribute.String("session.id", sessionID)))
	defer span.End()

	var wg sync.WaitGroup
	errs := make(chan error, len(cardOps))

	for _, op := range cardOps {
		if !op.present(cards) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := op.upsert(ctx, s, cards); err != nil {
				errs <- fmt.Errorf("%s: %w", op.name, err)
			}
		}()
	}

	wg.Wait()
	close(errs)

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
