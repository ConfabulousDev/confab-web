// Renders the transcript-tab content for OpenCode sessions (MVP).
//
// A virtualized list of render items (user / assistant / tool). Deliberately
// leaner than the Claude/Codex timelines — no minimap bar, cost side-rail, or
// Cmd-F search yet — but real: it fetches nothing itself (SessionViewer drives
// fetch/poll via the adapter) and renders the three categories with reasoning,
// tool I/O, status, deep-link scroll, and per-message cost badges in cost mode.

import { useEffect, useMemo, useRef } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { formatCost } from '@/utils/tokenStats';
import { cx } from '@/utils/utils';
import { retryOnAnimationFrame } from '@/components/transcript/timelineUtils';
import type { OpenCodeRenderItem } from './opencodeCategories';
import TranscriptPaneStatus from './TranscriptPaneStatus';
import styles from './OpenCodeTranscriptPane.module.css';

export interface OpenCodeTranscriptPaneProps {
  sessionId: string;
  /** Unfiltered render items — distinguishes "no transcript yet" from "filtered out". */
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
          <span className={styles.cost}>{formatCost(item.cost)}</span>
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
  return <ToolRow item={item} />;
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
              style={{ transform: `translateY(${virtualItem.start}px)` }}
            >
              <Row item={item} isCostMode={isCostMode} />
            </div>
          );
        })}
      </div>
    </div>
  );
}
