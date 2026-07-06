package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type opencodeProvider struct{}

func init() {
	RegisterProvider(&opencodeProvider{}, models.ProviderOpencode)
}

// Parse downloads the OpenCode main transcript and lists subagent rollout
// file names (file_type='agent') for lazy materialization later (CF-539).
// Per-line validation runs during the main download (see parseOpenCodeJSONL)
// and accumulates into the rollout's validationErrors slice using the shared
// LineValidationError type. Returns (nil, nil) when the session has no
// transcript row yet — precompute treats a nil rollout as an empty session
// and skips it.
func (p *opencodeProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	main, agentFileInfo, mainErrors, err := loadOpenCodeMainAndListAgents(ctx, input)
	if err != nil {
		return nil, err
	}
	if main == nil {
		return nil, nil
	}
	downloader := func(ctx context.Context, fileName string) ([]byte, error) {
		return input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, fileName)
	}
	return &opencodeRollout{
		main:             main,
		agentFileInfo:    agentFileInfo,
		downloader:       downloader,
		validationErrors: mainErrors,
		createdAt:        input.CreatedAt,
	}, nil
}

func (p *opencodeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return &ComputeResult{}
	}
	rollouts := r.materialize(ctx)
	result := computeFromOpenCodeRolloutAt(ctx, rollouts, r.createdAt)
	result.ValidationErrorCount = len(r.validationErrors)
	return result
}

func (p *opencodeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return ""
	}
	return extractOpenCodeSearchText(r.materialize(ctx))
}

func (p *opencodeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r, ok := rollout.(*opencodeRollout)
	if !ok || r == nil {
		return "", nil, nil
	}
	transcript, idMap := PrepareOpenCodeTranscript(r.materialize(ctx))
	return transcript, idMap, nil
}

func (p *opencodeProvider) ClearMessageIDs() bool {
	return false
}

func (p *opencodeProvider) DisplayName() string {
	return "OpenCode"
}

// materialize returns [main, ...subagents] in sync_files insertion order.
// First call downloads + parses each subagent (logging + appending a
// LineValidationError for any per-file failure to r.validationErrors);
// subsequent calls replay the cached slice without further S3 traffic. The
// cap (storage.MaxAgentFiles) is enforced silently — Codex parity, just log.
func (r *opencodeRollout) materialize(ctx context.Context) [][]*OpenCodeMessage {
	if r.cachedAgents == nil {
		limit := len(r.agentFileInfo)
		if limit > storage.MaxAgentFiles {
			slog.WarnContext(ctx, "opencode subagent file count exceeds cap; dropping overflow",
				"cap", storage.MaxAgentFiles, "count", limit)
			limit = storage.MaxAgentFiles
		}
		r.cachedAgents = make([][]*OpenCodeMessage, 0, limit)
		for _, info := range r.agentFileInfo[:limit] {
			messages, lineErrors, err := r.loadSubagent(ctx, info.FileName)
			if err != nil {
				r.validationErrors = append(r.validationErrors, LineValidationError{
					Errors: []ValidationError{{
						Path:    info.FileName,
						Message: err.Error(),
					}},
				})
				continue
			}
			// Prefix each per-line error path with the file name so operators
			// can correlate anomalies back to the source file.
			for i := range lineErrors {
				for j := range lineErrors[i].Errors {
					lineErrors[i].Errors[j].Path = fmt.Sprintf("%s:%s", info.FileName, lineErrors[i].Errors[j].Path)
				}
			}
			r.validationErrors = append(r.validationErrors, lineErrors...)
			r.cachedAgents = append(r.cachedAgents, messages)
		}
	}
	out := make([][]*OpenCodeMessage, 0, 1+len(r.cachedAgents))
	out = append(out, r.main)
	out = append(out, r.cachedAgents...)
	return out
}

// loadSubagent downloads and parses one subagent file. Returns parsed
// messages, per-line validation errors, and a fatal error if the whole file
// failed to download (caller appends a single LineValidationError in that
// case).
func (r *opencodeRollout) loadSubagent(ctx context.Context, fileName string) ([]*OpenCodeMessage, []LineValidationError, error) {
	raw, err := r.downloader(ctx, fileName)
	if err != nil {
		slog.WarnContext(ctx, "opencode subagent download failed", "file", fileName, "error", err)
		return nil, nil, fmt.Errorf("download failed: %w", err)
	}
	messages, lineErrors := parseOpenCodeJSONL(ctx, raw, fileName)
	return messages, lineErrors, nil
}

