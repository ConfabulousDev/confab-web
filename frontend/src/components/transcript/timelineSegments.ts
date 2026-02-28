import { useMemo, useCallback } from 'react';
import type { TranscriptLine } from '@/types';
import { isUserMessage, isAssistantMessage, isToolResultMessage } from '@/types';

type Speaker = 'user' | 'assistant';

export interface TimelineSegment {
  speaker: Speaker;
  durationMs: number;
  startIndex: number; // Index into messages array for scrolling
  endIndex: number;
  messageCount: number; // Number of messages in this segment
}

/**
 * Check if a user message is a human prompt (not a tool result)
 */
function isHumanPrompt(line: TranscriptLine): boolean {
  if (!isUserMessage(line)) return false;
  return !isToolResultMessage(line);
}

/**
 * Compute timeline segments from transcript messages.
 * Each segment represents contiguous time for one speaker.
 */
export function computeSegments(messages: TranscriptLine[]): TimelineSegment[] {
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

// --- Shared layout computation for bar components ---

const TIME_WEIGHT = 0.6;
const MESSAGE_WEIGHT = 0.4;
const MS_PER_MESSAGE = 10000;
const MIN_SEGMENT_PERCENT = 2;

export interface SegmentLayout {
  segments: TimelineSegment[];
  /** Visual height percentage for each segment (normalized to sum to 100) */
  heightPercents: number[];
  /** Total blended size across all segments (0 when empty) */
  totalSize: number;
  /** Position indicator percentage for a given selectedIndex */
  indicatorPosition: number;
  /** Find the segment containing a message index */
  findSegmentForIndex: (messageIndex: number) => { segment: TimelineSegment; segmentIndex: number } | null;
}

/**
 * Shared hook that computes segment layout for both TimelineBar and CostBar.
 * Produces identical sizing, display percentages, and position indicator logic.
 */
export function useSegmentLayout(messages: TranscriptLine[], selectedIndex: number): SegmentLayout {
  const segments = useMemo(() => computeSegments(messages), [messages]);

  const segmentSizes = useMemo(
    () => segments.map((seg) => {
      const timeComponent = seg.durationMs;
      const messageComponent = seg.messageCount * MS_PER_MESSAGE;
      return timeComponent * TIME_WEIGHT + messageComponent * MESSAGE_WEIGHT;
    }),
    [segments],
  );

  const totalSize = useMemo(
    () => segmentSizes.reduce((sum, size) => sum + size, 0),
    [segmentSizes],
  );

  const displayPercents = useMemo(() => {
    if (totalSize === 0) return segments.map(() => 0);
    const rawPercents = segmentSizes.map((size) => (size / totalSize) * 100);
    return rawPercents.map((p) => Math.max(p, MIN_SEGMENT_PERCENT));
  }, [segments, segmentSizes, totalSize]);

  const totalDisplayPercent = useMemo(
    () => displayPercents.reduce((sum, p) => sum + p, 0),
    [displayPercents],
  );

  const heightPercents = useMemo(() => {
    if (totalDisplayPercent === 0) return segments.map(() => 0);
    return displayPercents.map((p) => (p / totalDisplayPercent) * 100);
  }, [segments, displayPercents, totalDisplayPercent]);

  const findSegmentForIndex = useCallback(
    (messageIndex: number): { segment: TimelineSegment; segmentIndex: number } | null => {
      for (let i = 0; i < segments.length; i++) {
        const segment = segments[i];
        if (!segment) continue;
        if (messageIndex >= segment.startIndex && messageIndex <= segment.endIndex) {
          return { segment, segmentIndex: i };
        }
        if (messageIndex < segment.startIndex && i > 0) {
          const prevSegment = segments[i - 1];
          if (prevSegment && messageIndex > prevSegment.endIndex) {
            return { segment: prevSegment, segmentIndex: i - 1 };
          }
        }
      }
      if (segments.length > 0) {
        const lastIdx = segments.length - 1;
        const lastSegment = segments[lastIdx];
        if (lastSegment && messageIndex > lastSegment.endIndex) {
          return { segment: lastSegment, segmentIndex: lastIdx };
        }
      }
      return null;
    },
    [segments],
  );

  const indicatorPosition = useMemo(() => {
    if (segments.length === 0 || totalDisplayPercent === 0) return 0;

    const found = findSegmentForIndex(selectedIndex);
    if (!found) return 0;

    const { segment, segmentIndex } = found;

    // Accumulated height before this segment
    let startPercent = 0;
    for (let i = 0; i < segmentIndex; i++) {
      startPercent += heightPercents[i] ?? 0;
    }

    const segmentHeight = heightPercents[segmentIndex] ?? 0;
    const messageCount = segment.endIndex - segment.startIndex + 1;
    const localIndex = selectedIndex - segment.startIndex;
    const positionInSegment = (localIndex + 0.5) / messageCount;

    return startPercent + segmentHeight * positionInSegment;
  }, [selectedIndex, segments, heightPercents, totalDisplayPercent, findSegmentForIndex]);

  return { segments, heightPercents, totalSize, indicatorPosition, findSegmentForIndex };
}
