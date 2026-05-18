package api

import (
	"context"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/db"
	"github.com/ConfabulousDev/confab-web/internal/storage"
)

// Claude-style transcript assembly helpers used only by the
// condensed-transcript endpoint (external.go). The analytics path dispatches
// through the SessionProvider registry, which does its own file discovery.

type classifiedFiles struct {
	transcript *db.SyncFileDetail
	agents     []db.SyncFileDetail
	lineCount  int64
}

// classifySessionFiles separates session files into transcript + agent and
// totals the line count. Returns nil if no transcript file exists.
func classifySessionFiles(files []db.SyncFileDetail) *classifiedFiles {
	var result classifiedFiles
	for i := range files {
		switch files[i].FileType {
		case "transcript":
			result.transcript = &files[i]
		case "agent":
			result.agents = append(result.agents, files[i])
		}
	}
	if result.transcript == nil {
		return nil
	}
	result.lineCount = int64(result.transcript.LastSyncedLine)
	for _, af := range result.agents {
		result.lineCount += int64(af.LastSyncedLine)
	}
	return &result
}

// downloadMainFromFiles downloads and parses the main transcript. Returns
// (nil, nil) if the content is empty.
func downloadMainFromFiles(
	ctx context.Context,
	store *storage.S3Storage,
	files *classifiedFiles,
	sessionUserID int64,
	sessionProvider string,
	externalID string,
) (*analytics.TranscriptFile, error) {
	storageCtx, storageCancel := context.WithTimeout(ctx, StorageTimeout)
	defer storageCancel()

	mainContent, err := store.DownloadAndMergeChunks(storageCtx, sessionUserID, sessionProvider, externalID, files.transcript.FileName)
	if err != nil {
		return nil, err
	}
	if mainContent == nil {
		return nil, nil
	}

	fc, err := analytics.NewFileCollection(mainContent)
	if err != nil {
		return nil, err
	}
	return fc.Main, nil
}

// agentInfosFromFiles extracts AgentFileInfo descriptors, dropping files
// whose names don't yield an agent id.
func agentInfosFromFiles(files *classifiedFiles) []analytics.AgentFileInfo {
	infos := make([]analytics.AgentFileInfo, 0, len(files.agents))
	for _, af := range files.agents {
		agentID := analytics.ExtractAgentID(af.FileName)
		if agentID != "" {
			infos = append(infos, analytics.AgentFileInfo{FileName: af.FileName, AgentID: agentID})
		}
	}
	return infos
}

func newAPIAgentDownloader(store *storage.S3Storage, sessionUserID int64, sessionProvider string, externalID string) analytics.AgentDownloader {
	return func(ctx context.Context, fileName string) ([]byte, error) {
		return store.DownloadAndMergeChunks(ctx, sessionUserID, sessionProvider, externalID, fileName)
	}
}
