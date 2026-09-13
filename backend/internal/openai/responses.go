package openai

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ResponsesRequest is the request body for POST /v1/responses.
type ResponsesRequest struct {
	Model           string      `json:"model"`
	Instructions    string      `json:"instructions,omitempty"`
	Input           string      `json:"input"`
	MaxOutputTokens int         `json:"max_output_tokens,omitempty"`
	Reasoning       *Reasoning  `json:"reasoning,omitempty"`
	Store           *bool       `json:"store,omitempty"` // pointer so an explicit false is serialized (API default is true)
	Text            *TextConfig `json:"text,omitempty"`
}

// Reasoning configures reasoning effort for reasoning models.
type Reasoning struct {
	Effort string `json:"effort"`
}

// TextConfig configures the text output format.
type TextConfig struct {
	Format TextFormat `json:"format"`
}

// TextFormat selects structured output. For JSON schema output use
// Type "json_schema" with a Name, Strict, and the Schema document.
type TextFormat struct {
	Type   string          `json:"type"`
	Name   string          `json:"name,omitempty"`
	Strict bool            `json:"strict,omitempty"`
	Schema json.RawMessage `json:"schema,omitempty"`
}

// ResponsesResponse is the response body from POST /v1/responses.
type ResponsesResponse struct {
	ID                string             `json:"id"`
	Status            string             `json:"status"` // "completed", "incomplete", ...
	IncompleteDetails *IncompleteDetails `json:"incomplete_details,omitempty"`
	Output            []OutputItem       `json:"output"`
	Usage             Usage              `json:"usage"`
}

// IncompleteDetails explains why a response has status "incomplete".
type IncompleteDetails struct {
	Reason string `json:"reason"`
}

// OutputItem is one item of the response output. Only "message" items carry
// text; other types (e.g. "reasoning") are ignored by the helpers.
type OutputItem struct {
	Type    string        `json:"type"`
	Content []ContentPart `json:"content,omitempty"`
}

// ContentPart is a part of a message item: "output_text" (Text) or "refusal" (Refusal).
type ContentPart struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Refusal string `json:"refusal,omitempty"`
}

// Usage reports token counts. Reasoning tokens are a subset of OutputTokens.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// OutputText concatenates every output_text part of every message item.
func (r *ResponsesResponse) OutputText() string {
	return r.joinParts("output_text", func(p ContentPart) string { return p.Text })
}

// Refusal concatenates every refusal part of every message item.
func (r *ResponsesResponse) Refusal() string {
	return r.joinParts("refusal", func(p ContentPart) string { return p.Refusal })
}

func (r *ResponsesResponse) joinParts(partType string, value func(ContentPart) string) string {
	var sb strings.Builder
	for _, item := range r.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == partType {
				sb.WriteString(value(part))
			}
		}
	}
	return sb.String()
}

// APIError is a structured error response from the OpenAI API.
type APIError struct {
	Detail     ErrorDetail `json:"error"`
	StatusCode int         `json:"-"`
}

// ErrorDetail is the nested error object of an APIError.
type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("openai API error (status %d, type %s, code %s): %s",
		e.StatusCode, e.Detail.Type, e.Detail.Code, e.Detail.Message)
}
