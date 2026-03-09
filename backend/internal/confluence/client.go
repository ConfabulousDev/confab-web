// ABOUTME: Confluence REST API v1 client for creating pages from learning artifacts.
// ABOUTME: Provides markdown-to-storage-format conversion and Bearer token authentication.
package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("confab/confluence")

// Config holds the settings needed to connect to a Confluence instance.
type Config struct {
	BaseURL      string // e.g. "https://confluence.example.com"
	AuthToken    string // Personal access token or API token
	SpaceKey     string // Confluence space key (e.g. "ENG")
	ParentPageID string // Optional parent page ID to nest under
}

// PageResult contains the identifiers returned after creating a page.
type PageResult struct {
	PageID string
	WebURL string // Full URL to the created page
}

// Client is a Confluence REST API v1 client.
type Client struct {
	baseURL      string
	authToken    string
	spaceKey     string
	parentPageID string
	httpClient   *http.Client
}

// NewClient creates a new Confluence client from the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		authToken:    cfg.AuthToken,
		spaceKey:     cfg.SpaceKey,
		parentPageID: cfg.ParentPageID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsConfigured returns true if the minimum required fields (BaseURL and AuthToken) are set.
func (c *Client) IsConfigured() bool {
	return c.baseURL != "" && c.authToken != ""
}

// CreatePage creates a draft page in Confluence with the given title, markdown body, and labels.
func (c *Client) CreatePage(ctx context.Context, title string, bodyMarkdown string, labels []string) (*PageResult, error) {
	ctx, span := tracer.Start(ctx, "confluence.create_page",
		trace.WithAttributes(
			attribute.String("confluence.space_key", c.spaceKey),
			attribute.String("confluence.page_title", title),
		))
	defer span.End()

	storageBody := MarkdownToStorage(bodyMarkdown)

	// Build the request payload per Confluence REST API v1
	payload := map[string]interface{}{
		"type":   "page",
		"title":  "Learning: " + title,
		"status": "draft",
		"space":  map[string]string{"key": c.spaceKey},
		"body": map[string]interface{}{
			"storage": map[string]string{
				"value":          storageBody,
				"representation": "storage",
			},
		},
	}

	// Add parent page if configured
	if c.parentPageID != "" {
		payload["ancestors"] = []map[string]string{
			{"id": c.parentPageID},
		}
	}

	// Add labels (always include "confab" label)
	labelSet := make(map[string]struct{})
	labelSet["confab"] = struct{}{}
	for _, l := range labels {
		if l != "" {
			labelSet[l] = struct{}{}
		}
	}
	var labelEntries []map[string]string
	for name := range labelSet {
		labelEntries = append(labelEntries, map[string]string{
			"prefix": "global",
			"name":   name,
		})
	}
	payload["metadata"] = map[string]interface{}{
		"labels": labelEntries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal request")
		return nil, fmt.Errorf("confluence: failed to marshal request: %w", err)
	}

	url := c.baseURL + "/wiki/rest/api/content"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return nil, fmt.Errorf("confluence: failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "request failed")
		return nil, fmt.Errorf("confluence: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read response")
		return nil, fmt.Errorf("confluence: failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		span.SetStatus(codes.Error, "non-2xx response")
		span.SetAttributes(attribute.Int("confluence.status_code", resp.StatusCode))
		return nil, fmt.Errorf("confluence: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response to extract page ID and web URL
	var result struct {
		ID    string `json:"id"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to parse response")
		return nil, fmt.Errorf("confluence: failed to parse response: %w", err)
	}

	webURL := result.Links.WebUI
	if webURL != "" && !strings.HasPrefix(webURL, "http") {
		// _links.webui is relative; prepend the base URL
		webURL = c.baseURL + webURL
	}

	span.SetAttributes(attribute.String("confluence.page_id", result.ID))

	return &PageResult{
		PageID: result.ID,
		WebURL: webURL,
	}, nil
}

// MarkdownToStorage converts basic markdown to Confluence storage format.
// This is intentionally simple and covers the most common elements.
func MarkdownToStorage(md string) string {
	lines := strings.Split(md, "\n")
	var result []string
	inCodeBlock := false
	var codeBlockLang string
	var codeLines []string
	inList := false

	for _, line := range lines {
		// Handle code block fences
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				inCodeBlock = true
				codeBlockLang = strings.TrimPrefix(line, "```")
				codeBlockLang = strings.TrimSpace(codeBlockLang)
				codeLines = nil
				// Close any open list before code block
				if inList {
					result = append(result, "</ul>")
					inList = false
				}
			} else {
				// End code block
				inCodeBlock = false
				macro := `<ac:structured-macro ac:name="code">`
				if codeBlockLang != "" {
					macro += fmt.Sprintf(`<ac:parameter ac:name="language">%s</ac:parameter>`, escapeXML(codeBlockLang))
				}
				macro += `<ac:plain-text-body><![CDATA[` + strings.Join(codeLines, "\n") + `]]></ac:plain-text-body></ac:structured-macro>`
				result = append(result, macro)
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		// Empty line: close list if open, skip otherwise
		if strings.TrimSpace(line) == "" {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			continue
		}

		// Headings
		if strings.HasPrefix(line, "# ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, "<h1>"+convertInline(strings.TrimPrefix(line, "# "))+"</h1>")
			continue
		}
		if strings.HasPrefix(line, "## ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, "<h2>"+convertInline(strings.TrimPrefix(line, "## "))+"</h2>")
			continue
		}
		if strings.HasPrefix(line, "### ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, "<h3>"+convertInline(strings.TrimPrefix(line, "### "))+"</h3>")
			continue
		}
		if strings.HasPrefix(line, "#### ") {
			if inList {
				result = append(result, "</ul>")
				inList = false
			}
			result = append(result, "<h4>"+convertInline(strings.TrimPrefix(line, "#### "))+"</h4>")
			continue
		}

		// Unordered list items
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			if !inList {
				result = append(result, "<ul>")
				inList = true
			}
			content := line[2:]
			result = append(result, "<li>"+convertInline(content)+"</li>")
			continue
		}

		// Close list if we hit a non-list line
		if inList {
			result = append(result, "</ul>")
			inList = false
		}

		// Plain paragraph
		result = append(result, "<p>"+convertInline(line)+"</p>")
	}

	// Close any open list at end
	if inList {
		result = append(result, "</ul>")
	}

	return strings.Join(result, "\n")
}

// convertInline handles bold, italic, and inline code within a line.
func convertInline(s string) string {
	s = escapeXML(s)

	// Bold: **text** -> <strong>text</strong>
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	s = boldRe.ReplaceAllString(s, "<strong>$1</strong>")

	// Italic: *text* -> <em>text</em>  (must run after bold)
	italicRe := regexp.MustCompile(`\*(.+?)\*`)
	s = italicRe.ReplaceAllString(s, "<em>$1</em>")

	// Inline code: `text` -> <code>text</code>
	codeRe := regexp.MustCompile("`([^`]+)`")
	s = codeRe.ReplaceAllString(s, "<code>$1</code>")

	return s
}

// escapeXML escapes characters that are special in XML/HTML.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
