package analytics

import "time"

// ConversationResult contains conversation metrics.
type ConversationResult struct {
	// Turn counts
	UserTurns      int
	AssistantTurns int

	// Timing data - averages
	AvgAssistantTurnMs *int64
	AvgUserThinkingMs  *int64

	// Timing data - totals
	TotalAssistantDurationMs *int64
	TotalUserDurationMs      *int64

	// Utilization percentage (assistant time / total time * 100)
	AssistantUtilizationPct *float64
}

// ConversationAnalyzer extracts conversation metrics from transcripts.
// It only processes the main transcript for conversation flow.
//
// Turn semantics:
//   - UserTurns: Count of human prompts (user messages with string content)
//   - AssistantTurns: Count of user-prompt-triggered sequences that received at
//     least one assistant response (deduplicated by message.id to avoid
//     over-counting from multi-line-per-response and context replay).
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

	// Track seen message IDs for deduplication.
	// NOTE: This mirrors the message-ID dedup logic in AssistantMessageGroups(),
	// but is done inline here because we need to track per-turn state
	// (hadAssistantResponse must reset at each user-turn boundary).
	seenMessageIDs := make(map[string]bool)

	// Only process main transcript for conversation flow
	for _, line := range fc.Main.Lines {
		// Handle human prompts (start of a new user turn)
		if line.IsHumanMessage() {
			result.UserTurns++

			// If previous turn had assistant responses, count it as an assistant turn
			if hadAssistantResponse {
				result.AssistantTurns++
			}

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

		// Handle assistant messages for timing and turn tracking.
		// Deduplicate by message ID to avoid over-counting from
		// multi-line-per-response and context replay.
		if line.Type == "assistant" && line.Message != nil {
			msgID := line.GetMessageID()
			if msgID == "" || !seenMessageIDs[msgID] {
				if msgID != "" {
					seenMessageIDs[msgID] = true
				}
				// Only count unique assistant responses for hadAssistantResponse
				hadAssistantResponse = true
			}

			// Always track timing from latest timestamp (even replays have timestamps)
			if ts, err := line.GetTimestamp(); err == nil {
				lastAssistantTime = &ts
			}
		}
	}

	// Handle any unclosed assistant turn at end of session
	if hadAssistantResponse {
		result.AssistantTurns++
	}
	if lastHumanPromptTime != nil && lastAssistantTime != nil && hadAssistantResponse {
		duration := lastAssistantTime.Sub(*lastHumanPromptTime).Milliseconds()
		if duration >= 0 {
			assistantTurnDurations = append(assistantTurnDurations, duration)
		}
	}

	// Compute assistant turn duration stats
	if len(assistantTurnDurations) > 0 {
		var sum int64
		for _, d := range assistantTurnDurations {
			sum += d
		}
		avg := sum / int64(len(assistantTurnDurations))
		result.AvgAssistantTurnMs = &avg
		result.TotalAssistantDurationMs = &sum
	}

	// Compute user thinking time stats
	if len(userThinkingDurations) > 0 {
		var sum int64
		for _, d := range userThinkingDurations {
			sum += d
		}
		avg := sum / int64(len(userThinkingDurations))
		result.AvgUserThinkingMs = &avg
		result.TotalUserDurationMs = &sum
	}

	// Compute assistant utilization (% of time Claude was working vs user thinking)
	if result.TotalAssistantDurationMs != nil && result.TotalUserDurationMs != nil {
		totalTime := float64(*result.TotalAssistantDurationMs + *result.TotalUserDurationMs)
		if totalTime > 0 {
			utilization := float64(*result.TotalAssistantDurationMs) / totalTime * 100
			result.AssistantUtilizationPct = &utilization
		}
	}

	return result, nil
}
