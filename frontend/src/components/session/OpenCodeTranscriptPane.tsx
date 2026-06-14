// Renders the transcript-tab content for OpenCode sessions.
//
// A virtualized list of render items (user / assistant / tool) with a
// turn-based minimap / timeline bar alongside (the shared `TimelineBar`,
// reaching parity with Claude/Codex). Still leaner than the others — no cost
// side-rail or Cmd-F search yet — but real: it fetches nothing itself
// (SessionViewer drives fetch/poll via the adapter) and renders the three
// categories with reasoning, tool I/O, status, deep-link scroll, and
// per-message cost badges in cost mode.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { CostAmount } from '@/components/CostAmount';
import { cx } from '@/utils/utils';
import { retryOnAnimationFrame } from '@/components/transcript/timelineUtils';
import TimelineBar from '@/components/transcript/TimelineBar';
import type { OpenCodeRenderItem } from './opencodeCategories';
import { useOpenCodeSegmentLayout } from './opencodeTimelineSegments';
import OpenCodeUnknownItem from './OpenCodeUnknownItem';
import TranscriptPaneStatus from './TranscriptPaneStatus';
import styles from './OpenCodeTranscriptPane.module.css';

export interface OpenCodeTranscriptPaneProps {
  sessionId: string;
  /** Unfiltered render items — distinguishes "no transcript yet" from "filtered out". Drives bar segments. */
  items: OpenCodeRenderItem[];
  /** Post-filter render items — drives the row list. */
  filteredItems: OpenCodeRenderItem[];
  loading: boolean;
  error: string | null;
  /** Deep-link target, addressed by render-item id (message ULID / tool part id). */
  targetId?: string;
  /** When true, show per-assistant-message cost badges. */
  isCostMode?: boolean;
}

const ESTIMATED_ROW_HEIGHT = 120;

