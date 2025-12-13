package api

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// sanitizeTitleText removes HTML tags and decodes HTML entities
// Note: We don't HTML-escape here because React automatically escapes text content.
// Double-escaping would cause &gt; to display literally instead of as >
func sanitizeTitleText(input string) string {
	// Remove all HTML tags using regex
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlTagRegex.ReplaceAllString(input, "")

	// Decode HTML entities (e.g., &lt; -> <, &gt; -> >)
	decoded := html.UnescapeString(cleaned)

	// Trim whitespace
	return strings.TrimSpace(decoded)
}

// extractSessionTitle parses the first few lines of a transcript to extract a title
func extractSessionTitle(content []byte) string {
	if len(content) == 0 {
		return ""
	}

	// Parse JSONL to find summary or first text content
	lines := strings.Split(string(content), "\n")
	var firstTextContent string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		msgType, _ := entry["type"].(string)

		// Priority 1: Extract title from summary (best quality)
		if msgType == "summary" {
			if summary, ok := entry["summary"].(string); ok && summary != "" {
				return sanitizeTitleText(summary)
			}
		}

		// Collect first text content as fallback (from any user message)
		if firstTextContent == "" && msgType == "user" {
			if text := extractTextFromMessage(entry); text != "" {
				// Use first 100 characters as fallback title
				sanitized := sanitizeTitleText(text)
				if len(sanitized) > 100 {
					firstTextContent = sanitized[:100]
				} else {
					firstTextContent = sanitized
				}
			}
		}
	}

	return firstTextContent
}
