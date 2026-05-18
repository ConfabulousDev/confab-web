package analytics

// =============================================================================
// Agents
// =============================================================================

// AgentsResult contains agent invocation metrics.
type AgentsResult struct {
	TotalInvocations int
	AgentStats       map[string]*AgentStats
}

// isAgentToolName reports whether the given tool name is the agent tool.
// The tool was originally named "Task" and later renamed to "Agent";
// both names appear in transcripts.
func isAgentToolName(name string) bool {
	return name == "Task" || name == "Agent"
}

// AgentsAnalyzer extracts agent usage metrics from transcripts.
// It tracks invocations of the agent tool (named "Task" in older transcripts,
// "Agent" in newer ones) by subagent_type and their outcomes.
// Main-only: agent invocations are tracked in the main transcript.
type AgentsAnalyzer struct {
	result               AgentsResult
	toolUseIDToAgentType map[string]string
}

// ProcessFile accumulates agent metrics from a single file.
// Only the main transcript is processed.
func (a *AgentsAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if !isMain {
		return
	}

	a.result.AgentStats = make(map[string]*AgentStats)
	a.toolUseIDToAgentType = make(map[string]string)

	for _, line := range file.Lines {
		// Find agent tool_use blocks and extract subagent_type.
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if isAgentToolName(tool.Name) && tool.ID != "" {
					if subagentType, ok := tool.Input["subagent_type"].(string); ok && subagentType != "" {
						a.toolUseIDToAgentType[tool.ID] = subagentType
					}
				}
			}
		}

		// Find tool_result messages with agentId in top-level toolUseResult
		if line.IsToolResultMessage() && line.ToolUseResult != nil && line.ToolUseResult.AgentID != "" {
			var toolUseID string
			var isError bool
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" {
					toolUseID = block.ToolUseID
					isError = block.IsError
					break
				}
			}

			agentType := a.toolUseIDToAgentType[toolUseID]
			if agentType == "" {
				agentType = "unknown"
			}

			if a.result.AgentStats[agentType] == nil {
				a.result.AgentStats[agentType] = &AgentStats{}
			}

			a.result.TotalInvocations++
			if isError {
				a.result.AgentStats[agentType].Errors++
			} else {
				a.result.AgentStats[agentType].Success++
			}
		}
	}
}

// Finalize is a no-op for agents (main-only analyzer).
func (a *AgentsAnalyzer) Finalize(hasAgentFile func(string) bool) {}

// Result returns the accumulated agent metrics.
func (a *AgentsAnalyzer) Result() *AgentsResult {
	return &a.result
}

// Analyze processes the file collection and returns agent metrics.
func (a *AgentsAnalyzer) Analyze(fc *FileCollection) (*AgentsResult, error) {
	a.ProcessFile(fc.Main, true)
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}

// =============================================================================
// Skills
// =============================================================================

// SkillsResult contains skill invocation metrics.
type SkillsResult struct {
	TotalInvocations int
	SkillStats       map[string]*SkillStats
}

// SkillsAnalyzer extracts skill usage metrics from transcripts.
// It tracks invocations of the Skill tool by skill name and their outcomes.
// Main-only: skill invocations are tracked in the main transcript.
type SkillsAnalyzer struct {
	result               SkillsResult
	toolUseIDToSkillName map[string]string
}

// ProcessFile accumulates skill metrics from a single file.
// Only the main transcript is processed.
func (a *SkillsAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if !isMain {
		return
	}

	a.result.SkillStats = make(map[string]*SkillStats)
	a.toolUseIDToSkillName = make(map[string]string)

	for _, line := range file.Lines {
		// Find Skill tool_use blocks and extract skill name
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if tool.Name == "Skill" && tool.ID != "" {
					if skillName, ok := tool.Input["skill"].(string); ok && skillName != "" {
						a.toolUseIDToSkillName[tool.ID] = skillName
					}
				}
			}
		}

		// Find tool_result blocks for Skill invocations and determine outcome
		if line.IsToolResultMessage() {
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" && block.ToolUseID != "" {
					skillName := a.toolUseIDToSkillName[block.ToolUseID]
					if skillName == "" {
						continue
					}

					if a.result.SkillStats[skillName] == nil {
						a.result.SkillStats[skillName] = &SkillStats{}
					}

					a.result.TotalInvocations++
					if block.IsError {
						a.result.SkillStats[skillName].Errors++
					} else {
						a.result.SkillStats[skillName].Success++
					}
				}
			}
		}
	}

	// Second pass: detect command-expansion skill invocations
	for _, line := range file.Lines {
		skillName := line.GetCommandExpansionSkillName()
		if skillName == "" {
			continue
		}
		if a.result.SkillStats[skillName] == nil {
			a.result.SkillStats[skillName] = &SkillStats{}
		}
		a.result.TotalInvocations++
		a.result.SkillStats[skillName].Success++
	}
}

// Finalize is a no-op for skills (main-only analyzer).
func (a *SkillsAnalyzer) Finalize(hasAgentFile func(string) bool) {}

// Result returns the accumulated skill metrics.
func (a *SkillsAnalyzer) Result() *SkillsResult {
	return &a.result
}

// Analyze processes the file collection and returns skill metrics.
func (a *SkillsAnalyzer) Analyze(fc *FileCollection) (*SkillsResult, error) {
	a.ProcessFile(fc.Main, true)
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}
