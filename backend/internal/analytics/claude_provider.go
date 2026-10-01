package analytics

import (
	"context"
	"log/slog"

	"github.com/ConfabulousDev/confab-web/internal/models"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

type claudeProvider struct{}

// claudeRollout holds the main transcript plus the deps needed to stream
// agent files on demand. Agents are never memoized: each traversal
// (ComputeCards, PrepareTranscript, SearchText) streams them one at a time
// and drops each after processing, so peak memory is O(main) + O(largest
// agent) rather than O(all agents). A second traversal re-downloads (5m68).
// Single-goroutine per the Rollout contract; no mutex.
type claudeRollout struct {
	main        *TranscriptFile
	agentInfo   []AgentFileInfo
	journalInfo []WorkflowJournalInfo
	downloader  AgentDownloader
}

// WorkflowJournalInfo describes a workflow run journal file to download.
type WorkflowJournalInfo struct {
	RunID    string
	FileName string
}

func init() {
	RegisterProvider(&claudeProvider{}, models.ProviderClaudeCode, models.ProviderClaudeCodeLegacy)
}

func (p *claudeProvider) Parse(ctx context.Context, input ParseInput) (Rollout, error) {
	main, agentInfo, journalInfo, err := downloadClaudeMainAndListAgents(ctx, input)
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
		main:        main,
		agentInfo:   agentInfo,
		journalInfo: journalInfo,
		downloader:  downloader,
	}, nil
}

func (p *claudeProvider) ComputeCards(ctx context.Context, rollout Rollout) *ComputeResult {
	r := rollout.(*claudeRollout)
	computed, err := ComputeStreaming(ctx, r.main, r.agentProvider(), r.buildWorkflowInputs(ctx))
	if err != nil {
		return &ComputeResult{CardErrors: map[string]string{"compute": err.Error()}}
	}
	return computed
}

// buildWorkflowInputs resolves the runId for each agent file (from its path) and
// downloads each run's journal, so the WorkflowsAnalyzer can group agents and
// derive per-agent status. Returns inputs even for non-workflow sessions (empty
// maps), so the workflows card is always written (empty runs → hidden on the FE).
func (r *claudeRollout) buildWorkflowInputs(ctx context.Context) *WorkflowInputs {
	runIDByAgentID := make(map[string]string, len(r.agentInfo))
	for _, ai := range r.agentInfo {
		if runID := ExtractWorkflowRunID(ai.FileName); runID != "" {
			runIDByAgentID[ai.AgentID] = runID
		}
	}

	journals := make(map[string][]byte, len(r.journalInfo))
	for _, ji := range r.journalInfo {
		content, err := r.downloader(ctx, ji.FileName)
		if err != nil || content == nil {
			slog.Warn("failed to download workflow journal", "file", ji.FileName, "error", err)
			continue
		}
		journals[ji.RunID] = content
	}

	return &WorkflowInputs{RunIDByAgentID: runIDByAgentID, Journals: journals}
}

func (p *claudeProvider) SearchText(ctx context.Context, rollout Rollout) string {
	r := rollout.(*claudeRollout)
	var umb UserMessagesBuilder
	umb.ProcessFile(r.main)
	r.forEachAgent(ctx, umb.ProcessFile)
	return umb.Finish()
}

func (p *claudeProvider) PrepareTranscript(ctx context.Context, rollout Rollout) (string, map[int]string, error) {
	r := rollout.(*claudeRollout)
	tb := NewTranscriptBuilder(DefaultFormatConfig())
	tb.ProcessFile(r.main)
	r.forEachAgent(ctx, tb.ProcessFile)
	transcript, idMap := tb.Finish()
	return transcript, idMap, nil
}

func (p *claudeProvider) ClearMessageIDs() bool { return false }
func (p *claudeProvider) DisplayName() string   { return "Claude Code" }

// agentProvider returns a fresh AgentProvider that streams this rollout's
// agent files (capped at storage.MaxAgentFiles) one download+parse at a time.
func (r *claudeRollout) agentProvider() AgentProvider {
	return NewAgentProvider(r.agentInfo, r.downloader, storage.MaxAgentFiles)
}

// forEachAgent streams every agent file through fn, one at a time; the
// rollout keeps no reference to a parsed agent after fn returns.
// NewAgentProvider logs and skips per-file errors, so the loop always reaches
// EOF.
func (r *claudeRollout) forEachAgent(ctx context.Context, fn func(*TranscriptFile)) {
	next := r.agentProvider()
	for {
		agent, err := next(ctx)
		if err != nil {
			return
		}
		fn(agent)
	}
}

func downloadClaudeMainAndListAgents(ctx context.Context, input ParseInput) (*TranscriptFile, []AgentFileInfo, []WorkflowJournalInfo, error) {
	rows, err := input.DB.QueryContext(ctx, `
		SELECT file_name, file_type
		FROM sync_files
		WHERE session_id = $1 AND file_type IN ('transcript', 'agent', 'workflow_journal')
	`, input.SessionID)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()

	var mainFileName string
	var agentInfo []AgentFileInfo
	var journalInfo []WorkflowJournalInfo
	for rows.Next() {
		var fileName, fileType string
		if err := rows.Scan(&fileName, &fileType); err != nil {
			return nil, nil, nil, err
		}
		switch fileType {
		case "transcript":
			mainFileName = fileName
		case "agent":
			agentID := ExtractAgentID(fileName)
			if agentID != "" {
				agentInfo = append(agentInfo, AgentFileInfo{FileName: fileName, AgentID: agentID})
			}
		case "workflow_journal":
			if runID := ExtractWorkflowRunID(fileName); runID != "" {
				journalInfo = append(journalInfo, WorkflowJournalInfo{RunID: runID, FileName: fileName})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	if mainFileName == "" {
		return nil, nil, nil, nil
	}

	mainContent, err := input.Store.DownloadAndMergeChunks(ctx, input.UserID, input.Provider, input.ExternalID, mainFileName)
	if err != nil || mainContent == nil {
		return nil, nil, nil, err
	}
	main, err := parseTranscriptFile(mainContent, "")
	if err != nil {
		return nil, nil, nil, err
	}
	return main, agentInfo, journalInfo, nil
}
