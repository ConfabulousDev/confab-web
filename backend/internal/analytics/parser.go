package analytics

import (
	"encoding/json"
	"time"
)

// TranscriptLine is a minimal representation of a JSONL line.
// We only parse fields needed for analytics computation.
type TranscriptLine struct {
	Type string `json:"type"`

	// For assistant messages
	Message *AssistantMessageContent `json:"message,omitempty"`

	// For system messages
	Subtype          string           `json:"subtype,omitempty"`
	CompactMetadata  *CompactMetadata `json:"compactMetadata,omitempty"`
	LogicalParentUUID string          `json:"logicalParentUuid,omitempty"`
	Timestamp        string           `json:"timestamp,omitempty"`
	UUID             string           `json:"uuid,omitempty"`
}

// AssistantMessageContent contains the assistant message details.
type AssistantMessageContent struct {
	Model string      `json:"model"`
	Usage *TokenUsage `json:"usage,omitempty"`
}

// TokenUsage contains token counts from the API response.
type TokenUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens,omitempty"`
}

// CompactMetadata contains compaction trigger information.
type CompactMetadata struct {
	Trigger   string `json:"trigger"` // "auto" or "manual"
	PreTokens int64  `json:"preTokens"`
}

// ParseLine parses a single JSONL line for analytics purposes.
// Returns nil if the line is not relevant for analytics (e.g., user messages, summaries).
func ParseLine(data []byte) (*TranscriptLine, error) {
	var line TranscriptLine
	if err := json.Unmarshal(data, &line); err != nil {
		return nil, err
	}
	return &line, nil
}

// IsAssistantMessage returns true if this is an assistant message with usage stats.
func (l *TranscriptLine) IsAssistantMessage() bool {
	return l.Type == "assistant" && l.Message != nil && l.Message.Usage != nil
}

// IsCompactBoundary returns true if this is a system message marking a compaction.
func (l *TranscriptLine) IsCompactBoundary() bool {
	return l.Type == "system" && l.Subtype == "compact_boundary"
}

// GetTimestamp parses the timestamp field.
func (l *TranscriptLine) GetTimestamp() (time.Time, error) {
	if l.Timestamp == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, l.Timestamp)
}
