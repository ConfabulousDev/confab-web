package analytics

import (
	"bufio"
	"bytes"
	"time"
)

// TranscriptFile represents a parsed transcript (main or agent).
type TranscriptFile struct {
	Lines   []*TranscriptLine
	AgentID string // Empty for main transcript, set for agent files
}

// FileCollection contains all transcript data for a session.
// This includes the main transcript and any agent transcripts.
type FileCollection struct {
	Main   *TranscriptFile
	Agents []*TranscriptFile
}

// NewFileCollection creates a FileCollection from raw JSONL content.
// For now, agents is empty - this maintains existing functionality.
func NewFileCollection(mainContent []byte) (*FileCollection, error) {
	main, err := parseTranscriptFile(mainContent, "")
	if err != nil {
		return nil, err
	}

	return &FileCollection{
		Main:   main,
		Agents: nil,
	}, nil
}

// NewFileCollectionWithAgents creates a FileCollection with agent files.
// agentContents maps agentID to raw JSONL content.
func NewFileCollectionWithAgents(mainContent []byte, agentContents map[string][]byte) (*FileCollection, error) {
	main, err := parseTranscriptFile(mainContent, "")
	if err != nil {
		return nil, err
	}

	var agents []*TranscriptFile
	for agentID, content := range agentContents {
		agent, err := parseTranscriptFile(content, agentID)
		if err != nil {
			// Skip unparseable agent files rather than failing entirely
			continue
		}
		agents = append(agents, agent)
	}

	return &FileCollection{
		Main:   main,
		Agents: agents,
	}, nil
}

// AllFiles returns all transcript files (main + agents) for iteration.
func (fc *FileCollection) AllFiles() []*TranscriptFile {
	all := make([]*TranscriptFile, 0, 1+len(fc.Agents))
	all = append(all, fc.Main)
	all = append(all, fc.Agents...)
	return all
}

// MainLineCount returns the number of lines in the main transcript.
// Used for cache invalidation.
func (fc *FileCollection) MainLineCount() int64 {
	return int64(len(fc.Main.Lines))
}

// parseTranscriptFile parses raw JSONL content into a TranscriptFile.
func parseTranscriptFile(content []byte, agentID string) (*TranscriptFile, error) {
	var lines []*TranscriptLine

	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer size for large lines (some assistant messages can be huge)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		lineData := scanner.Bytes()
		if len(bytes.TrimSpace(lineData)) == 0 {
			continue
		}

		line, err := ParseLine(lineData)
		if err != nil {
			// Skip unparseable lines
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &TranscriptFile{
		Lines:   lines,
		AgentID: agentID,
	}, nil
}

// BuildTimestampMap builds a map of UUID -> timestamp for a transcript file.
// Useful for analyzers that need to reference timestamps across messages.
func (tf *TranscriptFile) BuildTimestampMap() map[string]time.Time {
	m := make(map[string]time.Time)
	for _, line := range tf.Lines {
		if line.UUID != "" {
			if ts, err := line.GetTimestamp(); err == nil {
				m[line.UUID] = ts
			}
		}
	}
	return m
}

// BuildToolUseIDToNameMap builds a map of tool_use ID -> tool name.
// Useful for attributing tool_result errors to specific tools.
func (tf *TranscriptFile) BuildToolUseIDToNameMap() map[string]string {
	m := make(map[string]string)
	for _, line := range tf.Lines {
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if tool.ID != "" && tool.Name != "" {
					m[tool.ID] = tool.Name
				}
			}
		}
	}
	return m
}
