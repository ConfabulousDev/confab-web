// ABOUTME: Tests for the Confluence REST API client.
// ABOUTME: Covers page creation, auth, error handling, IsConfigured, and markdown conversion.
package confluence_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/confluence"
)

func TestCreatePage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/rest/api/content" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "12345",
			"_links": map[string]string{
				"webui": "/wiki/spaces/ENG/pages/12345/Learning:+Test+Title",
			},
		})
	}))
	defer server.Close()

	client := confluence.NewClient(confluence.Config{
		BaseURL:      server.URL,
		AuthToken:    "test-token",
		SpaceKey:     "ENG",
		ParentPageID: "99",
	})

	result, err := client.CreatePage(context.Background(), "Test Title", "Some **bold** text", []string{"go", "testing"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.PageID != "12345" {
		t.Errorf("expected PageID 12345, got %s", result.PageID)
	}
	if !strings.Contains(result.WebURL, "/wiki/spaces/ENG/pages/12345") {
		t.Errorf("unexpected WebURL: %s", result.WebURL)
	}
}

func TestCreatePage_AuthTokenSent(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "1",
			"_links": map[string]string{"webui": "/page"},
		})
	}))
	defer server.Close()

	client := confluence.NewClient(confluence.Config{
		BaseURL:   server.URL,
		AuthToken: "my-secret-token",
		SpaceKey:  "TEAM",
	})

	_, err := client.CreatePage(context.Background(), "Auth Test", "body", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer my-secret-token"
	if receivedAuth != expected {
		t.Errorf("expected Authorization %q, got %q", expected, receivedAuth)
	}
}

func TestCreatePage_RequestPayload(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "1",
			"_links": map[string]string{"webui": "/page"},
		})
	}))
	defer server.Close()

	client := confluence.NewClient(confluence.Config{
		BaseURL:      server.URL,
		AuthToken:    "tok",
		SpaceKey:     "DEV",
		ParentPageID: "42",
	})

	_, err := client.CreatePage(context.Background(), "My Title", "hello", []string{"tag1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check title prefix
	if title, ok := receivedBody["title"].(string); !ok || title != "Learning: My Title" {
		t.Errorf("expected title 'Learning: My Title', got %v", receivedBody["title"])
	}

	// Check status is draft
	if status, ok := receivedBody["status"].(string); !ok || status != "draft" {
		t.Errorf("expected status 'draft', got %v", receivedBody["status"])
	}

	// Check space key
	space, ok := receivedBody["space"].(map[string]interface{})
	if !ok {
		t.Fatal("space field missing or wrong type")
	}
	if space["key"] != "DEV" {
		t.Errorf("expected space key 'DEV', got %v", space["key"])
	}

	// Check ancestors (parent page)
	ancestors, ok := receivedBody["ancestors"].([]interface{})
	if !ok || len(ancestors) == 0 {
		t.Fatal("ancestors field missing")
	}
	ancestor := ancestors[0].(map[string]interface{})
	if ancestor["id"] != "42" {
		t.Errorf("expected ancestor id '42', got %v", ancestor["id"])
	}
}

func TestCreatePage_ErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"bad request", 400, `{"message":"invalid title"}`},
		{"server error", 500, `{"message":"internal error"}`},
		{"forbidden", 403, `{"message":"no permission"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := confluence.NewClient(confluence.Config{
				BaseURL:   server.URL,
				AuthToken: "tok",
				SpaceKey:  "X",
			})

			_, err := client.CreatePage(context.Background(), "Fail", "body", nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.body) {
				t.Errorf("error should contain response body, got: %v", err)
			}
		})
	}
}

func TestIsConfigured(t *testing.T) {
	tests := []struct {
		name     string
		cfg      confluence.Config
		expected bool
	}{
		{"fully configured", confluence.Config{BaseURL: "https://c.example.com", AuthToken: "tok"}, true},
		{"missing token", confluence.Config{BaseURL: "https://c.example.com"}, false},
		{"missing base URL", confluence.Config{AuthToken: "tok"}, false},
		{"both empty", confluence.Config{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := confluence.NewClient(tt.cfg)
			if got := client.IsConfigured(); got != tt.expected {
				t.Errorf("IsConfigured() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMarkdownToStorage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
	}{
		{
			name:  "headings",
			input: "# Title\n## Subtitle\n### Section",
			contains: []string{
				"<h1>Title</h1>",
				"<h2>Subtitle</h2>",
				"<h3>Section</h3>",
			},
		},
		{
			name:  "bold and italic",
			input: "This is **bold** and *italic* text",
			contains: []string{
				"<strong>bold</strong>",
				"<em>italic</em>",
			},
		},
		{
			name:  "inline code",
			input: "Use `go test` to run tests",
			contains: []string{
				"<code>go test</code>",
			},
		},
		{
			name:  "code block",
			input: "```go\nfmt.Println(\"hello\")\n```",
			contains: []string{
				`ac:name="code"`,
				`ac:name="language">go</ac:parameter>`,
				`CDATA[fmt.Println`,
			},
		},
		{
			name:  "unordered list",
			input: "- item one\n- item two\n- item three",
			contains: []string{
				"<ul>",
				"<li>item one</li>",
				"<li>item two</li>",
				"</ul>",
			},
		},
		{
			name:  "plain paragraph",
			input: "Just a simple paragraph.",
			contains: []string{
				"<p>Just a simple paragraph.</p>",
			},
		},
		{
			name:  "html escaping",
			input: "Use <script> & \"quotes\"",
			contains: []string{
				"&lt;script&gt;",
				"&amp;",
				"&quot;quotes&quot;",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := confluence.MarkdownToStorage(tt.input)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q\ngot: %s", want, result)
				}
			}
		})
	}
}

func TestCreatePage_NoParentPage(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":     "1",
			"_links": map[string]string{"webui": "/page"},
		})
	}))
	defer server.Close()

	// No ParentPageID set
	client := confluence.NewClient(confluence.Config{
		BaseURL:   server.URL,
		AuthToken: "tok",
		SpaceKey:  "X",
	})

	_, err := client.CreatePage(context.Background(), "No Parent", "body", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ancestors should not be in the payload
	if _, ok := receivedBody["ancestors"]; ok {
		t.Error("expected no ancestors field when ParentPageID is empty")
	}
}
