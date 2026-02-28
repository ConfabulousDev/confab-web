import { useMemo, useRef, useCallback, useState, useEffect } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { TranscriptLine } from '@/types';
import { isAssistantMessage, isToolUseBlock } from '@/types';
import { useTranscriptSearch } from '@/hooks/useTranscriptSearch';
import { calculateMessageCost } from '@/utils/tokenStats';
import TimelineMessage from './TimelineMessage';
import TranscriptSearchBar from './TranscriptSearchBar';
import { getRoleLabel } from './messageCategories';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import { TimelineBar } from '@/components/transcript/TimelineBar';
import { CostBar } from '@/components/transcript/CostBar';
import styles from './MessageTimeline.module.css';

// Right offset for ScrollNavButtons when CostBar is visible.
// CostBar (22px) + gap (8px from --spacing-sm) + default right (24px from --spacing-xl) + 2px breathing room
const SCROLL_NAV_COST_MODE_RIGHT = 56;

interface MessageTimelineProps {
  messages: TranscriptLine[];
  allMessages: TranscriptLine[]; // Used for building tool name map
  targetMessageUuid?: string; // Deep-link target message UUID
  sessionId?: string; // Session ID for copy-link URLs
  isCostMode?: boolean; // When true, show cost heatmap and per-message cost badges
}

// Item types for virtual list
type VirtualItem =
  | { type: 'message'; message: TranscriptLine; index: number; filteredIndex: number }
  | { type: 'separator'; timestamp: string };

/**
 * Check if we should show a time separator between messages
 */
function shouldShowTimeSeparator(current: TranscriptLine, previous: TranscriptLine | undefined): boolean {
  if (!previous) return false;

  const currentTime = 'timestamp' in current && typeof current.timestamp === 'string' ? new Date(current.timestamp) : null;
  const previousTime = 'timestamp' in previous && typeof previous.timestamp === 'string' ? new Date(previous.timestamp) : null;

  if (!currentTime || !previousTime) return false;

  // Show separator if more than 5 minutes between messages
  const diff = currentTime.getTime() - previousTime.getTime();
  return diff > 5 * 60 * 1000;
}

/**
 * Format timestamp for time separator
 */
function formatTimeSeparator(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const messageDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());

  if (messageDate.getTime() === today.getTime()) {
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    });
  }

  return date.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Build a map of tool_use_id -> tool name for matching tool results
 */
function buildToolNameMap(messages: TranscriptLine[]): Map<string, string> {
  const map = new Map<string, string>();

  for (const message of messages) {
    if (isAssistantMessage(message)) {
      for (const block of message.message.content) {
        if (isToolUseBlock(block)) {
          map.set(block.id, block.name);
        }
      }
    }
  }

  return map;
}

/**
 * Repeatedly call `action` across animation frames until `shouldStop` returns
 * true or `maxAttempts` is reached. Useful for virtual scroll positioning where
 * item sizes are estimated until measured.
 */
function retryOnAnimationFrame(
  action: () => void,
  shouldStop: () => boolean,
  maxAttempts = 5,
): void {
  function attempt(n: number) {
    action();
    if (n < maxAttempts && !shouldStop()) {
      requestAnimationFrame(() => attempt(n + 1));
    }
  }
  attempt(0);
}

