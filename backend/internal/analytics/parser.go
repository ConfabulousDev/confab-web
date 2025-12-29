package analytics

import (
	"encoding/json"
	"fmt"
	"time"
)

// TranscriptLine represents a single line from a Claude Code transcript.
// Fields are parsed on-demand based on line type.
type TranscriptLine struct {
	Type      string `json:"type"`                // "user", "assistant", "system", "summary", etc.
	UUID      string `json:"uuid,omitempty"`      // Unique message identifier
	Timestamp string `json:"timestamp,omitempty"` // ISO 8601 timestamp

	// For assistant messages
	Message *MessageContent `json:"message,omitempty"`

	// For system messages
	Subtype           string           `json:"subtype,omitempty"`           // e.g., "compact_boundary"
	CompactMetadata   *CompactMetadata `json:"compactMetadata,omitempty"`   // Compaction info
	LogicalParentUUID string           `json:"logicalParentUuid,omitempty"` // Parent message UUID
}

// MessageContent contains message details for user/assistant messages.
type MessageContent struct {
	Role    string      `json:"role,omitempty"`    // "user" or "assistant"
	Model   string      `json:"model,omitempty"`   // Model ID (assistant only)
	Usage   *TokenUsage `json:"usage,omitempty"`   // Token usage (assistant only)
	Content interface{} `json:"content,omitempty"` // String or []ContentBlock

	// Assistant-specific fields
	StopReason string `json:"stop_reason,omitempty"` // "end_turn", "tool_use", "max_tokens"
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

// ContentBlock represents a content block in assistant messages.
type ContentBlock struct {
	Type    string `json:"type"`              // "text", "tool_use", "thinking", etc.
	Name    string `json:"name,omitempty"`    // Tool name (for tool_use)
	ID      string `json:"id,omitempty"`      // Tool use ID
	IsError bool   `json:"is_error,omitempty"` // For tool_result blocks
}

// ParseLine parses a single JSONL line for analytics purposes.
func ParseLine(data []byte) (*TranscriptLine, error) {
	var line TranscriptLine
	if err := json.Unmarshal(data, &line); err != nil {
		return nil, err
	}
	return &line, nil
}

// IsUserMessage returns true if this is a user message.
func (l *TranscriptLine) IsUserMessage() bool {
	return l.Type == "user"
}

// IsAssistantMessage returns true if this is an assistant message with usage stats.
func (l *TranscriptLine) IsAssistantMessage() bool {
	return l.Type == "assistant" && l.Message != nil && l.Message.Usage != nil
}

// IsCompactBoundary returns true if this is a system message marking a compaction.
func (l *TranscriptLine) IsCompactBoundary() bool {
	return l.Type == "system" && l.Subtype == "compact_boundary"
}

// ErrNoTimestamp is returned when a line has no timestamp field.
var ErrNoTimestamp = fmt.Errorf("line has no timestamp")

// GetTimestamp parses the timestamp field.
// Returns ErrNoTimestamp if the timestamp field is empty.
func (l *TranscriptLine) GetTimestamp() (time.Time, error) {
	if l.Timestamp == "" {
		return time.Time{}, ErrNoTimestamp
	}
	return time.Parse(time.RFC3339Nano, l.Timestamp)
}

// GetStopReason returns the stop reason for assistant messages.
func (l *TranscriptLine) GetStopReason() string {
	if l.Message == nil {
		return ""
	}
	return l.Message.StopReason
}

// GetModel returns the model ID for assistant messages.
func (l *TranscriptLine) GetModel() string {
	if l.Message == nil {
		return ""
	}
	return l.Message.Model
}

// GetContentBlocks parses and returns content blocks from assistant messages.
// Returns nil if content is not an array of blocks.
func (l *TranscriptLine) GetContentBlocks() []ContentBlock {
	if l.Message == nil || l.Message.Content == nil {
		return nil
	}

	// Content can be a string or array of blocks
	contentArray, ok := l.Message.Content.([]interface{})
	if !ok {
		return nil
	}

	var blocks []ContentBlock
	for _, item := range contentArray {
		blockMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		block := ContentBlock{}
		if t, ok := blockMap["type"].(string); ok {
			block.Type = t
		}
		if n, ok := blockMap["name"].(string); ok {
			block.Name = n
		}
		if id, ok := blockMap["id"].(string); ok {
			block.ID = id
		}
		if isErr, ok := blockMap["is_error"].(bool); ok {
			block.IsError = isErr
		}
		blocks = append(blocks, block)
	}

	return blocks
}

// GetToolUses returns tool_use blocks from the message content.
func (l *TranscriptLine) GetToolUses() []ContentBlock {
	blocks := l.GetContentBlocks()
	var tools []ContentBlock
	for _, b := range blocks {
		if b.Type == "tool_use" {
			tools = append(tools, b)
		}
	}
	return tools
}

// HasThinking returns true if the message contains a thinking block.
func (l *TranscriptLine) HasThinking() bool {
	for _, b := range l.GetContentBlocks() {
		if b.Type == "thinking" {
			return true
		}
	}
	return false
}
