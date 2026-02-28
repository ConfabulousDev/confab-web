import { useMemo, useCallback, useState, useRef } from 'react';
import type { TranscriptLine } from '@/types';
import { formatCost } from '@/utils/tokenStats';
import { useSegmentLayout, type TimelineSegment } from './timelineSegments';
import styles from './CostBar.module.css';

interface CostBarProps {
  messages: TranscriptLine[];
  messageCosts: Map<number, number>; // message index -> $ cost
  totalCost: number;
  selectedIndex: number;
  onSeek: (startIndex: number, endIndex: number) => void;
}

export function CostBar({ messages, messageCosts, totalCost, selectedIndex, onSeek }: CostBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoveredSegment, setHoveredSegment] = useState<{ segment: TimelineSegment; cost: number } | null>(null);
  const [tooltipPosition, setTooltipPosition] = useState({ top: 0, left: 0 });

  const { segments, heightPercents, totalSize, indicatorPosition } = useSegmentLayout(messages, selectedIndex);

  // Compute per-segment costs
  const segmentCosts = useMemo(() => {
    return segments.map((seg) => {
      let cost = 0;
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        cost += messageCosts.get(i) ?? 0;
      }
      return cost;
    });
  }, [segments, messageCosts]);

  // Compute alpha values: linear scale from 0.15 (cheapest non-zero) to 0.9 (most expensive)
  const segmentAlphas = useMemo(() => {
    const maxCost = Math.max(...segmentCosts, 0);
    if (maxCost === 0) return segmentCosts.map(() => 0);

    return segmentCosts.map((cost) => {
      if (cost === 0) return 0;
      return 0.15 + (cost / maxCost) * 0.75;
    });
  }, [segmentCosts]);

  const handleSegmentClick = useCallback(
    (segment: TimelineSegment) => {
      onSeek(segment.startIndex, segment.endIndex);
    },
    [onSeek],
  );

  const handleSegmentHover = useCallback(
    (segment: TimelineSegment | null, cost: number, event?: React.MouseEvent) => {
      if (!segment) {
        setHoveredSegment(null);
        return;
      }
      setHoveredSegment({ segment, cost });
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

  return (
    <div className={styles.costBar} ref={barRef}>
      <div className={styles.segmentsContainer}>
        {segments.map((segment, index) => {
          const alpha = segmentAlphas[index] ?? 0;
          const cost = segmentCosts[index] ?? 0;

          return (
            <div
              key={index}
              className={styles.segment}
              style={{
                height: `${heightPercents[index]}%`,
                background: alpha > 0 ? `rgba(239, 68, 68, ${alpha})` : 'transparent',
              }}
              onClick={() => handleSegmentClick(segment)}
              onMouseEnter={(e) => handleSegmentHover(segment, cost, e)}
              onMouseMove={(e) => handleSegmentHover(segment, cost, e)}
              onMouseLeave={() => handleSegmentHover(null, 0)}
            />
          );
        })}
      </div>

      <div className={styles.positionIndicator} style={{ top: `${indicatorPosition}%` }} />

      {hoveredSegment && (() => {
        const { cost } = hoveredSegment;
        const percent = totalCost > 0 ? ((cost / totalCost) * 100).toFixed(1) : '0';
        const tooltipText = cost > 0
          ? `${formatCost(cost)} (${percent}% of session)`
          : 'No cost';

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
