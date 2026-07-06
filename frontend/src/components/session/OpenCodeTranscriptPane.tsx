// Renders the transcript-tab content for OpenCode sessions.
//
// A virtualized list of render items (user / assistant / tool) with a
// turn-based minimap / timeline bar alongside (the shared `TimelineBar`),
// in cost mode the shared green `CostBar` side rail (hfk7), and Cmd-F
// in-transcript search (5p9j, via the shared `useTranscriptSearch` toolkit) —
// reaching parity with Claude/Codex. It fetches nothing itself (SessionViewer
// drives fetch/poll via the adapter) and renders the categories with
// reasoning, tool I/O, status, deep-link scroll, per-message cost badges in
// cost mode, and scroll-to + highlight of search matches in unmounted rows.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { CostAmount } from '@/components/CostAmount';
import { cx } from '@/utils/utils';
import { addCmdFListener, retryOnAnimationFrame } from '@/components/transcript/timelineUtils';
import { TimeSeparator } from '@/components/transcript/TimeSeparator';
import TimelineBar from '@/components/transcript/TimelineBar';
import { CostBar } from '@/components/transcript/CostBar';
import TranscriptSearchBar from '@/components/session/TranscriptSearchBar';
import { useTranscriptSearch } from '@/hooks/useTranscriptSearch';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
import { opencodeAdapter } from '@/providers/opencodeAdapter';
import type { TokenUsage } from '@/utils/tokenStats';
import type { OpenCodeRenderItem } from './opencodeCategories';
import { extractOpenCodeItemText } from './extractOpenCodeItemText';
import { useOpenCodeSegmentLayout } from './opencodeTimelineSegments';
import { buildVirtualItems } from './opencodeVirtualItems';
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
// 6h7m: initial size estimate for an injected divider row, before measurement.
const ESTIMATED_SEPARATOR_HEIGHT = 40;

// Zero-cost usage shim for assistant items that carry no `usage` — keeps the
// adapter's `calculateMessageCost` total-type happy; resolves to $0, skipped.
const EMPTY_USAGE: TokenUsage = {
  input: 0,
  output: 0,
  cacheWrite: 0,
  cacheWrite1h: 0,
  cacheRead: 0,
};

// 5p9j: true when `query` (case-insensitive) appears inside `text`. Used to
// force-open a collapsed <details> that holds the active match, so the search
// bar never counts a match the user can't see.
function containsQuery(text: string | undefined, query: string | undefined): boolean {
  if (!text || !query) return false;
  return text.toLowerCase().includes(query.toLowerCase());
}

