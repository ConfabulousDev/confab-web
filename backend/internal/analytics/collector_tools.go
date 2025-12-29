package analytics

// ToolsCollector extracts tool usage metrics from transcript lines.
type ToolsCollector struct {
	TotalCalls    int
	ToolBreakdown map[string]int
	ErrorCount    int
}

// NewToolsCollector creates a new tools collector.
func NewToolsCollector() *ToolsCollector {
	return &ToolsCollector{
		ToolBreakdown: make(map[string]int),
	}
}

// Collect processes a single line for tool metrics.
func (c *ToolsCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	// Count tool uses from assistant messages
	if line.IsAssistantMessage() {
		tools := line.GetToolUses()
		for _, tool := range tools {
			c.TotalCalls++
			if tool.Name != "" {
				c.ToolBreakdown[tool.Name]++
			}
		}
	}

	// Count tool errors from user messages (tool_result blocks)
	if line.IsUserMessage() {
		for _, block := range line.GetContentBlocks() {
			if block.Type == "tool_result" && block.IsError {
				c.ErrorCount++
			}
		}
	}
}

// Finalize is called after all lines are processed.
func (c *ToolsCollector) Finalize(ctx *CollectContext) {
	// No post-processing needed
}
