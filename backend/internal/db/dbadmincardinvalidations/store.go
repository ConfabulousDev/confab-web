// Package dbadmincardinvalidations provides DB operations for the
// admin_card_invalidations table. The table is both an audit log for admin-triggered
// card invalidations and the quota-bypass signal for smart-recap regeneration.
package dbadmincardinvalidations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/models"
)

// DefaultBatchSize is the number of sessions processed in a single Execute transaction.
// Each batch DELETEs from the selected card tables, marks session_title candidates, and
// INSERTs audit rows inside one transaction. Independent commits let Execute tolerate
// huge windows without a single giant transaction.
const DefaultBatchSize = 1000

// Store provides admin_card_invalidations database operations.
type Store struct {
	DB *db.DB

	// BatchSize overrides DefaultBatchSize when > 0. Used by tests.
	BatchSize int
}

func (s *Store) conn() *sql.DB { return s.DB.Conn() }

func (s *Store) batchSize() int {
	if s.BatchSize > 0 {
		return s.BatchSize
	}
	return DefaultBatchSize
}

// CountRequest describes a date-window + targets query. Used by both CountAffected
// (dry-run) and Execute (to scope DELETEs and title marks).
type CountRequest struct {
	StartDate time.Time
	EndDate   *time.Time // nil means open-ended (no upper bound)
	// CardTypes are invalidation targets: card table names and/or
	// analytics.SessionTitleInvalidationTarget.
	CardTypes []string
	// Providers restricts every selected target to sessions whose session_type is
	// in this list. Callers pass alias-expanded values (models.ExpandWithAliases).
	// Empty means all providers.
	Providers []string
}

// CountResult is the shape returned by CountAffected and echoed in Execute's response.
// AffectedSessions counts DISTINCT sessions in the date window (and provider filter)
// that have at least one row in any selected card table or, when session_title is
// selected, are a title candidate (union across targets).
// AffectedCards[target] is the per-table row count that would be / was deleted, and
// for session_title the number of title candidates that would be / were marked.
type CountResult struct {
	AffectedSessions int
	AffectedCards    map[string]int
}

// ExecuteRequest extends CountRequest with the admin identity and the reason string
// captured for the audit log.
type ExecuteRequest struct {
	CountRequest
	AdminUserID int64
	Reason      string
}

// ExecuteResult reports the outcome of a chunked execute. On success, CorrelationID
// is populated and Result contains the aggregated counts from all committed batches.
// On partial failure, Err is set and CompletedBatches / Result reflect the progress
// that committed before the failure.
type ExecuteResult struct {
	CorrelationID    uuid.UUID
	Result           CountResult
	CompletedBatches int
	Err              error
}

// AuditRow is a single row returned by ListRecent / ListByCorrelationID.
// AdminEmail is joined from users at read time; it is empty when the admin has been deleted.
type AuditRow struct {
	ID            int64
	SessionID     string
	AdminUserID   int64
	AdminEmail    string
	InvalidatedAt time.Time
	CardTypes     []string
	CorrelationID uuid.UUID
	Reason        string
}

// targets is a validated CardTypes list split by kind. Only cardTables may be
// interpolated into SQL; session_title is handled by the title candidate predicate.
type targets struct {
	cardTables []string
	titles     bool
}

// splitTargets rejects empty or unknown targets before anything reaches SQL.
func splitTargets(cardTypes []string) (targets, error) {
	var t targets
	if len(cardTypes) == 0 {
		return t, fmt.Errorf("card_types must be non-empty")
	}
	for _, ct := range cardTypes {
		switch {
		case ct == analytics.SessionTitleInvalidationTarget:
			t.titles = true
		case analytics.IsKnownCardTableName(ct):
			t.cardTables = append(t.cardTables, ct)
		default:
			return t, fmt.Errorf("unknown card_type: %s", ct)
		}
	}
	return t, nil
}

// unionSessionIDs returns a UNION ALL of `SELECT session_id FROM <table>` for the
// given card tables. Callers must pass only targets.cardTables from splitTargets.
func unionSessionIDs(cardTables []string) string {
	parts := make([]string, len(cardTables))
	for i, ct := range cardTables {
		parts[i] = fmt.Sprintf(`SELECT session_id FROM %s`, ct)
	}
	return strings.Join(parts, " UNION ALL ")
}

// sessionScope is the window + provider filter shared by every session-selecting
// query. It binds $1 (start), $2 (end, NULL = open), $3 (providers, NULL = all);
// build its arguments with scopeArgs.
const sessionScope = `s.last_message_at >= $1
	AND ($2::timestamptz IS NULL OR s.last_message_at < $2)
	AND ($3::text[] IS NULL OR s.session_type = ANY($3))`

