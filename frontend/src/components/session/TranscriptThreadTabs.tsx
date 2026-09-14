// et0r: subtab strip under the Transcript tab — `Main` pinned first, then one
// tab per transcript thread (subagent) in launch order. Provider-agnostic: it
// only knows `TranscriptThreadRef`s supplied by an adapter's `threads`
// capability via SessionViewer.

import { useEffect, useRef } from 'react';
import type { TranscriptThreadRef } from '@/providers/types';
import styles from './TranscriptThreadTabs.module.css';

interface TranscriptThreadTabsProps {
  threads: TranscriptThreadRef[];
  /** null = Main. */
  activeThreadId: string | null;
  onSelect: (threadId: string | null) => void;
  /** "← Launched from <label>" link back to the parent thread's launch row. */
  backLink?: { label: string; onClick: () => void };
}

export default function TranscriptThreadTabs({
  threads,
  activeThreadId,
  onSelect,
  backLink,
}: TranscriptThreadTabsProps) {
  const activeTabRef = useRef<HTMLButtonElement>(null);

  // Keep the active tab visible in a horizontally scrolled strip.
  useEffect(() => {
    activeTabRef.current?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' });
  }, [activeThreadId]);

  function renderTab(threadId: string | null, label: string) {
    const selected = threadId === activeThreadId;
    return (
      <button
        key={threadId ?? '\u0000main'}
        ref={selected ? activeTabRef : undefined}
        type="button"
        role="tab"
        aria-selected={selected}
        title={label}
        className={selected ? `${styles.tab} ${styles.tabActive}` : styles.tab}
        onClick={() => onSelect(threadId)}
      >
        <span className={styles.label}>{label}</span>
      </button>
    );
  }

  return (
    <div className={styles.bar}>
      <div role="tablist" aria-label="Transcript threads" className={styles.tablist}>
        {renderTab(null, 'Main')}
        {threads.map((thread) => renderTab(thread.id, thread.label))}
      </div>
      {backLink && (
        <button type="button" className={styles.backLink} onClick={backLink.onClick}>
          ← Launched from {backLink.label}
        </button>
      )}
    </div>
  );
}
