import { useMemo, useCallback, useState, useRef } from 'react';
import type { TranscriptLine } from '@/types';
import { isUserMessage, isAssistantMessage, isToolResultMessage } from '@/types';
import styles from './TimelineBar.module.css';

type Speaker = 'user' | 'assistant';

interface TimelineSegment {
  speaker: Speaker;
  durationMs: number;
  startIndex: number; // Index into messages array for scrolling
  endIndex: number;
  messageCount: number; // Number of messages in this segment
}

interface TimelineBarProps {
  messages: TranscriptLine[];
  /** Index of the currently selected/active message (drives position indicator) */
  selectedIndex: number;
  /** Set of message indices that are currently visible (not filtered out) */
  visibleIndices?: Set<number>;
  /** Callback when user clicks on the timeline to scroll */
  onSeek: (startIndex: number, endIndex: number) => void;
}

/**
 * Check if a user message is a human prompt (not a tool result)
 */
function isHumanPrompt(line: TranscriptLine): boolean {
  if (!isUserMessage(line)) return false;
  // After isUserMessage check, line is narrowed to UserMessage
  return !isToolResultMessage(line);
}

/**
 * Compute timeline segments from transcript messages.
 * Each segment represents contiguous time for one speaker.
 */
function computeSegments(messages: TranscriptLine[]): TimelineSegment[] {
  const segments: TimelineSegment[] = [];

  let lastHumanPromptTime: Date | null = null;
  let firstAssistantIndex: number | null = null; // First assistant msg in current turn
  let lastAssistantTime: Date | null = null;
  let lastAssistantIndex: number | null = null;
  let hadAssistantResponse = false;

  for (let i = 0; i < messages.length; i++) {
    const line = messages[i];
    if (!line) continue;

    // Handle human prompts (start of a new user turn)
    if (isHumanPrompt(line)) {
      const timestamp = 'timestamp' in line && typeof line.timestamp === 'string' ? line.timestamp : null;
      if (!timestamp) {
        // Can't compute timing without timestamp - reset state
        lastHumanPromptTime = null;
        firstAssistantIndex = null;
        lastAssistantTime = null;
        lastAssistantIndex = null;
        hadAssistantResponse = false;
        continue;
      }

      const ts = new Date(timestamp);

      // Close out the previous assistant segment if there was one
      if (lastHumanPromptTime && lastAssistantTime && hadAssistantResponse && firstAssistantIndex !== null && lastAssistantIndex !== null) {
        const duration = lastAssistantTime.getTime() - lastHumanPromptTime.getTime();
        if (duration > 0) {
          segments.push({
            speaker: 'assistant',
            durationMs: duration,
            startIndex: firstAssistantIndex, // Start at first assistant message
            endIndex: lastAssistantIndex,
            messageCount: lastAssistantIndex - firstAssistantIndex + 1,
          });
        }
      }

      // Calculate user thinking time (gap from last assistant to this prompt)
      if (lastAssistantTime && lastAssistantIndex !== null) {
        const userDuration = ts.getTime() - lastAssistantTime.getTime();
        if (userDuration > 0) {
          segments.push({
            speaker: 'user',
            durationMs: userDuration,
            startIndex: i, // Start at the user's prompt
            endIndex: i,
            messageCount: 1,
          });
        }
      } else if (segments.length === 0) {
        // First human prompt - create a minimal user segment so there's something to click
        segments.push({
          speaker: 'user',
          durationMs: 1000, // Nominal 1 second
          startIndex: i,
          endIndex: i,
          messageCount: 1,
        });
      }

      // Reset state for new turn
      lastHumanPromptTime = ts;
      firstAssistantIndex = null;
      lastAssistantTime = null;
      lastAssistantIndex = null;
      hadAssistantResponse = false;
      continue;
    }

    // Track assistant message timestamps
    if (isAssistantMessage(line)) {
      hadAssistantResponse = true;
      const timestamp = 'timestamp' in line ? line.timestamp : null;
      if (timestamp) {
        if (firstAssistantIndex === null) {
          firstAssistantIndex = i; // Track first assistant message in turn
        }
        lastAssistantTime = new Date(timestamp);
        lastAssistantIndex = i;
      }
    }
  }

  // Handle any unclosed assistant segment at end of session
  if (lastHumanPromptTime && lastAssistantTime && hadAssistantResponse && firstAssistantIndex !== null && lastAssistantIndex !== null) {
    const duration = lastAssistantTime.getTime() - lastHumanPromptTime.getTime();
    if (duration > 0) {
      segments.push({
        speaker: 'assistant',
        durationMs: duration,
        startIndex: firstAssistantIndex,
        endIndex: lastAssistantIndex,
        messageCount: lastAssistantIndex - firstAssistantIndex + 1,
      });
    }
  }

  return segments;
}

