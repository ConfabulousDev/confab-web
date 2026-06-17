package analytics

// extractCursorSearchText flattens user/assistant message text into the Weight
// C search-index content. Cursor records tool INPUTS only (no tool_result
// blocks), so there is no tool-output text to index — only the conversational
// text blocks contribute. Cursor's native bare `[REDACTED]` placeholder is
// stripped first so it never lands in the index (fa3h); blocks that were only
// the placeholder contribute nothing. Honors maxUserMessagesBytes via the
// shared searchTextBuilder.
func extractCursorSearchText(messages []*CursorMessage) string {
	if len(messages) == 0 {
		return ""
	}
	b := newSearchTextBuilder(maxUserMessagesBytes)
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				if cleaned := cleanCursorAssistantText(block.Text); cleaned != "" {
					b.Add(cleaned)
				}
			}
		}
	}
	return b.String()
}
