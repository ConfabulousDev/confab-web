// Package anthropic provides a client for the Anthropic Messages API.
package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("confab/anthropic")

const (
	defaultBaseURL = "https://api.anthropic.com"
	apiVersion     = "2023-06-01"
)

// Client is an HTTP client for the Anthropic API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// NewClient creates a new Anthropic API client.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func spanError(span trace.Span, err error) error {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return err
}

// CreateMessage sends a message to the Anthropic API and returns the response.
func (c *Client) CreateMessage(ctx context.Context, req *MessagesRequest) (*MessagesResponse, error) {
	ctx, span := tracer.Start(ctx, "anthropic.create_message",
		trace.WithAttributes(
			attribute.String("llm.model", req.Model),
			attribute.Int("llm.max_tokens", req.MaxTokens),
		))
	defer span.End()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, spanError(span, fmt.Errorf("failed to marshal request: %w", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, spanError(span, fmt.Errorf("failed to create request: %w", err))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, spanError(span, fmt.Errorf("failed to send request: %w", err))
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, spanError(span, fmt.Errorf("failed to read response: %w", err))
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(respBody, &apiErr); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("API error (status %d)", resp.StatusCode))
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, spanError(span, &apiErr)
	}

	var messagesResp MessagesResponse
	if err := json.Unmarshal(respBody, &messagesResp); err != nil {
		return nil, spanError(span, fmt.Errorf("failed to unmarshal response: %w", err))
	}

	span.SetAttributes(
		attribute.Int("llm.tokens.input", messagesResp.Usage.InputTokens),
		attribute.Int("llm.tokens.output", messagesResp.Usage.OutputTokens),
		attribute.Int("llm.tokens.cache_creation", messagesResp.Usage.CacheCreationInputTokens),
		attribute.Int("llm.tokens.cache_read", messagesResp.Usage.CacheReadInputTokens),
	)

	return &messagesResp, nil
}
