package analytics

import (
	"bufio"
	"bytes"
	"fmt"
	"time"
)

// CollectContext provides shared state during the single-pass collection.
// Collectors can read from and write to this context.
type CollectContext struct {
	// TimestampByUUID maps message UUIDs to their timestamps.
	// Built incrementally during the pass, used by collectors that need
	// to reference other messages (e.g., compaction time calculation).
	TimestampByUUID map[string]time.Time

	// ToolUseIDToName maps tool_use IDs to tool names.
	// Built from tool_use blocks, used to attribute tool_result errors
	// to specific tools.
	ToolUseIDToName map[string]string

	// LineCount tracks the total number of lines processed.
	LineCount int64
}

// Collector processes transcript lines and accumulates metrics.
// Each card type implements this interface.
type Collector interface {
	// Collect is called for each parsed line during the single pass.
	// The context provides shared state like timestamp lookups.
	Collect(line *TranscriptLine, ctx *CollectContext)

	// Finalize is called after all lines have been processed.
	// Use this for post-processing like computing averages.
	Finalize(ctx *CollectContext)
}

// RunCollectors performs a single pass through JSONL content,
// invoking all registered collectors for each line.
func RunCollectors(content []byte, collectors ...Collector) (*CollectContext, error) {
	ctx := &CollectContext{
		TimestampByUUID: make(map[string]time.Time),
		ToolUseIDToName: make(map[string]string),
	}

	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer size for large lines (some assistant messages can be huge)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		ctx.LineCount++

		line, err := ParseLine(scanner.Bytes())
		if err != nil {
			// Skip unparseable lines (e.g., malformed JSON)
			continue
		}

		// Build timestamp map for cross-reference lookups
		if line.UUID != "" && line.Timestamp != "" {
			if ts, err := line.GetTimestamp(); err == nil {
				ctx.TimestampByUUID[line.UUID] = ts
			}
		}

		// Invoke each collector
		for _, c := range collectors {
			c.Collect(line, ctx)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}

	// Finalize all collectors
	for _, c := range collectors {
		c.Finalize(ctx)
	}

	return ctx, nil
}
