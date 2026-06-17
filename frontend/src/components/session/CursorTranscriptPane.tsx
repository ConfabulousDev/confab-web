// Renders the transcript-tab content for Cursor sessions (18n2, MVP).
//
// A virtualized list of render items (user / assistant / tool) with Cmd-F
// in-transcript search (shared `useTranscriptSearch` toolkit), deep-link
// scroll-to, and a shared turn-based TimelineBar minimap (zztp). Leaner than
// Claude/Codex/OpenCode in one way: NO cost rail — Cursor's JSONL carries no
// token/cost data. Row times (and therefore the timeline segments) are
// ESTIMATED (ce79): interpolated over the session bounds, never sourced from
// the wire, so the bar hides entirely when the bounds are unknown and its
// tooltip flags the estimate. It fetches nothing itself (SessionViewer drives
// fetch/poll via the adapter).
//
// Tool rows render the call (name + one-line input summary) with NO output:
// Cursor records tool inputs only, never results.

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { cx } from '@/utils/utils';
import { addCmdFListener, retryOnAnimationFrame } from '@/components/transcript/timelineUtils';
import { formatCodexTimestamp } from '@/components/transcript/codex/codexFormat';
import TimelineBar from '@/components/transcript/TimelineBar';
import { useCursorSegmentLayout } from './cursorTimelineSegments';
import TranscriptSearchBar from '@/components/session/TranscriptSearchBar';
import { useTranscriptSearch } from '@/hooks/useTranscriptSearch';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
import RowActions from '@/components/transcript/RowActions';
import type { CursorRenderItem } from './cursorCategories';
import { attachCursorTimestamps } from '@/services/cursorTranscriptService';
import {
  buildCursorRowNav,
  cursorRowKindLabel,
  buildCursorRowCopyText,
} from './cursorRowNav';
import CursorContextSections from './CursorContextSections';
import CursorMessageBody from './CursorMessageBody';
import { extractCursorItemText } from './extractCursorItemText';
import TranscriptPaneStatus from './TranscriptPaneStatus';
import styles from './CursorTranscriptPane.module.css';

// Tooltip shown on every estimated row time — Cursor transcripts have no
// per-message timestamps, so these are interpolated, not real (ce79).
const ESTIMATED_TIME_TOOLTIP =
  'Estimated — Cursor transcripts have no per-message timestamps.';

// zztp: disclaimer appended to every TimelineBar segment tooltip — the segment
// durations come from the same estimated per-row times, so they are
// approximate, not real per-message wall-clock.
const TIMELINE_ESTIMATED_NOTE = 'Estimated times';

/** Row-header time marker for an estimated Cursor timestamp: a muted `~` prefix
 *  plus the formatted time, with the "estimated" tooltip. Renders nothing when
 *  the row has no timestamp (bounds unknown). */
function EstimatedTime({ timestamp }: { timestamp?: string }) {
  if (!timestamp) return null;
  return (
    <span className={styles.estimatedTime} title={ESTIMATED_TIME_TOOLTIP}>
      <span className={styles.estimatedTilde}>~</span>
      {formatCodexTimestamp(timestamp)}
    </span>
  );
}

export interface CursorTranscriptPaneProps {
  sessionId: string;
  /** Unfiltered render items — distinguishes "no transcript yet" from "filtered out". */
  items: CursorRenderItem[];
  /** Post-filter render items — drives the row list. */
  filteredItems: CursorRenderItem[];
  loading: boolean;
  error: string | null;
  /** Deep-link target, addressed by render-item id (synthetic line-based id). */
  targetId?: string;
  /** Session start bound (`first_seen`). With `lastSyncAt`, drives the ESTIMATED
   *  per-row timestamps (ce79). Omitted/absent → row headers show no time. */
  firstSeen?: string | null;
  /** Session end bound (`last_sync_at`). See `firstSeen`. */
  lastSyncAt?: string | null;
}

const ESTIMATED_ROW_HEIGHT = 120;

