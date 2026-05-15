package codex

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/ConfabulousDev/confab-web/internal/db"
)

// Rollout is one row of codex_rollouts. NULL string columns collapse to "".
type Rollout struct {
	ThreadUUID       string
	UserID           int64
	ParentThreadUUID *string
	HostedSessionID  string
	HostedFileName   string
	RolloutPath      string
	CWD              string
	Model            string
	Source           string
	ThreadSource     string
	AgentPath        string
	AgentRole        string
	AgentNickname    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// UpsertRolloutParams carries the wire fields from a sync/chunk codex_rollout
// metadata block. user_id is passed separately by the caller (the
// authenticated user from the request context).
type UpsertRolloutParams struct {
	ThreadUUID       string
	ParentThreadUUID *string
	HostedSessionID  string
	HostedFileName   string
	RolloutPath      string
	CWD              string
	Model            string
	Source           string
	ThreadSource     string
	AgentPath        string
	AgentRole        string
	AgentNickname    string
}

const rolloutColumns = `thread_uuid, user_id, parent_thread_uuid, hosted_session_id, hosted_file_name,
	rollout_path, COALESCE(cwd, ''), COALESCE(model, ''), COALESCE(source, ''),
	COALESCE(thread_source, ''), COALESCE(agent_path, ''),
	COALESCE(agent_role, ''), COALESCE(agent_nickname, ''),
	created_at, updated_at`

// scanRollout reads one row in the rolloutColumns order.
func scanRollout(scanner interface {
	Scan(dest ...interface{}) error
}) (*Rollout, error) {
	var r Rollout
	if err := scanner.Scan(
		&r.ThreadUUID,
		&r.UserID,
		&r.ParentThreadUUID,
		&r.HostedSessionID,
		&r.HostedFileName,
		&r.RolloutPath,
		&r.CWD,
		&r.Model,
		&r.Source,
		&r.ThreadSource,
		&r.AgentPath,
		&r.AgentRole,
		&r.AgentNickname,
		&r.CreatedAt,
		&r.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &r, nil
}

// UpsertRollout inserts or updates a codex_rollouts row.
//
// Semantics (locked by CF-385 spec):
//   - Composite PK (user_id, thread_uuid).
//   - First-write-wins on parent_thread_uuid: COALESCE preserves the existing
//     non-null parent across re-upserts.
//   - Free-form fields (rollout_path, cwd, model, source, thread_source,
//     agent_*) are preserved across re-upserts when the incoming value is
//     empty (COALESCE/NULLIF). Non-empty incoming values overwrite.
//   - updated_at advances on every successful call (NOW()).
func (s *Store) UpsertRollout(ctx context.Context, userID int64, p UpsertRolloutParams) error {
	ctx, span := tracer.Start(ctx, "db.upsert_rollout",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("thread.uuid", p.ThreadUUID),
		))
	defer span.End()

	const query = `
		INSERT INTO codex_rollouts (
			thread_uuid, user_id, parent_thread_uuid, hosted_session_id, hosted_file_name,
			rollout_path, cwd, model, source, thread_source,
			agent_path, agent_role, agent_nickname
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (user_id, thread_uuid) DO UPDATE SET
			parent_thread_uuid = COALESCE(codex_rollouts.parent_thread_uuid, EXCLUDED.parent_thread_uuid),
			hosted_session_id  = EXCLUDED.hosted_session_id,
			hosted_file_name   = EXCLUDED.hosted_file_name,
			rollout_path       = COALESCE(NULLIF(EXCLUDED.rollout_path,   ''), codex_rollouts.rollout_path),
			cwd                = COALESCE(NULLIF(EXCLUDED.cwd,            ''), codex_rollouts.cwd),
			model              = COALESCE(NULLIF(EXCLUDED.model,          ''), codex_rollouts.model),
			source             = COALESCE(NULLIF(EXCLUDED.source,         ''), codex_rollouts.source),
			thread_source      = COALESCE(NULLIF(EXCLUDED.thread_source,  ''), codex_rollouts.thread_source),
			agent_path         = COALESCE(NULLIF(EXCLUDED.agent_path,     ''), codex_rollouts.agent_path),
			agent_role         = COALESCE(NULLIF(EXCLUDED.agent_role,     ''), codex_rollouts.agent_role),
			agent_nickname     = COALESCE(NULLIF(EXCLUDED.agent_nickname, ''), codex_rollouts.agent_nickname),
			updated_at         = NOW()
	`

	_, err := s.conn().ExecContext(ctx, query,
		p.ThreadUUID, userID, p.ParentThreadUUID, p.HostedSessionID, p.HostedFileName,
		p.RolloutPath, p.CWD, p.Model, p.Source, p.ThreadSource,
		p.AgentPath, p.AgentRole, p.AgentNickname,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("upsert codex rollout: %w", err)
	}
	return nil
}

// GetRollout returns the rollout row owned by userID with the given thread UUID.
// Returns db.ErrRolloutNotFound when the row does not exist for this user.
func (s *Store) GetRollout(ctx context.Context, userID int64, threadUUID string) (*Rollout, error) {
	ctx, span := tracer.Start(ctx, "db.get_rollout",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("thread.uuid", threadUUID),
		))
	defer span.End()

	query := `SELECT ` + rolloutColumns + ` FROM codex_rollouts WHERE user_id = $1 AND thread_uuid = $2`
	row := s.conn().QueryRowContext(ctx, query, userID, threadUUID)
	r, err := scanRollout(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, db.ErrRolloutNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("get codex rollout: %w", err)
	}
	return r, nil
}

