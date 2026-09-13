package analytics

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/logger"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
	"github.com/ConfabulousDev/confab-web/internal/validation"
)

// Session title recompute (nbrd) — worker bucket 4.
//
// POST /admin/cards/invalidate with the session_title target sets
// sessions.title_recompute_requested_at on title candidates (see
// dbadmincardinvalidations). The worker drains marked sessions here, repairing
// first_user_message from data Confab already stores — stored objects are only
// read, never written:
//
//   - Codex: a NULL first_user_message is filled from the transcript's chunk 1,
//     using the same event_msg-only derivation and byte clamp as sync ingest.
//     Fill-NULL only; an existing value is never overwritten.
//   - Cursor: a stored title still wrapped in <user_query> is unwrapped with the
//     same helper sync ingest uses. An empty envelope is left as-is (clearing it
//     would hide the session from the list).
//
// Every write is guarded on the marker value read at the start, so an admin
// re-invalidation that lands mid-processing is not lost, and on the prior title
// (COALESCE / equality) so a concurrent ingest write always wins. Transient DB or
// object-store failures return an error and keep the marker for the next tick.

// Outcomes logged per session.
const (
	titleOutcomeFixed      = "fixed"
	titleOutcomeNotFixable = "not_fixable"
	titleOutcomeAlreadySet = "already_set"
	// titleOutcomeSuperseded: the guarded write matched no row because a concurrent
	// ingest write or re-invalidation changed the session after it was read.
	titleOutcomeSuperseded = "superseded"
)

// titleRecomputeBeforeWrite is a test seam (see export_test.go) invoked just before
// each guarded write. Always nil in production.
var titleRecomputeBeforeWrite func(ctx context.Context, sessionID string)