func scopeArgs(req CountRequest) []any {
	var end, providers any
	if req.EndDate != nil {
		end = *req.EndDate
	}
	if len(req.Providers) > 0 {
		providers = pq.Array(req.Providers)
	}
	return []any{req.StartDate, end, providers}
}

// Session types whose stored title the session_title recompute can repair, with
// legacy aliases so the predicate matches rows written by older binaries.
var (
	codexSessionTypes  = models.ExpandWithAliases([]string{models.ProviderCodex})
	cursorSessionTypes = models.ExpandWithAliases([]string{models.ProviderCursor})
)

// titleCandidatePredicate is the single definition of a session_title candidate:
// a Codex session with no first_user_message, or a Cursor session whose title still
// carries the <user_query> envelope. codexParam/cursorParam are the placeholders
// bound to titleCandidateArgs().
func titleCandidatePredicate(codexParam, cursorParam string) string {
	return fmt.Sprintf(`(
		(s.session_type = ANY(%s::text[]) AND s.first_user_message IS NULL)
		OR (s.session_type = ANY(%s::text[]) AND s.first_user_message LIKE '%%<user_query>%%')
	)`, codexParam, cursorParam)
}

func titleCandidateArgs() []any {
	return []any{pq.Array(codexSessionTypes), pq.Array(cursorSessionTypes)}
}

// affectedSessionsQuery returns the WHERE-scoped selection of sessions touched by
// any selected target (card rows ∪ title candidates) and its arguments. selectExpr
// is the projection, e.g. "COUNT(*)" or "s.id".
func affectedSessionsQuery(selectExpr, suffix string, req CountRequest, t targets) (string, []any) {
	args := scopeArgs(req)
	var clauses []string
	if len(t.cardTables) > 0 {
		clauses = append(clauses, fmt.Sprintf(`s.id IN (%s)`, unionSessionIDs(t.cardTables)))
	}
	if t.titles {
		clauses = append(clauses, titleCandidatePredicate("$4", "$5"))
		args = append(args, titleCandidateArgs()...)
	}
	query := fmt.Sprintf(`
		SELECT %s
		FROM sessions s
		WHERE %s
		  AND (%s)
		%s
	`, selectExpr, sessionScope, strings.Join(clauses, " OR "), suffix)
	return query, args
}

