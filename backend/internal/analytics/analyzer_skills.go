package analytics

// SkillsResult contains skill invocation metrics.
type SkillsResult struct {
	TotalInvocations int
	SkillStats       map[string]*SkillStats
}

// SkillsAnalyzer extracts skill usage metrics from transcripts.
// It tracks invocations of the Skill tool by skill name and their outcomes.
type SkillsAnalyzer struct{}

// Analyze processes the file collection and returns skill metrics.
func (a *SkillsAnalyzer) Analyze(fc *FileCollection) (*SkillsResult, error) {
	result := &SkillsResult{
		SkillStats: make(map[string]*SkillStats),
	}

	// Only process main transcript - skill invocations are tracked there
	// Build a map of tool_use_id -> skill name for Skill tools
	toolUseIDToSkillName := make(map[string]string)

	for _, line := range fc.Main.Lines {
		// Find Skill tool_use blocks and extract skill name
		if line.IsAssistantMessage() {
			for _, tool := range line.GetToolUses() {
				if tool.Name == "Skill" && tool.ID != "" {
					if skillName, ok := tool.Input["skill"].(string); ok && skillName != "" {
						toolUseIDToSkillName[tool.ID] = skillName
					}
				}
			}
		}

		// Find tool_result blocks for Skill invocations and determine outcome
		if line.IsToolResultMessage() {
			for _, block := range line.GetContentBlocks() {
				if block.Type == "tool_result" && block.ToolUseID != "" {
					// Look up the skill name from the tool_use_id
					skillName := toolUseIDToSkillName[block.ToolUseID]
					if skillName == "" {
						// Not a Skill tool result, skip
						continue
					}

					// Initialize stats if needed
					if result.SkillStats[skillName] == nil {
						result.SkillStats[skillName] = &SkillStats{}
					}

					result.TotalInvocations++
					if block.IsError {
						result.SkillStats[skillName].Errors++
					} else {
						result.SkillStats[skillName].Success++
					}
				}
			}
		}
	}

	return result, nil
}
