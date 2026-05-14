// Vertical turn-based navigation bar for Codex transcripts. One clickable
// segment per `CodexTurnSeparatorItem`, plus a trailing in-flight segment.
// Layout math is shared with Claude's TimelineBar via useCodexSegmentLayout
// → useBlendedSegmentLayout in ../timelineSegments.ts.

import { useCallback, useState, useRef } from 'react';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import { useCodexSegmentLayout, type CodexTimelineSegment } from './codexTimelineSegments';
import { formatDurationMs } from './codexFormat';
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

  const handleSegmentClick = useCallback(
    (segment: CodexTimelineSegment) => onSeek(segment.startIndex),
    [onSeek],
  );

  const handleSegmentHover = useCallback(
    (segment: CodexTimelineSegment | null, event?: React.MouseEvent) => {
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
        {segments.map((segment, index) => (
          <div
            key={index}
            data-codex-segment
            data-turn-index={segment.turnIndex}
            className={styles.segment}
            style={{ height: `${heightPercents[index]}%` }}
            onClick={() => handleSegmentClick(segment)}
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

      {hoveredSegment ? (
        <div
          className={styles.tooltip}
          style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
        >
          {formatTooltip(hoveredSegment)}
        </div>
      ) : null}
    </div>
  );
}

function formatTooltip(segment: CodexTimelineSegment): string {
  const itemLabel = segment.messageCount === 1 ? 'item' : 'items';
  const duration = formatDurationMs(segment.durationMs);
  const ttftPart =
    segment.timeToFirstTokenMs !== undefined
      ? ` · TTFT ${formatDurationMs(segment.timeToFirstTokenMs)}`
      : '';
  return `Turn ${segment.turnIndex} — ${duration}${ttftPart}, ${segment.messageCount} ${itemLabel}`;
}
