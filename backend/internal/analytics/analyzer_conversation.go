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
type ConversationAnalyzer struct {
	result ConversationResult
}

// ProcessFile processes a single file. Only the main file is used.
func (a *ConversationAnalyzer) ProcessFile(file *TranscriptFile, isMain bool) {
	if !isMain {
		return
	}

	var assistantTurnDurations []int64
	var userThinkingDurations []int64

	var lastHumanPromptTime *time.Time
	var lastAssistantTime *time.Time
	var hadAssistantResponse bool

	seenMessageIDs := make(map[string]bool)

	for _, line := range file.Lines {
		if line.IsHumanMessage() {
			a.result.UserTurns++

			if hadAssistantResponse {
				a.result.AssistantTurns++
			}

			ts, err := line.GetTimestamp()
			if err != nil {
				lastHumanPromptTime = nil
				lastAssistantTime = nil
				hadAssistantResponse = false
				continue
			}

			if lastHumanPromptTime != nil && lastAssistantTime != nil && hadAssistantResponse {
				duration := lastAssistantTime.Sub(*lastHumanPromptTime).Milliseconds()
				if duration >= 0 {
					assistantTurnDurations = append(assistantTurnDurations, duration)
				}
			}

			if lastAssistantTime != nil {
				thinkingTime := ts.Sub(*lastAssistantTime).Milliseconds()
				if thinkingTime >= 0 {
					userThinkingDurations = append(userThinkingDurations, thinkingTime)
				}
			}

			lastHumanPromptTime = &ts
			lastAssistantTime = nil
			hadAssistantResponse = false
			continue
		}

		if line.Type == "assistant" && line.Message != nil {
			msgID := line.GetMessageID()
			if msgID == "" || !seenMessageIDs[msgID] {
				if msgID != "" {
					seenMessageIDs[msgID] = true
				}
				hadAssistantResponse = true
			}

			if ts, err := line.GetTimestamp(); err == nil {
				lastAssistantTime = &ts
			}
		}
	}

	// Handle unclosed turn at end of session
	if hadAssistantResponse {
		a.result.AssistantTurns++
	}
	if lastHumanPromptTime != nil && lastAssistantTime != nil && hadAssistantResponse {
		duration := lastAssistantTime.Sub(*lastHumanPromptTime).Milliseconds()
		if duration >= 0 {
			assistantTurnDurations = append(assistantTurnDurations, duration)
		}
	}

	// Compute timing stats
	if len(assistantTurnDurations) > 0 {
		var sum int64
		for _, d := range assistantTurnDurations {
			sum += d
		}
		avg := sum / int64(len(assistantTurnDurations))
		a.result.AvgAssistantTurnMs = &avg
		a.result.TotalAssistantDurationMs = &sum
	}

	if len(userThinkingDurations) > 0 {
		var sum int64
		for _, d := range userThinkingDurations {
			sum += d
		}
		avg := sum / int64(len(userThinkingDurations))
		a.result.AvgUserThinkingMs = &avg
		a.result.TotalUserDurationMs = &sum
	}

	if a.result.TotalAssistantDurationMs != nil && a.result.TotalUserDurationMs != nil {
		totalTime := float64(*a.result.TotalAssistantDurationMs + *a.result.TotalUserDurationMs)
		if totalTime > 0 {
			utilization := float64(*a.result.TotalAssistantDurationMs) / totalTime * 100
			a.result.AssistantUtilizationPct = &utilization
		}
	}
}

// Finalize is a no-op for conversation (main-only analyzer).
func (a *ConversationAnalyzer) Finalize(hasAgentFile func(string) bool) {}

// Result returns the accumulated conversation metrics.
func (a *ConversationAnalyzer) Result() *ConversationResult {
	return &a.result
}

// Analyze processes the file collection and returns conversation metrics.
func (a *ConversationAnalyzer) Analyze(fc *FileCollection) (*ConversationResult, error) {
	a.ProcessFile(fc.Main, true)
	a.Finalize(fc.HasAgentFile)
	return a.Result(), nil
}
