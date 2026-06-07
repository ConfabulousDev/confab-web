package analytics

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/models"
)

type opencodeProvider struct{}

func init() {
	RegisterProvider(&opencodeProvider{}, models.ProviderOpencode)
}

// Parse downloads the OpenCode transcript (one JSON message per JSONL line:
// {info, parts}), deserializes each line into an OpenCodeMessage, and returns
// the assembled rollout. There are no subagent/journal sidecar files for
// OpenCode, so a single transcript download is all that's needed. Returns
// (nil, nil) when the session has no transcript row yet — precompute treats a
// nil rollout as an empty session and skips it.
func (p *opencodeProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	messages, err := loadOpenCodeMessages(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, nil
	}
	return &opencodeRollout{Messages: messages}, nil
}

// loadOpenCodeMessages reads the transcript file name from sync_files,
// downloads + merges its chunks, and parses each non-empty JSONL line into an
// OpenCodeMessage. Malformed lines are logged and skipped rather than failing
// the whole session.
func loadOpenCodeMessages(ctx context.Context, input ParseInput) ([]*OpenCodeMessage, error) {
	var fileName string
	err := input.DB.QueryRowContext(ctx, `
		SELECT file_name
		FROM sync_files
		WHERE session_id = $1 AND file_type = 'transcript'
		ORDER BY id ASC
		LIMIT 1
	`, input.SessionID).Scan(&fileName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	raw, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, fileName)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var messages []*OpenCodeMessage
	var skipped int
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var msg OpenCodeMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			skipped++
			continue
		}
		messages = append(messages, &msg)
	}
	if skipped > 0 {
		slog.WarnContext(ctx, "opencode transcript had unparseable lines",
			"session_id", input.SessionID, "skipped", skipped, "parsed", len(messages))
	}
	return messages, nil
}

func (p *opencodeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return &ComputeResult{}
	}
	return ComputeFromOpenCodeRollout(ctx, r)
}

func (p *opencodeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return ""
	}
	return extractOpenCodeSearchText(r)
}

func (p *opencodeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return "", nil, nil
	}
	transcript, idMap := PrepareOpenCodeTranscript(r)
	return transcript, idMap, nil
}

func (p *opencodeProvider) ClearMessageIDs() bool {
	return false
}

func (p *opencodeProvider) DisplayName() string {
	return "OpenCode"
}
