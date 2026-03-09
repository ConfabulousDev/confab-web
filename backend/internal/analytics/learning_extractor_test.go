// ABOUTME: Tests for the LearningExtractor that analyzes transcripts for reusable insights.
// ABOUTME: Uses httptest mock servers to simulate Anthropic API responses.
package analytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
	"github.com/ConfabulousDev/confab-web/internal/anthropic"
)

// mockLearningResponse returns a valid Anthropic API response with learning candidates.
func mockLearningResponse() anthropic.MessagesResponse {
	return anthropic.MessagesResponse{
		ID:         "msg_test",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{{
			Type: "text",
			Text: `[{"title":"Test Learning","body":"This is a test learning body.","tags":["test","example"]}]`,
		}},
		Usage: anthropic.Usage{InputTokens: 100, OutputTokens: 50},
	}
}

// mockEmptyLearningResponse returns a valid response with no learnings found.
func mockEmptyLearningResponse() anthropic.MessagesResponse {
	return anthropic.MessagesResponse{
		ID:         "msg_test_empty",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{{
			Type: "text",
			Text: `[]`,
		}},
		Usage: anthropic.Usage{InputTokens: 80, OutputTokens: 5},
	}
}

// mockMalformedLearningResponse returns a response with invalid JSON.
func mockMalformedLearningResponse() anthropic.MessagesResponse {
	return anthropic.MessagesResponse{
		ID:         "msg_test_bad",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{{
			Type: "text",
			Text: `this is not valid json at all`,
		}},
		Usage: anthropic.Usage{InputTokens: 80, OutputTokens: 10},
	}
}

// newMockLearningServer creates an HTTP test server that returns the given response.
func newMockLearningServer(t *testing.T, resp anthropic.MessagesResponse) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
}

func TestLearningExtractor_SuccessfulExtraction(t *testing.T) {
	server := newMockLearningServer(t, mockLearningResponse())
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "Some transcript content", "session-1", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Title != "Test Learning" {
		t.Errorf("title = %q, want %q", c.Title, "Test Learning")
	}
	if c.Body != "This is a test learning body." {
		t.Errorf("body = %q, want %q", c.Body, "This is a test learning body.")
	}
	if len(c.Tags) != 2 || c.Tags[0] != "test" || c.Tags[1] != "example" {
		t.Errorf("tags = %v, want [test example]", c.Tags)
	}
}

func TestLearningExtractor_EmptyArrayResponse(t *testing.T) {
	server := newMockLearningServer(t, mockEmptyLearningResponse())
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "Simple transcript", "session-2", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestLearningExtractor_MalformedJSONResponse(t *testing.T) {
	server := newMockLearningServer(t, mockMalformedLearningResponse())
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "Some transcript", "session-3", 42)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if candidates != nil {
		t.Errorf("expected nil candidates on error, got %v", candidates)
	}
}

func TestLearningExtractor_EmptyTranscript(t *testing.T) {
	// No server needed — should fail before making any HTTP call
	client := anthropic.NewClient("test-key")
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "", "session-4", 42)
	if err == nil {
		t.Fatal("expected error for empty transcript, got nil")
	}
	if candidates != nil {
		t.Errorf("expected nil candidates on error, got %v", candidates)
	}
}

func TestLearningExtractor_TranscriptTruncation(t *testing.T) {
	maxChars := 100
	// Build a transcript longer than maxChars
	longTranscript := strings.Repeat("x", maxChars+500)

	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the request body to verify truncation
		var req anthropic.MessagesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			return
		}
		if len(req.Messages) > 0 {
			receivedBody = req.Messages[0].Content
		}
		resp := mockEmptyLearningResponse()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{
		MaxTranscriptChars: maxChars,
	})

	_, err := extractor.Extract(context.Background(), longTranscript, "session-5", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the transcript was truncated
	if !strings.Contains(receivedBody, "[Transcript truncated due to length]") {
		t.Error("expected truncation marker in sent transcript")
	}
	// The sent content should be maxChars + the truncation message, not the full length
	if len(receivedBody) >= len(longTranscript) {
		t.Errorf("transcript should have been truncated: sent %d chars, original %d chars", len(receivedBody), len(longTranscript))
	}
}

func TestLearningExtractor_MultipleCandidates(t *testing.T) {
	multiResp := anthropic.MessagesResponse{
		ID:         "msg_test_multi",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{{
			Type: "text",
			Text: `[
				{"title":"Learning One","body":"First insight.","tags":["go"]},
				{"title":"Learning Two","body":"Second insight.","tags":["docker","networking"]},
				{"title":"Learning Three","body":"Third insight.","tags":[]}
			]`,
		}},
		Usage: anthropic.Usage{InputTokens: 200, OutputTokens: 100},
	}

	server := newMockLearningServer(t, multiResp)
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "Complex transcript", "session-6", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	if candidates[0].Title != "Learning One" {
		t.Errorf("first title = %q, want %q", candidates[0].Title, "Learning One")
	}
	if candidates[2].Tags == nil {
		t.Error("expected non-nil tags slice even when empty")
	}
}

func TestLearningExtractor_NilTagsNormalized(t *testing.T) {
	// Response with null tags should be normalized to empty slice
	resp := anthropic.MessagesResponse{
		ID:         "msg_test_nil_tags",
		Type:       "message",
		Role:       "assistant",
		StopReason: "end_turn",
		Content: []anthropic.ContentBlock{{
			Type: "text",
			Text: `[{"title":"No Tags","body":"Body text.","tags":null}]`,
		}},
		Usage: anthropic.Usage{InputTokens: 50, OutputTokens: 20},
	}

	server := newMockLearningServer(t, resp)
	defer server.Close()

	client := anthropic.NewClient("test-key", anthropic.WithBaseURL(server.URL))
	extractor := analytics.NewLearningExtractor(client, "test-model", analytics.LearningExtractorConfig{})

	candidates, err := extractor.Extract(context.Background(), "Transcript", "session-7", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Tags == nil {
		t.Error("nil tags should be normalized to empty slice")
	}
	if len(candidates[0].Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(candidates[0].Tags))
	}
}
