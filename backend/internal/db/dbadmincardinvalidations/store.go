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
)

// DefaultBatchSize is the number of sessions processed in a single Execute transaction.
// Each batch DELETEs from the selected card tables and INSERTs audit rows inside one
// transaction. Independent commits let Execute tolerate huge windows without a single
// giant transaction.
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

// CountRequest describes a date-window + card-types query. Used by both CountAffected
// (dry-run) and Execute (to scope DELETEs).
type CountRequest struct {
	StartDate time.Time
	EndDate   *time.Time // nil means open-ended (no upper bound)
	CardTypes []string
}

// CountResult is the shape returned by CountAffected and echoed in Execute's response.
// AffectedSessions counts DISTINCT sessions in the date window that have at least
// one row in any selected card table (intersection semantic).
// AffectedCards[tableName] is the per-table row count that would be / was deleted.
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

// validateCardTypes rejects empty or unknown table names before they reach SQL.
func validateCardTypes(cardTypes []string) error {
	if len(cardTypes) == 0 {
		return fmt.Errorf("card_types must be non-empty")
	}
	for _, ct := range cardTypes {
		if !analytics.IsKnownCardTableName(ct) {
			return fmt.Errorf("unknown card_type: %s", ct)
		}
	}
	return nil
}

// unionSessionIDs returns a UNION ALL of `SELECT session_id FROM <table>` for the
// given card tables. Callers must have validated cardTypes via validateCardTypes
// before interpolating the result into SQL.
func unionSessionIDs(cardTypes []string) string {
	parts := make([]string, len(cardTypes))
	for i, ct := range cardTypes {
		parts[i] = fmt.Sprintf(`SELECT session_id FROM %s`, ct)
	}
	return strings.Join(parts, " UNION ALL ")
}

// endArg converts an optional upper bound into a driver value. Nil stays nil so
// the `$2::timestamptz IS NULL` guard in the query skips the upper-bound check.
func endArg(end *time.Time) interface{} {
	if end == nil {
		return nil
	}
	return *end
}

// CountAffected returns the distinct-session count and per-table row counts for the
// date window. Intersection semantic: a session is counted only when it has at least
// one row in one of the selected card tables.
func (s *Store) CountAffected(ctx context.Context, req CountRequest) (*CountResult, error) {
	if err := validateCardTypes(req.CardTypes); err != nil {
		return nil, err
	}

	result := &CountResult{AffectedCards: make(map[string]int, len(req.CardTypes))}
	end := endArg(req.EndDate)

	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT s.id)
		FROM sessions s
		WHERE s.last_message_at >= $1
		  AND ($2::timestamptz IS NULL OR s.last_message_at < $2)
		  AND s.id IN (%s)
	`, unionSessionIDs(req.CardTypes))
	if err := s.conn().QueryRowContext(ctx, countQuery, req.StartDate, end).Scan(&result.AffectedSessions); err != nil {
		return nil, fmt.Errorf("count affected sessions: %w", err)
	}

	for _, ct := range req.CardTypes {
		q := fmt.Sprintf(`
			SELECT COUNT(*) FROM %s c
			JOIN sessions s ON s.id = c.session_id
			WHERE s.last_message_at >= $1
			  AND ($2::timestamptz IS NULL OR s.last_message_at < $2)
		`, ct)
		var n int
		if err := s.conn().QueryRowContext(ctx, q, req.StartDate, end).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", ct, err)
		}
		result.AffectedCards[ct] = n
	}

	return result, nil
}

// selectSessionIDs returns the ordered list of session IDs in the date window that
// have at least one row in any selected card table (intersection semantic).
func (s *Store) selectSessionIDs(ctx context.Context, req CountRequest) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT DISTINCT s.id
		FROM sessions s
		WHERE s.last_message_at >= $1
		  AND ($2::timestamptz IS NULL OR s.last_message_at < $2)
		  AND s.id IN (%s)
		ORDER BY s.id
	`, unionSessionIDs(req.CardTypes))

	rows, err := s.conn().QueryContext(ctx, query, req.StartDate, endArg(req.EndDate))
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
// selected card tables and inserts audit rows inside a single transaction that
// commits independently. Stops at the first failure, returning partial progress.
func (s *Store) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	if err := validateCardTypes(req.CardTypes); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return nil, fmt.Errorf("reason must be non-empty")
	}

	ids, err := s.selectSessionIDs(ctx, req.CountRequest)
	if err != nil {
		return nil, err
	}

	// Pre-seed AffectedCards with zero for every selected table so the response
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
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]

		perTable, err := s.executeBatch(ctx, req, batch, res.CorrelationID)
		if err != nil {
			res.Err = err
			return res, err
		}
		res.Result.AffectedSessions += len(batch)
		for ct, n := range perTable {
			res.Result.AffectedCards[ct] += n
		}
		res.CompletedBatches++
	}

	return res, nil
}

// executeBatch runs one batch transaction: DELETE from each selected card table
// and INSERT one audit row per session.
func (s *Store) executeBatch(ctx context.Context, req ExecuteRequest, batch []string, correlationID uuid.UUID) (map[string]int, error) {
	tx, err := s.conn().BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	perTable := make(map[string]int, len(req.CardTypes))
	for _, ct := range req.CardTypes {
		result, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE session_id = ANY($1)`, ct), pq.Array(batch))
		if err != nil {
			return nil, fmt.Errorf("delete %s: %w", ct, err)
		}
		n, _ := result.RowsAffected()
		perTable[ct] = int(n)
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
	return perTable, nil
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
