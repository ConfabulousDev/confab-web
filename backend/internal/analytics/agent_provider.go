package analytics

import (
	"context"
	"io"
	"log/slog"
)

// AgentFileInfo describes an agent file to download.
type AgentFileInfo struct {
	FileName string
	AgentID  string
}

// AgentDownloader downloads raw JSONL content for an agent file.
// This interface decouples the analytics package from the storage package.
type AgentDownloader func(ctx context.Context, fileName string) ([]byte, error)

// NewAgentProvider creates an AgentProvider that downloads and parses agent files one at a time.
// Files are processed sequentially; each downloaded file is parsed into a TranscriptFile,
// then the raw bytes are discarded before the next download.
//
// maxAgents caps the number of agents processed (0 = unlimited).
// Files that fail to download or parse are skipped with a warning log.
func NewAgentProvider(agents []AgentFileInfo, download AgentDownloader, maxAgents int) AgentProvider {
	idx := 0
	processed := 0

	return func(ctx context.Context) (*TranscriptFile, error) {
		for idx < len(agents) {
			if maxAgents > 0 && processed >= maxAgents {
				slog.Warn("agent file cap reached, skipping remaining agents",
					"cap", maxAgents,
					"remaining", len(agents)-idx,
				)
				return nil, io.EOF
			}

			agent := agents[idx]
			idx++

			content, err := download(ctx, agent.FileName)
			if err != nil {
				slog.Warn("failed to download agent file, skipping",
					"file", agent.FileName,
					"error", err,
				)
				continue
			}
			if len(content) == 0 {
				continue
			}

			tf, err := parseTranscriptFile(content, agent.AgentID)
			if err != nil {
				slog.Warn("failed to parse agent file, skipping",
					"file", agent.FileName,
					"error", err,
				)
				continue
			}

			processed++
			return tf, nil
		}

		return nil, io.EOF
	}
}
