package analytics

// opencodeToolOutputPreviewBytes caps how much of any single tool output we
// fold into the search index. Per-output bound; the overall index size is
// further bounded by searchTextBuilder's maxUserMessagesBytes cap.
const opencodeToolOutputPreviewBytes = 500

// extractOpenCodeSearchText flattens user/assistant text and tool
// names+outputs across [main, ...subagents] into the Weight C search-index
// content. Honors maxUserMessagesBytes with UTF-8-safe truncation via the
// shared searchTextBuilder (CF-539 — subagent transcripts contribute to
// search recall; the overall cap prevents an N-subagent session from
// exploding the index row).
func extractOpenCodeSearchText(rollouts [][]*OpenCodeMessage) string {
	if len(rollouts) == 0 {
		return ""
	}
	b := newSearchTextBuilder(maxUserMessagesBytes)
	for _, messages := range rollouts {
		for _, msg := range messages {
			for _, p := range msg.Parts {
				switch p.Type {
				case "text":
					b.Add(p.Text)
				case "tool":
					b.Add(p.Tool)
					if p.State != nil && p.State.Output != "" {
						output := p.State.Output
						if len(output) > opencodeToolOutputPreviewBytes {
							output = output[:opencodeToolOutputPreviewBytes]
						}
						b.Add(output)
					}
				}
			}
		}
	}
	return b.String()
}
