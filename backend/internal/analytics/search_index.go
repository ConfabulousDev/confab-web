package analytics

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxUserMessagesBytes = 500 * 1024 // 500KB

// SearchIndexContent holds the three weighted text components for the search index.
type SearchIndexContent struct {
	MetadataText     string // Weight A: titles, summary, first user message
	RecapText        string // Weight B: smart recap content
	UserMessagesText string // Weight C: human messages from transcript
	MetadataHash     string // MD5 hash of metadata fields for change detection
}

// CombinedText returns all text concatenated for storage in content_text.
func (c *SearchIndexContent) CombinedText() string {
	parts := make([]string, 0, 3)
	if c.MetadataText != "" {
		parts = append(parts, c.MetadataText)
	}
	if c.RecapText != "" {
		parts = append(parts, c.RecapText)
	}
	if c.UserMessagesText != "" {
		parts = append(parts, c.UserMessagesText)
	}
	return strings.Join(parts, "\n")
}

// ExtractSearchContent builds the search index content for a session.
// It queries metadata and recap from the DB, and extracts user messages from the file collection.
func ExtractSearchContent(ctx context.Context, db *sql.DB, sessionID string, fc *FileCollection) (*SearchIndexContent, error) {
	content := &SearchIndexContent{}

	// Weight A: metadata from sessions table
	metadataText, metadataHash, err := extractMetadata(ctx, db, sessionID)
	if err != nil {
		return nil, fmt.Errorf("extracting metadata: %w", err)
	}
	content.MetadataText = metadataText
	content.MetadataHash = metadataHash

	// Weight B: recap from session_card_smart_recap
	recapText, err := extractRecapText(ctx, db, sessionID)
	if err != nil {
		return nil, fmt.Errorf("extracting recap: %w", err)
	}
	content.RecapText = recapText

	// Weight C: user messages from transcript
	content.UserMessagesText = ExtractUserMessagesText(fc)

	return content, nil
}

// extractMetadata queries session metadata fields and computes their MD5 hash.
func extractMetadata(ctx context.Context, db *sql.DB, sessionID string) (text, hash string, err error) {
	var customTitle, suggestedTitle, summary, firstMsg sql.NullString
	query := `SELECT custom_title, suggested_session_title, summary, first_user_message FROM sessions WHERE id = $1`
	err = db.QueryRowContext(ctx, query, sessionID).Scan(&customTitle, &suggestedTitle, &summary, &firstMsg)
	if err != nil {
		return "", "", err
	}

	parts := make([]string, 0, 4)
	if customTitle.Valid && customTitle.String != "" {
		parts = append(parts, customTitle.String)
	}
	if suggestedTitle.Valid && suggestedTitle.String != "" {
		parts = append(parts, suggestedTitle.String)
	}
	if summary.Valid && summary.String != "" {
		parts = append(parts, summary.String)
	}
	if firstMsg.Valid && firstMsg.String != "" {
		parts = append(parts, firstMsg.String)
	}

	text = strings.Join(parts, "\n")

	// Hash for change detection: MD5 of concatenated raw values (empty string for NULL)
	hashInput := customTitle.String + "|" + suggestedTitle.String + "|" + summary.String + "|" + firstMsg.String
	hash = fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))

	return text, hash, nil
}

// extractRecapText queries the smart recap card and flattens all text content.
func extractRecapText(ctx context.Context, db *sql.DB, sessionID string) (string, error) {
	var recap sql.NullString
	var wentWellJSON, wentBadJSON, humanSugJSON, envSugJSON, ctxSugJSON []byte

	query := `
		SELECT recap, went_well, went_bad, human_suggestions, environment_suggestions, default_context_suggestions
		FROM session_card_smart_recap
		WHERE session_id = $1
	`
	err := db.QueryRowContext(ctx, query, sessionID).Scan(
		&recap, &wentWellJSON, &wentBadJSON, &humanSugJSON, &envSugJSON, &ctxSugJSON,
	)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	parts := make([]string, 0)
	if recap.Valid && recap.String != "" {
		parts = append(parts, recap.String)
	}

	// Flatten JSONB string arrays
	for _, jsonBytes := range [][]byte{wentWellJSON, wentBadJSON, humanSugJSON, envSugJSON, ctxSugJSON} {
		items := flattenJSONStringArray(jsonBytes)
		parts = append(parts, items...)
	}

	return strings.Join(parts, "\n"), nil
}

// flattenJSONStringArray parses a JSON array of strings and returns non-empty items.
func flattenJSONStringArray(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	// Simple parsing: split by `","` after trimming brackets
	s := strings.TrimSpace(string(data))
	if s == "[]" || s == "null" || s == "" {
		return nil
	}
	// Remove outer brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}

	var items []string
	// Split on "," (quoted comma) to handle JSON string arrays
	// This is a simple approach that works for well-formed arrays without escaped quotes
	for _, raw := range splitJSONArray(s) {
		raw = strings.TrimSpace(raw)
		// Remove surrounding quotes
		if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
			raw = raw[1 : len(raw)-1]
		}
		// Unescape common JSON escapes
		raw = strings.ReplaceAll(raw, `\"`, `"`)
		raw = strings.ReplaceAll(raw, `\\`, `\`)
		if raw != "" {
			items = append(items, raw)
		}
	}
	return items
}

// splitJSONArray splits a JSON array body on commas that are between elements.
func splitJSONArray(s string) []string {
	var parts []string
	var current strings.Builder
	inString := false
	escaped := false

	for _, ch := range s {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			current.WriteRune(ch)
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			current.WriteRune(ch)
			continue
		}
		if ch == ',' && !inString {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// ExtractUserMessagesText extracts text from human messages in the transcript.
// Skips tool results, skill expansions, and command expansions.
// Truncates output at maxUserMessagesBytes.
func ExtractUserMessagesText(fc *FileCollection) string {
	if fc == nil {
		return ""
	}

	var b strings.Builder
	totalBytes := 0

	for _, tf := range fc.AllFiles() {
		for _, line := range tf.Lines {
			if !line.IsHumanMessage() {
				continue
			}
			if line.IsSkillExpansionMessage() {
				continue
			}
			if line.IsCommandExpansionMessage() {
				continue
			}

			text := getStringContent(line)
			if text == "" {
				continue
			}

			if totalBytes+len(text)+1 > maxUserMessagesBytes {
				// Truncate: add what fits
				remaining := maxUserMessagesBytes - totalBytes
				if remaining > 1 && b.Len() > 0 {
					b.WriteByte('\n')
					remaining--
				}
				if remaining > 0 {
					// Back up to a valid UTF-8 boundary
					for remaining > 0 && !utf8.RuneStart(text[remaining]) {
						remaining--
					}
					b.WriteString(text[:remaining])
				}
				return b.String()
			}

			if b.Len() > 0 {
				b.WriteByte('\n')
				totalBytes++
			}
			b.WriteString(text)
			totalBytes += len(text)
		}
	}

	return b.String()
}
