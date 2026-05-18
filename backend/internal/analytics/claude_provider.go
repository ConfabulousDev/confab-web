package analytics

import (
	"context"
	"io"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type claudeProvider struct{}

// claudeRollout holds the main transcript plus the deps needed to stream
// agent files on demand. cachedAgents memoizes parsed agent files after the
// first traversal so subsequent provider methods reuse them without a second
// S3 download. Single-goroutine per the Rollout contract; no mutex.
type claudeRollout struct {
	main         *TranscriptFile
	agentInfo    []AgentFileInfo
	downloader   AgentDownloader
	cachedAgents []*TranscriptFile
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
	computed, err := ComputeStreaming(ctx, r.main, r.agentProvider(ctx))
	if err != nil {
		return &ComputeResult{CardErrors: map[string]string{"compute": err.Error()}}
	}
	return computed
}

func (p *claudeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r := rollout.(*claudeRollout)
	var umb UserMessagesBuilder
	umb.ProcessFile(r.main)
	for _, agent := range r.materializeAgents(ctx) {
		umb.ProcessFile(agent)
	}
	return umb.Finish()
}

func (p *claudeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r := rollout.(*claudeRollout)
	tb := NewTranscriptBuilder(DefaultFormatConfig())
	tb.ProcessFile(r.main)
	for _, agent := range r.materializeAgents(ctx) {
		tb.ProcessFile(agent)
	}
	transcript, idMap := tb.Finish()
	return transcript, idMap, nil
}

func (p *claudeProvider) ClearMessageIDs() bool { return false }
func (p *claudeProvider) DisplayName() string   { return "Claude Code" }

// agentProvider returns an AgentProvider that streams agent files and caches
// each yielded TranscriptFile on r.cachedAgents. After EOF the cache is fully
// populated; later calls replay from the cache without touching the
// downloader.
func (r *claudeRollout) agentProvider(ctx context.Context) AgentProvider {
	if r.cachedAgents != nil {
		idx := 0
		return func(_ context.Context) (*TranscriptFile, error) {
			if idx >= len(r.cachedAgents) {
				return nil, io.EOF
			}
			tf := r.cachedAgents[idx]
			idx++
			return tf, nil
		}
	}
	base := NewAgentProvider(r.agentInfo, r.downloader, storage.MaxAgentFiles)
	collected := make([]*TranscriptFile, 0, len(r.agentInfo))
	return func(ctx context.Context) (*TranscriptFile, error) {
		tf, err := base(ctx)
		if err != nil {
			if err == io.EOF {
				r.cachedAgents = collected
			}
			return tf, err
		}
		collected = append(collected, tf)
		return tf, nil
	}
}

// materializeAgents drains agentProvider once, returning the full parsed
// agent set (and priming the cache). NewAgentProvider already logs and
// skips per-file errors, so the drain always reaches EOF.
func (r *claudeRollout) materializeAgents(ctx context.Context) []*TranscriptFile {
	ap := r.agentProvider(ctx)
	for {
		if _, err := ap(ctx); err != nil {
			break
		}
	}
	return r.cachedAgents
}

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