// CountAffected returns the distinct-session count and per-target counts for the
// date window and provider filter. A session is counted when it has at least one row
// in a selected card table or is a title candidate while session_title is selected.
func (s *Store) CountAffected(ctx context.Context, req CountRequest) (*CountResult, error) {
	t, err := splitTargets(req.CardTypes)
	if err != nil {
		return nil, err
	}

	result := &CountResult{AffectedCards: make(map[string]int, len(req.CardTypes))}

	countQuery, countArgs := affectedSessionsQuery("COUNT(*)", "", req, t)
	if err := s.conn().QueryRowContext(ctx, countQuery, countArgs...).Scan(&result.AffectedSessions); err != nil {
		return nil, fmt.Errorf("count affected sessions: %w", err)
	}

	for _, ct := range t.cardTables {
		q := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s c
			JOIN sessions s ON s.id = c.session_id
			WHERE %s
		`, ct, sessionScope)
		var n int
		if err := s.conn().QueryRowContext(ctx, q, scopeArgs(req)...).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", ct, err)
		}
		result.AffectedCards[ct] = n
	}

	if t.titles {
		q := fmt.Sprintf(`
			SELECT COUNT(*) FROM sessions s
			WHERE %s AND %s
		`, sessionScope, titleCandidatePredicate("$4", "$5"))
		var n int
		if err := s.conn().QueryRowContext(ctx, q, append(scopeArgs(req), titleCandidateArgs()...)...).Scan(&n); err != nil {
			return nil, fmt.Errorf("count session_title candidates: %w", err)
		}
		result.AffectedCards[analytics.SessionTitleInvalidationTarget] = n
	}

	return result, nil
}

// selectSessionIDs returns the ordered IDs of every session CountAffected counts.
func (s *Store) selectSessionIDs(ctx context.Context, req CountRequest, t targets) ([]string, error) {
	query, args := affectedSessionsQuery("s.id", "ORDER BY s.id", req, t)
	rows, err := s.conn().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select session ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Execute runs the invalidation in chunked batches. Each batch DELETEs from the
// selected card tables, marks title candidates, and inserts audit rows inside a
// single transaction that commits independently. Stops at the first failure,
// returning partial progress.
func (s *Store) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	t, err := splitTargets(req.CardTypes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("reason must be non-empty")
	}

	ids, err := s.selectSessionIDs(ctx, req.CountRequest, t)
	if err != nil {
		return nil, err
	}

	// Pre-seed AffectedCards with zero for every selected target so the response
	// shape matches dry-run even when 0 batches run.
	affectedCards := make(map[string]int, len(req.CardTypes))
	for _, ct := range req.CardTypes {
		affectedCards[ct] = 0
	}
	res := &ExecuteResult{
		CorrelationID: uuid.New(),
		Result:        CountResult{AffectedCards: affectedCards},
	}

	batchSize := s.batchSize()
	for start := 0; start < len(ids); start += batchSize {
		end := min(start+batchSize, len(ids))
		batch := ids[start:end]

		perTarget, err := s.executeBatch(ctx, req, t, batch, res.CorrelationID)
		if err != nil {
			res.Err = err
			return res, err
		}
		res.Result.AffectedSessions += len(batch)
		for ct, n := range perTarget {
			res.Result.AffectedCards[ct] += n
		}
		res.CompletedBatches++
	}

	return res, nil
}

// executeBatch runs one batch transaction: DELETE from each selected card table,
// mark the batch's title candidates when session_title is selected, and INSERT one
// audit row per session.
func (s *Store) executeBatch(ctx context.Context, req ExecuteRequest, t targets, batch []string, correlationID uuid.UUID) (map[string]int, error) {
	tx, err := s.conn().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	perTarget := make(map[string]int, len(req.CardTypes))
	for _, ct := range t.cardTables {
		result, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE session_id = ANY($1)`, ct), pq.Array(batch))
		if err != nil {
			return nil, fmt.Errorf("delete %s: %w", ct, err)
		}
		n, _ := result.RowsAffected()
		perTarget[ct] = int(n)
	}

	if t.titles {
		result, err := tx.ExecContext(ctx, `
			UPDATE sessions s SET title_recompute_requested_at = NOW()
			WHERE s.id = ANY($1) AND `+titleCandidatePredicate("$2", "$3"),
			append([]any{pq.Array(batch)}, titleCandidateArgs()...)...)
		if err != nil {
			return nil, fmt.Errorf("mark session_title candidates: %w", err)
		}
		n, _ := result.RowsAffected()
		perTarget[analytics.SessionTitleInvalidationTarget] = int(n)
	}

	for _, sid := range batch {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO admin_card_invalidations (
				session_id, admin_user_id, card_types, correlation_id, reason
			) VALUES ($1, $2, $3, $4, $5)
		`, sid, req.AdminUserID, pq.Array(req.CardTypes), correlationID, req.Reason); err != nil {
			return nil, fmt.Errorf("insert audit row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return perTarget, nil
}

const listColumns = `
	aci.id, aci.session_id, aci.admin_user_id,
	COALESCE(u.email, '') AS admin_email,
	aci.invalidated_at, aci.card_types, aci.correlation_id, aci.reason
`

// ListRecent returns up to `limit` of the most recent audit rows. limit <= 0 uses 500.
func (s *Store) ListRecent(ctx context.Context, limit int) ([]AuditRow, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.conn().QueryContext(ctx, `
		SELECT `+listColumns+`
		FROM admin_card_invalidations aci
		LEFT JOIN users u ON u.id = aci.admin_user_id
		ORDER BY aci.invalidated_at DESC, aci.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return scanAuditRows(rows)
}

// ListByCorrelationID returns all audit rows for a single correlation_id.
func (s *Store) ListByCorrelationID(ctx context.Context, correlationID uuid.UUID) ([]AuditRow, error) {
	rows, err := s.conn().QueryContext(ctx, `
		SELECT `+listColumns+`
		FROM admin_card_invalidations aci
		LEFT JOIN users u ON u.id = aci.admin_user_id
		WHERE aci.correlation_id = $1
		ORDER BY aci.id
	`, correlationID)
	if err != nil {
		return nil, err
	}
	return scanAuditRows(rows)
}

func scanAuditRows(rows *sql.Rows) ([]AuditRow, error) {
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var r AuditRow
		var cardTypes pq.StringArray
		if err := rows.Scan(&r.ID, &r.SessionID, &r.AdminUserID, &r.AdminEmail,
			&r.InvalidatedAt, &cardTypes, &r.CorrelationID, &r.Reason); err != nil {
			return nil, err
		}
		r.CardTypes = cardTypes
		out = append(out, r)
	}
	return out, rows.Err()
}
