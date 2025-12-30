package analytics

// ToolStats holds success and error counts for a single tool.
type ToolStats struct {
	Success int `json:"success"`
	Errors  int `json:"errors"`
}

// ToolsCollector extracts tool usage metrics from transcript lines.
type ToolsCollector struct {
	TotalCalls int
	ErrorCount int
	// ToolStats maps tool name to success/error counts
	ToolStats map[string]*ToolStats
}

// NewToolsCollector creates a new tools collector.
func NewToolsCollector() *ToolsCollector {
	return &ToolsCollector{
		ToolStats: make(map[string]*ToolStats),
	}
}

// Collect processes a single line for tool metrics.
func (c *ToolsCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	// Count tool uses from assistant messages and record ID -> name mapping
	if line.IsAssistantMessage() {
		tools := line.GetToolUses()
		for _, tool := range tools {
			c.TotalCalls++
			if tool.Name != "" {
				// Record ID -> name mapping for later error attribution
				if tool.ID != "" {
					ctx.ToolUseIDToName[tool.ID] = tool.Name
				}
				// Initialize stats if needed
				if c.ToolStats[tool.Name] == nil {
					c.ToolStats[tool.Name] = &ToolStats{}
				}
				// Count as success initially (may be decremented if error found)
				c.ToolStats[tool.Name].Success++
			}
		}
	}

	// Count tool errors from user messages (tool_result blocks)
	if line.IsUserMessage() {
		for _, block := range line.GetContentBlocks() {
			if block.Type == "tool_result" && block.IsError {
				c.ErrorCount++
				// Look up the tool name from the ID
				if toolName := ctx.ToolUseIDToName[block.ToolUseID]; toolName != "" {
					if c.ToolStats[toolName] == nil {
						c.ToolStats[toolName] = &ToolStats{}
					}
					// Move from success to error
					c.ToolStats[toolName].Success--
					c.ToolStats[toolName].Errors++
				}
			}
		}
	}

	// Count tool calls from subagent/Task results
	// We only have the total count, not per-tool breakdown, so add to TotalCalls only
	for _, result := range line.GetAgentResults() {
		c.TotalCalls += result.TotalToolUseCount
	}
}

// Finalize is called after all lines are processed.
func (c *ToolsCollector) Finalize(ctx *CollectContext) {
	// Ensure no negative success counts (in case of data inconsistency)
	for _, stats := range c.ToolStats {
		if stats.Success < 0 {
			stats.Success = 0
		}
	}
}