function ToolRow({
  item,
  searchQuery,
  isCurrentSearchMatch,
}: {
  item: Extract<OpenCodeRenderItem, { kind: 'tool' }>;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}) {
  const isError = item.status === 'error';
  // 5p9j: force the output <details> open when this row is the active match and
  // the query is inside the (otherwise collapsed) output.
  const outputForceOpen =
    isCurrentSearchMatch && containsQuery(item.output, searchQuery) ? true : undefined;
  return (
    <div className={cx(styles.row, styles.toolRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Tool</span>
        <span className={styles.toolName}>{item.toolName}</span>
        <span className={cx(styles.status, isError ? styles.statusError : styles.statusOk)}>
          {item.status}
        </span>
      </div>
      {item.input ? (
        <pre className={styles.toolInput}>
          {renderTextWithHighlight(item.input, searchQuery, isCurrentSearchMatch)}
        </pre>
      ) : null}
      {item.output ? (
        <details className={styles.details} open={outputForceOpen}>
          <summary className={styles.summary}>Output</summary>
          <pre className={styles.toolOutput}>
            {renderTextWithHighlight(item.output, searchQuery, isCurrentSearchMatch)}
          </pre>
        </details>
      ) : null}
    </div>
  );
}

function AssistantRow({
  item,
  isCostMode,
  messageCost,
  searchQuery,
  isCurrentSearchMatch,
}: {
  item: Extract<OpenCodeRenderItem, { kind: 'assistant' }>;
  isCostMode?: boolean;
  /**
   * hfk7: pre-computed $ cost for this row, routed through
   * `opencodeAdapter.calculateMessageCost` so the badge and the CostBar rail
   * total agree (a fallback-priced message with no `info.cost` still shows a
   * badge and counts toward the rail). `undefined` when cost mode is off or
   * the cost is zero.
   */
  messageCost?: number;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}) {
  // 5p9j: force the reasoning <details> open when this row is the active match
  // and the query lives in the (collapsed) reasoning text.
  const reasoningForceOpen =
    isCurrentSearchMatch && containsQuery(item.reasoning, searchQuery) ? true : undefined;
  return (
    <div className={cx(styles.row, styles.assistantRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Assistant</span>
        {item.model ? <span className={styles.model}>{item.model}</span> : null}
        {isCostMode && typeof messageCost === 'number' && messageCost > 0 ? (
          <CostAmount usd={messageCost} className={styles.cost} />
        ) : null}
      </div>
      {item.reasoning ? (
        <details className={styles.details} open={reasoningForceOpen}>
          <summary className={styles.summary}>Reasoning</summary>
          <div className={styles.reasoning}>
            {renderTextWithHighlight(item.reasoning, searchQuery, isCurrentSearchMatch)}
          </div>
        </details>
      ) : null}
      {item.text ? (
        <div className={styles.text}>
          {renderTextWithHighlight(item.text, searchQuery, isCurrentSearchMatch)}
        </div>
      ) : null}
    </div>
  );
}

function Row({
  item,
  isCostMode,
  messageCost,
  searchQuery,
  isCurrentSearchMatch,
}: {
  item: OpenCodeRenderItem;
  isCostMode?: boolean;
  messageCost?: number;
  /** 5p9j: search query (when the search bar is open) — highlights matches. */
  searchQuery?: string;
  /** 5p9j: this row is the active (n-of-N) match — amber highlight + force-open. */
  isCurrentSearchMatch?: boolean;
}) {
  if (item.kind === 'user') {
    return (
      <div className={cx(styles.row, styles.userRow)}>
        <div className={styles.rowHeader}>
          <span className={styles.roleLabel}>User</span>
        </div>
        <div className={styles.text}>
          {renderTextWithHighlight(item.text, searchQuery, isCurrentSearchMatch)}
        </div>
      </div>
    );
  }
  if (item.kind === 'assistant') {
    return (
      <AssistantRow
        item={item}
        isCostMode={isCostMode}
        messageCost={messageCost}
        searchQuery={searchQuery}
        isCurrentSearchMatch={isCurrentSearchMatch}
      />
    );
  }
  if (item.kind === 'tool') {
    return (
      <ToolRow item={item} searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />
    );
  }
  return (
    <OpenCodeUnknownItem
      item={item}
      searchQuery={searchQuery}
      isCurrentSearchMatch={isCurrentSearchMatch}
    />
  );
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

  // 5p9j: Cmd-F transcript search, parameterized over `filteredItems` so
  // matches respect the active filter (a natural consequence of indexing the
  // filtered list). `extractOpenCodeItemText` is a stable module-level fn
  // (see its file note), so the hook's search index doesn't churn every
  // render.
  const search = useTranscriptSearch(filteredItems, extractOpenCodeItemText);
  useEffect(() => addCmdFListener(search.open), [search.open]);

  // 6h7m: virtual-list layer — filtered rows + injected day/idle-gap
  // dividers. OpenCode previously had NO separator rows at all, so the
  // filtered index WAS the virtual index everywhere; now that a divider can
  // precede any row, every consumer that used to scroll to (or report) a raw
  // filtered index must go through `filteredIndexToVirtualIndex` below.
  const virtualItems = useMemo(() => buildVirtualItems(filteredItems), [filteredItems]);

  const filteredIndexToVirtualIndex = useMemo(() => {
    const map = new Map<number, number>();
    virtualItems.forEach((vi, virtualIndex) => {
      if (vi.type === 'item') map.set(vi.index, virtualIndex);
    });
    return map;
  }, [virtualItems]);

  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
  const virtualizer = useVirtualizer({
    count: virtualItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) =>
      virtualItems[index]?.type === 'separator' ? ESTIMATED_SEPARATOR_HEIGHT : ESTIMATED_ROW_HEIGHT,
    overscan: 8,
  });

  const targetIndex = useMemo(() => {
    if (!targetId) return -1;
    return filteredItems.findIndex((it) => it.id === targetId);
  }, [filteredItems, targetId]);

  // The timeline bar's segments index into the unfiltered `items` array, so we
  // translate between the filtered-list index the pane holds internally and
  // the unfiltered index the bar speaks, keyed by the stable render-item `id`.
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

  // hfk7: cost map keyed by UNFILTERED items index — the same axis the segment
  // layout speaks, so the rail and bar line up. Built only in cost mode (skips
  // pricing lookups otherwise). Routed through the adapter's
  // `calculateMessageCost` (prefers reported `info.cost`, else the pricing
  // fallback) so the rail total and per-row badges share one source of truth.
  // Records strictly-positive costs only; zero-cost rows render no badge.
  // NOTE: a filtered-out assistant row STILL contributes here — keying by the
  // unfiltered index is load-bearing.
  const { costByIndex, totalCost } = useMemo(() => {
    const map = new Map<number, number>();
    if (!isCostMode) return { costByIndex: map, totalCost: 0 };
    let total = 0;
    items.forEach((item, idx) => {
      if (item.kind !== 'assistant') return;
      // `model`/`usage` are optional on OpenCode assistant items; the adapter
      // prefers the reported `info.cost` and otherwise prices via the table
      // (an absent model/usage there resolves to $0, which we then skip).
      const cost = opencodeAdapter.calculateMessageCost(
        item.model ?? '',
        item.usage ?? EMPTY_USAGE,
        item,
      );
      if (cost > 0) {
        map.set(idx, cost);
        total += cost;
      }
    });
    return { costByIndex: map, totalCost: total };
  }, [items, isCostMode]);

  // hfk7: assistant-render-items per segment, for the CostBar's density math.
  // OpenCode assistant items are 1:1 with API calls, so no dedup is needed
  // (cf. Claude, where multiple JSONL lines share `message.id`). Order matches
  // `segmentLayout.segments`.
  const costSegmentUniqueCounts = useMemo<number[]>(() => {
    if (!isCostMode) return [];
    return segmentLayout.segments.map((seg) => {
      let n = 0;
      for (let i = seg.startIndex; i <= seg.endIndex; i++) {
        if (items[i]?.kind === 'assistant') n++;
      }
      return n;
    });
  }, [isCostMode, segmentLayout.segments, items]);

  // Track the first visible row so the bar indicator has something to point at
  // when the user hasn't hovered a row. 6h7m: skip divider rows and report
  // the first visible ITEM's filtered index (mirrors ClaudeMessageTimeline's
  // updateFirstVisible / CursorTranscriptPane's equivalent).
  const updateFirstVisible = useCallback(() => {
    const visible = virtualizer.getVirtualItems();
    for (const vItem of visible) {
      const vi = virtualItems[vItem.index];
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

  // 6h7m: `filteredIdx` is the FILTERED index (the axis skip-nav/deep-
  // link/bar-seek all speak) — translate through `filteredIndexToVirtualIndex`
  // before scrolling, so a divider preceding the target row doesn't throw off
  // the landing position.
  const scrollToItem = useCallback(
    (filteredIdx: number) => {
      const virtualIndex = filteredIndexToVirtualIndex.get(filteredIdx);
      if (virtualIndex === undefined) return;
      retryOnAnimationFrame(
        () => virtualizer.scrollToIndex(virtualIndex, { align: 'start' }),
        () => false,
      );
      setSelectedIndex(filteredIdx);
    },
    [filteredIndexToVirtualIndex, virtualizer],
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
  // 6h7m: translate the target's filtered index through
  // `filteredIndexToVirtualIndex` — a divider may now precede it.
  useEffect(() => {
    if (targetIndex < 0 || hasScrolledToTarget.current) return;
    const virtualIndex = filteredIndexToVirtualIndex.get(targetIndex);
    if (virtualIndex === undefined) return;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'start' }),
      () => false,
    );
    hasScrolledToTarget.current = true;
  }, [targetIndex, filteredIndexToVirtualIndex, virtualizer]);

  // 5p9j: scroll to the current search match, then bring its first <mark> into
  // view inside the row. 6h7m: the match's filtered index is no longer its
  // virtual index once dividers can be injected, so translate through
  // `filteredIndexToVirtualIndex` first. Structurally mirrors
  // `CodexMessageTimeline.tsx`: scrollToIndex retries across frames as
  // measurements settle, so we wait a few frames before locating the <mark> to
  // avoid the bring-into-view being clobbered by a retry. This is what surfaces
  // matches that live in rows the virtualizer hasn't mounted yet.
  useEffect(() => {
    if (search.currentMatchFilteredIndex === null) return;
    const idx = search.currentMatchFilteredIndex;
    const virtualIndex = filteredIndexToVirtualIndex.get(idx);
    if (virtualIndex === undefined) return;

    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
      () => false,
    );
    setSelectedIndex(idx);

    let cancelled = false;
    const scrollToIndexFrames = 6;
    const maxMarkRetries = 10;
    function scrollToMark(attempt: number) {
      if (cancelled || attempt >= maxMarkRetries) return;
      const scrollEl = parentRef.current;
      if (!scrollEl) return;
      const rowEl = scrollEl.querySelector(`[data-index="${virtualIndex}"]`);
      if (!rowEl) {
        requestAnimationFrame(() => scrollToMark(attempt + 1));
        return;
      }
      const mark = rowEl.querySelector('mark');
      if (mark) {
        mark.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
      } else {
        requestAnimationFrame(() => scrollToMark(attempt + 1));
      }
    }
    function delayThenScroll(framesLeft: number) {
      if (cancelled) return;
      if (framesLeft <= 0) {
        scrollToMark(0);
        return;
      }
      requestAnimationFrame(() => delayThenScroll(framesLeft - 1));
    }
    delayThenScroll(scrollToIndexFrames);

    return () => {
      cancelled = true;
    };
  }, [search.currentMatchFilteredIndex, filteredIndexToVirtualIndex, virtualizer]);

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
            const vi = virtualItems[virtualItem.index];
            if (!vi) return null;

            const slotStyle = { transform: `translateY(${virtualItem.start}px)` };

            // 6h7m: injected day/idle-gap divider.
            if (vi.type === 'separator') {
              return (
                <div
                  key={virtualItem.key}
                  ref={virtualizer.measureElement}
                  data-index={virtualItem.index}
                  className={styles.slot}
                  style={slotStyle}
                >
                  <TimeSeparator label={vi.label} />
                </div>
              );
            }

            const item = vi.item;
            const filteredIndex = vi.index;
            const isTarget = targetId !== undefined && item.id === targetId;
            // hfk7: cost badges read the SAME unfiltered-keyed map the rail
            // uses, so badge and rail always agree.
            const unfilteredIdx = idToUnfilteredIndex.get(item.id);
            const messageCost =
              isCostMode && unfilteredIdx !== undefined
                ? costByIndex.get(unfilteredIdx)
                : undefined;
            const isCurrentSearchMatch = search.currentMatchFilteredIndex === filteredIndex;
            const searchQuery = search.isOpen ? search.highlightQuery : undefined;
            return (
              <div
                key={virtualItem.key}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                className={cx(styles.slot, isTarget ? styles.slotTarget : undefined)}
                onMouseEnter={() => setSelectedIndex(filteredIndex)}
                style={slotStyle}
              >
                <Row
                  item={item}
                  isCostMode={isCostMode}
                  messageCost={messageCost}
                  searchQuery={searchQuery}
                  isCurrentSearchMatch={isCurrentSearchMatch}
                />
              </div>
            );
          })}
        </div>
      </div>

      {/* hfk7: shared green cost rail, gated on cost mode. CostBar.onSeek
          passes (start, end); only start matters, so reuse onSeekFromBar (it
          maps unfiltered→first-visible-filtered→scrollToIndex). */}
      <div
        className={cx(styles.costBarWrapper, isCostMode && styles.costBarWrapperVisible)}
      >
        {isCostMode && (
          <CostBar
            layout={segmentLayout}
            costByIndex={costByIndex}
            segmentUniqueCounts={costSegmentUniqueCounts}
            totalCost={totalCost}
            onSeek={onSeekFromBar}
          />
        )}
      </div>

      <TimelineBar
        layout={segmentLayout}
        visibleIndices={visibleIndices}
        onSeek={onSeekFromBar}
        assistantLabel="Assistant"
      />

      {/* 5p9j: shared Cmd-F search bar. position:fixed, so it overlays the
          .container without disturbing the scroll/CostBar/TimelineBar layout. */}
      {search.isOpen && (
        <TranscriptSearchBar
          query={search.query}
          onQueryChange={search.setQuery}
          currentMatch={search.matches.length > 0 ? search.currentMatchIndex + 1 : 0}
          totalMatches={search.matches.length}
          onNext={search.goToNextMatch}
          onPrev={search.goToPreviousMatch}
          onClose={search.close}
          inputRef={search.inputRef}
        />
      )}
    </div>
  );
}
