// Shared vertical turn-based navigation bar for transcript panes. Each turn
// emits up to two clickable segments — a user thinking-gap segment and an
// assistant body segment (tools fold into the assistant stripe). Consumed by
// both Codex (`assistantLabel="Codex"`) and OpenCode (`assistantLabel="Assistant"`).
//
// CF-361: when `visibleIndices` is supplied, segments whose entire item
// range is filtered out render greyed-out (`.filtered`) and non-clickable.
// The hover tooltip appends `(N filtered)` when some — but not all — of a
// segment's items are visible.

import { useCallback, useState, useRef } from 'react';
import { cx } from '@/utils/utils';
import { formatDuration } from './timelineFormat';
import type { BlendedSegmentLayout, SpeakerSegment } from './timelineSegments';
import styles from './TimelineBar.module.css';

export interface TimelineBarProps<S extends SpeakerSegment> {
  /**
   * Precomputed segment layout. The same layout instance can feed both
   * `TimelineBar` and `CostBar` so the two side-by-side rails line up
   * row-for-row. Caller derives via the provider's segment-layout hook.
   */
  layout: BlendedSegmentLayout<S>;
  /**
   * CF-361: indices into the unfiltered item array whose category is
   * currently visible under the active filter. `undefined` means "no filter
   * applied" — every segment renders in its speaker color and no `(filtered)`
   * suffix appears. An empty set means everything is filtered out.
   */
  visibleIndices?: Set<number>;
  /** Click-to-seek callback; receives the segment's startIndex (into items). */
  onSeek: (startIndex: number) => void;
  /** Tooltip label for assistant segments (e.g. "Codex", "Assistant"). */
  assistantLabel: string;
  /**
   * zztp: optional suffix appended to every segment tooltip (new line). Cursor
   * passes an "estimated times" disclaimer because its per-row timestamps are
   * interpolated, not real (ce79); other providers omit it.
   */
  tooltipNote?: string;
}

export default function TimelineBar<S extends SpeakerSegment>({
  layout,
  visibleIndices,
  onSeek,
  assistantLabel,
  tooltipNote,
}: TimelineBarProps<S>) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<S | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const { segments, heightPercents, totalSize, indicatorPosition } = layout;

  const isSegmentFiltered = useCallback(
    (segment: S): boolean => {
      if (!visibleIndices) return false;
      for (let i = segment.startIndex; i <= segment.endIndex; i++) {
        if (visibleIndices.has(i)) return false;
      }
      return true;
    },
    [visibleIndices],
  );

  const handleSegmentHover = useCallback(
    (segment: S | null, event?: React.MouseEvent) => {
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
              data-timeline-segment
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
          {formatTooltip(hoveredSegment, visibleIndices, assistantLabel)}
          {tooltipNote && <div className={styles.tooltipNote}>{tooltipNote}</div>}
        </div>
      )}
    </div>
  );
}

function formatTooltip(
  segment: SpeakerSegment,
  visibleIndices: Set<number> | undefined,
  assistantLabel: string,
): string {
  const speaker = segment.speaker === 'user' ? 'User' : assistantLabel;
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