// ListSubtree returns the row for rootThreadUUID plus all descendants reachable
// via parent_thread_uuid edges within user_id's namespace, ordered by
// created_at ASC. Uses UNION (not UNION ALL) so any cycle in the input
// terminates naturally via per-iteration row deduplication.
func (s *Store) ListSubtree(ctx context.Context, userID int64, rootThreadUUID string) ([]*Rollout, error) {
	ctx, span := tracer.Start(ctx, "db.list_subtree",
		trace.WithAttributes(
			attribute.Int64("user.id", userID),
			attribute.String("thread.uuid", rootThreadUUID),
		))
	defer span.End()

	// The recursive CTE selects raw columns (no COALESCE) so the recursive
	// JOIN compares parent_thread_uuid against the unaltered thread_uuid; the
	// final SELECT reuses rolloutColumns to apply COALESCE consistently with
	// GetRollout's projection.
	query := `
		WITH RECURSIVE subtree AS (
			SELECT thread_uuid, user_id, parent_thread_uuid, hosted_session_id, hosted_file_name,
				   rollout_path, cwd, model, source, thread_source,
				   agent_path, agent_role, agent_nickname, created_at, updated_at
			  FROM codex_rollouts
			 WHERE user_id = $1 AND thread_uuid = $2
			UNION
			SELECT cr.thread_uuid, cr.user_id, cr.parent_thread_uuid, cr.hosted_session_id,
				   cr.hosted_file_name, cr.rollout_path, cr.cwd, cr.model, cr.source,
				   cr.thread_source, cr.agent_path, cr.agent_role, cr.agent_nickname,
				   cr.created_at, cr.updated_at
			  FROM codex_rollouts cr
			  JOIN subtree s
				ON cr.parent_thread_uuid = s.thread_uuid
			   AND cr.user_id = s.user_id
		)
		SELECT ` + rolloutColumns + `
		  FROM subtree
		 ORDER BY created_at ASC
	`

	rows, err := s.conn().QueryContext(ctx, query, userID, rootThreadUUID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("list codex rollout subtree: %w", err)
	}
	defer rows.Close()

	var out []*Rollout
	for rows.Next() {
		r, err := scanRollout(rows)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("scan subtree row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("iterate subtree rows: %w", err)
	}
	return out, nil
}
