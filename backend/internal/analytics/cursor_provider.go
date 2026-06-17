package analytics

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type cursorProvider struct{}

// cursorRollout holds a parsed Cursor session: the main thread's messages loaded
// eagerly during Parse, plus the list of subagent agent-file names (file_type=
// 'agent', wc9t) kept until first traversal then cached on cachedAgents. Cursor
// stores subagent transcripts under
// agent-transcripts/<root>/subagents/<uuid>.jsonl with the identical line
// envelope as the main thread (plus a subagent-only UpdateCurrentStep marker);
// the CLI uploads them as file_type='agent'. Their analytics merge into the
// parent session (tools/code/agents); the conversation card and session window
// stay main-only (mirrors OpenCode CF-539). Single-goroutine per the Rollout
// contract.
//
// bounds carries the main-thread session-level timing anchors (created_at
// refines first_seen at ingest; last_message_at/last_sync_at) used to estimate
// DurationMs, since Cursor JSONL lines have no per-line timestamps.
type cursorRollout struct {
	main             []*CursorMessage
	agentFileNames   []string
	downloader       cursorAgentDownloader
	cachedAgents     [][]*CursorMessage
	validationErrors []LineValidationError
	bounds           CursorSessionBounds
}

type cursorAgentDownloader func(ctx context.Context, fileName string) ([]byte, error)

func init() {
	RegisterProvider(&cursorProvider{}, models.ProviderCursor)
}

// Parse downloads the Cursor main transcript (file_type='transcript') and
// parses it into typed conversation messages, and lists subagent rollout file
// names (file_type='agent') for lazy materialization later (wc9t). Returns
// (nil, nil) when the session has no transcript row yet — precompute treats a
// nil rollout as an empty session and skips it.
func (p *cursorProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	messages, agentFileNames, lineErrors, found, err := loadCursorMainAndListAgents(ctx, input)
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
	// Fold the per-session model (cursor_session_meta sidecar) into bounds so
	// compute can surface it in models_used. Absent → empty string → no model.
	model, err := loadCursorSessionMeta(ctx, input)
	if err != nil {
		return nil, err
	}
	bounds.Model = model
	downloader := func(ctx context.Context, fileName string) ([]byte, error) {
		return input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, fileName)
	}
	return &cursorRollout{
		main:             messages,
		agentFileNames:   agentFileNames,
		downloader:       downloader,
		validationErrors: lineErrors,
		bounds:           bounds,
	}, nil
}

func (p *cursorProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return ComputeFromCursorRollout(ctx, nil, CursorSessionBounds{})
	}
	rollouts := r.materialize(ctx)
	result := ComputeFromCursorRollout(ctx, rollouts, r.bounds)
	result.ValidationErrorCount = len(r.validationErrors)
	return result
}

func (p *cursorProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return ""
	}
	return extractCursorSearchText(r.materialize(ctx))
}

func (p *cursorProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r, ok := rollout.(*cursorRollout)
	if !ok || r == nil {
		return "", nil, nil
	}
	// Transcript rendering stays main-thread only (provider parity — Codex/
	// OpenCode don't render subagent threads in the main pane). Subagents
	// contribute to analytics and search recall, not the transcript view.
	transcript, idMap := PrepareCursorTranscript(r.main)
	return transcript, idMap, nil
}

// materialize returns [main, ...subagents] in sync_files insertion order. The
// first call downloads + parses each subagent file (logging + appending a
// LineValidationError for any per-file failure to r.validationErrors, prefixing
// per-line error paths with the file name); subsequent calls replay the cached
// slice without further S3 traffic. The cap (storage.MaxAgentFiles) is enforced
// silently — OpenCode/Codex parity, just log. Mirrors opencodeRollout.materialize.
func (r *cursorRollout) materialize(ctx context.Context) [][]*CursorMessage {
	if r.cachedAgents == nil {
		limit := len(r.agentFileNames)
		if limit > storage.MaxAgentFiles {
			slog.WarnContext(ctx, "cursor subagent file count exceeds cap; dropping overflow",
				"cap", storage.MaxAgentFiles, "count", limit)
			limit = storage.MaxAgentFiles
		}
		r.cachedAgents = make([][]*CursorMessage, 0, limit)
		for _, fileName := range r.agentFileNames[:limit] {
			messages, lineErrors, err := r.loadSubagent(ctx, fileName)
			if err != nil {
				r.validationErrors = append(r.validationErrors, LineValidationError{
					Errors: []ValidationError{{
						Path:    fileName,
						Message: err.Error(),
					}},
				})
				continue
			}
			for i := range lineErrors {
				for j := range lineErrors[i].Errors {
					lineErrors[i].Errors[j].Path = fmt.Sprintf("%s:%s", fileName, lineErrors[i].Errors[j].Path)
				}
			}
			r.validationErrors = append(r.validationErrors, lineErrors...)
			r.cachedAgents = append(r.cachedAgents, messages)
		}
	}
	out := make([][]*CursorMessage, 0, 1+len(r.cachedAgents))
	out = append(out, r.main)
	out = append(out, r.cachedAgents...)
	return out
}

