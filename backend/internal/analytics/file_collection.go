package analytics

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"
)

// TranscriptFile represents a parsed transcript (main or agent).
type TranscriptFile struct {
	Lines            []*TranscriptLine
	AgentID          string // Empty for main transcript, set for agent files
	ValidationErrors []LineValidationError
	TotalLines       int // Total lines processed (including invalid ones)

	// Cached result of AssistantMessageGroups (computed on first call)
	cachedGroups    []AssistantMessageGroup
	groupsComputed  bool
}

// FileCollection contains all transcript data for a session.
// This includes the main transcript and any agent transcripts.
type FileCollection struct {
	Main   *TranscriptFile
	Agents []*TranscriptFile
}

// NewFileCollection creates a FileCollection from raw JSONL content.
// This is a convenience wrapper for sessions without agent files.
func NewFileCollection(mainContent []byte) (*FileCollection, error) {
	return NewFileCollectionWithAgents(mainContent, nil)
}

// NewFileCollectionWithAgents creates a FileCollection from main + agent content.
// agentContents maps agentID (extracted from filename) -> JSONL bytes.
// Missing or empty agent content is skipped gracefully.
func NewFileCollectionWithAgents(mainContent []byte, agentContents map[string][]byte) (*FileCollection, error) {
	main, err := parseTranscriptFile(mainContent, "")
	if err != nil {
		return nil, err
	}

	var agents []*TranscriptFile
	for agentID, content := range agentContents {
		if len(content) == 0 {
			continue
		}
		agent, err := parseTranscriptFile(content, agentID)
		if err != nil {
			// Skip unparseable agent files, continue with others
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

// TotalLineCount returns the sum of lines across all files (main + agents).
// Used for cache invalidation when agent files are present.
func (fc *FileCollection) TotalLineCount() int64 {
	total := int64(len(fc.Main.Lines))
	for _, agent := range fc.Agents {
		total += int64(len(agent.Lines))
	}
	return total
}

// HasAgentFile returns true if we have an agent file with the given ID.
// Used to avoid double-counting when toolUseResult data is also present.
func (fc *FileCollection) HasAgentFile(agentID string) bool {
	for _, agent := range fc.Agents {
		if agent.AgentID == agentID {
			return true
		}
	}
	return false
}

// AgentCount returns the number of agent files in the collection.
func (fc *FileCollection) AgentCount() int {
	return len(fc.Agents)
}

// ValidationErrorCount returns the total number of validation errors across all files.
func (fc *FileCollection) ValidationErrorCount() int {
	count := len(fc.Main.ValidationErrors)
	for _, agent := range fc.Agents {
		count += len(agent.ValidationErrors)
	}
	return count
}

// parseTranscriptFile parses raw JSONL content into a TranscriptFile.
// It validates each line and collects validation errors.
func parseTranscriptFile(content []byte, agentID string) (*TranscriptFile, error) {
	var lines []*TranscriptLine
	var validationErrors []LineValidationError
	lineNumber := 0

	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer size for large lines (some assistant messages can be huge)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		lineNumber++
		lineData := scanner.Bytes()
		if len(bytes.TrimSpace(lineData)) == 0 {
			continue
		}

		// First, parse into a raw map for validation
		var rawMap map[string]interface{}
		if err := json.Unmarshal(lineData, &rawMap); err != nil {
			// JSON parse error - add as validation error
			validationErrors = append(validationErrors, LineValidationError{
				Line:    lineNumber,
				RawJSON: truncateJSON(string(lineData), 200),
				Errors: []ValidationError{{
					Path:    "(root)",
					Message: "invalid JSON: " + err.Error(),
				}},
			})
			continue
		}

		// Validate the line against schema
		errors := ValidateLine(rawMap)
		if len(errors) > 0 {
			msgType, _ := rawMap["type"].(string)
			validationErrors = append(validationErrors, LineValidationError{
				Line:        lineNumber,
				RawJSON:     truncateJSON(string(lineData), 200),
				MessageType: msgType,
				Errors:      errors,
			})
			continue
		}

		// Validation passed - parse into TranscriptLine
		line, err := ParseLine(lineData)
		if err != nil {
			// This shouldn't happen if validation passed, but handle it
			validationErrors = append(validationErrors, LineValidationError{
				Line:    lineNumber,
				RawJSON: truncateJSON(string(lineData), 200),
				Errors: []ValidationError{{
					Path:    "(root)",
					Message: "parse error after validation: " + err.Error(),
				}},
			})
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &TranscriptFile{
		Lines:            lines,
		AgentID:          agentID,
		ValidationErrors: validationErrors,
		TotalLines:       lineNumber,
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

// AssistantMessageGroup represents a deduplicated API response.
// Multiple JSONL lines can share the same message.id (one per content block),
// and the same message.id can reappear later via context replay.
// This struct merges all occurrences into a single logical response.
type AssistantMessageGroup struct {
	MessageID  string      // API message ID (empty for lines without one)
	FinalUsage *TokenUsage // Usage from the last occurrence (final output_tokens)
	Model      string      // Model from the first occurrence
	HasText    bool        // True if ANY line in the group has text content
	HasToolUse bool        // True if ANY line in the group has tool_use
	HasThinking bool       // True if ANY line in the group has thinking
	IsFastMode bool        // True if any line has speed="fast"
}

// AssistantMessageGroups groups assistant lines by message.id and returns
// deduplicated groups. Each group's FinalUsage comes from the last occurrence
// of the message ID, and Model comes from the first occurrence.
// Lines without a message ID get their own individual group.
// Groups are returned in order of first occurrence.
// The result is cached after the first call.
func (tf *TranscriptFile) AssistantMessageGroups() []AssistantMessageGroup {
	if tf.groupsComputed {
		return tf.cachedGroups
	}

	// Map message ID → index in the result slice (for O(1) merges)
	idToIndex := make(map[string]int)
	var groups []AssistantMessageGroup

	for _, line := range tf.Lines {
		if line.Type != "assistant" || line.Message == nil {
			continue
		}

		msgID := line.GetMessageID()

		hasText := line.HasTextContent()
		hasToolUse := line.HasToolUse()
		hasThinking := line.HasThinking()
		isFast := line.Message.Usage != nil && line.Message.Usage.Speed == SpeedFast

		if msgID == "" {
			// No message ID — create standalone group in order
			g := AssistantMessageGroup{
				Model:       line.GetModel(),
				HasText:     hasText,
				HasToolUse:  hasToolUse,
				HasThinking: hasThinking,
				IsFastMode:  isFast,
			}
			if line.Message.Usage != nil {
				usage := *line.Message.Usage
				g.FinalUsage = &usage
			}
			groups = append(groups, g)
			continue
		}

		if idx, ok := idToIndex[msgID]; ok {
			// Subsequent occurrence — merge flags, update usage (last wins)
			groups[idx].HasText = groups[idx].HasText || hasText
			groups[idx].HasToolUse = groups[idx].HasToolUse || hasToolUse
			groups[idx].HasThinking = groups[idx].HasThinking || hasThinking
			groups[idx].IsFastMode = groups[idx].IsFastMode || isFast
			if line.Message.Usage != nil {
				usage := *line.Message.Usage
				groups[idx].FinalUsage = &usage
			}
		} else {
			// First occurrence — append to result, record index
			g := AssistantMessageGroup{
				MessageID:   msgID,
				Model:       line.GetModel(),
				HasText:     hasText,
				HasToolUse:  hasToolUse,
				HasThinking: hasThinking,
				IsFastMode:  isFast,
			}
			if line.Message.Usage != nil {
				usage := *line.Message.Usage
				g.FinalUsage = &usage
			}
			idToIndex[msgID] = len(groups)
			groups = append(groups, g)
		}
	}

	tf.cachedGroups = groups
	tf.groupsComputed = true
	return groups
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
