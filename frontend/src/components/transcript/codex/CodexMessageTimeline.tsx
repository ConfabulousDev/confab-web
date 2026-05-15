// Virtualized timeline that renders a stream of Codex render items, with
// the navigation chrome the Claude transcript view also has:
//   - vertical turn-based timeline bar (click-to-seek + position indicator)
//   - floating scroll-to-top / scroll-to-bottom buttons
//   - row hover → selection state, fed back into the bar
//   - >5min idle gaps render a horizontal time-separator divider
//   - CF-360: deep-link to a row by `lineId`, copy-text/copy-link/skip-nav
//     chrome on every row.
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
import { buildVirtualItems, skipNavKey, skipNavLabel } from './codexVirtualItems';
import styles from './CodexMessageTimeline.module.css';

export interface CodexMessageTimelineProps {
  /**
   * Unfiltered item stream — drives the timeline bar's segment layout so
   * turn boundaries stay correct even when individual rows are filtered out.
   */
  items: CodexRenderItem[];
  /**
   * Post-filter item stream — drives row rendering, skip-nav maps, and the
   * deep-link target index. Equals `items` when no filter is active.
   */
  filteredItems: CodexRenderItem[];
  /**
   * CF-361: indices into `items` whose category passes the active filter.
   * Forwarded to `CodexTimelineBar` for greyed-segment rendering and tooltip
   * filtered-count display. `undefined` ⇒ no filtering applied.
   */
  visibleIndices?: Set<number>;
  /** Session ID — used by per-row Copy Link to build deep-link URLs. */
  sessionId: string;
  /** Deep-link target row, addressed by its stable `lineId` (CF-360). */
  targetLineId?: string;
}

// Conservative initial estimate — virtualizer measures real heights after
// first paint. Slightly oversized to favor scroll smoothness over density.
const ESTIMATED_ITEM_HEIGHT = 120;
const ESTIMATED_SEPARATOR_HEIGHT = 40;

export default function CodexMessageTimeline({
  items,
  filteredItems,
  visibleIndices,
  sessionId,
  targetLineId,
}: CodexMessageTimelineProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const hasScrolledToTarget = useRef(false);

  const virtualItems = useMemo(() => buildVirtualItems(filteredItems), [filteredItems]);

  // CF-360/CF-361: map lineId → position in filteredItems[] so deep-link lookup
  // resolves to the actual row index in the visible list. Built off
  // `filteredItems` so it stays in sync with what the virtualizer renders.
  const lineIdToItemIndex = useMemo(() => {
    const map = new Map<string, number>();
    filteredItems.forEach((item, idx) => {
      map.set(item.lineId, idx);
    });
    return map;
  }, [filteredItems]);

  // CF-360: next-/prev-of-same-kind skip-nav maps, keyed by filteredItems
  // index so navigation jumps through visible rows only. Items whose
  // `skipNavKey` returns null don't participate (dividers).
  const { nextOfSameKind, prevOfSameKind } = useMemo(() => {
    const next = new Map<number, number>();
    const prev = new Map<number, number>();
    const lastSeenByKey = new Map<string, number>();
    filteredItems.forEach((item, idx) => {
      const key = skipNavKey(item);
      if (key === null) return;
      const prevIdx = lastSeenByKey.get(key);
      if (prevIdx !== undefined) {
        next.set(prevIdx, idx);
        prev.set(idx, prevIdx);
      }
      lastSeenByKey.set(key, idx);
    });
    return { nextOfSameKind: next, prevOfSameKind: prev };
  }, [filteredItems]);

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

  // CF-361: the timeline bar's segments index into the unfiltered `items`
  // array, so we need a translation layer between the filteredItems index
  // we hold internally and the unfiltered index the bar speaks. Inverse
  // map: lineId → position in unfiltered `items`.
  const lineIdToUnfilteredIndex = useMemo(() => {
    const map = new Map<string, number>();
    items.forEach((item, idx) => map.set(item.lineId, idx));
    return map;
  }, [items]);

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

  // CF-360: reset the scroll guard when the deep-link target changes. Handles
  // both initial mount (guard starts false) and intra-page navigation (user
  // clicks copy-link on a different row).
  useEffect(() => {
    hasScrolledToTarget.current = false;
  }, [targetLineId]);

  // CF-360: scroll-to-target effect. Polling-aware — depends on
  // `lineIdToItemIndex`, which changes when items grow, so an in-flight
  // session whose target arrives later still lands. `hasScrolledToTarget`
  // ensures we only scroll once per target.
  useEffect(() => {
    if (!targetLineId || hasScrolledToTarget.current) return;
    const itemIndex = lineIdToItemIndex.get(targetLineId);
    if (itemIndex === undefined) return;
    const virtualIndex = itemIndexToVirtualIndex.get(itemIndex);
    if (virtualIndex === undefined) return;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
      () => false,
    );
    setSelectedIndex(itemIndex);
    hasScrolledToTarget.current = true;
  }, [targetLineId, lineIdToItemIndex, itemIndexToVirtualIndex, virtualizer]);

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

  // CF-361: distinct empty state when the active filter hides every row.
  // Mirrors `MessageTimeline.tsx:419-423` text.
  if (filteredItems.length === 0) {
    return (
      <div className={styles.empty}>
        <p>No items to display</p>
        <p className={styles.emptyHint}>Try adjusting your filters</p>
      </div>
    );
  }

  const effectiveSelectedIndex = selectedIndex ?? firstVisibleIndex;

  // Translate the filteredItems-keyed selection back to an unfiltered-items
  // index for the bar's position indicator. The selected row is always
  // visible, so its lineId is guaranteed to be in `lineIdToUnfilteredIndex`.
  const selectedFilteredItem = filteredItems[effectiveSelectedIndex];
  const selectedUnfilteredIndex = selectedFilteredItem
    ? (lineIdToUnfilteredIndex.get(selectedFilteredItem.lineId) ?? 0)
    : 0;

  // CF-361: bar click → scroll to first visible item at or after `unfilteredStart`.
  // We only get clicks on un-filtered segments (the bar gates filtered ones),
  // so at least one item in the segment range is in `lineIdToItemIndex`.
  const onSeekFromBar = (unfilteredStart: number): void => {
    for (let i = unfilteredStart; i < items.length; i++) {
      const candidate = items[i];
      if (!candidate) continue;
      const filteredIdx = lineIdToItemIndex.get(candidate.lineId);
      if (filteredIdx !== undefined) {
        scrollToItem(filteredIdx);
        return;
      }
    }
  };

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
            const isDeepLinkTarget =
              targetLineId !== undefined && vi.item.lineId === targetLineId;
            const nextIdx = nextOfSameKind.get(vi.index);
            const prevIdx = prevOfSameKind.get(vi.index);
            const onSkipToNext =
              nextIdx !== undefined ? () => scrollToItem(nextIdx) : undefined;
            const onSkipToPrevious =
              prevIdx !== undefined ? () => scrollToItem(prevIdx) : undefined;

            return (
              <div
                key={virtualItem.key}
                data-index={virtualItem.index}
                ref={virtualizer.measureElement}
                className={cx(styles.slot, styles.row)}
                onMouseEnter={() => setSelectedIndex(vi.index)}
                style={slotStyle}
              >
                {renderItem(vi.item, {
                  isSelected,
                  isNewSpeaker: vi.isNewSpeaker,
                  isDeepLinkTarget,
                  sessionId,
                  onSkipToNext,
                  onSkipToPrevious,
                })}
              </div>
            );
          })}
        </div>
      </div>

      <CodexTimelineBar
        items={items}
        selectedIndex={selectedUnfilteredIndex}
        visibleIndices={visibleIndices}
        onSeek={onSeekFromBar}
      />
    </div>
  );
}

