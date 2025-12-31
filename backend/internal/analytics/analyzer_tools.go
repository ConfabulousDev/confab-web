package analytics

// ToolStats holds success and error counts for a single tool.
type ToolStats struct {
	Success int `json:"success"`
	Errors  int `json:"errors"`
}

// ToolsResult contains tool usage metrics.
type ToolsResult struct {
	TotalCalls int
	ErrorCount int
	ToolStats  map[string]*ToolStats
}

// ToolsAnalyzer extracts tool usage metrics from transcripts.
// It processes all files (main + agents) to get complete tool breakdown.
type ToolsAnalyzer struct{}

// Analyze processes the file collection and returns tool metrics.
func (a *ToolsAnalyzer) Analyze(fc *FileCollection) (*ToolsResult, error) {
	result := &ToolsResult{
		ToolStats: make(map[string]*ToolStats),
	}

	// Process all files - main and agents
	for _, file := range fc.AllFiles() {
		a.processFile(file, result)
	}

	// Ensure no negative success counts (in case of data inconsistency)
	for _, stats := range result.ToolStats {
		if stats.Success < 0 {
			stats.Success = 0
		}
	}

	return result, nil
}

// processFile processes a single transcript file for tool metrics.
func (a *ToolsAnalyzer) processFile(file *TranscriptFile, result *ToolsResult) {
	// Build tool ID -> name map for this file
	toolIDToName := file.BuildToolUseIDToNameMap()

	for _, line := range file.Lines {
		// Count tool uses from assistant messages
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				result.TotalCalls++
				if tool.Name != "" {
					if result.ToolStats[tool.Name] == nil {
						result.ToolStats[tool.Name] = &ToolStats{}
					}
					result.ToolStats[tool.Name].Success++
				}
			}
		}

		// Count tool errors from user messages (tool_result blocks)
		if line.IsUserMessage() {
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" && block.IsError {
					result.ErrorCount++
					// Look up the tool name from the ID
					if toolName := toolIDToName[block.ToolUseID]; toolName != "" {
						if result.ToolStats[toolName] == nil {
							result.ToolStats[toolName] = &ToolStats{}
						}
						// Move from success to error
						result.ToolStats[toolName].Success--
						result.ToolStats[toolName].Errors++
					}
				}
			}
		}
	}
}
