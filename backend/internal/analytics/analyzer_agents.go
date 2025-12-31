package analytics

// AgentsResult contains agent invocation metrics.
type AgentsResult struct {
	TotalInvocations int
	AgentStats       map[string]*AgentStats
}

// AgentsAnalyzer extracts agent/Task usage metrics from transcripts.
// It tracks invocations of the Task tool by subagent_type and their outcomes.
type AgentsAnalyzer struct{}

// Analyze processes the file collection and returns agent metrics.
func (a *AgentsAnalyzer) Analyze(fc *FileCollection) (*AgentsResult, error) {
	result := &AgentsResult{
		AgentStats: make(map[string]*AgentStats),
	}

	// Only process main transcript - agent invocations are tracked there
	// Build a map of tool_use_id -> subagent_type for Task tools
	toolUseIDToAgentType := make(map[string]string)

	for _, line := range fc.Main.Lines {
		// Find Task tool_use blocks and extract subagent_type
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if tool.Name == "Task" && tool.ID != "" {
					if subagentType, ok := tool.Input["subagent_type"].(string); ok && subagentType != "" {
						toolUseIDToAgentType[tool.ID] = subagentType
					}
				}
			}
		}

		// Find tool_result blocks with agentId and determine outcome
		if line.IsToolResultMessage() {
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" && block.ToolUseResult != nil && block.ToolUseResult.AgentID != "" {
					// Look up the agent type from the tool_use_id
					agentType := toolUseIDToAgentType[block.ToolUseID]
					if agentType == "" {
						// Fallback: use "unknown" if we can't find the type
						agentType = "unknown"
					}

					// Initialize stats if needed
					if result.AgentStats[agentType] == nil {
						result.AgentStats[agentType] = &AgentStats{}
					}

					result.TotalInvocations++
					if block.IsError {
						result.AgentStats[agentType].Errors++
					} else {
						result.AgentStats[agentType].Success++
					}
				}
			}
		}
	}

	return result, nil
}