// loadOpenCodeMainAndListAgents loads the main transcript and lists subagent
// rollout file names (file_type='agent'). Returns (nil, nil, nil, nil) when
// the session has no transcript row yet.
func loadOpenCodeMainAndListAgents(ctx context.Context, input ParseInput) ([]*OpenCodeMessage, []opencodeAgentFileInfo, []LineValidationError, error) {
	rows, err := input.DB.QueryContext(ctx, `
		SELECT file_name, file_type
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent')
		ORDER BY id ASC
	`, input.SessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	var mainFileName string
	var agentFileInfo []opencodeAgentFileInfo
	for rows.Next() {
		var fileName, fileType string
		if err := rows.Scan(&fileName, &fileType); err != nil {
			return nil, nil, nil, err
		}
		switch fileType {
		case "transcript":
			if mainFileName == "" {
				mainFileName = fileName
			}
		case "agent":
			agentFileInfo = append(agentFileInfo, opencodeAgentFileInfo{FileName: fileName})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	if mainFileName == "" {
		// No transcript row yet — precompute treats a nil rollout as an empty
		// session and skips it (preserving the original loadOpenCodeMessages
		// behavior).
		return nil, nil, nil, nil
	}

	raw, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil {
		return nil, nil, nil, err
	}
	if raw == nil {
		return nil, nil, nil, nil
	}
	messages, lineErrors := parseOpenCodeJSONL(ctx, raw, mainFileName)
	return messages, agentFileInfo, lineErrors, nil
}

// parseOpenCodeJSONL does the two-pass parse: first to map[string]interface{}
// for ValidateOpenCodeLine, then to typed OpenCodeMessage. Per-line errors
// accumulate as LineValidationError entries (matching Claude's wire shape);
// the typed message is kept and contributes to compute, even when validation
// finds issues — same "validate but don't drop" policy as Claude. Lines that
// fail JSON unmarshal entirely become a single-error LineValidationError
// with path "" and a "json unmarshal failed" message.
func parseOpenCodeJSONL(ctx context.Context, raw []byte, fileName string) ([]*OpenCodeMessage, []LineValidationError) {
	var messages []*OpenCodeMessage
	var lineErrors []LineValidationError
	lineNum := 0
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lineNum++

		var rawMap map[string]interface{}
		if err := json.Unmarshal(line, &rawMap); err != nil {
			lineErrors = append(lineErrors, LineValidationError{
				Line:    lineNum,
				RawJSON: truncateJSON(string(line), 200),
				Errors: []ValidationError{{
					Path:    "",
					Message: fmt.Sprintf("json unmarshal failed: %v", err),
				}},
			})
			continue
		}

		if schemaErrors := ValidateOpenCodeLine(rawMap); len(schemaErrors) > 0 {
			var msgType string
			if info, ok := rawMap["info"].(map[string]interface{}); ok {
				msgType, _ = info["role"].(string)
			}
			lineErrors = append(lineErrors, LineValidationError{
				Line:        lineNum,
				RawJSON:     truncateJSON(string(line), 200),
				MessageType: msgType,
				Errors:      schemaErrors,
			})
		}

		var msg OpenCodeMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Typed unmarshal failure after map unmarshal succeeded is rare
			// (usually a type mismatch in a typed field). Record + skip.
			lineErrors = append(lineErrors, LineValidationError{
				Line:    lineNum,
				RawJSON: truncateJSON(string(line), 200),
				Errors: []ValidationError{{
					Path:    "",
					Message: fmt.Sprintf("typed unmarshal failed: %v", err),
				}},
			})
			continue
		}
		messages = append(messages, &msg)
	}
	if len(lineErrors) > 0 {
		slog.WarnContext(ctx, "opencode transcript had validation errors",
			"file", fileName, "errors", len(lineErrors), "parsed", len(messages))
	}
	return messages, lineErrors
}
