package anthropic

import "fmt"

// MessagesRequest represents a request to the Messages API.
type MessagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"`
	Messages    []Message `json:"messages"`
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// GetTextContent extracts all text content from the response.
func (r *MessagesResponse) GetTextContent() string {
	var text string
	for _, block := range r.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
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
