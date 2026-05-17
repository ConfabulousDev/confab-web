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
type ToolsAnalyzer struct {
	result   ToolsResult
	mainFile *TranscriptFile
}

// ProcessFile accumulates tool metrics from a single file.
func (a *ToolsAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.mainFile = file
		a.result.ToolStats = make(map[string]*ToolStats)
	}

	toolIDToName := file.BuildToolUseIDToNameMap()

	for _, line := range file.Lines {
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				a.result.TotalCalls++
				if tool.Name != "" {
					if a.result.ToolStats[tool.Name] == nil {
						a.result.ToolStats[tool.Name] = &ToolStats{}
					}
					a.result.ToolStats[tool.Name].Success++
				}
			}
		}

		if line.IsUserMessage() {
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" && block.IsError {
					a.result.ErrorCount++
					if toolName := toolIDToName[block.ToolUseID]; toolName != "" {
						if a.result.ToolStats[toolName] == nil {
							a.result.ToolStats[toolName] = &ToolStats{}
						}
						a.result.ToolStats[toolName].Success--
						a.result.ToolStats[toolName].Errors++
					}
				}
			}
		}
	}
}

// Finalize runs fallback logic for agents without files.
func (a *ToolsAnalyzer) Finalize(hasAgentFile func(string) bool) {
	// Fallback: count tool calls from subagent results when we don't have the file
	if a.mainFile != nil {
		for _, line := range a.mainFile.Lines {
			if line.IsUserMessage() {
				for _, agentResult := range line.GetAgentResults() {
					if !hasAgentFile(agentResult.AgentID) {
						a.result.TotalCalls += agentResult.TotalToolUseCount
					}
				}
			}
		}
	}

	// Ensure no negative success counts
	for _, stats := range a.result.ToolStats {
		if stats.Success < 0 {
			stats.Success = 0
		}
	}
}

// Result returns the accumulated tool metrics.
func (a *ToolsAnalyzer) Result() *ToolsResult {
	return &a.result
}

// Analyze processes the file collection and returns tool metrics.
func (a *ToolsAnalyzer) Analyze(fc *FileCollection) (*ToolsResult, error) {
	a.ProcessFile(fc.Main, true)
	for _, agent := range fc.Agents {
		a.ProcessFile(agent, false)
	}
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}
