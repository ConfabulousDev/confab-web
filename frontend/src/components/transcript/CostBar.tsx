import { useMemo, useCallback, useState, useRef } from 'react';
import { formatCost } from '@/utils/tokenStats';
import type {
  BlendedSegment,
  BlendedSegmentLayout,
} from './timelineSegments';
import styles from './CostBar.module.css';

// CF-362: provider-agnostic CostBar. The caller (`MessageTimeline` for Claude,
// `CodexMessageTimeline` for Codex) computes the segment layout, per-index
// cost map, and per-segment unique-call counts. CostBar only handles the
// per-segment cost sum, density-based green alpha, position indicator, and
// hover tooltip — none of which depend on the underlying transcript shape.

interface CostBarProps {
  /** Segment layout shared with the timeline bar so the two rails line up. */
  layout: BlendedSegmentLayout<BlendedSegment>;
  /** Map keyed by index into the unfiltered items/messages array → $ cost. */
  costByIndex: Map<number, number>;
  /**
   * Per-segment count of unique API calls. Drives density (cost-per-call) so
   * intensity reflects expensive INDIVIDUAL calls rather than long segments.
   * - Claude: caller dedupes by `message.id` (multiple JSONL lines per call).
   * - Codex: caller counts assistant-kind items (1:1 with calls).
   * Order must match `layout.segments`.
   */
  segmentUniqueCounts: number[];
  totalCost: number;
  onSeek: (startIndex: number, endIndex: number) => void;
}

export function CostBar({
  layout,
  costByIndex,
  segmentUniqueCounts,
  totalCost,
  onSeek,
}: CostBarProps) {
  const { segments, heightPercents, totalSize, indicatorPosition } = layout;
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<{
    segmentIndex: number;
    cost: number;
  } | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const segmentCosts = useMemo(() => {
    return segments.map((seg) => {
      let cost = 0;
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        cost += costByIndex.get(i) ?? 0;
      }
      return cost;
    });
  }, [segments, costByIndex]);

  // Density = cost per unique API call. Highlights expensive single calls
  // rather than long-running ones.
  const segmentAlphas = useMemo(() => {
    const densities = segments.map((_, i) => {
      const cost = segmentCosts[i] ?? 0;
      const uniqueCount = segmentUniqueCounts[i] ?? 0;
      if (cost === 0 || uniqueCount === 0) return 0;
      return cost / uniqueCount;
    });
    const maxDensity = Math.max(...densities, 0);
    if (maxDensity === 0) return densities.map(() => 0);
    return densities.map((density) =>
      density === 0 ? 0 : 0.15 + (density / maxDensity) * 0.75,
    );
  }, [segments, segmentCosts, segmentUniqueCounts]);

  const handleSegmentClick = useCallback(
    (segmentIndex: number) => {
      const segment = segments[segmentIndex];
      if (!segment) return;
      onSeek(segment.startIndex, segment.endIndex);
    },
    [segments, onSeek],
  );

  const handleSegmentHover = useCallback(
    (segmentIndex: number | null, cost: number, event?: React.MouseEvent) => {
      if (segmentIndex == null) {
        setHoveredSegment(null);
        return;
      }
      setHoveredSegment({ segmentIndex, cost });
      if (event && barRef.current) {
        const barRect = barRef.current.getBoundingClientRect();
        setTooltipPosition({ top: event.clientY, left: barRect.left });
      }
    },
    [],
  );

  if (segments.length === 0 || totalSize === 0 || totalCost === 0) {
    return null;
  }

  const hoveredSeg = hoveredSegment ? segments[hoveredSegment.segmentIndex] : undefined;
  const hoveredSegmentMessageCount = hoveredSeg
    ? hoveredSeg.endIndex - hoveredSeg.startIndex + 1
    : 0;

  return (
    <div
      className={styles.costBar}
      ref={barRef}
      title="Color intensity = cost per message"
    >
      <div className={styles.segmentsContainer}>
        {segments.map((_segment, index) => {
          const alpha = segmentAlphas[index] ?? 0;
          const cost = segmentCosts[index] ?? 0;
          return (
            <div
              key={index}
              className={styles.segment}
              style={{
                height: `${heightPercents[index]}%`,
                background: alpha > 0 ? `rgba(22, 163, 74, ${alpha})` : 'transparent',
              }}
              onClick={() => handleSegmentClick(index)}
              onMouseEnter={(e) => handleSegmentHover(index, cost, e)}
              onMouseMove={(e) => handleSegmentHover(index, cost, e)}
              onMouseLeave={() => handleSegmentHover(null, 0)}
            />
          );
        })}
      </div>

      <div
        className={styles.positionIndicator}
        style={{ top: `${indicatorPosition}%` }}
      />

      {hoveredSegment && (
        <CostTooltip
          hoveredSegment={hoveredSegment}
          segmentUniqueCount={segmentUniqueCounts[hoveredSegment.segmentIndex] ?? 0}
          segmentMessageCount={hoveredSegmentMessageCount}
          totalCost={totalCost}
          tooltipPosition={tooltipPosition}
        />
      )}
    </div>
  );
}

interface CostTooltipProps {
  hoveredSegment: { segmentIndex: number; cost: number };
  segmentUniqueCount: number;
  segmentMessageCount: number;
  totalCost: number;
  tooltipPosition: { top: number; left: number };
}

function CostTooltip({
  hoveredSegment,
  segmentUniqueCount,
  segmentMessageCount,
  totalCost,
  tooltipPosition,
}: CostTooltipProps) {
  const { cost } = hoveredSegment;
  const percent = totalCost > 0 ? ((cost / totalCost) * 100).toFixed(1) : '0';
  const denom = segmentUniqueCount > 0 ? segmentUniqueCount : segmentMessageCount;
  const costPerMsg = denom > 0 ? cost / denom : 0;

  return (
    <div
      className={styles.tooltip}
      style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
    >
      {cost > 0 ? (
        <>
          <div className={styles.tooltipTotal}>
            {formatCost(cost)} ({percent}%)
          </div>
          <div className={styles.tooltipDensity}>
            {formatCost(costPerMsg)}/msg &times; {denom}
          </div>
        </>
      ) : (
        'No cost'
      )}
    </div>
  );
}