interface RenderFlags {
  isSelected: boolean;
  isNewSpeaker: boolean;
  isDeepLinkTarget: boolean;
  sessionId: string;
  onSkipToNext?: () => void;
  onSkipToPrevious?: () => void;
}

function renderItem(item: CodexRenderItem, flags: RenderFlags) {
  // Per-renderer dispatch. Divider/edge kinds (turn_separator, reasoning_hidden,
  // compacted, unknown) do not get skip-nav callbacks per CF-360 — only user /
  // assistant / tool_call participate in skip chains.
  const kindLabel = skipNavLabel(item);
  switch (item.kind) {
    case 'user':
      return <CodexUserMessage item={item} {...flags} kindLabel={kindLabel} />;
    case 'assistant':
      return <CodexAssistantMessage item={item} {...flags} kindLabel={kindLabel} />;
    case 'tool_call':
      return <CodexToolCallBlock item={item} {...flags} kindLabel={kindLabel} />;
    case 'turn_separator':
      return (
        <CodexTurnSeparator
          item={item}
          isSelected={flags.isSelected}
          isNewSpeaker={flags.isNewSpeaker}
          isDeepLinkTarget={flags.isDeepLinkTarget}
          sessionId={flags.sessionId}
        />
      );
    case 'reasoning_hidden':
      return (
        <CodexReasoningHidden
          item={item}
          isSelected={flags.isSelected}
          isNewSpeaker={flags.isNewSpeaker}
          isDeepLinkTarget={flags.isDeepLinkTarget}
          sessionId={flags.sessionId}
        />
      );
    case 'compacted':
      return (
        <CodexCompactedDivider
          item={item}
          isSelected={flags.isSelected}
          isNewSpeaker={flags.isNewSpeaker}
          isDeepLinkTarget={flags.isDeepLinkTarget}
          sessionId={flags.sessionId}
        />
      );
    case 'unknown':
      return (
        <CodexUnknownItem
          item={item}
          isSelected={flags.isSelected}
          isNewSpeaker={flags.isNewSpeaker}
          isDeepLinkTarget={flags.isDeepLinkTarget}
          sessionId={flags.sessionId}
        />
      );
    default: {
      // Exhaustiveness check: if a new variant is added without a case
      // above, TypeScript will catch it here.
      const _exhaustive: never = item;
      return _exhaustive;
    }
  }
}
