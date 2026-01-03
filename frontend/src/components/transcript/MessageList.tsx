import { useMemo, useRef, forwardRef, useImperativeHandle, useEffect, useCallback } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import type { TranscriptLine, AgentNode, SessionDetail } from '@/types';
import { getAgentInsertionIndex } from '@/services/agentTreeBuilder';
import Message from './Message';
import AgentPanel from './AgentPanel';
import styles from './MessageList.module.css';

// Item types for virtual list
type VirtualItem =
  | { type: 'message'; message: TranscriptLine; index: number }
  | { type: 'separator'; timestamp: string }
  | { type: 'agent'; agent: AgentNode };

interface MessageListProps {
  messages: TranscriptLine[];
  agents: AgentNode[];
  session: SessionDetail;
  /** Callback when scroll position changes (0-1 progress) */
  onScrollProgress?: (progress: number) => void;
}

export interface MessageListHandle {
  scrollToEnd: () => void;
  scrollToMessage: (messageIndex: number) => void;
}

const MessageList = forwardRef<MessageListHandle, MessageListProps>(
  ({ messages, agents, session, onScrollProgress }, ref) => {
    const parentRef = useRef<HTMLDivElement>(null);

    // Build a map of where to insert agents
    const agentInsertionMap = useMemo(() => {
      const map = new Map<number, AgentNode[]>();
      agents.forEach((agent) => {
        const insertIndex = getAgentInsertionIndex(messages, agent.parentMessageId);
        const existing = map.get(insertIndex) || [];
        existing.push(agent);
        map.set(insertIndex, existing);
      });
      return map;
    }, [messages, agents]);

    // Flatten messages, separators, and agents into a single virtual list
    const virtualItems = useMemo<VirtualItem[]>(() => {
      const items: VirtualItem[] = [];

      messages.forEach((message, index) => {
        // Add time separator if needed
        const prevMessage = index > 0 ? messages[index - 1] : undefined;
        if (shouldShowTimeSeparator(message, prevMessage ?? null)) {
          if ('timestamp' in message) {
            items.push({ type: 'separator', timestamp: message.timestamp });
          }
        }

        // Add message
        items.push({ type: 'message', message, index });

        // Add agents after this message
        const agentsAtIndex = agentInsertionMap.get(index + 1);
        if (agentsAtIndex) {
          agentsAtIndex.forEach((agent) => {
            items.push({ type: 'agent', agent });
          });
        }
      });

      return items;
    }, [messages, agentInsertionMap]);

    // Setup virtual scrolling with TanStack Virtual
    // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is the best option for virtualization; the warning is a known limitation
    const virtualizer = useVirtualizer({
      count: virtualItems.length,
      getScrollElement: () => parentRef.current,
      estimateSize: (index) => {
        const item = virtualItems[index];
        if (!item) return 150;

        // Dynamic size estimation based on item type
        switch (item.type) {
          case 'separator':
            return 40; // Time separators are small
          case 'agent':
            return 200; // Agent panels are larger
          case 'message': {
            const msg = item.message;
            // Estimate based on message type and content
            if (msg.type === 'user') return 80;
            if (msg.type === 'assistant') {
              const contentLength = JSON.stringify(msg).length;
              if (contentLength > 1000) return 300;
              if (contentLength > 500) return 200;
              return 120;
            }
            // Default for other types (system, summary, etc.)
            return 100;
          }
          default:
            return 150;
        }
      },
      overscan: 5, // Number of items to render outside visible area
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

    // Expose scrollToEnd and scrollToMessage methods via ref
    useImperativeHandle(ref, () => ({
      scrollToEnd: () => {
        if (virtualItems.length > 0) {
          virtualizer.scrollToIndex(virtualItems.length - 1, { align: 'end' });
        }
      },
      scrollToMessage: (messageIndex: number) => {
        const virtualIndex = messageIndexToVirtualIndex.get(messageIndex);
        if (virtualIndex !== undefined) {
          virtualizer.scrollToIndex(virtualIndex, { align: 'start' });
        }
      },
    }));

    // Track scroll progress
    const handleScroll = useCallback(() => {
      if (!parentRef.current || !onScrollProgress) return;

      const { scrollTop, scrollHeight, clientHeight } = parentRef.current;
      const maxScroll = scrollHeight - clientHeight;
      if (maxScroll <= 0) {
        onScrollProgress(0);
        return;
      }

      const progress = Math.min(1, Math.max(0, scrollTop / maxScroll));
      onScrollProgress(progress);
    }, [onScrollProgress]);

    // Attach scroll listener
    useEffect(() => {
      const scrollElement = parentRef.current;
      if (!scrollElement || !onScrollProgress) return;

      scrollElement.addEventListener('scroll', handleScroll, { passive: true });
      // Initial progress update
      handleScroll();

      return () => {
        scrollElement.removeEventListener('scroll', handleScroll);
      };
    }, [handleScroll, onScrollProgress]);

    // Check if we should show a time separator
    function shouldShowTimeSeparator(current: TranscriptLine, previous: TranscriptLine | null): boolean {
      if (!previous) return false;

      const currentTime = 'timestamp' in current ? new Date(current.timestamp) : null;
      const previousTime = 'timestamp' in previous ? new Date(previous.timestamp) : null;

      if (!currentTime || !previousTime) return false;

      // Show separator if more than 5 minutes between messages
      const diff = currentTime.getTime() - previousTime.getTime();
      return diff > 5 * 60 * 1000; // 5 minutes in milliseconds
    }

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

    if (virtualItems.length === 0) {
      return (
        <div className={styles.emptyState}>
          <div className={styles.emptyIcon}>📋</div>
          <p>No messages in this session</p>
        </div>
      );
    }

    return (
      <div ref={parentRef} className={styles.messageListWrapper}>
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
                    <span className={styles.timeSeparatorLine}></span>
                    <span className={styles.timeSeparatorText}>{formatTimeSeparator(item.timestamp)}</span>
                    <span className={styles.timeSeparatorLine}></span>
                  </div>
                ) : item.type === 'message' ? (
                  <div className={styles.messageWrapper}>
                    <Message
                      message={item.message}
                      index={item.index}
                      previousMessage={item.index > 0 ? messages[item.index - 1] : undefined}
                    />
                  </div>
                ) : item.type === 'agent' ? (
                  <div className={styles.agentWrapper}>
                    <AgentPanel agent={item.agent} session={session} depth={0} />
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    );
  }
);

MessageList.displayName = 'MessageList';

export default MessageList;
