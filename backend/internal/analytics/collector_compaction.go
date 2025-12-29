package analytics

// CompactionCollector tracks compaction events and timing.
type CompactionCollector struct {
	AutoCount   int
	ManualCount int
	AvgTimeMs   *int

	// Internal state for averaging
	compactionTimes []int64
}

// NewCompactionCollector creates a new CompactionCollector.
func NewCompactionCollector() *CompactionCollector {
	return &CompactionCollector{}
}

// Collect processes a line for compaction metrics.
func (c *CompactionCollector) Collect(line *TranscriptLine, ctx *CollectContext) {
	if !line.IsCompactBoundary() {
		return
	}

	if line.CompactMetadata == nil {
		return
	}

	switch line.CompactMetadata.Trigger {
	case "auto":
		c.AutoCount++

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
		c.ManualCount++
	}
}

// Finalize computes the average compaction time.
func (c *CompactionCollector) Finalize(ctx *CollectContext) {
	if len(c.compactionTimes) == 0 {
		return
	}

	var sum int64
	for _, t := range c.compactionTimes {
		sum += t
	}
	avg := int(sum / int64(len(c.compactionTimes)))
	c.AvgTimeMs = &avg
}
