package analytics

import "time"

// SessionCollector extracts session-level metrics from transcript lines.
type SessionCollector struct {
	UserTurns      int
	AssistantTurns int
	ModelsUsed     map[string]bool
	firstTimestamp *time.Time
	lastTimestamp  *time.Time
}

// NewSessionCollector creates a new session collector.
func NewSessionCollector() *SessionCollector {
	return &SessionCollector{
		ModelsUsed: make(map[string]bool),
	}
}

// Collect processes a single line for session metrics.
func (c *SessionCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	// Count turns
	if line.IsUserMessage() {
		c.UserTurns++
	}
	if line.IsAssistantMessage() {
		c.AssistantTurns++

		// Track models used
		if model := line.GetModel(); model != "" {
			c.ModelsUsed[model] = true
		}
	}

	// Track first and last timestamps for duration
	ts, err := line.GetTimestamp()
	if err != nil {
		return
	}
	if c.firstTimestamp == nil || ts.Before(*c.firstTimestamp) {
		c.firstTimestamp = &ts
	}
	if c.lastTimestamp == nil || ts.After(*c.lastTimestamp) {
		c.lastTimestamp = &ts
	}
}

// Finalize is called after all lines are processed.
func (c *SessionCollector) Finalize(ctx *CollectContext) {
	// No post-processing needed
}

// DurationMs returns the session duration in milliseconds, or nil if not computable.
func (c *SessionCollector) DurationMs() *int64 {
	if c.firstTimestamp == nil || c.lastTimestamp == nil {
		return nil
	}
	if c.firstTimestamp.Equal(*c.lastTimestamp) {
		return nil
	}
	d := c.lastTimestamp.Sub(*c.firstTimestamp).Milliseconds()
	return &d
}

// ModelsList returns a list of unique model IDs used.
func (c *SessionCollector) ModelsList() []string {
	models := make([]string, 0, len(c.ModelsUsed))
	for m := range c.ModelsUsed {
		models = append(models, m)
	}
	return models
}
