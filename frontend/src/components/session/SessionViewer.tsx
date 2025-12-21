import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { fetchParsedTranscript, fetchNewTranscriptMessages } from '@/services/transcriptService';
import { useVisibility } from '@/hooks/useVisibility';
import { countCategories, type MessageCategory } from './messageCategories';
import SessionHeader from './SessionHeader';
import SessionStatsSidebar from './SessionStatsSidebar';
import MessageTimeline from './MessageTimeline';
import styles from './SessionViewer.module.css';

// Polling interval for new transcript messages (30 seconds)
const TRANSCRIPT_POLL_INTERVAL_MS = 30000;

interface SessionViewerProps {
  session: SessionDetail;
  shareToken?: string;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
}

function SessionViewer({ session, shareToken, onShare, onDelete, onSessionUpdate, isOwner = true, isShared = false }: SessionViewerProps) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>([]);

  // Track the current line count for incremental fetching
  const lineCountRef = useRef(0);

  // Track visibility for smart polling
  const isVisible = useVisibility();

  // Filter state - user, assistant, system visible by default
  const [visibleCategories, setVisibleCategories] = useState<Set<MessageCategory>>(
    new Set(['user', 'assistant', 'system'])
  );

  // Compute category counts
  const categoryCounts = useMemo(() => countCategories(messages), [messages]);

  // Filter messages based on visible categories
  const filteredMessages = useMemo(() => {
    return messages.filter((message) => visibleCategories.has(message.type));
  }, [messages, visibleCategories]);

  // Get transcript file name
  const transcriptFileName = useMemo(() => {
    const transcriptFile = session.files.find((f) => f.file_type === 'transcript');
    return transcriptFile?.file_name;
  }, [session.files]);

  // Load transcript initially
  useEffect(() => {
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id]);

  async function loadTranscript() {
    setLoading(true);
    setError(null);
    lineCountRef.current = 0;

    try {
      if (!transcriptFileName) {
        throw new Error('No transcript file found');
      }

      const parsed = await fetchParsedTranscript(session.id, transcriptFileName, shareToken);
      setMessages(parsed.messages);
      lineCountRef.current = parsed.messages.length;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load transcript');
      console.error('Failed to load transcript:', e);
    } finally {
      setLoading(false);
    }
  }

  // Poll for new messages when visible
  useEffect(() => {
    if (!isVisible || loading || !transcriptFileName) {
      return;
    }

    const pollForNewMessages = async () => {
      try {
        const { newMessages, newTotalLineCount } = await fetchNewTranscriptMessages(
          session.id,
          transcriptFileName,
          lineCountRef.current,
          shareToken
        );

        if (newMessages.length > 0) {
          setMessages((prev) => [...prev, ...newMessages]);
          lineCountRef.current = newTotalLineCount;
        }
      } catch (e) {
        // Don't show error for polling failures - just log
        console.warn('Failed to poll for new messages:', e);
      }
    };

    // Set up polling interval
    const intervalId = setInterval(pollForNewMessages, TRANSCRIPT_POLL_INTERVAL_MS);

    // Cleanup
    return () => {
      clearInterval(intervalId);
    };
  }, [isVisible, loading, session.id, transcriptFileName, shareToken]);

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

    // Compute duration from session timestamps (consistent with listing view)
    let durationMs: number | undefined;
    if (session.first_seen && session.last_sync_at) {
      const start = new Date(session.first_seen).getTime();
      const end = new Date(session.last_sync_at).getTime();
      if (end > start) {
        durationMs = end - start;
      }
    }

    // Get session date
    const sessionDate = session.first_seen ? new Date(session.first_seen) : undefined;

    return { model, durationMs, sessionDate };
  }, [messages, session.first_seen, session.last_sync_at]);

  return (
    <div className={styles.sessionViewer}>
      <SessionStatsSidebar messages={messages} loading={loading} />

      <div className={styles.mainContent}>
        <SessionHeader
          sessionId={session.id}
          title={session.custom_title ?? session.summary ?? session.first_user_message ?? undefined}
          hasCustomTitle={!!session.custom_title}
          autoTitle={session.summary ?? session.first_user_message ?? undefined}
          externalId={session.external_id}
          model={sessionMeta.model}
          durationMs={sessionMeta.durationMs}
          sessionDate={sessionMeta.sessionDate}
          gitInfo={session.git_info}
          onShare={onShare}
          onDelete={onDelete}
          onSessionUpdate={onSessionUpdate}
          isOwner={isOwner}
          isShared={isShared}
          categoryCounts={categoryCounts}
          visibleCategories={visibleCategories}
          onToggleCategory={toggleCategory}
        />

        <div className={styles.timelineContainer}>
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