// loadSubagent downloads and parses one subagent file. Returns parsed messages,
// per-line validation errors, and a fatal error if the whole file failed to
// download (caller appends a single LineValidationError in that case).
func (r *cursorRollout) loadSubagent(ctx context.Context, fileName string) ([]*CursorMessage, []LineValidationError, error) {
	raw, err := r.downloader(ctx, fileName)
	if err != nil {
		slog.WarnContext(ctx, "cursor subagent download failed", "file", fileName, "error", err)
		return nil, nil, fmt.Errorf("download failed: %w", err)
	}
	messages, lineErrors := parseCursorJSONL(ctx, raw, fileName)
	return messages, lineErrors, nil
}

// ClearMessageIDs returns true: Cursor JSONL has no stable message or tool ids,
// so smart-recap items cannot deep-link to a frontend anchor (Codex precedent).
func (p *cursorProvider) ClearMessageIDs() bool { return true }

func (p *cursorProvider) DisplayName() string { return "Cursor" }

// loadCursorMainAndListAgents reads the first file_type='transcript' row for the
// session (the main thread), downloads + parses it, and lists every
// file_type='agent' row's file name (subagent transcripts, wc9t) for lazy
// materialization. The bool return distinguishes "no transcript row yet"
// (found=false → nil rollout) from "empty/parsed transcript". Agent files are
// listed even when the main transcript exists but is empty.
func loadCursorMainAndListAgents(ctx context.Context, input ParseInput) ([]*CursorMessage, []string, []LineValidationError, bool, error) {
	rows, err := input.DB.QueryContext(ctx, `
		SELECT file_name, file_type
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent')
		ORDER BY id ASC
	`, input.SessionID)
	if err != nil {
		return nil, nil, nil, false, err
	}
	defer rows.Close()

	var mainFileName string
	var agentFileNames []string
	for rows.Next() {
		var fileName, fileType string
		if err := rows.Scan(&fileName, &fileType); err != nil {
			return nil, nil, nil, false, err
		}
		switch fileType {
		case "transcript":
			if mainFileName == "" {
				mainFileName = fileName
			}
		case "agent":
			agentFileNames = append(agentFileNames, fileName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, false, err
	}
	if mainFileName == "" {
		// No transcript row yet — treat as an empty session (nil rollout). An
		// agent file arriving before its main transcript is held until the main
		// lands (precompute reruns on each sync), so subagent activity is never
		// attributed to a session with no main thread.
		return nil, nil, nil, false, nil
	}

	raw, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if raw == nil {
		return nil, nil, nil, false, nil
	}

	messages, lineErrors := parseCursorJSONL(ctx, raw, mainFileName)
	return messages, agentFileNames, lineErrors, true, nil
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

// loadCursorSessionMeta reads the per-session model from the cursor_session_meta
// sidecar (zsr6), the only source of a Cursor session's model name (the synced
// JSONL has none). Returns "" when no row exists — compute then leaves
// models_used empty rather than inventing a model. Reads input.DB directly,
// mirroring loadCursorSessionBounds, so the analytics package stays free of a
// db/cursor import.
func loadCursorSessionMeta(ctx context.Context, input ParseInput) (string, error) {
	var model string
	row := input.DB.QueryRowContext(ctx, `
		SELECT model
		FROM cursor_session_meta
		WHERE session_id = $1
	`, input.SessionID)
	if err := row.Scan(&model); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return model, nil
}
