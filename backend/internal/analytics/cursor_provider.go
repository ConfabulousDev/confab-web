package analytics

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

type cursorProvider struct{}

// cursorRollout holds a parsed Cursor session. v1 is main-transcript-only —
// subagent files (file_type='agent') are deferred (see follow-up). Single-
// goroutine per the Rollout contract.
//
// bounds carries the session-level timing anchors (created_at refines
// first_seen at ingest; last_message_at/last_sync_at) used to estimate
// DurationMs, since Cursor JSONL lines have no per-line timestamps.
type cursorRollout struct {
	messages         []*CursorMessage
	validationErrors []LineValidationError
	bounds           CursorSessionBounds
}

func init() {
	RegisterProvider(&cursorProvider{}, models.ProviderCursor)
}

// Parse downloads the Cursor main transcript (file_type='transcript') and
// parses it into typed conversation messages. Returns (nil, nil) when the
// session has no transcript row yet — precompute treats a nil rollout as an
// empty session and skips it.
func (p *cursorProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	messages, lineErrors, found, err := loadCursorMain(ctx, input)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	bounds, err := loadCursorSessionBounds(ctx, input)
	if err != nil {
		return nil, err
	}
	return &cursorRollout{
		messages:         messages,
		validationErrors: lineErrors,
		bounds:           bounds,
	}, nil
}

func (p *cursorProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return ComputeFromCursorRollout(ctx, nil, CursorSessionBounds{})
	}
	result := ComputeFromCursorRollout(ctx, r.messages, r.bounds)
	result.ValidationErrorCount = len(r.validationErrors)
	return result
}

func (p *cursorProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return ""
	}
	return extractCursorSearchText(r.messages)
}

func (p *cursorProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return "", nil, nil
	}
	transcript, idMap := PrepareCursorTranscript(r.messages)
	return transcript, idMap, nil
}

// ClearMessageIDs returns true: Cursor JSONL has no stable message or tool ids,
// so smart-recap items cannot deep-link to a frontend anchor (Codex precedent).
func (p *cursorProvider) ClearMessageIDs() bool { return true }

func (p *cursorProvider) DisplayName() string { return "Cursor" }

// loadCursorMain reads the first file_type='transcript' row for the session,
// downloads + parses it. The bool return distinguishes "no transcript row yet"
// (found=false → nil rollout) from "empty/parsed transcript".
func loadCursorMain(ctx context.Context, input ParseInput) ([]*CursorMessage, []LineValidationError, bool, error) {
	var mainFileName string
	row := input.DB.QueryRowContext(ctx, `
		SELECT file_name
		FROM sync_files
		WHERE session_id = $1 AND file_type = 'transcript'
		ORDER BY id ASC
		LIMIT 1
	`, input.SessionID)
	if err := row.Scan(&mainFileName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No transcript row yet — treat as an empty session (nil rollout).
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}

	raw, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil {
		return nil, nil, false, err
	}
	if raw == nil {
		return nil, nil, false, nil
	}

	messages, lineErrors := parseCursorJSONL(ctx, raw, mainFileName)
	return messages, lineErrors, true, nil
}

// loadCursorSessionBounds reads the session's timing anchors used to estimate
// DurationMs. created_at has already been folded into first_seen at ingest (the
// sync handler lowers first_seen when an earlier created_at arrives), so this
// reads the two persisted columns — first_seen (start) and last_message_at /
// last_sync_at (end). All are nullable; a missing anchor yields a nil pointer
// and compute degrades to a nil duration.
func loadCursorSessionBounds(ctx context.Context, input ParseInput) (CursorSessionBounds, error) {
	var firstSeen time.Time
	var lastMessageAt, lastSyncAt *time.Time
	row := input.DB.QueryRowContext(ctx, `
		SELECT first_seen, last_message_at, last_sync_at
		FROM sessions
		WHERE id = $1
	`, input.SessionID)
	if err := row.Scan(&firstSeen, &lastMessageAt, &lastSyncAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CursorSessionBounds{}, nil
		}
		return CursorSessionBounds{}, err
	}
	return CursorSessionBounds{
		FirstSeen:     &firstSeen,
		LastMessageAt: lastMessageAt,
		LastSyncAt:    lastSyncAt,
	}, nil
}
