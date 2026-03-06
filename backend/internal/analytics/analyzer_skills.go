package analytics

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
