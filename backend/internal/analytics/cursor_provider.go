package analytics

import (
	"context"
	"database/sql"
	"errors"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

type cursorProvider struct{}

// cursorRollout holds a parsed Cursor session. v1 is main-transcript-only —
// subagent files (file_type='agent') are deferred (see follow-up). Single-
// goroutine per the Rollout contract.
type cursorRollout struct {
	messages         []*CursorMessage
	validationErrors []LineValidationError
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
	return &cursorRollout{
		messages:         messages,
		validationErrors: lineErrors,
	}, nil
}

func (p *cursorProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return ComputeFromCursorRollout(ctx, nil)
	}
	result := ComputeFromCursorRollout(ctx, r.messages)
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
