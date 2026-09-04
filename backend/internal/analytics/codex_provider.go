package analytics

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/codex"
	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type codexProvider struct{}

// codexRollout mirrors claudeRollout: main rollout parsed eagerly during
// Parse, subagent file infos held until the first traversal, then cached on
// cachedAgents so subsequent methods reuse the parsed result instead of
// re-downloading. Single-goroutine per the Rollout contract.
type codexRollout struct {
	main          *codex.ParsedRollout
	agentFileInfo []codexAgentFileInfo
	downloader    codexAgentDownloader
	cachedAgents  []*codex.ParsedRollout
}

type codexAgentFileInfo struct {
	FileName string
}

type codexAgentDownloader func(ctx context.Context, fileName string) ([]byte, error)

func init() {
	RegisterProvider(&codexProvider{}, models.ProviderCodex)
}

func (p *codexProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	main, agentFileInfo, err := loadCodexMainAndListAgents(ctx, input)
	if err != nil {
		return nil, err
	}
	if main == nil {
		return nil, nil
	}
	downloader := func(ctx context.Context, fileName string) ([]byte, error) {
		return input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, fileName)
	}
	return &codexRollout{
		main:          main,
		agentFileInfo: agentFileInfo,
		downloader:    downloader,
	}, nil
}

func (p *codexProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r := rollout.(*codexRollout)
	return ComputeFromCodexRollout(ctx, r.materialize(ctx))
}

func (p *codexProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r := rollout.(*codexRollout)
	return ExtractCodexUserMessagesText(r.materialize(ctx))
}

func (p *codexProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r := rollout.(*codexRollout)
	transcript, idMap := PrepareCodexTranscript(r.materialize(ctx))
	return transcript, idMap, nil
}

func (p *codexProvider) ClearMessageIDs() bool { return true }
func (p *codexProvider) DisplayName() string   { return "Codex" }

// materialize returns [main, ...subagents] in sync_files insertion order.
// First call downloads + parses each subagent (logging + recording any
// per-file failure as a synthetic ValidationError on main); subsequent calls
// replay the cached slice without further S3 traffic.
func (r *codexRollout) materialize(ctx context.Context) []*codex.ParsedRollout {
	if r.cachedAgents == nil {
		limit := len(r.agentFileInfo)
		if limit > storage.MaxAgentFiles {
			slog.WarnContext(ctx, "codex subagent file count exceeds cap; dropping overflow",
				"cap", storage.MaxAgentFiles, "count", limit)
			limit = storage.MaxAgentFiles
		}
		r.cachedAgents = make([]*codex.ParsedRollout, 0, limit)
		for _, info := range r.agentFileInfo[:limit] {
			parsed, err := r.loadSubagent(ctx, info.FileName)
			if err != nil {
				r.main.ValidationErrors = append(r.main.ValidationErrors, codex.ValidationError{
					Reason: fmt.Sprintf("subagent %q: %v", info.FileName, err),
				})
				continue
			}
			r.cachedAgents = append(r.cachedAgents, parsed)
		}
	}
	out := make([]*codex.ParsedRollout, 0, 1+len(r.cachedAgents))
	out = append(out, r.main)
	out = append(out, r.cachedAgents...)
	return out
}

// loadSubagent downloads and parses one subagent file. On success it also
// prefixes each ValidationError with the file name so operators can correlate
// anomalies back to the source file.
func (r *codexRollout) loadSubagent(ctx context.Context, fileName string) (*codex.ParsedRollout, error) {
	raw, err := r.downloader(ctx, fileName)
	if err != nil {
		slog.WarnContext(ctx, "codex subagent download failed", "file", fileName, "error", err)
		return nil, fmt.Errorf("download failed: %w", err)
	}
	parsed, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		slog.WarnContext(ctx, "codex subagent parse failed", "file", fileName, "error", err)
		return nil, fmt.Errorf("parse failed: %w", err)
	}
	for j := range parsed.ValidationErrors {
		parsed.ValidationErrors[j].Reason = fmt.Sprintf("subagent %q: %s", fileName, parsed.ValidationErrors[j].Reason)
	}
	return parsed, nil
}

// loadCodexMainAndListAgents downloads + parses the main transcript and
// lists subagent rollout file names. Returns (nil, nil, nil) when the session
// has no transcript row yet.
func loadCodexMainAndListAgents(ctx context.Context, input ParseInput) (*codex.ParsedRollout, []codexAgentFileInfo, error) {
	rows, err := input.DB.QueryContext(ctx, `
		SELECT file_name, file_type
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent')
		ORDER BY id ASC
	`, input.SessionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var mainFileName string
	var agentFileInfo []codexAgentFileInfo
	for rows.Next() {
		var fileName, fileType string
		if err := rows.Scan(&fileName, &fileType); err != nil {
			return nil, nil, err
		}
		switch fileType {
		case "transcript":
			if mainFileName == "" {
				mainFileName = fileName
			}
		case "agent":
			agentFileInfo = append(agentFileInfo, codexAgentFileInfo{FileName: fileName})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if mainFileName == "" {
		return nil, nil, nil
	}

	raw, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil || raw == nil {
		return nil, nil, err
	}
	main, err := codex.ParseRollout(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("parse codex rollout: %w", err)
	}
	if len(main.ValidationErrors) > 0 {
		slog.WarnContext(ctx, "codex rollout parse warnings",
			"session_id", input.SessionID,
			"validation_errors", len(main.ValidationErrors),
		)
	}
	return main, agentFileInfo, nil
}
