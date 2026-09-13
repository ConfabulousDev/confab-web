package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MessagesRequest represents a request to the Messages API.
type MessagesRequest struct {
	Model        string          `json:"model"`
	MaxTokens    int             `json:"max_tokens"`
	Temperature  *float64        `json:"temperature,omitempty"`
	System       string          `json:"system,omitempty"`
	Messages     []Message       `json:"messages"`
	Thinking     *ThinkingConfig `json:"thinking,omitempty"`
	OutputConfig *OutputConfig   `json:"output_config,omitempty"`
}

// ThinkingConfig controls extended thinking (e.g. Type "disabled").
type ThinkingConfig struct {
	Type string `json:"type"`
}

// OutputConfig shapes the model's output (structured outputs).
type OutputConfig struct {
	Format *OutputFormat `json:"format,omitempty"`
}

// OutputFormat constrains the response, e.g. Type "json_schema" with a JSON Schema.
type OutputFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

// Message represents a conversation message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MessagesResponse represents a response from the Messages API.
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence"`
	Usage        Usage          `json:"usage"`
}

// ContentBlock represents a content block in the response.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Usage represents token usage information.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// GetTextContent extracts all text content from the response.
func (r *MessagesResponse) GetTextContent() string {
	var text strings.Builder
	for _, block := range r.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

// APIError represents an error response from the Anthropic API.
type APIError struct {
	Type        string       `json:"type"`
	ErrorDetail ErrorDetails `json:"error"`
	StatusCode  int          `json:"-"`
}

// ErrorDetails contains the error details.
type ErrorDetails struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("anthropic API error (status %d, type %s): %s", e.StatusCode, e.ErrorDetail.Type, e.ErrorDetail.Message)
}
