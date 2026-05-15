// Vertical turn-based navigation bar for Codex transcripts. Each turn
// emits up to two clickable segments — a user thinking-gap segment and an
// assistant body segment — matching the Claude TimelineBar shape.
//
// CF-361: when `visibleIndices` is supplied, segments whose entire item
// range is filtered out render greyed-out (`.filtered`) and non-clickable.
// The hover tooltip appends `(N filtered)` when some — but not all — of a
// segment's items are visible.

import { useCallback, useState, useRef } from 'react';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import { cx } from '@/utils/utils';
import { formatDuration } from '../timelineFormat';
import { useCodexSegmentLayout, type CodexTimelineSegment } from './codexTimelineSegments';
import styles from './CodexTimelineBar.module.css';

export interface CodexTimelineBarProps {
  items: CodexRenderItem[];
  /** Index (into items) of the currently selected/active row — drives indicator. */
  selectedIndex: number;
  /**
   * CF-361: indices into `items` whose category is currently visible under
   * the active filter. `undefined` means "no filter applied" — every segment
   * renders in its speaker color and no `(filtered)` suffix appears. An
   * empty set means everything is filtered out.
   */
  visibleIndices?: Set<number>;
  /** Click-to-seek callback; receives the segment's startIndex (into items). */
  onSeek: (startIndex: number) => void;
}

export default function CodexTimelineBar({ items, selectedIndex, visibleIndices, onSeek }: CodexTimelineBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<CodexTimelineSegment | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const { segments, heightPercents, totalSize, indicatorPosition } =
    useCodexSegmentLayout(items, selectedIndex);

  const isSegmentFiltered = useCallback(
    (segment: CodexTimelineSegment): boolean => {
      if (!visibleIndices) return false;
      for (let i = segment.startIndex; i <= segment.endIndex; i++) {
        if (visibleIndices.has(i)) return false;
      }
      return true;
    },
    [visibleIndices],
  );

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
        {segments.map((segment, index) => {
          const filtered = isSegmentFiltered(segment);
          return (
            <div
              key={index}
              data-codex-segment
              data-turn-index={segment.turnIndex}
              className={cx(
                styles.segment,
                filtered ? styles.filtered : styles[segment.speaker],
              )}
              style={{ height: `${heightPercents[index]}%` }}
              onClick={() => !filtered && onSeek(segment.startIndex)}
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

      {hoveredSegment && (
        <div
          className={styles.tooltip}
          style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
        >
          {formatTooltip(hoveredSegment, visibleIndices)}
        </div>
      )}
    </div>
  );
}

function formatTooltip(
  segment: CodexTimelineSegment,
  visibleIndices: Set<number> | undefined,
): string {
  const speaker = segment.speaker === 'user' ? 'User' : 'Codex';
  const itemLabel = segment.messageCount === 1 ? 'item' : 'items';
  const base = `${speaker}: ${formatDuration(segment.durationMs)}, ${segment.messageCount} ${itemLabel}`;

  if (!visibleIndices) return base;

  let visibleCount = 0;
  for (let i = segment.startIndex; i <= segment.endIndex; i++) {
    if (visibleIndices.has(i)) visibleCount++;
  }
  const filteredCount = segment.messageCount - visibleCount;
  return filteredCount > 0 ? `${base} (${filteredCount} filtered)` : base;
}
