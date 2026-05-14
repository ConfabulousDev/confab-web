// Virtualized timeline that renders a stream of Codex render items, with
// the navigation chrome the Claude transcript view also has:
//   - vertical turn-based timeline bar (click-to-seek + position indicator)
//   - floating scroll-to-top / scroll-to-bottom buttons
//   - row hover → selection state, fed back into the bar
//   - >5min idle gaps render a horizontal time-separator divider
//
// Structure mirrors `components/session/MessageTimeline.tsx`; only the
// data shape and renderer dispatch are Codex-specific.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { CodexRenderItem } from '@/types/codexRenderItem';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import { cx } from '@/utils/utils';
import { formatTimeSeparator, retryOnAnimationFrame } from '../timelineUtils';
import CodexUserMessage from './CodexUserMessage';
import CodexAssistantMessage from './CodexAssistantMessage';
import CodexToolCallBlock from './CodexToolCallBlock';
import CodexTurnSeparator from './CodexTurnSeparator';
import CodexReasoningHidden from './CodexReasoningHidden';
import CodexCompactedDivider from './CodexCompactedDivider';
import CodexUnknownItem from './CodexUnknownItem';
import CodexTimelineBar from './CodexTimelineBar';
import { buildVirtualItems } from './codexVirtualItems';
import styles from './CodexMessageTimeline.module.css';

export interface CodexMessageTimelineProps {
  items: CodexRenderItem[];
}

// Conservative initial estimate — virtualizer measures real heights after
// first paint. Slightly oversized to favor scroll smoothness over density.
const ESTIMATED_ITEM_HEIGHT = 120;
const ESTIMATED_SEPARATOR_HEIGHT = 40;

export default function CodexMessageTimeline({ items }: CodexMessageTimelineProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);

  const virtualItems = useMemo(() => buildVirtualItems(items), [items]);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
  const virtualizer = useVirtualizer({
    count: virtualItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => {
      const vi = virtualItems[index];
      if (!vi) return ESTIMATED_ITEM_HEIGHT;
      return vi.type === 'separator' ? ESTIMATED_SEPARATOR_HEIGHT : ESTIMATED_ITEM_HEIGHT;
    },
    overscan: 8,
  });

  // Map real item index → virtual list index for click-to-seek.
  const itemIndexToVirtualIndex = useMemo(() => {
    const map = new Map<number, number>();
    virtualItems.forEach((vi, idx) => {
      if (vi.type === 'item') map.set(vi.index, idx);
    });
    return map;
  }, [virtualItems]);

  // Track first visible item index (skipping separator rows) so the bar
  // indicator has something to point at when the user hasn't explicitly
  // hovered a row.
  const updateFirstVisible = useCallback(() => {
    const visible = virtualizer.getVirtualItems();
    for (const v of visible) {
      const vi = virtualItems[v.index];
      if (vi && vi.type === 'item') {
        setFirstVisibleIndex(vi.index);
        return;
      }
    }
  }, [virtualizer, virtualItems]);

  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    el.addEventListener('scroll', updateFirstVisible, { passive: true });
    updateFirstVisible();
    return () => el.removeEventListener('scroll', updateFirstVisible);
  }, [updateFirstVisible]);

  const scrollToItem = useCallback(
    (itemIndex: number) => {
      const virtualIndex = itemIndexToVirtualIndex.get(itemIndex);
      if (virtualIndex === undefined) return;
      retryOnAnimationFrame(
        () => virtualizer.scrollToIndex(virtualIndex, { align: 'start' }),
        () => false,
      );
      setSelectedIndex(itemIndex);
    },
    [itemIndexToVirtualIndex, virtualizer],
  );

  const scrollToTop = useCallback(() => {
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(0, { align: 'start' }),
      () => {
        const first = virtualizer.getVirtualItems()[0];
        return !!first && first.index === 0;
      },
    );
  }, [virtualizer]);

  const scrollToBottom = useCallback(() => {
    const lastIndex = virtualItems.length - 1;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(lastIndex, { align: 'end' }),
      () => {
        const visible = virtualizer.getVirtualItems();
        const last = visible[visible.length - 1];
        return !!last && last.index >= lastIndex;
      },
    );
  }, [virtualizer, virtualItems.length]);

  if (items.length === 0) {
    return (
      <div className={styles.empty}>
        No conversation content yet for this Codex session.
      </div>
    );
  }

  const effectiveSelectedIndex = selectedIndex ?? firstVisibleIndex;

  return (
    <div className={styles.container}>
      <div ref={parentRef} className={styles.scroll}>
        <ScrollNavButtons
          scrollRef={parentRef}
          onScrollToTop={scrollToTop}
          onScrollToBottom={scrollToBottom}
          contentDependency={items.length}
        />
        <div
          className={styles.virtualizer}
          style={{ height: `${virtualizer.getTotalSize()}px` }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const vi = virtualItems[virtualItem.index];
            if (!vi) return null;

            const slotStyle = { transform: `translateY(${virtualItem.start}px)` };

            if (vi.type === 'separator') {
              return (
                <div
                  key={virtualItem.key}
                  data-codex-time-separator
                  ref={virtualizer.measureElement}
                  data-index={virtualItem.index}
                  className={cx(styles.slot, styles.timeSeparator)}
                  style={slotStyle}
                >
                  <span className={styles.separatorLine} />
                  <span className={styles.separatorText}>
                    {formatTimeSeparator(vi.timestamp)}
                  </span>
                  <span className={styles.separatorLine} />
                </div>
              );
            }

            const isSelected = vi.index === selectedIndex;
            return (
              <div
                key={virtualItem.key}
                data-index={virtualItem.index}
                ref={virtualizer.measureElement}
                className={cx(styles.slot, styles.row)}
                onMouseEnter={() => setSelectedIndex(vi.index)}
                style={slotStyle}
              >
                {renderItem(vi.item, { isSelected, isNewSpeaker: vi.isNewSpeaker })}
              </div>
            );
          })}
        </div>
      </div>

      <CodexTimelineBar
        items={items}
        selectedIndex={effectiveSelectedIndex}
        onSeek={scrollToItem}
      />
    </div>
  );
}

interface RenderFlags {
  isSelected: boolean;
  isNewSpeaker: boolean;
}

function renderItem(item: CodexRenderItem, flags: RenderFlags) {
  switch (item.kind) {
    case 'user':
      return <CodexUserMessage item={item} {...flags} />;
    case 'assistant':
      return <CodexAssistantMessage item={item} {...flags} />;
    case 'tool_call':
      return <CodexToolCallBlock item={item} {...flags} />;
    case 'turn_separator':
      return <CodexTurnSeparator item={item} {...flags} />;
    case 'reasoning_hidden':
      return <CodexReasoningHidden item={item} {...flags} />;
    case 'compacted':
      return <CodexCompactedDivider item={item} {...flags} />;
    case 'unknown':
      return <CodexUnknownItem item={item} {...flags} />;
    default: {
      // Exhaustiveness check: if a new variant is added without a case
      // above, TypeScript will catch it here.
      const _exhaustive: never = item;
      return _exhaustive;
    }
  }
}
