package analytics

import (
	"context"
	"io"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type claudeProvider struct{}

type claudeRollout struct {
	main       *TranscriptFile
	agentInfo  []AgentFileInfo
	downloader AgentDownloader
}

func init() {
	RegisterProvider(&claudeProvider{}, models.ProviderClaudeCode, models.ProviderClaudeCodeLegacy)
}

func (p *claudeProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	main, agentInfo, err := downloadClaudeMainAndListAgents(ctx, input)
	if err != nil {
		return nil, err
	}
	if main == nil {
		return nil, nil
	}
	downloader := func(ctx context.Context, fileName string) ([]byte, error) {
		return input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, fileName)
	}
	return &claudeRollout{
		main:       main,
		agentInfo:  agentInfo,
		downloader: downloader,
	}, nil
}

func (p *claudeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r := rollout.(*claudeRollout)
	computed, err := ComputeStreaming(ctx, r.main, r.newAgentProvider())
	if err != nil {
		return &ComputeResult{CardErrors: map[string]string{"compute": err.Error()}}
	}
	return computed
}

func (p *claudeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r := rollout.(*claudeRollout)
	var umb UserMessagesBuilder
	umb.ProcessFile(r.main)
	drainAgentProvider(ctx, r.newAgentProvider(), func(agent *TranscriptFile) {
		umb.ProcessFile(agent)
	})
	return umb.Finish()
}

func (p *claudeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r := rollout.(*claudeRollout)
	tb := NewTranscriptBuilder(DefaultFormatConfig())
	tb.ProcessFile(r.main)
	drainAgentProvider(ctx, r.newAgentProvider(), func(agent *TranscriptFile) {
		tb.ProcessFile(agent)
	})
	transcript, idMap := tb.Finish()
	return transcript, idMap, nil
}

func (p *claudeProvider) ClearMessageIDs() bool {
	return false
}

func (r *claudeRollout) newAgentProvider() AgentProvider {
	return NewAgentProvider(r.agentInfo, r.downloader, storage.MaxAgentFiles)
}

// drainAgentProvider reads all files from an AgentProvider, calling fn for each.
// Errors from the provider are silently skipped because the provider has already
// logged them.
func drainAgentProvider(ctx context.Context, provider AgentProvider, fn func(*TranscriptFile)) {
	for {
		agent, err := provider(ctx)
		if err == io.EOF {
			return
		}
		if err != nil {
			continue
		}
		fn(agent)
	}
}

// downloadClaudeMainAndListAgents downloads the main transcript and returns
// agent file metadata. Agent files are listed but not downloaded until a
// provider method streams them.
func downloadClaudeMainAndListAgents(ctx context.Context, input ParseInput) (*TranscriptFile, []AgentFileInfo, error) {
	rows, err := input.DB.QueryContext(ctx, `
		SELECT file_name, file_type
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent')
	`, input.SessionID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var mainFileName string
	var agentInfo []AgentFileInfo
	for rows.Next() {
		var fileName, fileType string
		if err := rows.Scan(&fileName, &fileType); err != nil {
			return nil, nil, err
		}
		switch fileType {
		case "transcript":
			mainFileName = fileName
		case "agent":
			agentID := ExtractAgentID(fileName)
			if agentID != "" {
				agentInfo = append(agentInfo, AgentFileInfo{FileName: fileName, AgentID: agentID})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if mainFileName == "" {
		return nil, nil, nil
	}

	mainContent, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil || mainContent == nil {
		return nil, nil, err
	}
	main, err := parseTranscriptFile(mainContent, "")
	if err != nil {
		return nil, nil, err
	}
	return main, agentInfo, nil
}
