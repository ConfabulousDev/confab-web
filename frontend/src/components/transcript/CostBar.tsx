import { useMemo, useCallback, useState, useRef } from 'react';
import type { TranscriptLine } from '@/types';
import { isAssistantMessage } from '@/types';
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
  const [hoveredSegment, setHoveredSegment] = useState<{ segmentIndex: number; cost: number } | null>(null);
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

  // Count unique assistant message IDs per segment for accurate density.
  // Multiple JSONL lines share the same message.id (one per content block),
  // and context replay re-logs the same message.id later.
  const segmentUniqueMsgCounts = useMemo(() => {
    return segments.map((seg) => {
      const seen = new Set<string>();
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        const msg = messages[i];
        if (msg && isAssistantMessage(msg)) {
          seen.add(msg.message.id);
        }
      }
      return seen.size;
    });
  }, [segments, messages]);

  // Compute alpha values based on cost density (cost per unique API call)
  // Intensity reflects how expensive each API call is, not total segment cost
  const segmentAlphas = useMemo(() => {
    const densities = segments.map((_, i) => {
      const cost = segmentCosts[i] ?? 0;
      const uniqueCount = segmentUniqueMsgCounts[i] ?? 0;
      if (cost === 0 || uniqueCount === 0) return 0;
      return cost / uniqueCount;
    });
    const maxDensity = Math.max(...densities, 0);
    if (maxDensity === 0) return densities.map(() => 0);

    return densities.map((density) => {
      if (density === 0) return 0;
      return 0.15 + (density / maxDensity) * 0.75;
    });
  }, [segments, segmentCosts, segmentUniqueMsgCounts]);

  const handleSegmentClick = useCallback(
    (segment: TimelineSegment) => {
      onSeek(segment.startIndex, segment.endIndex);
    },
    [onSeek],
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

  return (
    <div className={styles.costBar} ref={barRef} title="Color intensity = cost per message">
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
                background: alpha > 0 ? `rgba(22, 163, 74, ${alpha})` : 'transparent',
              }}
              onClick={() => handleSegmentClick(segment)}
              onMouseEnter={(e) => handleSegmentHover(index, cost, e)}
              onMouseMove={(e) => handleSegmentHover(index, cost, e)}
              onMouseLeave={() => handleSegmentHover(null, 0)}
            />
          );
        })}
      </div>

      <div className={styles.positionIndicator} style={{ top: `${indicatorPosition}%` }} />

      {hoveredSegment && <CostTooltip
        hoveredSegment={hoveredSegment}
        segments={segments}
        segmentUniqueMsgCounts={segmentUniqueMsgCounts}
        totalCost={totalCost}
        tooltipPosition={tooltipPosition}
      />}
    </div>
  );
}

function CostTooltip({ hoveredSegment, segments, segmentUniqueMsgCounts, totalCost, tooltipPosition }: {
  hoveredSegment: { segmentIndex: number; cost: number };
  segments: TimelineSegment[];
  segmentUniqueMsgCounts: number[];
  totalCost: number;
  tooltipPosition: { top: number; left: number };
}) {
  const { segmentIndex, cost } = hoveredSegment;
  const segment = segments[segmentIndex]!;
  const percent = totalCost > 0 ? ((cost / totalCost) * 100).toFixed(1) : '0';
  const uniqueCount = segmentUniqueMsgCounts[segmentIndex] ?? segment.messageCount;
  const costPerMsg = uniqueCount > 0 ? cost / uniqueCount : 0;

  return (
    <div
      className={styles.tooltip}
      style={{ top: tooltipPosition.top, left: tooltipPosition.left }}
    >
      {cost > 0 ? (
        <>
          <div className={styles.tooltipTotal}>{formatCost(cost)} ({percent}%)</div>
          <div className={styles.tooltipDensity}>
            {formatCost(costPerMsg)}/msg &times; {uniqueCount}
          </div>
        </>
      ) : (
        'No cost'
      )}
    </div>
  );
}
