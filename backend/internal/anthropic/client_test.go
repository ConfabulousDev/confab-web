package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateMessage(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify headers
			if r.Header.Get("X-API-Key") != "test-key" {
				t.Errorf("expected X-API-Key header to be test-key, got %s", r.Header.Get("X-API-Key"))
			}
			if r.Header.Get("anthropic-version") != apiVersion {
				t.Errorf("expected anthropic-version header to be %s, got %s", apiVersion, r.Header.Get("anthropic-version"))
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type header to be application/json, got %s", r.Header.Get("Content-Type"))
			}

			// Verify request body
			var req MessagesRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if req.Model != "claude-haiku-4-5-20251101" {
				t.Errorf("expected model to be claude-haiku-4-5-20251101, got %s", req.Model)
			}
			if req.MaxTokens != 1400 {
				t.Errorf("expected max_tokens to be 1400, got %d", req.MaxTokens)
			}

			// Return mock response
			resp := MessagesResponse{
				ID:   "msg_123",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: `{"recap":"test recap"}`},
				},
				Model:      "claude-haiku-4-5-20251101",
				StopReason: "end_turn",
				Usage: Usage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		resp, err := client.CreateMessage(context.Background(), &MessagesRequest{
			Model:     "claude-haiku-4-5-20251101",
			MaxTokens: 1400,
			System:    "You are a helpful assistant.",
			Messages: []Message{
				{Role: "user", Content: "Analyze this session"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if resp.ID != "msg_123" {
			t.Errorf("expected ID to be msg_123, got %s", resp.ID)
		}
		if resp.Usage.InputTokens != 100 {
			t.Errorf("expected InputTokens to be 100, got %d", resp.Usage.InputTokens)
		}
		if resp.Usage.OutputTokens != 50 {
			t.Errorf("expected OutputTokens to be 50, got %d", resp.Usage.OutputTokens)
		}
		if resp.GetTextContent() != `{"recap":"test recap"}` {
			t.Errorf("unexpected text content: %s", resp.GetTextContent())
		}
	})

	t.Run("API error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			resp := APIError{
				Type: "error",
				ErrorDetail: ErrorDetails{
					Type:    "invalid_request_error",
					Message: "Invalid model specified",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		_, err := client.CreateMessage(context.Background(), &MessagesRequest{
			Model:     "invalid-model",
			MaxTokens: 100,
			Messages: []Message{
				{Role: "user", Content: "test"},
			},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("expected status code 400, got %d", apiErr.StatusCode)
		}
		if apiErr.ErrorDetail.Type != "invalid_request_error" {
			t.Errorf("expected error type invalid_request_error, got %s", apiErr.ErrorDetail.Type)
		}
	})

	t.Run("rate limit error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
			resp := APIError{
				Type: "error",
				ErrorDetail: ErrorDetails{
					Type:    "rate_limit_error",
					Message: "Rate limit exceeded",
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		_, err := client.CreateMessage(context.Background(), &MessagesRequest{
			Model:     "claude-haiku-4-5-20251101",
			MaxTokens: 100,
			Messages: []Message{
				{Role: "user", Content: "test"},
			},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected *APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusTooManyRequests {
			t.Errorf("expected status code 429, got %d", apiErr.StatusCode)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClient("test-key", WithBaseURL(server.URL))

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.CreateMessage(ctx, &MessagesRequest{
			Model:     "claude-haiku-4-5-20251101",
			MaxTokens: 100,
			Messages: []Message{
				{Role: "user", Content: "test"},
			},
		})
		if err == nil {
			t.Fatal("expected error due to cancelled context, got nil")
		}
	})
}

func TestClientOptions(t *testing.T) {
	t.Run("WithTimeout", func(t *testing.T) {
		client := NewClient("test-key", WithTimeout(5*time.Second))
		if client.httpClient.Timeout != 5*time.Second {
			t.Errorf("expected timeout to be 5s, got %v", client.httpClient.Timeout)
		}
	})

	t.Run("WithBaseURL", func(t *testing.T) {
		client := NewClient("test-key", WithBaseURL("https://custom.api.com"))
		if client.baseURL != "https://custom.api.com" {
			t.Errorf("expected baseURL to be https://custom.api.com, got %s", client.baseURL)
		}
	})
}

func TestMessagesResponse_GetTextContent(t *testing.T) {
	resp := MessagesResponse{
		Content: []ContentBlock{
			{Type: "text", Text: "Hello "},
			{Type: "text", Text: "world!"},
		},
	}
	if resp.GetTextContent() != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %s", resp.GetTextContent())
	}
}