function ToolRow({
  item,
  searchQuery,
  isCurrentSearchMatch,
  rowActions,
}: {
  item: Extract<CursorRenderItem, { kind: 'tool' }>;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
  /** a9gr: per-row action cluster (copy text / copy link / skip nav). */
  rowActions?: ReactNode;
}) {
  return (
    <div className={cx(styles.row, styles.toolRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Tool</span>
        <span className={styles.toolName}>{item.toolName}</span>
        <EstimatedTime timestamp={item.timestamp} />
        {rowActions}
      </div>
      {item.input ? (
        <pre className={styles.toolInput}>
          {renderTextWithHighlight(item.input, searchQuery, isCurrentSearchMatch)}
        </pre>
      ) : null}
    </div>
  );
}

function Row({
  item,
  searchQuery,
  isCurrentSearchMatch,
  rowActions,
}: {
  item: CursorRenderItem;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
  /** a9gr: per-row action cluster (copy text / copy link / skip nav). */
  rowActions?: ReactNode;
}) {
  if (item.kind === 'user') {
    return (
      <div className={cx(styles.row, styles.userRow)}>
        <div className={styles.rowHeader}>
          <span className={styles.roleLabel}>User</span>
          <EstimatedTime timestamp={item.timestamp} />
          {rowActions}
        </div>
        <CursorMessageBody
          text={item.text}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
        <CursorContextSections sections={item.sections ?? []} />
      </div>
    );
  }
  if (item.kind === 'assistant') {
    return (
      <div className={cx(styles.row, styles.assistantRow)}>
        <div className={styles.rowHeader}>
          <span className={styles.roleLabel}>Assistant</span>
          <EstimatedTime timestamp={item.timestamp} />
          {rowActions}
        </div>
        <CursorMessageBody
          text={item.text}
          searchQuery={searchQuery}
          isCurrentSearchMatch={isCurrentSearchMatch}
        />
      </div>
    );
  }
  return (
    <ToolRow
      item={item}
      searchQuery={searchQuery}
      isCurrentSearchMatch={isCurrentSearchMatch}
      rowActions={rowActions}
    />
  );
}

export default function CursorTranscriptPane({
  sessionId,
  items,
  filteredItems,
  loading,
  error,
  targetId,
  firstSeen,
  lastSyncAt,
}: CursorTranscriptPaneProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const hasScrolledToTarget = useRef(false);
  // zztp: index of the topmost visible row, so the bar indicator has something
  // to point at before the user hovers a row.
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);

  // Estimate per-row timestamps over the FULL item stream (ce79), so each row's
  // time reflects its true position in the session — independent of which
  // categories are currently filtered in. The timeline segments (zztp) also
  // read these timestamps, so compute the stamped stream once and look it up by
  // id when rendering the filtered rows. A no-op (timestamps undefined) when
  // bounds are unknown.
  const stampedItems = useMemo(
    () => attachCursorTimestamps(items, { start: firstSeen, end: lastSyncAt }),
    [items, firstSeen, lastSyncAt],
  );
  const timestampById = useMemo(
    () => new Map(stampedItems.map((it) => [it.id, it.timestamp])),
    [stampedItems],
  );

  // Cmd-F transcript search over the filtered list. Cursor has no separator
  // rows, so the filtered index IS the virtual index — no indirection.
  // `extractCursorItemText` is a stable module-level fn so the search index
  // doesn't churn every render.
  const search = useTranscriptSearch(filteredItems, extractCursorItemText);
  useEffect(() => addCmdFListener(search.open), [search.open]);

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

  // a9gr: same-kind skip-nav neighbor maps over the FILTERED items (so skip
  // jumps between visible rows only). The filtered index IS the virtual index
  // for Cursor — no separator rows — so we scroll straight to it.
  const { nextOfSameKind, prevOfSameKind } = useMemo(
    () => buildCursorRowNav(filteredItems),
    [filteredItems],
  );

  const scrollToRow = useCallback(
    (index: number) => {
      retryOnAnimationFrame(
        () => virtualizer.scrollToIndex(index, { align: 'start' }),
        () => false,
      );
    },
    [virtualizer],
  );

  // Map each item id to its index in the UNFILTERED stream — the axis both the
  // segment layout and the position indicator speak. Cursor has no separator
  // rows, so ids are the only stable handle between the filtered and unfiltered
  // arrays. Mirrors OpenCodeTranscriptPane.
  const idToUnfilteredIndex = useMemo(() => {
    const map = new Map<string, number>();
    items.forEach((item, idx) => map.set(item.id, idx));
    return map;
  }, [items]);

  // zztp / CF-361 parity: the set of UNFILTERED indices whose item survives the
  // active filter, so the bar greys out fully-filtered segments. `undefined`
  // when nothing is filtered.
  const visibleIndices = useMemo(() => {
    if (filteredItems.length === items.length) return undefined;
    const set = new Set<number>();
    filteredItems.forEach((item) => {
      const idx = idToUnfilteredIndex.get(item.id);
      if (idx !== undefined) set.add(idx);
    });
    return set;
  }, [items.length, filteredItems, idToUnfilteredIndex]);

  // The segment layout indexes the UNFILTERED stream, but the indicator tracks
  // the topmost visible FILTERED row — translate that row's id back to its
  // unfiltered index.
  const selectedUnfilteredIndex = useMemo(() => {
    const selected = filteredItems[firstVisibleIndex];
    if (!selected) return 0;
    return idToUnfilteredIndex.get(selected.id) ?? 0;
  }, [filteredItems, firstVisibleIndex, idToUnfilteredIndex]);

  // zztp: turn-based timeline segments over the estimated-timestamp stream. The
  // layout is empty (and the bar self-hides) whenever the row times are unknown.
  const segmentLayout = useCursorSegmentLayout(stampedItems, selectedUnfilteredIndex);

  // Track the topmost visible row so the bar indicator has something to point at
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

  // Bar click → scroll to the first visible item at or after the segment's
  // unfiltered start index. The bar only fires clicks on un-filtered segments,
  // so at least one item in the range maps into the filtered list.
  const onSeekFromBar = useCallback(
    (unfilteredStart: number) => {
      const filteredIdById = new Map<string, number>();
      filteredItems.forEach((it, idx) => filteredIdById.set(it.id, idx));
      for (let i = unfilteredStart; i < items.length; i++) {
        const candidate = items[i];
        if (!candidate) continue;
        const filteredIdx = filteredIdById.get(candidate.id);
        if (filteredIdx !== undefined) {
          scrollToRow(filteredIdx);
          return;
        }
      }
    },
    [items, filteredItems, scrollToRow],
  );

  // Re-arm the one-shot scroll when the deep-link target changes.
  useEffect(() => {
    hasScrolledToTarget.current = false;
  }, [targetId]);

  // Scroll to the deep-link target once, after it resolves (items may stream in).
  useEffect(() => {
    if (targetIndex < 0 || hasScrolledToTarget.current) return;
    scrollToRow(targetIndex);
    hasScrolledToTarget.current = true;
  }, [targetIndex, scrollToRow]);

  // Scroll to the current search match, then bring its first <mark> into view.
  // The match's filtered index IS its virtual index (no divider rows). Mirrors
  // the OpenCode pane's settle-then-locate sequence so matches in unmounted
  // rows still surface.
  useEffect(() => {
    if (search.currentMatchFilteredIndex === null) return;
    const idx = search.currentMatchFilteredIndex;

    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(idx, { align: 'center' }),
      () => false,
    );

    let cancelled = false;
    const scrollToIndexFrames = 6;
    const maxMarkRetries = 10;
    function scrollToMark(attempt: number) {
      if (cancelled || attempt >= maxMarkRetries) return;
      const scrollEl = parentRef.current;
      if (!scrollEl) return;
      const rowEl = scrollEl.querySelector(`[data-index="${idx}"]`);
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
  }, [search.currentMatchFilteredIndex, virtualizer]);

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
            const rawItem = filteredItems[virtualItem.index];
            if (!rawItem) return null;
            // Overlay the estimated timestamp (looked up by id from the full
            // stream) onto the filtered render item for this row.
            const item: CursorRenderItem = { ...rawItem, timestamp: timestampById.get(rawItem.id) };
            const isTarget = targetId !== undefined && item.id === targetId;
            const isCurrentSearchMatch = search.currentMatchFilteredIndex === virtualItem.index;
            const searchQuery = search.isOpen ? search.highlightQuery : undefined;
            // a9gr: per-row action cluster. Deep-link uses the synthetic stable
            // `item.id` (estimated timestamps collide/shift — the existing
            // resolver matches `item.id` directly); copy-text is the raw row
            // payload; skip jumps to the next/prev same-kind row.
            const nextIdx = nextOfSameKind.get(virtualItem.index);
            const prevIdx = prevOfSameKind.get(virtualItem.index);
            const rowActions = (
              <RowActions
                sessionId={sessionId}
                deepLinkMsg={item.id}
                copyText={buildCursorRowCopyText(item)}
                onSkipToNext={nextIdx !== undefined ? () => scrollToRow(nextIdx) : undefined}
                onSkipToPrevious={prevIdx !== undefined ? () => scrollToRow(prevIdx) : undefined}
                kindLabel={cursorRowKindLabel(item)}
              />
            );
            return (
              <div
                key={virtualItem.key}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                className={cx(styles.slot, isTarget ? styles.slotTarget : undefined)}
                style={{ transform: `translateY(${virtualItem.start}px)` }}
              >
                <Row
                  item={item}
                  searchQuery={searchQuery}
                  isCurrentSearchMatch={isCurrentSearchMatch}
                  rowActions={rowActions}
                />
              </div>
            );
          })}
        </div>
      </div>

      {/* zztp: shared turn-based minimap. Self-hides when the estimated row
          times are unknown (empty layout). assistantLabel="Assistant" matches
          the OpenCode pane for cross-provider tooltip consistency; the
          estimated-times note flags that the durations are interpolated. */}
      <TimelineBar
        layout={segmentLayout}
        visibleIndices={visibleIndices}
        onSeek={onSeekFromBar}
        assistantLabel="Assistant"
        tooltipNote={TIMELINE_ESTIMATED_NOTE}
      />

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