/**
 * Format duration for timeline tooltip display.
 *
 * NOTE: This variant differs from utils/formatting.ts and SessionCard:
 * - Shows "5m 30s" (includes seconds for timing precision)
 * - Shows "500ms" for sub-second durations (useful for quick messages)
 *
 * Same implementation as ConversationCard - both need precise timing display.
 */
function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    const remainingSeconds = seconds % 60;
    return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
  }
  if (seconds > 0) {
    return `${seconds}s`;
  }
  return `${ms}ms`;
}

export function TimelineBar({ messages, selectedIndex, visibleIndices, onSeek }: TimelineBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<TimelineSegment | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const segments = useMemo(() => computeSegments(messages), [messages]);

  // Calculate effective size for each segment (blend of time and message count)
  // This reduces variance and makes segments easier to click
  const segmentSizes = useMemo(() => {
    // Weight factors: blend duration with message count
    const TIME_WEIGHT = 0.6;
    const MESSAGE_WEIGHT = 0.4;
    const MS_PER_MESSAGE = 10000; // Assume ~10 seconds per message as baseline

    return segments.map(seg => {
      const timeComponent = seg.durationMs;
      const messageComponent = seg.messageCount * MS_PER_MESSAGE;
      return timeComponent * TIME_WEIGHT + messageComponent * MESSAGE_WEIGHT;
    });
  }, [segments]);

  const totalSize = useMemo(
    () => segmentSizes.reduce((sum, size) => sum + size, 0),
    [segmentSizes]
  );

  // Minimum segment height for visibility (as percentage)
  const MIN_SEGMENT_PERCENT = 2;

  // Calculate display percentages with minimum height applied
  // IMPORTANT: Use these for both rendering AND position calculation to stay in sync
  const displayPercents = useMemo(() => {
    if (totalSize === 0) return segments.map(() => 0);

    const rawPercents = segmentSizes.map(size => (size / totalSize) * 100);
    // Apply minimum height
    return rawPercents.map(p => Math.max(p, MIN_SEGMENT_PERCENT));
  }, [segments, segmentSizes, totalSize]);

  // Total display height (may exceed 100% due to minimum heights)
  const totalDisplayPercent = useMemo(
    () => displayPercents.reduce((sum, p) => sum + p, 0),
    [displayPercents]
  );

  // Find which segment contains a given message index
  const findSegmentForIndex = useCallback((messageIndex: number): { segment: TimelineSegment; segmentIndex: number } | null => {
    for (let i = 0; i < segments.length; i++) {
      const segment = segments[i];
      if (!segment) continue;

      // Check if message falls within this segment
      if (messageIndex >= segment.startIndex && messageIndex <= segment.endIndex) {
        return { segment, segmentIndex: i };
      }

      // Check if message falls in gap before this segment (associate with previous segment)
      if (messageIndex < segment.startIndex && i > 0) {
        const prevSegment = segments[i - 1];
        if (prevSegment && messageIndex > prevSegment.endIndex) {
          return { segment: prevSegment, segmentIndex: i - 1 };
        }
      }
    }

    // If past all segments, return last segment
    if (segments.length > 0) {
      const lastIdx = segments.length - 1;
      const lastSegment = segments[lastIdx];
      if (lastSegment && messageIndex > lastSegment.endIndex) {
        return { segment: lastSegment, segmentIndex: lastIdx };
      }
    }

    return null;
  }, [segments]);

  // Calculate the visual start position (as percentage) for a segment
  // Uses displayPercents to match the rendered segment heights
  const getSegmentStartPercent = useCallback((segmentIndex: number): number => {
    let accumulatedPercent = 0;
    for (let i = 0; i < segmentIndex; i++) {
      const displayPct = displayPercents[i];
      if (displayPct !== undefined) {
        // Normalize to 100% total
        accumulatedPercent += (displayPct / totalDisplayPercent) * 100;
      }
    }
    return accumulatedPercent;
  }, [displayPercents, totalDisplayPercent]);

  // Calculate indicator position based on selected message
  // This is the ONLY place position is computed - always derived from selectedIndex
  const indicatorPosition = useMemo(() => {
    if (segments.length === 0 || totalDisplayPercent === 0) {
      return 0;
    }

    // Step 1: Find which segment contains the selected message
    const found = findSegmentForIndex(selectedIndex);
    if (!found) {
      return 0;
    }

    const { segment, segmentIndex } = found;
    const segmentStartPercent = getSegmentStartPercent(segmentIndex);
    // Use display percent (with min height applied) normalized to 100%
    const displayPct = displayPercents[segmentIndex] ?? 0;
    const segmentPercent = (displayPct / totalDisplayPercent) * 100;

    // Step 2: Calculate position within the segment
    // Each message occupies a "band" - position at center of band to avoid overlap
    // with adjacent segments (last msg of segment N != first msg of segment N+1)
    const messageCount = segment.endIndex - segment.startIndex + 1;
    const localIndex = selectedIndex - segment.startIndex;
    // Position at center of this message's band: (localIndex + 0.5) / messageCount
    const positionInSegment = (localIndex + 0.5) / messageCount;

    return segmentStartPercent + (segmentPercent * positionInSegment);
  }, [selectedIndex, segments, displayPercents, totalDisplayPercent, findSegmentForIndex, getSegmentStartPercent]);

  const handleSegmentClick = useCallback(
    (segment: TimelineSegment) => {
      // Pass full range so caller can find first visible message
      onSeek(segment.startIndex, segment.endIndex);
    },
    [onSeek]
  );

  // Determine if a segment has any visible messages
  const isSegmentFiltered = useCallback(
    (segment: TimelineSegment): boolean => {
      if (!visibleIndices) return false; // No filtering active
      for (let i = segment.startIndex; i <= segment.endIndex; i++) {
        if (visibleIndices.has(i)) return false;
      }
      return true; // All messages in segment are filtered out
    },
    [visibleIndices]
  );

  const handleSegmentHover = useCallback(
    (segment: TimelineSegment | null, event?: React.MouseEvent) => {
      setHoveredSegment(segment);
      if (segment && event && barRef.current) {
        const barRect = barRef.current.getBoundingClientRect();
        // Use screen coordinates for fixed positioning
        setTooltipPosition({ top: event.clientY, left: barRect.left });
      }
    },
    []
  );

  // Don't render if no meaningful segments
  if (segments.length === 0 || totalSize === 0) {
    return null;
  }

  return (
    <div className={styles.timelineBar} ref={barRef}>
      <div className={styles.segmentsContainer}>
        {segments.map((segment, index) => {
          // Use pre-computed display percent, normalized to 100%
          const displayPct = displayPercents[index] ?? 0;
          const heightPercent = (displayPct / totalDisplayPercent) * 100;
          const filtered = isSegmentFiltered(segment);

          return (
            <div
              key={index}
              className={`${styles.segment} ${filtered ? styles.filtered : styles[segment.speaker]}`}
              style={{ height: `${heightPercent}%` }}
              onClick={() => handleSegmentClick(segment)}
              onMouseEnter={(e) => handleSegmentHover(segment, e)}
              onMouseMove={(e) => handleSegmentHover(segment, e)}
              onMouseLeave={() => handleSegmentHover(null)}
            />
          );
        })}
      </div>

      {/* Position indicator */}
      <div
        className={styles.positionIndicator}
        style={{ top: `${indicatorPosition}%` }}
      />

      {/* Tooltip */}
      {hoveredSegment && (() => {
        // Count visible messages in this segment
        let visibleCount = hoveredSegment.messageCount;
        if (visibleIndices && visibleIndices.size > 0) {
          visibleCount = 0;
          for (let idx = hoveredSegment.startIndex; idx <= hoveredSegment.endIndex; idx++) {
            if (visibleIndices.has(idx)) visibleCount++;
          }
        }
        const filteredCount = hoveredSegment.messageCount - visibleCount;
        const speaker = hoveredSegment.speaker === 'assistant' ? 'Claude' : 'User';

        const msgLabel = hoveredSegment.messageCount === 1 ? 'msg' : 'msgs';
        const filterLabel = filteredCount > 0 ? ` (${filteredCount} filtered)` : '';
        const tooltipText = `${speaker}: ${formatDuration(hoveredSegment.durationMs)}, ${hoveredSegment.messageCount} ${msgLabel}${filterLabel}`;

        return (
          <div
            className={styles.tooltip}
            style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
          >
            {tooltipText}
          </div>
        );
      })()}
    </div>
  );
}

// Default export only - TimelineBar also exported as named from component