function MessageTimeline({ messages, allMessages, targetMessageUuid, sessionId, isCostMode }: MessageTimelineProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const [firstVisibleIndex, setFirstVisibleIndex] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const hasScrolledToTarget = useRef(false);

  // Transcript search
  const search = useTranscriptSearch(messages);

  // Build tool name map from all messages (not just filtered)
  const toolNameMap = useMemo(() => buildToolNameMap(allMessages), [allMessages]);

  // Compute per-message cost map (allMessages index → $ cost) — only when cost mode is on
  const messageCosts = useMemo(() => {
    const map = new Map<number, number>();
    if (!isCostMode) return map;
    for (let i = 0; i < allMessages.length; i++) {
      const cost = calculateMessageCost(allMessages[i]!);
      if (cost > 0) map.set(i, cost);
    }
    return map;
  }, [allMessages, isCostMode]);

  const totalCost = useMemo(() => {
    let sum = 0;
    for (const cost of messageCosts.values()) sum += cost;
    return sum;
  }, [messageCosts]);

  // Build UUID-to-allMessages-index map for deep-linking
  const uuidToAllIndex = useMemo(() => {
    const map = new Map<string, number>();
    for (const [i, msg] of allMessages.entries()) {
      if ('uuid' in msg && typeof msg.uuid === 'string') {
        map.set(msg.uuid, i);
      }
    }
    return map;
  }, [allMessages]);

  // Derive the allMessages index for the deep-link target
  const targetMessageAllIndex = targetMessageUuid !== undefined
    ? uuidToAllIndex.get(targetMessageUuid) ?? null
    : null;

  // Build a map from message reference to its index in allMessages
  // This is needed because TimelineBar uses allMessages indices
  const messageToAllIndex = useMemo(() => {
    const map = new Map<TranscriptLine, number>();
    allMessages.forEach((msg, idx) => map.set(msg, idx));
    return map;
  }, [allMessages]);

  // Set of allMessages indices that are in the filtered view
  // Used by TimelineBar to show filtered-out segments as grey
  const visibleIndices = useMemo(() => {
    const set = new Set<number>();
    messages.forEach((msg) => {
      const idx = messageToAllIndex.get(msg);
      if (idx !== undefined) set.add(idx);
    });
    return set;
  }, [messages, messageToAllIndex]);

  // Build virtual items list with time separators
  // Note: item.index is the index in allMessages (for TimelineBar compatibility)
  // item.filteredIndex is the index in the filtered messages array
  const virtualItems = useMemo<VirtualItem[]>(() => {
    const items: VirtualItem[] = [];

    messages.forEach((message, filteredIndex) => {
      const prevMessage = filteredIndex > 0 ? messages[filteredIndex - 1] : undefined;

      // Add time separator if needed
      if (shouldShowTimeSeparator(message, prevMessage)) {
        if ('timestamp' in message && typeof message.timestamp === 'string') {
          items.push({ type: 'separator', timestamp: message.timestamp });
        }
      }

      // Use allMessages index for TimelineBar compatibility
      const allIndex = messageToAllIndex.get(message) ?? filteredIndex;
      items.push({ type: 'message', message, index: allIndex, filteredIndex });
    });

    return items;
  }, [messages, messageToAllIndex]);

  // Precompute next/prev same-role indices for skip navigation
  const { nextOfSameRole, prevOfSameRole } = useMemo(() => {
    const next = new Map<number, number>();
    const prev = new Map<number, number>();
    // Track last seen index per role label
    const lastSeenByRole = new Map<string, number>();

    for (let i = 0; i < messages.length; i++) {
      const msg = messages[i];
      if (!msg) continue;
      const label = getRoleLabel(msg);
      const prevIdx = lastSeenByRole.get(label);
      if (prevIdx !== undefined) {
        next.set(prevIdx, i);
        prev.set(i, prevIdx);
      }
      lastSeenByRole.set(label, i);
    }

    return { nextOfSameRole: next, prevOfSameRole: prev };
  }, [messages]);

  // Setup virtual scrolling
  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
  const virtualizer = useVirtualizer({
    count: virtualItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => {
      const item = virtualItems[index];
      if (!item) return 100;

      if (item.type === 'separator') {
        return 40;
      }

      // Estimate based on message type
      const msg = item.message;
      if (msg.type === 'user') return 80;
      if (msg.type === 'assistant') {
        const contentLength = JSON.stringify(msg).length;
        if (contentLength > 2000) return 400;
        if (contentLength > 1000) return 250;
        if (contentLength > 500) return 150;
        return 100;
      }
      return 80;
    },
    overscan: 5,
  });

  // Build a map from message index to virtual item index for scrollToMessage
  const messageIndexToVirtualIndex = useMemo(() => {
    const map = new Map<number, number>();
    virtualItems.forEach((item, virtualIndex) => {
      if (item.type === 'message') {
        map.set(item.index, virtualIndex);
      }
    });
    return map;
  }, [virtualItems]);

  // Reset scroll guard when target changes
  useEffect(() => {
    hasScrolledToTarget.current = false;
  }, [targetMessageUuid]);

  // Scroll to deep-link target on load
  useEffect(() => {
    if (targetMessageAllIndex === null || hasScrolledToTarget.current) return;

    const virtualIndex = messageIndexToVirtualIndex.get(targetMessageAllIndex);
    if (virtualIndex === undefined) return;

    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
      () => false, // always retry - sizes are estimated until measured
    );
    setSelectedIndex(targetMessageAllIndex);
    hasScrolledToTarget.current = true;
  }, [targetMessageAllIndex, messageIndexToVirtualIndex, virtualizer]);

  // Intercept Cmd/Ctrl+F to open transcript search
  const openSearch = search.open;
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
        e.preventDefault();
        openSearch();
      }
    }
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [openSearch]);

  // Scroll to current search match, then scroll first <mark> into view within the message
  useEffect(() => {
    if (search.currentMatchFilteredIndex === null) return;

    // Convert filteredIndex → allMessages index → virtualIndex
    const matchedMessage = messages[search.currentMatchFilteredIndex];
    if (!matchedMessage) return;
    const allIndex = messageToAllIndex.get(matchedMessage);
    if (allIndex === undefined) return;
    const virtualIndex = messageIndexToVirtualIndex.get(allIndex);
    if (virtualIndex === undefined) return;

    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
      () => false,
    );
    setSelectedIndex(allIndex);

    // After the message scrolls into view, scroll the first <mark> into view.
    // Wait for scrollToIndex retries to settle (6 frames) before starting,
    // otherwise scrollToIndex will override our scrollIntoView.
    // Then retry across frames in case the virtualizer hasn't finished
    // rendering tall messages yet.
    let cancelled = false;
    const scrollToIndexFrames = 6;
    const maxMarkRetries = 10;
    function scrollToMark(attempt: number) {
      if (cancelled || attempt >= maxMarkRetries) return;
      const scrollEl = parentRef.current;
      if (!scrollEl) return;
      const messageEl = scrollEl.querySelector(`[data-index="${virtualIndex}"]`);
      if (!messageEl) {
        requestAnimationFrame(() => scrollToMark(attempt + 1));
        return;
      }
      const mark = messageEl.querySelector('mark');
      if (mark) {
        mark.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
      } else {
        requestAnimationFrame(() => scrollToMark(attempt + 1));
      }
    }
    // Delay start so scrollToIndex retries don't override the mark scroll
    function delayThenScroll(framesLeft: number) {
      if (cancelled) return;
      if (framesLeft <= 0) { scrollToMark(0); return; }
      requestAnimationFrame(() => delayThenScroll(framesLeft - 1));
    }
    delayThenScroll(scrollToIndexFrames);

    return () => { cancelled = true; };
  }, [search.currentMatchFilteredIndex, messages, messageToAllIndex, messageIndexToVirtualIndex, virtualizer]);

  // Track first visible message for TimelineBar position indicator
  const updateFirstVisible = useCallback(() => {
    const visibleItems = virtualizer.getVirtualItems();
    if (visibleItems.length === 0) return;

    // Find first visible message (skip separators)
    for (const vItem of visibleItems) {
      const item = virtualItems[vItem.index];
      if (item && item.type === 'message') {
        setFirstVisibleIndex(item.index);
        return;
      }
    }
  }, [virtualizer, virtualItems]);

  // Attach scroll listener
  useEffect(() => {
    const scrollElement = parentRef.current;
    if (!scrollElement) return;

    scrollElement.addEventListener('scroll', updateFirstVisible, { passive: true });
    updateFirstVisible(); // Initial position

    return () => {
      scrollElement.removeEventListener('scroll', updateFirstVisible);
    };
  }, [updateFirstVisible]);

  // Scroll to a message in the given range (used by TimelineBar)
  // Tries each index from startIndex to endIndex until finding one in view
  const scrollToMessage = useCallback((startIndex: number, endIndex: number) => {
    for (let i = startIndex; i <= endIndex; i++) {
      const virtualIndex = messageIndexToVirtualIndex.get(i);
      if (virtualIndex !== undefined) {
        virtualizer.scrollToIndex(virtualIndex, { align: 'start' });
        setSelectedIndex(i);
        return;
      }
    }
    // No messages in range are visible (all filtered out)
  }, [messageIndexToVirtualIndex, virtualizer]);

  // Handle message hover
  const handleMessageHover = useCallback((messageIndex: number | null) => {
    setSelectedIndex(messageIndex);
  }, []);

  // Scroll to a message by its filtered index (used by skip navigation)
  const scrollToFilteredIndex = useCallback((filteredIndex: number) => {
    const msg = messages[filteredIndex];
    if (!msg) return;
    const allIndex = messageToAllIndex.get(msg);
    if (allIndex === undefined) return;
    const virtualIndex = messageIndexToVirtualIndex.get(allIndex);
    if (virtualIndex === undefined) return;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(virtualIndex, { align: 'center' }),
      () => false,
    );
    setSelectedIndex(allIndex);
  }, [messages, messageToAllIndex, messageIndexToVirtualIndex, virtualizer]);

  // Selected message drives the position indicator
  // Falls back to first visible message when nothing is explicitly selected
  const effectiveSelectedIndex = selectedIndex ?? firstVisibleIndex;

  const scrollToTop = useCallback(() => {
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(0, { align: 'start' }),
      () => {
        const items = virtualizer.getVirtualItems();
        const first = items[0];
        return !!first && first.index === 0;
      },
    );
  }, [virtualizer]);

  const scrollToBottom = useCallback(() => {
    const lastIndex = virtualItems.length - 1;
    retryOnAnimationFrame(
      () => virtualizer.scrollToIndex(lastIndex, { align: 'end' }),
      () => {
        const items = virtualizer.getVirtualItems();
        const last = items[items.length - 1];
        return !!last && last.index >= lastIndex;
      },
    );
  }, [virtualizer, virtualItems.length]);

  if (messages.length === 0) {
    return (
      <div className={styles.emptyState}>
        <p>No messages to display</p>
        <p className={styles.emptyHint}>Try adjusting your filters</p>
      </div>
    );
  }

  return (
    <div className={styles.timelineContainer}>
      <div ref={parentRef} className={styles.timeline}>
        <ScrollNavButtons
          scrollRef={parentRef}
          onScrollToTop={scrollToTop}
          onScrollToBottom={scrollToBottom}
          contentDependency={messages.length}
          onSearchClick={search.open}
          rightOffset={isCostMode ? SCROLL_NAV_COST_MODE_RIGHT : undefined}
        />

        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            width: '100%',
            position: 'relative',
          }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => {
            const item = virtualItems[virtualItem.index];
            if (!item) return null;

            const isMessage = item.type === 'message';
            const isSelected = isMessage && item.index === selectedIndex;

            return (
              <div
                key={virtualItem.index}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${virtualItem.start}px)`,
                }}
                ref={virtualizer.measureElement}
                data-index={virtualItem.index}
                onMouseEnter={isMessage ? () => handleMessageHover(item.index) : undefined}
              >
                {item.type === 'separator' ? (
                  <div className={styles.timeSeparator}>
                    <span className={styles.separatorLine} />
                    <span className={styles.separatorText}>{formatTimeSeparator(item.timestamp)}</span>
                    <span className={styles.separatorLine} />
                  </div>
                ) : (
                  <TimelineMessage
                    message={item.message}
                    toolNameMap={toolNameMap}
                    previousMessage={item.filteredIndex > 0 ? messages[item.filteredIndex - 1] : undefined}
                    isSelected={isSelected}
                    isDeepLinkTarget={targetMessageAllIndex !== null && item.index === targetMessageAllIndex}
                    isCurrentSearchMatch={search.currentMatchFilteredIndex === item.filteredIndex}
                    searchQuery={search.isOpen ? search.highlightQuery : undefined}
                    sessionId={sessionId}
                    roleLabel={getRoleLabel(item.message)}
                    isCostMode={isCostMode}
                    messageCost={isCostMode ? messageCosts.get(item.index) : undefined}
                    onSkipToNext={nextOfSameRole.has(item.filteredIndex)
                      ? () => scrollToFilteredIndex(nextOfSameRole.get(item.filteredIndex)!)
                      : undefined}
                    onSkipToPrevious={prevOfSameRole.has(item.filteredIndex)
                      ? () => scrollToFilteredIndex(prevOfSameRole.get(item.filteredIndex)!)
                      : undefined}
                  />
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className={`${styles.costBarWrapper} ${isCostMode ? styles.costBarWrapperVisible : ''}`}>
        {isCostMode && (
          <CostBar
            messages={allMessages}
            messageCosts={messageCosts}
            totalCost={totalCost}
            selectedIndex={effectiveSelectedIndex}
            onSeek={scrollToMessage}
          />
        )}
      </div>

      <TimelineBar
        messages={allMessages}
        selectedIndex={effectiveSelectedIndex}
        visibleIndices={visibleIndices}
        onSeek={scrollToMessage}
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

export default MessageTimeline;
