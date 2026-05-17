package analytics

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// LoadCodexRollout downloads and parses the Codex transcript for a session,
// returning the parsed rollout shape consumed by ComputeFromCodexRollout and
// PrepareCodexTranscript.
//
// Returns (nil, nil) when the session has no transcript row (caller should
// treat as an empty session). Parser-level validation errors are logged via
// slog but do not fail the call — the rollout is still returned. Storage and
// SQL errors are propagated.
//
// Shared by codexProvider and the on-demand API handler
// (HandleGetSessionAnalytics, CF-364). Both paths must produce the same rollout
// from the same bytes, so the download → parse → log path is centralized here.
func LoadCodexRollout(
	ctx context.Context,
	db *sql.DB,
	store *storage.S3Storage,
	sessionID string,
	userID int64,
	provider string,
	externalID string,
) (*codex.ParsedRollout, error) {
	var fileName string
	err := db.QueryRowContext(ctx,
		`SELECT file_name FROM sync_files WHERE session_id = $1 AND file_type = 'transcript' LIMIT 1`,
		sessionID,
	).Scan(&fileName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	raw, err := store.DownloadAndMergeChunks(ctx, userID, provider, externalID, fileName)
	if err != nil || raw == nil {
		return nil, err
	}

	rollout, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse codex rollout: %w", err)
	}
	if len(rollout.ValidationErrors) > 0 {
		slog.WarnContext(ctx, "codex rollout parse warnings",
			"session_id", sessionID,
			"validation_errors", len(rollout.ValidationErrors),
		)
	}
	return rollout, nil
}
