import { useMemo, useRef, useCallback } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { TranscriptLine } from '@/types';
import TimelineMessage from './TimelineMessage';
import ScrollNavButtons from '@/components/ScrollNavButtons';
import styles from './MessageTimeline.module.css';

interface MessageTimelineProps {
  messages: TranscriptLine[];
  allMessages: TranscriptLine[]; // Used for building tool name map
}

// Item types for virtual list
type VirtualItem =
  | { type: 'message'; message: TranscriptLine; index: number }
  | { type: 'separator'; timestamp: string };

/**
 * Check if we should show a time separator between messages
 */
function shouldShowTimeSeparator(current: TranscriptLine, previous: TranscriptLine | undefined): boolean {
  if (!previous) return false;

  const currentTime = 'timestamp' in current ? new Date(current.timestamp) : null;
  const previousTime = 'timestamp' in previous ? new Date(previous.timestamp) : null;

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
    if (message.type === 'assistant') {
      for (const block of message.message.content) {
        if (block.type === 'tool_use') {
          map.set(block.id, block.name);
        }
      }
    }
  }

  return map;
}

function MessageTimeline({ messages, allMessages }: MessageTimelineProps) {
  const parentRef = useRef<HTMLDivElement>(null);

  // Build tool name map from all messages (not just filtered)
  const toolNameMap = useMemo(() => buildToolNameMap(allMessages), [allMessages]);

  // Build virtual items list with time separators
  const virtualItems = useMemo<VirtualItem[]>(() => {
    const items: VirtualItem[] = [];

    messages.forEach((message, index) => {
      const prevMessage = index > 0 ? messages[index - 1] : undefined;

      // Add time separator if needed
      if (shouldShowTimeSeparator(message, prevMessage)) {
        if ('timestamp' in message) {
          items.push({ type: 'separator', timestamp: message.timestamp });
        }
      }

      // Add message
      items.push({ type: 'message', message, index });
    });

    return items;
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

  const scrollToTop = useCallback(() => {
    const scrollWithRetry = (attempts = 0) => {
      virtualizer.scrollToIndex(0, { align: 'start' });

      if (attempts < 5) {
        requestAnimationFrame(() => {
          const items = virtualizer.getVirtualItems();
          const firstVisible = items[0];
          if (!firstVisible || firstVisible.index > 0) {
            scrollWithRetry(attempts + 1);
          }
        });
      }
    };

    scrollWithRetry();
  }, [virtualizer]);

  const scrollToBottom = useCallback(() => {
    const lastIndex = virtualItems.length - 1;

    // With dynamic sizes, scrollToIndex may not reach the true end on first try
    // because item sizes are estimated until measured. We retry until the last
    // item is actually visible.
    const scrollWithRetry = (attempts = 0) => {
      virtualizer.scrollToIndex(lastIndex, { align: 'end' });

      if (attempts < 5) {
        requestAnimationFrame(() => {
          const items = virtualizer.getVirtualItems();
          const lastVisible = items[items.length - 1];
          if (!lastVisible || lastVisible.index < lastIndex) {
            scrollWithRetry(attempts + 1);
          }
        });
      }
    };

    scrollWithRetry();
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
    <div ref={parentRef} className={styles.timeline}>
      <ScrollNavButtons
        scrollRef={parentRef}
        onScrollToTop={scrollToTop}
        onScrollToBottom={scrollToBottom}
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
                  previousMessage={item.index > 0 ? messages[item.index - 1] : undefined}
                />
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default MessageTimeline;
