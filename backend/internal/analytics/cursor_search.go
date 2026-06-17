package analytics

// extractCursorSearchText flattens user/assistant message text into the Weight
// C search-index content. Cursor records tool INPUTS only (no tool_result
// blocks), so there is no tool-output text to index — only the conversational
// text blocks contribute. Honors maxUserMessagesBytes via the shared
// searchTextBuilder.
func extractCursorSearchText(messages []*CursorMessage) string {
	if len(messages) == 0 {
		return ""
	}
	b := newSearchTextBuilder(maxUserMessagesBytes)
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				b.Add(block.Text)
			}
		}
	}
	return b.String()
}
