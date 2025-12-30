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
	Type          string                 `json:"type"`                   // "text", "tool_use", "thinking", etc.
	Name          string                 `json:"name,omitempty"`         // Tool name (for tool_use)
	ID            string                 `json:"id,omitempty"`           // Tool use ID (for tool_use)
	Input         map[string]interface{} `json:"input,omitempty"`        // Tool input parameters (for tool_use)
	ToolUseID     string                 `json:"tool_use_id,omitempty"`  // Reference to tool_use ID (for tool_result)
	IsError       bool                   `json:"is_error,omitempty"`     // For tool_result blocks
	ToolUseResult *ToolUseResult         `json:"toolUseResult,omitempty"` // For tool_result blocks (agent results)
}

// ToolUseResult contains metadata from subagent/Task tool executions.
// This is embedded in tool_result content blocks when the tool was a Task/agent.
type ToolUseResult struct {
	AgentID           string      `json:"agentId,omitempty"`           // Subagent identifier (present for agent results)
	Usage             *TokenUsage `json:"usage,omitempty"`             // Cumulative token usage for the agent
	TotalTokens       int64       `json:"totalTokens,omitempty"`       // Total tokens used by the agent
	TotalToolUseCount int         `json:"totalToolUseCount,omitempty"` // Number of tool calls made by the agent
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
		if input, ok := blockMap["input"].(map[string]interface{}); ok {
			block.Input = input
		}
		if toolUseID, ok := blockMap["tool_use_id"].(string); ok {
			block.ToolUseID = toolUseID
		}
		if isErr, ok := blockMap["is_error"].(bool); ok {
			block.IsError = isErr
		}
		// Parse toolUseResult for tool_result blocks (contains agent usage data)
		if tur, ok := blockMap["toolUseResult"].(map[string]interface{}); ok {
			block.ToolUseResult = parseToolUseResult(tur)
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

// IsHumanMessage returns true if this is a user message with human-typed content (not tool_result).
// This distinguishes actual user input from tool result messages which are also type "user".
func (l *TranscriptLine) IsHumanMessage() bool {
	if l.Type != "user" {
		return false
	}
	// Check if content is a string (human input) vs array (tool_result)
	if l.Message == nil || l.Message.Content == nil {
		return false
	}
	_, isString := l.Message.Content.(string)
	return isString
}

// IsToolResultMessage returns true if this is a user message containing tool_result blocks.
func (l *TranscriptLine) IsToolResultMessage() bool {
	if l.Type != "user" {
		return false
	}
	// If content is an array, it's tool results
	if l.Message == nil || l.Message.Content == nil {
		return false
	}
	_, isArray := l.Message.Content.([]interface{})
	return isArray
}

// HasTextContent returns true if the message contains text content.
// For assistant messages, this checks for "text" type blocks or string content.
func (l *TranscriptLine) HasTextContent() bool {
	if l.Message == nil || l.Message.Content == nil {
		return false
	}
	// String content is text
	if _, isString := l.Message.Content.(string); isString {
		return true
	}
	// Check for text blocks in array content
	for _, b := range l.GetContentBlocks() {
		if b.Type == "text" {
			return true
		}
	}
	return false
}

// HasToolUse returns true if the message contains tool_use blocks.
func (l *TranscriptLine) HasToolUse() bool {
	for _, b := range l.GetContentBlocks() {
		if b.Type == "tool_use" {
			return true
		}
	}
	return false
}

// GetAgentUsage returns token usage from subagent/Task tool results.
// This extracts usage data from toolUseResult blocks that have agentId.
// Returns nil if no agent usage is found.
func (l *TranscriptLine) GetAgentUsage() []*TokenUsage {
	var usages []*TokenUsage
	for _, result := range l.GetAgentResults() {
		if result.Usage != nil {
			usages = append(usages, result.Usage)
		}
	}
	return usages
}

// GetAgentResults returns all ToolUseResult from subagent/Task tool results.
// This extracts full agent result metadata including tool counts.
// Returns nil if no agent results are found.
func (l *TranscriptLine) GetAgentResults() []*ToolUseResult {
	if !l.IsToolResultMessage() {
		return nil
	}

	var results []*ToolUseResult
	for _, block := range l.GetContentBlocks() {
		if block.Type == "tool_result" && block.ToolUseResult != nil {
			// Only include results that have agentId (subagent results)
			if block.ToolUseResult.AgentID != "" {
				results = append(results, block.ToolUseResult)
			}
		}
	}
	return results
}

// parseToolUseResult extracts ToolUseResult from a map.
func parseToolUseResult(m map[string]interface{}) *ToolUseResult {
	result := &ToolUseResult{}

	if agentID, ok := m["agentId"].(string); ok {
		result.AgentID = agentID
	}
	if totalTokens, ok := m["totalTokens"].(float64); ok {
		result.TotalTokens = int64(totalTokens)
	}
	if totalToolUseCount, ok := m["totalToolUseCount"].(float64); ok {
		result.TotalToolUseCount = int(totalToolUseCount)
	}

	// Parse usage sub-object
	if usageMap, ok := m["usage"].(map[string]interface{}); ok {
		result.Usage = &TokenUsage{}
		if v, ok := usageMap["input_tokens"].(float64); ok {
			result.Usage.InputTokens = int64(v)
		}
		if v, ok := usageMap["output_tokens"].(float64); ok {
			result.Usage.OutputTokens = int64(v)
		}
		if v, ok := usageMap["cache_creation_input_tokens"].(float64); ok {
			result.Usage.CacheCreationInputTokens = int64(v)
		}
		if v, ok := usageMap["cache_read_input_tokens"].(float64); ok {
			result.Usage.CacheReadInputTokens = int64(v)
		}
	}

	return result
}
