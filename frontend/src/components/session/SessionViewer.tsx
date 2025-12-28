import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { fetchParsedTranscript, fetchNewTranscriptMessages } from '@/services/transcriptService';
import { useVisibility } from '@/hooks/useVisibility';
import { countCategories, type MessageCategory } from './messageCategories';
import SessionHeader from './SessionHeader';
import SessionStatsSidebar from './SessionStatsSidebar';
import MessageTimeline from './MessageTimeline';
import SessionAnalyticsPanel from './SessionAnalyticsPanel';
import styles from './SessionViewer.module.css';

type ViewTab = 'transcript' | 'analytics';

// Polling interval for new transcript messages (15 seconds)
const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

interface SessionViewerProps {
  session: SessionDetail;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: import('@/services/api').SessionAnalytics;
}

function SessionViewer({ session, onShare, onDelete, onSessionUpdate, isOwner = true, isShared = false, initialMessages, initialAnalytics }: SessionViewerProps) {
  const [activeTab, setActiveTab] = useState<ViewTab>('transcript');
  const [loading, setLoading] = useState(!initialMessages);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>(initialMessages ?? []);

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

  // Get transcript file info
  const transcriptFile = useMemo(() => {
    return session.files.find((f) => f.file_type === 'transcript');
  }, [session.files]);

  const transcriptFileName = transcriptFile?.file_name;

  // Load transcript initially (skip if initialMessages provided for Storybook)
  useEffect(() => {
    if (initialMessages !== undefined) return;
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id, initialMessages]);

  async function loadTranscript() {
    setLoading(true);
    setError(null);
    lineCountRef.current = 0;

    try {
      if (!transcriptFileName) {
        throw new Error('No transcript file found');
      }

      // Skip cache on initial load to ensure fresh data when navigating to a session
      const parsed = await fetchParsedTranscript(session.id, transcriptFileName, true);
      setMessages(parsed.messages);
      lineCountRef.current = parsed.messages.length;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load transcript');
      console.error('Failed to load transcript:', e);
    } finally {
      setLoading(false);
    }
  }

  // Poll for new messages when visible (skip if initialMessages provided for Storybook)
  useEffect(() => {
    if (initialMessages !== undefined || !isVisible || loading || !transcriptFileName) {
      return;
    }

    const pollForNewMessages = async () => {
      try {
        const { newMessages, newTotalLineCount } = await fetchNewTranscriptMessages(
          session.id,
          transcriptFileName,
          lineCountRef.current
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
  }, [initialMessages, isVisible, loading, session.id, transcriptFileName]);

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
      <SessionStatsSidebar
        messages={messages}
        loading={loading}
        sessionId={session.id}
        isOwner={isOwner}
      />

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
          categoryCounts={activeTab === 'transcript' ? categoryCounts : undefined}
          visibleCategories={activeTab === 'transcript' ? visibleCategories : undefined}
          onToggleCategory={activeTab === 'transcript' ? toggleCategory : undefined}
        />

        {/* Tabs */}
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${activeTab === 'transcript' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('transcript')}
          >
            Transcript
          </button>
          <button
            className={`${styles.tab} ${activeTab === 'analytics' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('analytics')}
          >
            Analytics
          </button>
        </div>

        {/* Tab Content */}
        <div className={styles.tabContent}>
          {activeTab === 'transcript' ? (
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
          ) : (
            <SessionAnalyticsPanel sessionId={session.id} initialAnalytics={initialAnalytics} />
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