function ToolRow({ item }: { item: Extract<OpenCodeRenderItem, { kind: 'tool' }> }) {
  const isError = item.status === 'error';
  return (
    <div className={cx(styles.row, styles.toolRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Tool</span>
        <span className={styles.toolName}>{item.toolName}</span>
        <span className={cx(styles.status, isError ? styles.statusError : styles.statusOk)}>
          {item.status}
        </span>
      </div>
      {item.input ? <pre className={styles.toolInput}>{item.input}</pre> : null}
      {item.output ? (
        <details className={styles.details}>
          <summary className={styles.summary}>Output</summary>
          <pre className={styles.toolOutput}>{item.output}</pre>
        </details>
      ) : null}
    </div>
  );
}

function AssistantRow({
  item,
  isCostMode,
}: {
  item: Extract<OpenCodeRenderItem, { kind: 'assistant' }>;
  isCostMode?: boolean;
}) {
  return (
    <div className={cx(styles.row, styles.assistantRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Assistant</span>
        {item.model ? <span className={styles.model}>{item.model}</span> : null}
        {isCostMode && typeof item.cost === 'number' ? (
          <CostAmount usd={item.cost} className={styles.cost} />
        ) : null}
      </div>
      {item.reasoning ? (
        <details className={styles.details}>
          <summary className={styles.summary}>Reasoning</summary>
          <div className={styles.reasoning}>{item.reasoning}</div>
        </details>
      ) : null}
      {item.text ? <div className={styles.text}>{item.text}</div> : null}
    </div>
  );
}

function Row({ item, isCostMode }: { item: OpenCodeRenderItem; isCostMode?: boolean }) {
  if (item.kind === 'user') {
    return (
      <div className={cx(styles.row, styles.userRow)}>
        <div className={styles.rowHeader}>
          <span className={styles.roleLabel}>User</span>
        </div>
        <div className={styles.text}>{item.text}</div>
      </div>
    );
  }
  if (item.kind === 'assistant') {
    return <AssistantRow item={item} isCostMode={isCostMode} />;
  }
  if (item.kind === 'tool') {
    return <ToolRow item={item} />;
  }
  return <OpenCodeUnknownItem item={item} />;
}

export default function OpenCodeTranscriptPane({
  items,
  filteredItems,
  loading,
  error,
  targetId,
  isCostMode,
}: OpenCodeTranscriptPaneProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const hasScrolledToTarget = useRef(false);
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
  const virtualizer = useVirtualizer({
    count: filteredItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => ESTIMATED_ROW_HEIGHT,
    overscan: 8,
  });

  const targetIndex = useMemo(() => {
    if (!targetId) return -1;
    return filteredItems.findIndex((it) => it.id === targetId);
  }, [filteredItems, targetId]);

  // The timeline bar's segments index into the unfiltered `items` array, so we
  // translate between the filtered-list index the pane holds internally and
  // the unfiltered index the bar speaks. OpenCode has no separator rows, so a
  // filtered index IS the virtual index — the only translation is filtered↔
  // unfiltered, keyed by the stable render-item `id`.
  const idToUnfilteredIndex = useMemo(() => {
    const map = new Map<string, number>();
    items.forEach((item, idx) => map.set(item.id, idx));
    return map;
  }, [items]);

  // CF-361 parity: the set of unfiltered indices whose item survives the
  // active filter, so the bar greys out fully-filtered segments. `undefined`
  // when nothing is filtered (filteredItems === items length-wise).
  const visibleIndices = useMemo(() => {
    if (filteredItems.length === items.length) return undefined;
    const visibleIds = new Set(filteredItems.map((it) => it.id));
    const set = new Set<number>();
    items.forEach((item, idx) => {
      if (visibleIds.has(item.id)) set.add(idx);
    });
    return set;
  }, [items, filteredItems]);

  // Selection plumbing: `selectedIndex`/`firstVisibleIndex` are filtered-list
  // indices; the segment layout indexes the unfiltered array, so translate the
  // active row's `id` back. Lifted above the early-return so the layout hook
  // has a stable input across renders.
  const effectiveSelectedIndex = selectedIndex ?? firstVisibleIndex;
  const selectedUnfilteredIndex = useMemo(() => {
    const selected = filteredItems[effectiveSelectedIndex];
    if (!selected) return 0;
    return idToUnfilteredIndex.get(selected.id) ?? 0;
  }, [filteredItems, effectiveSelectedIndex, idToUnfilteredIndex]);

  const segmentLayout = useOpenCodeSegmentLayout(items, selectedUnfilteredIndex);

  // Track the first visible row so the bar indicator has something to point at
  // when the user hasn't hovered a row.
  const updateFirstVisible = useCallback(() => {
    const visible = virtualizer.getVirtualItems();
    const first = visible[0];
    if (first) setFirstVisibleIndex(first.index);
  }, [virtualizer]);

  useEffect(() => {
    const el = parentRef.current;
    if (!el) return;
    el.addEventListener('scroll', updateFirstVisible, { passive: true });
    updateFirstVisible();
    return () => el.removeEventListener('scroll', updateFirstVisible);
  }, [updateFirstVisible]);

  const scrollToItem = useCallback(
    (filteredIdx: number) => {
      retryOnAnimationFrame(
        () => virtualizer.scrollToIndex(filteredIdx, { align: 'start' }),
        () => false,
      );
      setSelectedIndex(filteredIdx);
    },
    [virtualizer],
  );

  // Bar click → scroll to the first visible item at or after `unfilteredStart`.
  // The bar only fires clicks on un-filtered segments, so at least one item in
  // the range maps into the filtered list.
  const onSeekFromBar = useCallback(
    (unfilteredStart: number) => {
      const filteredIds = new Map<string, number>();
      filteredItems.forEach((it, idx) => filteredIds.set(it.id, idx));
      for (let i = unfilteredStart; i < items.length; i++) {
        const candidate = items[i];
        if (!candidate) continue;
        const filteredIdx = filteredIds.get(candidate.id);
        if (filteredIdx !== undefined) {
          scrollToItem(filteredIdx);
          return;
        }
      }
    },
    [items, filteredItems, scrollToItem],
  );

  // Re-arm the one-shot scroll when the deep-link target changes, so intra-page
  // navigation (?msg= changes while the pane stays mounted) re-scrolls.
  useEffect(() => {
    hasScrolledToTarget.current = false;
  }, [targetId]);

  // Scroll to the deep-link target once, after it resolves (items may stream in).
  // Retry across frames: a row's real height isn't measured until after first
  // paint, so a single scrollToIndex can land at the estimate-based offset.
  useEffect(() => {
    if (targetIndex < 0 || hasScrolledToTarget.current) return;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(targetIndex, { align: 'start' }),
      () => false,
    );
    hasScrolledToTarget.current = true;
  }, [targetIndex, virtualizer]);

  if (loading || error) {
    return <TranscriptPaneStatus loading={loading} error={error} />;
  }

  if (items.length === 0) {
    return (
      <div className={styles.empty}>
        <p>No transcript yet</p>
        <p className={styles.emptyHint}>Messages will appear as they sync</p>
      </div>
    );
  }

  if (filteredItems.length === 0) {
    return (
      <div className={styles.empty}>
        <p>No items to display</p>
        <p className={styles.emptyHint}>Try adjusting your filters</p>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <div ref={parentRef} className={styles.scroll}>
        <div className={styles.virtualizer} style={{ height: `${virtualizer.getTotalSize()}px` }}>
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const item = filteredItems[virtualItem.index];
            if (!item) return null;
            const isTarget = targetId !== undefined && item.id === targetId;
            return (
              <div
                key={virtualItem.key}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                className={cx(styles.slot, isTarget ? styles.slotTarget : undefined)}
                onMouseEnter={() => setSelectedIndex(virtualItem.index)}
                style={{ transform: `translateY(${virtualItem.start}px)` }}
              >
                <Row item={item} isCostMode={isCostMode} />
              </div>
            );
          })}
        </div>
      </div>

      <TimelineBar
        layout={segmentLayout}
        visibleIndices={visibleIndices}
        onSeek={onSeekFromBar}
        assistantLabel="Assistant"
      />
    </div>
  );
}
