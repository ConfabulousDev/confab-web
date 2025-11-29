package discovery

import (
	"bufio"
	"encoding/json"
	"html"
	"os"
	"regexp"
	"strings"
)

// MaxLinesForTitle limits how many lines we read when extracting a title
// Summaries typically appear in the first few lines of a transcript
const MaxLinesForTitle = 50

// MaxTitleLength is the maximum length for extracted titles
const MaxTitleLength = 100

// htmlTagRegex matches HTML tags for removal
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// ExtractSessionTitle reads a transcript file and extracts a title.
// Priority 1: Look for type:"summary" entries and use the summary field
// Priority 2: Use the first user message content (truncated to MaxTitleLength chars)
func ExtractSessionTitle(transcriptPath string) string {
	file, err := os.Open(transcriptPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var firstUserMessage string
	linesRead := 0

	for scanner.Scan() && linesRead < MaxLinesForTitle {
		linesRead++
		line := scanner.Text()
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

		// Collect first user message as fallback
		if firstUserMessage == "" && msgType == "user" {
			if message, ok := entry["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok && content != "" {
					sanitized := sanitizeTitleText(content)
					if len(sanitized) > MaxTitleLength {
						firstUserMessage = sanitized[:MaxTitleLength]
					} else {
						firstUserMessage = sanitized
					}
				}
			}
		}
	}

	return firstUserMessage
}

// sanitizeTitleText removes HTML tags and decodes HTML entities
func sanitizeTitleText(input string) string {
	// Remove all HTML tags
	cleaned := htmlTagRegex.ReplaceAllString(input, "")

	// Decode HTML entities (e.g., &lt; -> <, &gt; -> >)
	decoded := html.UnescapeString(cleaned)

	// Replace newlines and excessive whitespace with single space
	decoded = strings.Join(strings.Fields(decoded), " ")

	// Trim whitespace
	return strings.TrimSpace(decoded)
}
