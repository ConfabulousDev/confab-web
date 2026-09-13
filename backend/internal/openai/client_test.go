package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func boolPtr(b bool) *bool { return &b }

func strictRequest() *ResponsesRequest {
	return &ResponsesRequest{
		Model:           "gpt-5.6-luna",
		Instructions:    "You are a summarizer.",
		Input:           "Summarize this transcript",
		MaxOutputTokens: 1000,
		Reasoning:       &Reasoning{Effort: "none"},
		Store:           boolPtr(false),
		Text: &TextConfig{Format: TextFormat{
			Type:   "json_schema",
			Name:   "smart_recap",
			Strict: true,
			Schema: json.RawMessage(`{"type":"object","properties":{},"required":[],"additionalProperties":false}`),
		}},
	}
}

func TestCreateResponse_SendsResponsesAPIRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/responses" {
			t.Errorf("path = %s, want /v1/responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["model"] != "gpt-5.6-luna" {
			t.Errorf("model = %v", body["model"])
		}
		if body["instructions"] != "You are a summarizer." {
			t.Errorf("instructions = %v", body["instructions"])
		}
		if body["input"] != "Summarize this transcript" {
			t.Errorf("input = %v", body["input"])
		}
		if body["max_output_tokens"] != float64(1000) {
			t.Errorf("max_output_tokens = %v", body["max_output_tokens"])
		}
		reasoning, _ := body["reasoning"].(map[string]any)
		if reasoning["effort"] != "none" {
			t.Errorf("reasoning.effort = %v, want none", reasoning["effort"])
		}
		store, present := body["store"]
		if !present || store != false {
			t.Errorf("store must be serialized as explicit false, got present=%v value=%v", present, store)
		}
		for _, forbidden := range []string{"temperature", "top_p"} {
			if _, ok := body[forbidden]; ok {
				t.Errorf("request must not include %q (GPT-5.x reasoning models reject it)", forbidden)
			}
		}
		text, _ := body["text"].(map[string]any)
		format, _ := text["format"].(map[string]any)
		if format["type"] != "json_schema" || format["name"] != "smart_recap" || format["strict"] != true {
			t.Errorf("text.format = %v, want strict json_schema named smart_recap", format)
		}
		if _, ok := format["schema"].(map[string]any); !ok {
			t.Errorf("text.format.schema must be a JSON object, got %T", format["schema"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "resp_123",
			"status": "completed",
			"output": [
				{"type": "reasoning", "content": []},
				{"type": "message", "content": [
					{"type": "output_text", "text": "Hello "},
					{"type": "output_text", "text": "world"}
				]}
			],
			"usage": {"input_tokens": 120, "output_tokens": 45}
		}`))
	}))
	defer server.Close()

	client := NewClient("test-key", WithBaseURL(server.URL))
	resp, err := client.CreateResponse(context.Background(), strictRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "resp_123" || resp.Status != "completed" {
		t.Errorf("ID/Status = %q/%q", resp.ID, resp.Status)
	}
	if got := resp.OutputText(); got != "Hello world" {
		t.Errorf("OutputText() = %q, want %q", got, "Hello world")
	}
	if resp.Usage.InputTokens != 120 || resp.Usage.OutputTokens != 45 {
		t.Errorf("Usage = %+v", resp.Usage)
	}
}

func TestResponsesResponse_OutputTextSkipsNonMessageItemsAndNonTextParts(t *testing.T) {
	resp := ResponsesResponse{Output: []OutputItem{
		{Type: "reasoning", Content: []ContentPart{{Type: "output_text", Text: "ignored reasoning"}}},
		{Type: "message", Content: []ContentPart{
			{Type: "output_text", Text: "a"},
			{Type: "refusal", Refusal: "nope"},
			{Type: "output_text", Text: "b"},
		}},
	}}
	if got := resp.OutputText(); got != "ab" {
		t.Errorf("OutputText() = %q, want %q", got, "ab")
	}
	if got := resp.Refusal(); got != "nope" {
		t.Errorf("Refusal() = %q, want %q", got, "nope")
	}
}

func TestResponsesResponse_EmptyOutput(t *testing.T) {
	var resp ResponsesResponse
	if resp.OutputText() != "" || resp.Refusal() != "" {
		t.Errorf("empty response should have empty OutputText and Refusal")
	}
}

func TestCreateResponse_ParsesIncompleteStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"resp_1","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":10,"output_tokens":1000}}`))
	}))
	defer server.Close()

	resp, err := NewClient("k", WithBaseURL(server.URL)).CreateResponse(context.Background(), strictRequest())
	if err != nil {
		t.Fatalf("an incomplete response is not a transport error: %v", err)
	}
	if resp.Status != "incomplete" {
		t.Errorf("Status = %q, want incomplete", resp.Status)
	}
	if resp.IncompleteDetails == nil || resp.IncompleteDetails.Reason != "max_output_tokens" {
		t.Errorf("IncompleteDetails = %+v, want reason max_output_tokens", resp.IncompleteDetails)
	}
}

func TestCreateResponse_APIErrorIsStructured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Unsupported parameter: 'temperature'","type":"invalid_request_error","code":"unsupported_parameter"}}`))
	}))
	defer server.Close()

	_, err := NewClient("k", WithBaseURL(server.URL)).CreateResponse(context.Background(), strictRequest())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Detail.Type != "invalid_request_error" || apiErr.Detail.Code != "unsupported_parameter" {
		t.Errorf("Detail = %+v", apiErr.Detail)
	}
	want := "openai API error (status 400, type invalid_request_error, code unsupported_parameter): Unsupported parameter: 'temperature'"
	if apiErr.Error() != want {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), want)
	}
}

func TestCreateResponse_NonJSONErrorBodyReturnsPlainError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>bad gateway</html>"))
	}))
	defer server.Close()

	_, err := NewClient("k", WithBaseURL(server.URL)).CreateResponse(context.Background(), strictRequest())
	if err == nil {
		t.Fatal("expected error for 502")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("non-JSON body must not produce *APIError, got %v", apiErr)
	}
	if !strings.Contains(err.Error(), "502") || !strings.Contains(err.Error(), "bad gateway") {
		t.Errorf("error should include status and body, got %q", err.Error())
	}
}

func TestCreateResponse_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewClient("k", WithBaseURL(server.URL)).CreateResponse(ctx, strictRequest()); err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("k")
	if c.baseURL != "https://api.openai.com" {
		t.Errorf("default baseURL = %q", c.baseURL)
	}
	if c.httpClient.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v", c.httpClient.Timeout)
	}
	if NewClient("k", WithBaseURL("http://x")).baseURL != "http://x" {
		t.Error("WithBaseURL not applied")
	}
}
