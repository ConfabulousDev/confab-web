import { useState, useEffect, useMemo, useCallback } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { fetchParsedTranscript } from '@/services/transcriptService';
import { countCategories, type MessageCategory } from './messageCategories';
import SessionHeader from './SessionHeader';
import FilterSidebar from './FilterSidebar';
import MessageTimeline from './MessageTimeline';
import styles from './SessionViewer.module.css';

interface SessionViewerProps {
  session: SessionDetail;
  shareToken?: string;
  onShare?: () => void;
  onDelete?: () => void;
  isOwner?: boolean;
  isShared?: boolean;
}

function SessionViewer({ session, shareToken, onShare, onDelete, isOwner = true, isShared = false }: SessionViewerProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>([]);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  // Filter state - all categories visible by default
  const [visibleCategories, setVisibleCategories] = useState<Set<MessageCategory>>(
    new Set(['user', 'assistant', 'system', 'file-history-snapshot', 'summary', 'queue-operation'])
  );

  // Compute category counts
  const categoryCounts = useMemo(() => countCategories(messages), [messages]);

  // Filter messages based on visible categories
  const filteredMessages = useMemo(() => {
    return messages.filter((message) => visibleCategories.has(message.type));
  }, [messages, visibleCategories]);

  // Load transcript
  useEffect(() => {
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id]);

  async function loadTranscript() {
    setLoading(true);
    setError(null);

    try {
      const transcriptFile = session.files.find((f) => f.file_type === 'transcript');
      if (!transcriptFile) {
        throw new Error('No transcript file found');
      }

      const parsed = await fetchParsedTranscript(session.id, transcriptFile.file_name, shareToken);
      setMessages(parsed.messages);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load transcript');
      console.error('Failed to load transcript:', e);
    } finally {
      setLoading(false);
    }
  }

  // Toggle a category's visibility
  const toggleCategory = useCallback((category: MessageCategory) => {
    setVisibleCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) {
        next.delete(category);
      } else {
        next.add(category);
      }
      return next;
    });
  }, []);

  // Compute session metadata for header
  const sessionMeta = useMemo(() => {
    // Find first assistant message to get model
    const firstAssistant = messages.find((m) => m.type === 'assistant');
    const model = firstAssistant?.type === 'assistant' ? firstAssistant.message.model : undefined;

    // Compute duration
    let durationMs: number | undefined;
    if (messages.length > 0) {
      const firstMessage = messages[0];
      const lastMessage = messages[messages.length - 1];
      if (firstMessage && lastMessage) {
        const firstTimestamp = 'timestamp' in firstMessage ? new Date(firstMessage.timestamp) : null;
        const lastTimestamp = 'timestamp' in lastMessage ? new Date(lastMessage.timestamp) : null;
        if (firstTimestamp && lastTimestamp) {
          durationMs = lastTimestamp.getTime() - firstTimestamp.getTime();
        }
      }
    }

    // Get session date
    const sessionDate = session.first_seen ? new Date(session.first_seen) : undefined;

    return { model, durationMs, sessionDate };
  }, [messages, session.first_seen]);

  return (
    <div className={styles.sessionViewer}>
      <div className={`${styles.headerContainer} ${sidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
        <SessionHeader
          title={session.title ?? undefined}
          externalId={session.external_id}
          model={sessionMeta.model}
          durationMs={sessionMeta.durationMs}
          sessionDate={sessionMeta.sessionDate}
          gitInfo={session.git_info}
          onShare={onShare}
          onDelete={onDelete}
          isOwner={isOwner}
          isShared={isShared}
        />
      </div>

      <div className={styles.mainContent}>
        <FilterSidebar
          counts={categoryCounts}
          visibleCategories={visibleCategories}
          onToggleCategory={toggleCategory}
          collapsed={sidebarCollapsed}
          onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
        />

        <div className={`${styles.timelineContainer} ${sidebarCollapsed ? styles.sidebarCollapsed : ''}`}>
          {loading ? (
            <div className={styles.loading}>Loading transcript...</div>
          ) : error ? (
            <div className={styles.error}>
              <strong>Error:</strong> {error}
            </div>
          ) : (
            <MessageTimeline messages={filteredMessages} allMessages={messages} />
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