// FindTitleRecomputeSessions returns up to limit sessions marked for title
// recompute, oldest request first. Provider is normalized to canonical form;
// TotalLines is unused (0).
func (p *Precomputer) FindTitleRecomputeSessions(ctx context.Context, limit int) ([]StaleSession, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, user_id, external_id, session_type
		FROM sessions
		WHERE title_recompute_requested_at IS NOT NULL
		  AND session_type = ANY($1)
		ORDER BY title_recompute_requested_at
		LIMIT $2
	`, pq.Array(models.AllowedProviders), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []StaleSession
	for rows.Next() {
		var s StaleSession
		var rawProvider string
		if err := rows.Scan(&s.SessionID, &s.UserID, &s.ExternalID, &rawProvider); err != nil {
			return nil, err
		}
		s.Provider = models.NormalizeProvider(rawProvider)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// RecomputeSessionTitle repairs one marked session's first_user_message and clears
// its marker. A session that is no longer marked (or no longer exists) is a no-op.
func (p *Precomputer) RecomputeSessionTitle(ctx context.Context, s StaleSession) error {
	var current sql.NullString
	var marker sql.NullTime
	err := p.db.QueryRowContext(ctx,
		`SELECT first_user_message, title_recompute_requested_at FROM sessions WHERE id = $1`, s.SessionID,
	).Scan(&current, &marker)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read session title: %w", err)
	}
	if !marker.Valid {
		return nil
	}
	seen := marker.Time

	var outcome string
	switch s.Provider {
	case models.ProviderCodex:
		outcome, err = p.recomputeCodexTitle(ctx, s, current, seen)
	case models.ProviderCursor:
		outcome, err = p.recomputeCursorTitle(ctx, s, current, seen)
	default:
		// Never marked by the candidate predicate; clear defensively.
		outcome, err = p.clearTitleMarker(ctx, s.SessionID, seen, titleOutcomeNotFixable)
	}
	if err != nil {
		return err
	}

	logger.Ctx(ctx).Info("session title recompute",
		"session_id", s.SessionID,
		"provider", s.Provider,
		"outcome", outcome,
	)
	return nil
}

func (p *Precomputer) recomputeCodexTitle(ctx context.Context, s StaleSession, current sql.NullString, seen time.Time) (string, error) {
	if current.Valid {
		return p.clearTitleMarker(ctx, s.SessionID, seen, titleOutcomeAlreadySet)
	}

	derived, err := p.deriveCodexFirstUserMessage(ctx, s)
	if err != nil {
		return "", err
	}
	if derived == "" {
		return p.clearTitleMarker(ctx, s.SessionID, seen, titleOutcomeNotFixable)
	}

	return p.guardedTitleWrite(ctx, s.SessionID, titleOutcomeFixed, `
		UPDATE sessions
		SET first_user_message = COALESCE(first_user_message, $2), title_recompute_requested_at = NULL
		WHERE id = $1 AND title_recompute_requested_at = $3
	`, s.SessionID, derived, seen)
}

func (p *Precomputer) recomputeCursorTitle(ctx context.Context, s StaleSession, current sql.NullString, seen time.Time) (string, error) {
	if current.Valid {
		if cleaned := ExtractCursorUserPrompt(current.String); cleaned != "" && cleaned != current.String {
			return p.guardedTitleWrite(ctx, s.SessionID, titleOutcomeFixed, `
				UPDATE sessions
				SET first_user_message = $2, title_recompute_requested_at = NULL
				WHERE id = $1 AND first_user_message = $3 AND title_recompute_requested_at = $4
			`, s.SessionID, cleaned, current.String, seen)
		}
	}
	outcome := titleOutcomeNotFixable
	if current.Valid && !strings.Contains(current.String, "<user_query>") {
		outcome = titleOutcomeAlreadySet
	}
	return p.clearTitleMarker(ctx, s.SessionID, seen, outcome)
}

// deriveCodexFirstUserMessage reads only the main transcript's chunk 1 and returns
// the first human prompt, clamped exactly as sync ingest clamps it. Returns "" with
// a nil error when nothing is derivable (no transcript, no chunk 1, no event_msg
// user message).
func (p *Precomputer) deriveCodexFirstUserMessage(ctx context.Context, s StaleSession) (string, error) {
	mainFileName, _, err := listCodexSyncFiles(ctx, p.db, s.SessionID)
	if err != nil {
		return "", fmt.Errorf("resolve codex transcript file: %w", err)
	}
	if mainFileName == "" {
		return "", nil
	}

	keys, err := p.store.ListChunks(ctx, s.UserID, s.Provider, s.ExternalID, mainFileName)
	if errors.Is(err, storage.ErrTooManyChunks) {
		// Permanent for this session; retrying every tick would never succeed.
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("list codex chunks: %w", err)
	}

	for _, key := range keys {
		if first, _, ok := storage.ParseChunkKey(key); !ok || first != 1 {
			continue
		}
		data, err := p.store.Download(ctx, key)
		if err != nil {
			return "", fmt.Errorf("download codex chunk 1: %w", err)
		}
		return firstCodexUserMessage(data), nil
	}
	return "", nil
}

// firstCodexUserMessage mirrors the sync ingest derivation (internal/api/sync.go):
// codex.UserMessageFromLine per line until one yields text, then the rune-safe
// byte clamp to the first_user_message column limit.
func firstCodexUserMessage(chunk []byte) string {
	for len(chunk) > 0 {
		line := chunk
		if i := bytes.IndexByte(chunk, '\n'); i >= 0 {
			line, chunk = chunk[:i], chunk[i+1:]
		} else {
			chunk = nil
		}
		if msg := codex.UserMessageFromLine(string(line)); msg != "" {
			return validation.TruncateToByteLimit(msg, validation.MaxFirstUserMessageLength)
		}
	}
	return ""
}

// clearTitleMarker clears the marker without touching the title, guarded on the
// marker value read at the start of processing.
func (p *Precomputer) clearTitleMarker(ctx context.Context, sessionID string, seen time.Time, outcome string) (string, error) {
	return p.guardedTitleWrite(ctx, sessionID, outcome, `
		UPDATE sessions SET title_recompute_requested_at = NULL
		WHERE id = $1 AND title_recompute_requested_at = $2
	`, sessionID, seen)
}

// guardedTitleWrite runs a guarded UPDATE and returns outcome, or
// titleOutcomeSuperseded when the guard matched no row.
func (p *Precomputer) guardedTitleWrite(ctx context.Context, sessionID, outcome, query string, args ...any) (string, error) {
	if titleRecomputeBeforeWrite != nil {
		titleRecomputeBeforeWrite(ctx, sessionID)
	}
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return "", fmt.Errorf("write session title: %w", err)
	}
	if n, err := result.RowsAffected(); err == nil && n == 0 {
		return titleOutcomeSuperseded, nil
	}
	return outcome, nil
}
