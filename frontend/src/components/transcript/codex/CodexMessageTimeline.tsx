// Virtualized timeline that renders a stream of Codex render items.
// Mirrors MessageTimeline's structure but consumes the normalized
// CodexRenderItem union instead of TranscriptLine.

import { useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import CodexUserMessage from './CodexUserMessage';
import CodexAssistantMessage from './CodexAssistantMessage';
import CodexToolCallBlock from './CodexToolCallBlock';
import CodexTurnSeparator from './CodexTurnSeparator';
import CodexReasoningHidden from './CodexReasoningHidden';
import CodexCompactedDivider from './CodexCompactedDivider';
import CodexUnknownItem from './CodexUnknownItem';
import styles from './CodexMessageTimeline.module.css';

export interface CodexMessageTimelineProps {
  items: CodexRenderItem[];
}

// Conservative initial estimate — virtualizer measures real heights after
// first paint. Slightly oversized to favor scroll smoothness over density.
const ESTIMATED_ITEM_HEIGHT = 120;

export default function CodexMessageTimeline({ items }: CodexMessageTimelineProps) {
  const parentRef = useRef<HTMLDivElement>(null);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ESTIMATED_ITEM_HEIGHT,
    overscan: 8,
  });

  if (items.length === 0) {
    return (
      <div className={styles.empty}>
        No conversation content yet for this Codex session.
      </div>
    );
  }

  return (
    <div ref={parentRef} className={styles.scroll}>
      <div
        className={styles.virtualizer}
        style={{ height: `${virtualizer.getTotalSize()}px` }}
      >
        {virtualizer.getVirtualItems().map((virtualItem) => {
          const item = items[virtualItem.index];
          if (!item) return null;
          return (
            <div
              key={virtualItem.key}
              data-index={virtualItem.index}
              ref={virtualizer.measureElement}
              className={styles.row}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualItem.start}px)`,
              }}
            >
              {renderItem(item)}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function renderItem(item: CodexRenderItem) {
  switch (item.kind) {
    case 'user':
      return <CodexUserMessage item={item} />;
    case 'assistant':
      return <CodexAssistantMessage item={item} />;
    case 'tool_call':
      return <CodexToolCallBlock item={item} />;
    case 'turn_separator':
      return <CodexTurnSeparator item={item} />;
    case 'reasoning_hidden':
      return <CodexReasoningHidden item={item} />;
    case 'compacted':
      return <CodexCompactedDivider item={item} />;
    case 'unknown':
      return <CodexUnknownItem item={item} />;
    default: {
      // Exhaustiveness check: if a new variant is added without a case
      // above, TypeScript will catch it here.
      const _exhaustive: never = item;
      return _exhaustive;
    }
  }
}
