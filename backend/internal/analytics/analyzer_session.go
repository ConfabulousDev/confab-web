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
type SessionAnalyzer struct{}

// Analyze processes the file collection and returns session metrics.
func (a *SessionAnalyzer) Analyze(fc *FileCollection) (*SessionResult, error) {
	result := &SessionResult{}
	modelsUsed := make(map[string]bool)

	var firstTimestamp, lastTimestamp *time.Time
	var compactionTimes []int64

	// Build timestamp map for compaction time calculation
	timestampByUUID := fc.Main.BuildTimestampMap()

	// Only process main transcript for session stats
	for _, line := range fc.Main.Lines {
		// Count all messages
		result.TotalMessages++

		// Count user messages and breakdown
		if line.IsUserMessage() {
			result.UserMessages++

			if line.IsHumanMessage() {
				result.HumanPrompts++
			} else if line.IsToolResultMessage() {
				result.ToolResults++
			}
		}

		// Count assistant messages and breakdown
		if line.IsAssistantMessage() {
			result.AssistantMessages++

			// Track models used
			if model := line.GetModel(); model != "" {
				modelsUsed[model] = true
			}

			// Categorize by content type
			hasText := line.HasTextContent()
			hasToolUse := line.HasToolUse()
			hasThinking := line.HasThinking()

			if hasText {
				result.TextResponses++
			} else if hasToolUse && !hasThinking {
				result.ToolCalls++
			} else if hasThinking && !hasToolUse {
				result.ThinkingBlocks++
			}
		}

		// Track first and last timestamps for duration
		ts, err := line.GetTimestamp()
		if err == nil {
			if firstTimestamp == nil || ts.Before(*firstTimestamp) {
				firstTimestamp = &ts
			}
			if lastTimestamp == nil || ts.After(*lastTimestamp) {
				lastTimestamp = &ts
			}
		}

		// Process compaction events
		if line.IsCompactBoundary() && line.CompactMetadata != nil {
			switch line.CompactMetadata.Trigger {
			case "auto":
				result.CompactionAuto++

				// Calculate compaction time only for auto compactions
				if line.LogicalParentUUID != "" {
					if parentTime, ok := timestampByUUID[line.LogicalParentUUID]; ok {
						if compactTime, err := line.GetTimestamp(); err == nil {
							delta := compactTime.Sub(parentTime).Milliseconds()
							if delta >= 0 {
								compactionTimes = append(compactionTimes, delta)
							}
						}
					}
				}

			case "manual":
				result.CompactionManual++
			}
		}
	}

	// Compute duration
	if firstTimestamp != nil && lastTimestamp != nil && !firstTimestamp.Equal(*lastTimestamp) {
		d := lastTimestamp.Sub(*firstTimestamp).Milliseconds()
		result.DurationMs = &d
	}

	// Collect models from agent files
	for _, agent := range fc.Agents {
		for _, line := range agent.Lines {
			if model := line.GetModel(); model != "" {
				modelsUsed[model] = true
			}
		}
	}

	// Compute models list
	result.ModelsUsed = make([]string, 0, len(modelsUsed))
	for m := range modelsUsed {
		result.ModelsUsed = append(result.ModelsUsed, m)
	}

	// Compute average compaction time
	if len(compactionTimes) > 0 {
		var sum int64
		for _, t := range compactionTimes {
			sum += t
		}
		avg := int(sum / int64(len(compactionTimes)))
		result.CompactionAvgTimeMs = &avg
	}

	return result, nil
}
