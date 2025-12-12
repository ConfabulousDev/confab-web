# Measuring Compaction Time in Claude Code Transcripts

## Background

When a Claude Code session exceeds context limits, compaction occurs. This process summarizes the conversation history to free up context space. Understanding compaction timing could help users:
- Identify sessions with excessive compaction overhead
- Understand the relationship between conversation length and compaction frequency
- Optimize their workflow to minimize compaction disruptions

## JSONL Structure Around Compaction

A compaction event produces this sequence in the transcript:

```
Line N-1: Last pre-compaction entry (user or assistant message)
          UUID: "746ecd97-..."
          timestamp: "2025-12-11T16:53:39.890Z"

Line N:   compact_boundary (system message)
          UUID: "2f6724c6-..."
          parentUuid: null  ← breaks the chain
          logicalParentUuid: "746ecd97-..."  ← points to line N-1
          timestamp: "2025-12-11T16:54:33.387Z"
          compactMetadata: { trigger: "auto", preTokens: 161305 }

Line N+1: Summary message (user type, but synthetic)
          UUID: "1f3cd4dd-..."
          parentUuid: "2f6724c6-..."  ← points to compact_boundary
          timestamp: "2025-12-11T16:54:33.387Z"  ← same as compact_boundary
          isCompactSummary: true
          isVisibleInTranscriptOnly: true
          content: "This session is being continued from a previous conversation..."

Line N+2: First post-compaction assistant response
          UUID: "0a700dc3-..."
          parentUuid: "1f3cd4dd-..."  ← points to summary
          timestamp: "2025-12-11T16:54:37.602Z"
```

## What "Compaction Time" Could Mean

### Option A: Time Until Compaction Triggered

**Definition**: Time from the last user/assistant message to when compaction was triggered.

**Calculation**: `compact_boundary.timestamp - logicalParent.timestamp`

**What it measures**: How long the user/assistant continued working before hitting the context limit. This is more about session pacing than compaction performance.

**Usefulness**: Low. This doesn't measure compaction itself—it measures the gap before compaction happened, which varies based on user activity.

### Option B: Compaction + Summary Generation Time

**Definition**: Time from compact_boundary to the first post-summary assistant response.

**Calculation**: `firstPostSummaryAssistant.timestamp - compact_boundary.timestamp`

**What it measures**: The combined time for:
1. Generating the conversation summary (server-side)
2. Claude processing the summary and generating a response

**Problem**: This conflates two distinct operations. The summary generation is the "compaction" part, but we can't separate it from the subsequent response time.

**Example from real data**:
- compact_boundary: 16:54:33.387Z
- First assistant response: 16:54:37.602Z
- Measured time: ~4.2 seconds

### Option C: Summary Message Timestamp Delta

**Definition**: Time between compact_boundary and summary message timestamps.

**Calculation**: `summaryMessage.timestamp - compact_boundary.timestamp`

**Problem**: In practice, these timestamps are identical (both "2025-12-11T16:54:33.387Z" in our sample). The summary message appears to be generated synchronously with the compact_boundary entry, so this delta is always ~0.

### Option D: Infer from preTokens

**Definition**: Instead of time, measure the "weight" of compaction by looking at how many tokens were compressed.

**Calculation**: Use `compactMetadata.preTokens` directly.

**What it measures**: The size of the context that was summarized. Larger preTokens values indicate more substantial compaction events.

**Usefulness**: Moderate. Doesn't measure time, but indicates compaction "effort". Could correlate with actual compaction time on Anthropic's servers.

## Challenges

### 1. No Explicit End Marker

The transcript doesn't record when compaction/summarization actually completed. We only see:
- When compaction started (compact_boundary timestamp)
- When the next assistant response arrived

### 2. Timestamps May Be Client-Side

It's unclear whether timestamps represent:
- When the event was written to the JSONL file (client-side)
- When the event occurred on Anthropic's servers

If client-side, network latency could skew measurements.

### 3. User Think Time Conflation

Option B includes any time the user spent reading the summary before the assistant responded. If the assistant auto-continues (as in our sample), this is minimal. But in interactive sessions, this could add significant variance.

### 4. Summary Generation is Server-Side

The actual summarization happens on Anthropic's infrastructure. We only observe the results in the transcript, not the process itself.

## Recommendation

### For MVP: Skip Timing, Use Counts + preTokens

The current implementation (Total/Auto/Manual counts) is solid. Consider adding:

```typescript
interface CompactionStats {
  total: number;
  auto: number;
  manual: number;
  // New fields:
  totalPreTokens: number;      // Sum of all preTokens
  avgPreTokens: number;        // Average tokens before compaction
  maxPreTokens: number;        // Largest compaction event
}
```

This gives users insight into compaction "weight" without the ambiguity of time measurement.

### For Future: Option B with Caveats

If we want to show timing, Option B (compact_boundary → first assistant response) is the most practical:

```typescript
interface CompactionStats {
  // ... existing fields ...

  // Timing (with caveats)
  avgResponseTimeMs: number;   // Average time to first response after compaction
  maxResponseTimeMs: number;   // Longest wait after compaction
}
```

**Important**: Label this as "Time to first response after compaction" not "Compaction time" to avoid implying we're measuring the summarization process itself.

### Display Considerations

If showing timing stats:
- Use tooltips to explain what's actually being measured
- Consider showing as a range or percentiles rather than a single average
- Flag outliers that might indicate user think time (e.g., >30s)

## Data Model Changes

To implement timing, we'd need to:

1. **During parsing**: Track the timestamp of each compact_boundary
2. **Find next assistant**: Locate the first assistant message after the summary
3. **Calculate delta**: Subtract timestamps
4. **Handle edge cases**:
   - Compaction at end of session (no subsequent assistant message)
   - Multiple rapid compactions
   - Missing timestamps

## Conclusion

Compaction "time" is inherently ambiguous in the transcript format. The safest approach is:

1. **Now**: Show counts (Total/Auto/Manual) ✓ Done
2. **Soon**: Add preTokens statistics (total, avg, max)
3. **Later**: Add "time to response after compaction" with clear labeling

This gives users actionable insights without overpromising on what we can actually measure.
