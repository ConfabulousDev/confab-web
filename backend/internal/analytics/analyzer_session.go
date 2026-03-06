package analytics

import "time"

// SessionResult contains session-level metrics.
type SessionResult struct {
	// Message counts
	TotalMessages     int
	UserMessages      int
	AssistantMessages int

	// Message type breakdown
	HumanPrompts   int
	ToolResults    int
	TextResponses  int
	ToolCalls      int
	ThinkingBlocks int

	// Session metadata
	DurationMs *int64
	ModelsUsed []string

	// Compaction stats
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int
}

// SessionAnalyzer extracts session-level metrics from transcripts.
// It processes main transcript for session stats, and all files for models.
type SessionAnalyzer struct {
	result          SessionResult
	modelsUsed      map[string]bool
	firstTimestamp  *time.Time
	lastTimestamp   *time.Time
	compactionTimes []int64
}

// ProcessFile accumulates session metrics from a single file.
func (a *SessionAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if isMain {
		a.modelsUsed = make(map[string]bool)
		a.processMainFile(file)
	}

	// Collect models from all files
	for _, group := range file.AssistantMessageGroups() {
		if group.Model != "" && group.Model != "<synthetic>" {
			a.modelsUsed[group.Model] = true
		}
	}
}

// processMainFile handles main-transcript-specific logic.
func (a *SessionAnalyzer) processMainFile(file *TranscriptFile) {
	timestampByUUID := file.BuildTimestampMap()

	for _, line := range file.Lines {
		a.result.TotalMessages++

		if line.IsUserMessage() {
			a.result.UserMessages++
			if line.IsHumanMessage() {
				a.result.HumanPrompts++
			} else if line.IsToolResultMessage() {
				a.result.ToolResults++
			}
		}

		ts, err := line.GetTimestamp()
		if err == nil {
			if a.firstTimestamp == nil || ts.Before(*a.firstTimestamp) {
				a.firstTimestamp = &ts
			}
			if a.lastTimestamp == nil || ts.After(*a.lastTimestamp) {
				a.lastTimestamp = &ts
			}
		}

		if line.IsCompactBoundary() && line.CompactMetadata != nil {
			switch line.CompactMetadata.Trigger {
			case "auto":
				a.result.CompactionAuto++
				if line.LogicalParentUUID != "" {
					if parentTime, ok := timestampByUUID[line.LogicalParentUUID]; ok {
						if compactTime, err := line.GetTimestamp(); err == nil {
							delta := compactTime.Sub(parentTime).Milliseconds()
							if delta >= 0 {
								a.compactionTimes = append(a.compactionTimes, delta)
							}
						}
					}
				}
			case "manual":
				a.result.CompactionManual++
			}
		}
	}

	// Count assistant messages using deduplicated groups
	for _, group := range file.AssistantMessageGroups() {
		a.result.AssistantMessages++
		if group.HasText {
			a.result.TextResponses++
		}
		if group.HasToolUse {
			a.result.ToolCalls++
		}
		if group.HasThinking {
			a.result.ThinkingBlocks++
		}
	}
}

// Finalize computes derived metrics.
func (a *SessionAnalyzer) Finalize(hasAgentFile func(string) bool) {
	// Compute duration
	if a.firstTimestamp != nil && a.lastTimestamp != nil && !a.firstTimestamp.Equal(*a.lastTimestamp) {
		d := a.lastTimestamp.Sub(*a.firstTimestamp).Milliseconds()
		a.result.DurationMs = &d
	}

	// Compute models list
	a.result.ModelsUsed = make([]string, 0, len(a.modelsUsed))
	for m := range a.modelsUsed {
		a.result.ModelsUsed = append(a.result.ModelsUsed, m)
	}

	// Compute average compaction time
	if len(a.compactionTimes) > 0 {
		var sum int64
		for _, t := range a.compactionTimes {
			sum += t
		}
		avg := int(sum / int64(len(a.compactionTimes)))
		a.result.CompactionAvgTimeMs = &avg
	}
}

// Result returns the accumulated session metrics.
func (a *SessionAnalyzer) Result() *SessionResult {
	return &a.result
}

// Analyze processes the file collection and returns session metrics.
func (a *SessionAnalyzer) Analyze(fc *FileCollection) (*SessionResult, error) {
	a.ProcessFile(fc.Main, true)
	for _, agent := range fc.Agents {
		a.ProcessFile(agent, false)
	}
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}
