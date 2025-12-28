package analytics

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// ComputeResult contains the computed analytics from JSONL content.
type ComputeResult struct {
	// Token stats
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64

	// Cost
	EstimatedCostUSD decimal.Decimal

	// Compaction stats
	CompactionAuto      int
	CompactionManual    int
	CompactionAvgTimeMs *int
}

// ComputeFromJSONL computes analytics from JSONL content.
// It processes lines incrementally without loading all data into memory.
func ComputeFromJSONL(content []byte) (*ComputeResult, error) {
	result := &ComputeResult{
		EstimatedCostUSD: decimal.Zero,
	}

	// Build a map of uuid → timestamp for compaction time calculation
	timestampByUUID := make(map[string]time.Time)

	// Track compaction times for averaging
	var compactionTimes []int64

	scanner := bufio.NewScanner(bytes.NewReader(content))
	// Increase buffer size for large lines (some assistant messages can be huge)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		line, err := ParseLine(scanner.Bytes())
		if err != nil {
			// Skip unparseable lines (e.g., malformed JSON)
			continue
		}

		// Store timestamp for UUID lookup (needed for compaction time calculation)
		if line.UUID != "" && line.Timestamp != "" {
			if ts, err := line.GetTimestamp(); err == nil {
				timestampByUUID[line.UUID] = ts
			}
		}

		// Process assistant messages for token stats
		if line.IsAssistantMessage() {
			usage := line.Message.Usage
			result.InputTokens += usage.InputTokens
			result.OutputTokens += usage.OutputTokens
			result.CacheCreationTokens += usage.CacheCreationInputTokens
			result.CacheReadTokens += usage.CacheReadInputTokens

			// Calculate cost for this message
			pricing := GetPricing(line.Message.Model)
			cost := CalculateCost(
				pricing,
				usage.InputTokens,
				usage.OutputTokens,
				usage.CacheCreationInputTokens,
				usage.CacheReadInputTokens,
			)
			result.EstimatedCostUSD = result.EstimatedCostUSD.Add(cost)
		}

		// Process compaction boundaries
		if line.IsCompactBoundary() {
			if line.CompactMetadata != nil {
				switch line.CompactMetadata.Trigger {
				case "auto":
					result.CompactionAuto++

					// Calculate compaction time only for auto compactions
					// Manual compactions include user think time, not just processing time
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
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}

	// Calculate average compaction time
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

// ToSessionAnalytics converts a ComputeResult to a SessionAnalytics model.
func (r *ComputeResult) ToSessionAnalytics(sessionID string, analyticsVersion int, upToLine int64) *SessionAnalytics {
	return &SessionAnalytics{
		SessionID:           sessionID,
		AnalyticsVersion:    analyticsVersion,
		UpToLine:            upToLine,
		ComputedAt:          time.Now().UTC(),
		InputTokens:         r.InputTokens,
		OutputTokens:        r.OutputTokens,
		CacheCreationTokens: r.CacheCreationTokens,
		CacheReadTokens:     r.CacheReadTokens,
		EstimatedCostUSD:    r.EstimatedCostUSD,
		CompactionAuto:      r.CompactionAuto,
		CompactionManual:    r.CompactionManual,
		CompactionAvgTimeMs: r.CompactionAvgTimeMs,
		Details:             make(map[string]interface{}),
	}
}

// ToResponse converts a SessionAnalytics to an API response.
func (s *SessionAnalytics) ToResponse() *AnalyticsResponse {
	return &AnalyticsResponse{
		ComputedAt:    s.ComputedAt,
		ComputedLines: s.UpToLine,
		Tokens: TokenStats{
			Input:         s.InputTokens,
			Output:        s.OutputTokens,
			CacheCreation: s.CacheCreationTokens,
			CacheRead:     s.CacheReadTokens,
		},
		Cost: CostStats{
			EstimatedUSD: s.EstimatedCostUSD,
		},
		Compaction: CompactionInfo{
			Auto:      s.CompactionAuto,
			Manual:    s.CompactionManual,
			AvgTimeMs: s.CompactionAvgTimeMs,
		},
	}
}
