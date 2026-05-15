// Vertical turn-based navigation bar for Codex transcripts. Each turn
// emits up to two clickable segments — a user thinking-gap segment and an
// assistant body segment — matching the Claude TimelineBar shape.

import { useCallback, useState, useRef } from 'react';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import { formatDuration } from '../timelineFormat';
import { useCodexSegmentLayout, type CodexTimelineSegment } from './codexTimelineSegments';
import styles from './CodexTimelineBar.module.css';

export interface CodexTimelineBarProps {
  items: CodexRenderItem[];
  /** Index (into items) of the currently selected/active row — drives indicator. */
  selectedIndex: number;
  /** Click-to-seek callback; receives the segment's startIndex (into items). */
  onSeek: (startIndex: number) => void;
}

export default function CodexTimelineBar({ items, selectedIndex, onSeek }: CodexTimelineBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<CodexTimelineSegment | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const { segments, heightPercents, totalSize, indicatorPosition } =
    useCodexSegmentLayout(items, selectedIndex);

  const handleSegmentHover = useCallback(
    (segment: CodexTimelineSegment | null, event?: React.MouseEvent) => {
      setHoveredSegment(segment);
      if (segment && event && barRef.current) {
        setTooltipPosition({ top: event.clientY, left: barRef.current.getBoundingClientRect().left });
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
        {segments.map((segment, index) => (
          <div
            key={index}
            data-codex-segment
            data-turn-index={segment.turnIndex}
            className={`${styles.segment} ${styles[segment.speaker]}`}
            style={{ height: `${heightPercents[index]}%` }}
            onClick={() => onSeek(segment.startIndex)}
            onMouseEnter={(e) => handleSegmentHover(segment, e)}
            onMouseMove={(e) => handleSegmentHover(segment, e)}
            onMouseLeave={() => handleSegmentHover(null)}
          />
        ))}
      </div>

      <div
        className={styles.positionIndicator}
        style={{ top: `${indicatorPosition}%` }}
      />

      {hoveredSegment && (
        <div
          className={styles.tooltip}
          style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
        >
          {formatTooltip(hoveredSegment)}
        </div>
      )}
    </div>
  );
}

function formatTooltip(segment: CodexTimelineSegment): string {
  const speaker = segment.speaker === 'user' ? 'User' : 'Codex';
  const itemLabel = segment.messageCount === 1 ? 'item' : 'items';
  return `${speaker}: ${formatDuration(segment.durationMs)}, ${segment.messageCount} ${itemLabel}`;
}
