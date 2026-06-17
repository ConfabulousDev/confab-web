package analytics

// extractCursorSearchText flattens user/assistant message text into the Weight
// C search-index content. Cursor records tool INPUTS only (no tool_result
// blocks), so there is no tool-output text to index — only the conversational
// text blocks contribute. Per role: user text is unwrapped from its
// `<user_query>` envelope so the injected-context tags never pollute the index
// (nfbe); assistant text has Cursor's native bare `[REDACTED]` placeholder
// stripped (fa3h). Blocks that contribute nothing after cleaning are skipped.
// Honors maxUserMessagesBytes via the shared searchTextBuilder.
func extractCursorSearchText(messages []*CursorMessage) string {
	if len(messages) == 0 {
		return ""
	}
	b := newSearchTextBuilder(maxUserMessagesBytes)
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type != "text" {
				continue
			}
			var cleaned string
			if msg.Role == "user" {
				cleaned = parseCursorUserPrompt(block.Text)
			} else {
				cleaned = cleanCursorAssistantText(block.Text)
			}
			if cleaned != "" {
				b.Add(cleaned)
			}
		}
	}
	return b.String()
}
