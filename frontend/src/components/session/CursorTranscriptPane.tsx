// Renders the transcript-tab content for Cursor sessions (18n2, MVP).
//
// A virtualized list of render items (user / assistant / tool) with Cmd-F
// in-transcript search (shared `useTranscriptSearch` toolkit) and deep-link
// scroll-to. Intentionally leaner than Claude/Codex/OpenCode: NO minimap /
// timeline bar and NO cost rail — Cursor's JSONL carries no timestamps (no
// timeline axis) and no token/cost data (no cost rail). It fetches nothing
// itself (SessionViewer drives fetch/poll via the adapter).
//
// Tool rows render the call (name + one-line input summary) with NO output:
// Cursor records tool inputs only, never results.

import { useEffect, useMemo, useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { cx } from '@/utils/utils';
import { addCmdFListener, retryOnAnimationFrame } from '@/components/transcript/timelineUtils';
import TranscriptSearchBar from '@/components/session/TranscriptSearchBar';
import { useTranscriptSearch } from '@/hooks/useTranscriptSearch';
import { renderTextWithHighlight } from '@/utils/renderHighlight';
import type { CursorRenderItem } from './cursorCategories';
import { extractCursorItemText } from './extractCursorItemText';
import TranscriptPaneStatus from './TranscriptPaneStatus';
import styles from './CursorTranscriptPane.module.css';

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
}

const ESTIMATED_ROW_HEIGHT = 120;

function ToolRow({
  item,
  searchQuery,
  isCurrentSearchMatch,
}: {
  item: Extract<CursorRenderItem, { kind: 'tool' }>;
  searchQuery?: string;
  isCurrentSearchMatch?: boolean;
}) {
  return (
    <div className={cx(styles.row, styles.toolRow)}>
      <div className={styles.rowHeader}>
        <span className={styles.roleLabel}>Tool</span>
        <span className={styles.toolName}>{item.toolName}</span>
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
}: {
  item: CursorRenderItem;
  searchQuery?: string;
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
      <div className={cx(styles.row, styles.assistantRow)}>
        <div className={styles.rowHeader}>
          <span className={styles.roleLabel}>Assistant</span>
        </div>
        <div className={styles.text}>
          {renderTextWithHighlight(item.text, searchQuery, isCurrentSearchMatch)}
        </div>
      </div>
    );
  }
  return (
    <ToolRow item={item} searchQuery={searchQuery} isCurrentSearchMatch={isCurrentSearchMatch} />
  );
}

export default function CursorTranscriptPane({
  items,
  filteredItems,
  loading,
  error,
  targetId,
}: CursorTranscriptPaneProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const hasScrolledToTarget = useRef(false);

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

  // Re-arm the one-shot scroll when the deep-link target changes.
  useEffect(() => {
    hasScrolledToTarget.current = false;
  }, [targetId]);

  // Scroll to the deep-link target once, after it resolves (items may stream in).
  useEffect(() => {
    if (targetIndex < 0 || hasScrolledToTarget.current) return;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(targetIndex, { align: 'start' }),
      () => false,
    );
    hasScrolledToTarget.current = true;
  }, [targetIndex, virtualizer]);

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
            const item = filteredItems[virtualItem.index];
            if (!item) return null;
            const isTarget = targetId !== undefined && item.id === targetId;
            const isCurrentSearchMatch = search.currentMatchFilteredIndex === virtualItem.index;
            const searchQuery = search.isOpen ? search.highlightQuery : undefined;
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
                />
              </div>
            );
          })}
        </div>
      </div>

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
