package analytics

import "time"

// ConversationResult contains conversation metrics.
type ConversationResult struct {
	// Turn counts
	UserTurns      int
	AssistantTurns int

	// Timing data
	AvgAssistantTurnMs *int64
	AvgUserThinkingMs  *int64
}

// ConversationAnalyzer extracts conversation metrics from transcripts.
// It only processes the main transcript for conversation flow.
//
// Turn semantics:
//   - UserTurns: Count of human prompts (user messages with string content)
//   - AssistantTurns: Count of assistant messages with text content (visible responses)
//
// Turn timing semantics:
//   - Assistant Turn Duration: Time from user prompt to the last assistant message
//     before the next user prompt (total response time including tool calls).
//   - User Thinking Time: Time from the last assistant message to the next user prompt.
type ConversationAnalyzer struct{}

// Analyze processes the file collection and returns conversation metrics.
func (a *ConversationAnalyzer) Analyze(fc *FileCollection) (*ConversationResult, error) {
	result := &ConversationResult{}

	var assistantTurnDurations []int64
	var userThinkingDurations []int64

	// State machine for tracking turn boundaries
	var lastHumanPromptTime *time.Time
	var lastAssistantTime *time.Time
	var hadAssistantResponse bool

	// Only process main transcript for conversation flow
	for _, line := range fc.Main.Lines {
		// Handle human prompts (start of a new user turn)
		if line.IsHumanMessage() {
			result.UserTurns++

			// Timing computation requires timestamp
			ts, err := line.GetTimestamp()
			if err != nil {
				// Still count the turn, but skip timing
				lastHumanPromptTime = nil
				lastAssistantTime = nil
				hadAssistantResponse = false
				continue
			}

			// Close out the previous turn if there was an assistant response
			if lastHumanPromptTime != nil && lastAssistantTime != nil && hadAssistantResponse {
				// Assistant turn duration: user prompt to last assistant message
				duration := lastAssistantTime.Sub(*lastHumanPromptTime).Milliseconds()
				if duration >= 0 {
					assistantTurnDurations = append(assistantTurnDurations, duration)
				}
			}

			// Calculate user thinking time (gap from last assistant to this prompt)
			if lastAssistantTime != nil {
				thinkingTime := ts.Sub(*lastAssistantTime).Milliseconds()
				if thinkingTime >= 0 {
					userThinkingDurations = append(userThinkingDurations, thinkingTime)
				}
			}

			// Reset state for new turn
			lastHumanPromptTime = &ts
			lastAssistantTime = nil
			hadAssistantResponse = false
			continue
		}

		// Handle assistant messages with text (visible responses = assistant turns)
		if line.IsAssistantMessage() && line.HasTextContent() {
			result.AssistantTurns++
		}

		// Handle all assistant messages for timing (including tool-only responses)
		if line.IsAssistantMessage() {
			hadAssistantResponse = true
			if ts, err := line.GetTimestamp(); err == nil {
				lastAssistantTime = &ts
			}
		}
	}

	// Handle any unclosed assistant turn at end of session
	if lastHumanPromptTime != nil && lastAssistantTime != nil && hadAssistantResponse {
		duration := lastAssistantTime.Sub(*lastHumanPromptTime).Milliseconds()
		if duration >= 0 {
			assistantTurnDurations = append(assistantTurnDurations, duration)
		}
	}

	// Compute average assistant turn duration
	if len(assistantTurnDurations) > 0 {
		var sum int64
		for _, d := range assistantTurnDurations {
			sum += d
		}
		avg := sum / int64(len(assistantTurnDurations))
		result.AvgAssistantTurnMs = &avg
	}

	// Compute average user thinking time
	if len(userThinkingDurations) > 0 {
		var sum int64
		for _, d := range userThinkingDurations {
			sum += d
		}
		avg := sum / int64(len(userThinkingDurations))
		result.AvgUserThinkingMs = &avg
	}

	return result, nil
}
