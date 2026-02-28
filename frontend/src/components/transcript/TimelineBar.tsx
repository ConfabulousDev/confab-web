import { useCallback, useState, useRef } from 'react';
import type { TranscriptLine } from '@/types';
import { useSegmentLayout, type TimelineSegment } from './timelineSegments';
import styles from './TimelineBar.module.css';

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

  const { segments, heightPercents, totalSize, indicatorPosition } = useSegmentLayout(messages, selectedIndex);

  const handleSegmentClick = useCallback(
    (segment: TimelineSegment) => {
      onSeek(segment.startIndex, segment.endIndex);
    },
    [onSeek],
  );

  // Determine if a segment has any visible messages
  const isSegmentFiltered = useCallback(
    (segment: TimelineSegment): boolean => {
      if (!visibleIndices) return false;
      for (let i = segment.startIndex; i <= segment.endIndex; i++) {
        if (visibleIndices.has(i)) return false;
      }
      return true;
    },
    [visibleIndices],
  );

  const handleSegmentHover = useCallback(
    (segment: TimelineSegment | null, event?: React.MouseEvent) => {
      setHoveredSegment(segment);
      if (segment && event && barRef.current) {
        const barRect = barRef.current.getBoundingClientRect();
        setTooltipPosition({ top: event.clientY, left: barRect.left });
      }
    },
    [],
  );

  if (segments.length === 0 || totalSize === 0) {
    return null;
  }

  return (
    <div className={styles.timelineBar} ref={barRef}>
      <div className={styles.segmentsContainer}>
        {segments.map((segment, index) => {
          const filtered = isSegmentFiltered(segment);

          return (
            <div
              key={index}
              className={`${styles.segment} ${filtered ? styles.filtered : styles[segment.speaker]}`}
              style={{ height: `${heightPercents[index]}%` }}
              onClick={() => handleSegmentClick(segment)}
              onMouseEnter={(e) => handleSegmentHover(segment, e)}
              onMouseMove={(e) => handleSegmentHover(segment, e)}
              onMouseLeave={() => handleSegmentHover(null)}
            />
          );
        })}
      </div>

      <div
        className={styles.positionIndicator}
        style={{ top: `${indicatorPosition}%` }}
      />

      {hoveredSegment && (() => {
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
