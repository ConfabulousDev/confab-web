package analytics

import "time"

// SessionCollector extracts session-level metrics from transcript lines.
// This includes turn counts, duration, models used, and compaction statistics.
type SessionCollector struct {
	// Turn counts
	UserTurns      int
	AssistantTurns int

	// Models tracking
	ModelsUsed map[string]bool

	// Timestamps for duration calculation
	firstTimestamp *time.Time
	lastTimestamp  *time.Time

	// Compaction stats (merged from CompactionCollector)
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int
	compactionTimes     []int64 // Internal state for averaging
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
	if err == nil {
		if c.firstTimestamp == nil || ts.Before(*c.firstTimestamp) {
			c.firstTimestamp = &ts
		}
		if c.lastTimestamp == nil || ts.After(*c.lastTimestamp) {
			c.lastTimestamp = &ts
		}
	}

	// Process compaction events
	c.collectCompaction(line, ctx)
}

// collectCompaction handles compaction-specific collection (merged from CompactionCollector).
func (c *SessionCollector) collectCompaction(line *TranscriptLine, ctx *CollectContext) {
	if !line.IsCompactBoundary() {
		return
	}

	if line.CompactMetadata == nil {
		return
	}

	switch line.CompactMetadata.Trigger {
	case "auto":
		c.CompactionAuto++

		// Calculate compaction time only for auto compactions.
		// Manual compactions include user think time, not just processing time.
		if line.LogicalParentUUID != "" {
			if parentTime, ok := ctx.TimestampByUUID[line.LogicalParentUUID]; ok {
				if compactTime, err := line.GetTimestamp(); err == nil {
					delta := compactTime.Sub(parentTime).Milliseconds()
					if delta >= 0 {
						c.compactionTimes = append(c.compactionTimes, delta)
					}
				}
			}
		}

	case "manual":
		c.CompactionManual++
	}
}

// Finalize is called after all lines are processed.
func (c *SessionCollector) Finalize(ctx *CollectContext) {
	// Compute average compaction time
	if len(c.compactionTimes) > 0 {
		var sum int64
		for _, t := range c.compactionTimes {
			sum += t
		}
		avg := int(sum / int64(len(c.compactionTimes)))
		c.CompactionAvgTimeMs = &avg
	}
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
