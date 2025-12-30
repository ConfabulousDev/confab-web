package analytics

import "time"

// ConversationCollector extracts conversation metrics from transcript lines.
// It counts turns and computes average durations for assistant turns and user thinking time.
//
// Turn semantics:
//   - UserTurns: Count of human prompts (user messages with string content)
//   - AssistantTurns: Count of assistant messages with text content (visible responses)
//
// Turn timing semantics:
//   - Assistant Turn Duration: Time from user prompt to the last assistant message
//     before the next user prompt (total response time including tool calls).
//   - User Thinking Time: Time from the last assistant message to the next user prompt.
//
// The collector tracks timestamps as it processes lines and computes averages in Finalize.
type ConversationCollector struct {
	// Turn counts
	UserTurns      int // Count of human prompts
	AssistantTurns int // Count of assistant text responses

	// Timing data (computed in Finalize)
	AvgAssistantTurnMs *int64
	AvgUserThinkingMs  *int64

	// Internal state for timing computation
	assistantTurnDurations []int64 // Duration of each assistant turn in ms
	userThinkingDurations  []int64 // Duration of each user thinking period in ms

	// State machine for tracking turn boundaries
	lastHumanPromptTime *time.Time // Timestamp of the most recent human prompt
	lastAssistantTime   *time.Time // Last assistant message seen
	hadAssistantResponse bool      // Whether we've seen any assistant response since last user prompt
}

// NewConversationCollector creates a new conversation collector.
func NewConversationCollector() *ConversationCollector {
	return &ConversationCollector{}
}

// Collect processes a single line for conversation metrics.
func (c *ConversationCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	// Handle human prompts (start of a new user turn)
	if line.IsHumanMessage() {
		c.UserTurns++

		// Timing computation requires timestamp
		ts, err := line.GetTimestamp()
		if err != nil {
			// Still count the turn, but skip timing
			c.lastHumanPromptTime = nil
			c.lastAssistantTime = nil
			c.hadAssistantResponse = false
			return
		}

		// Close out the previous turn if there was an assistant response
		if c.lastHumanPromptTime != nil && c.lastAssistantTime != nil && c.hadAssistantResponse {
			// Assistant turn duration: user prompt to last assistant message
			duration := c.lastAssistantTime.Sub(*c.lastHumanPromptTime).Milliseconds()
			if duration >= 0 {
				c.assistantTurnDurations = append(c.assistantTurnDurations, duration)
			}
		}

		// Calculate user thinking time (gap from last assistant to this prompt)
		if c.lastAssistantTime != nil {
			thinkingTime := ts.Sub(*c.lastAssistantTime).Milliseconds()
			if thinkingTime >= 0 {
				c.userThinkingDurations = append(c.userThinkingDurations, thinkingTime)
			}
		}

		// Reset state for new turn
		c.lastHumanPromptTime = &ts
		c.lastAssistantTime = nil
		c.hadAssistantResponse = false
		return
	}

	// Handle assistant messages with text (visible responses = assistant turns)
	if line.IsAssistantMessage() && line.HasTextContent() {
		c.AssistantTurns++
	}

	// Handle all assistant messages for timing (including tool-only responses)
	if line.IsAssistantMessage() {
		c.hadAssistantResponse = true
		if ts, err := line.GetTimestamp(); err == nil {
			c.lastAssistantTime = &ts
		}
	}

	// Tool results don't affect turn boundaries - they're part of the assistant turn
}

// Finalize computes averages after all lines are processed.
func (c *ConversationCollector) Finalize(ctx *CollectContext) {
	// Handle any unclosed assistant turn at end of session
	if c.lastHumanPromptTime != nil && c.lastAssistantTime != nil && c.hadAssistantResponse {
		duration := c.lastAssistantTime.Sub(*c.lastHumanPromptTime).Milliseconds()
		if duration >= 0 {
			c.assistantTurnDurations = append(c.assistantTurnDurations, duration)
		}
	}

	// Compute average assistant turn duration
	if len(c.assistantTurnDurations) > 0 {
		var sum int64
		for _, d := range c.assistantTurnDurations {
			sum += d
		}
		avg := sum / int64(len(c.assistantTurnDurations))
		c.AvgAssistantTurnMs = &avg
	}

	// Compute average user thinking time
	if len(c.userThinkingDurations) > 0 {
		var sum int64
		for _, d := range c.userThinkingDurations {
			sum += d
		}
		avg := sum / int64(len(c.userThinkingDurations))
		c.AvgUserThinkingMs = &avg
	}
}
