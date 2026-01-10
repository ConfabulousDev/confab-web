import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { fetchParsedTranscript, fetchNewTranscriptMessages } from '@/services/transcriptService';
import { useVisibility } from '@/hooks/useVisibility';
import { computeSessionMeta } from '@/utils/sessionMeta';
import {
  countHierarchicalCategories,
  messageMatchesFilter,
  DEFAULT_FILTER_STATE,
  type MessageCategory,
  type UserSubcategory,
  type AssistantSubcategory,
  type FilterState,
} from './messageCategories';
import SessionHeader from './SessionHeader';
import MessageTimeline from './MessageTimeline';
import SessionSummaryPanel from './SessionSummaryPanel';
import styles from './SessionViewer.module.css';

export type ViewTab = 'summary' | 'transcript';

// Polling interval for new transcript messages (15 seconds)
const TRANSCRIPT_POLL_INTERVAL_MS = 15000;

interface SessionViewerProps {
  session: SessionDetail;
  onShare?: () => void;
  onDelete?: () => void;
  onSessionUpdate?: (session: SessionDetail) => void;
  isOwner?: boolean;
  isShared?: boolean;
  /** Controlled active tab - if provided, component is controlled */
  activeTab?: ViewTab;
  /** Callback when tab changes - required if activeTab is provided */
  onTabChange?: (tab: ViewTab) => void;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: import('@/services/api').SessionAnalytics;
  /** For Storybook: pass GitHub links directly instead of fetching from API */
  initialGithubLinks?: import('@/services/api').GitHubLink[];
}

function SessionViewer({ session, onShare, onDelete, onSessionUpdate, isOwner = true, isShared = false, activeTab: controlledTab, onTabChange, initialMessages, initialAnalytics, initialGithubLinks }: SessionViewerProps) {
  // Support both controlled and uncontrolled modes
  const [uncontrolledTab, setUncontrolledTab] = useState<ViewTab>('summary');
  const activeTab = controlledTab ?? uncontrolledTab;
  const setActiveTab = onTabChange ?? setUncontrolledTab;
  const [loading, setLoading] = useState(!initialMessages);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>(initialMessages ?? []);

  // Track the current line count for incremental fetching
  const lineCountRef = useRef(0);

  // Track visibility for smart polling
  const isVisible = useVisibility();

  // Filter state - hierarchical visibility for subcategories
  const [filterState, setFilterState] = useState<FilterState>(DEFAULT_FILTER_STATE);

  // Compute hierarchical category counts
  const categoryCounts = useMemo(() => countHierarchicalCategories(messages), [messages]);

  // Filter messages based on filter state
  const filteredMessages = useMemo(() => {
    return messages.filter((message) => messageMatchesFilter(message, filterState));
  }, [messages, filterState]);

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
      // Use totalLines (not messages.length) to track line_offset accurately
      // This accounts for parse errors and ensures we don't re-fetch lines
      lineCountRef.current = parsed.totalLines;
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

  // Toggle a top-level category's visibility (toggles all subcategories for hierarchical cats)
  const toggleCategory = useCallback((category: MessageCategory) => {
    setFilterState((prev) => {
      const next = { ...prev };
      if (category === 'user') {
        // Toggle all user subcategories
        const allVisible = prev.user.prompt && prev.user['tool-result'] && prev.user.skill;
        next.user = { prompt: !allVisible, 'tool-result': !allVisible, skill: !allVisible };
      } else if (category === 'assistant') {
        // Toggle all assistant subcategories
        const allVisible = prev.assistant.text && prev.assistant['tool-use'] && prev.assistant.thinking;
        next.assistant = { text: !allVisible, 'tool-use': !allVisible, thinking: !allVisible };
      } else {
        // Flat category - simple toggle
        next[category] = !prev[category];
      }
      return next;
    });
  }, []);

  // Toggle a user subcategory's visibility
  const toggleUserSubcategory = useCallback((subcategory: UserSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      user: { ...prev.user, [subcategory]: !prev.user[subcategory] },
    }));
  }, []);

  // Toggle an assistant subcategory's visibility
  const toggleAssistantSubcategory = useCallback((subcategory: AssistantSubcategory) => {
    setFilterState((prev) => ({
      ...prev,
      assistant: { ...prev.assistant, [subcategory]: !prev.assistant[subcategory] },
    }));
  }, []);

  // Compute session metadata for header
  const sessionMeta = useMemo(() => {
    // Find first assistant message to get model
    const firstAssistant = messages.find((m) => m.type === 'assistant');
    const model = firstAssistant?.type === 'assistant' ? firstAssistant.message.model : undefined;

    // Compute duration and date from message timestamps (matches analytics calculation)
    const { durationMs, sessionDate } = computeSessionMeta(messages, {
      firstSeen: session.first_seen,
      lastSyncAt: session.last_sync_at,
    });

    return { model, durationMs, sessionDate };
  }, [messages, session.first_seen, session.last_sync_at]);

  return (
    <div className={styles.sessionViewer}>
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
          filterState={activeTab === 'transcript' ? filterState : undefined}
          onToggleCategory={activeTab === 'transcript' ? toggleCategory : undefined}
          onToggleUserSubcategory={activeTab === 'transcript' ? toggleUserSubcategory : undefined}
          onToggleAssistantSubcategory={activeTab === 'transcript' ? toggleAssistantSubcategory : undefined}
        />

        {/* Tabs */}
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${activeTab === 'summary' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('summary')}
          >
            Summary
          </button>
          <button
            className={`${styles.tab} ${activeTab === 'transcript' ? styles.tabActive : ''}`}
            onClick={() => setActiveTab('transcript')}
          >
            Transcript
          </button>
        </div>

        {/* Tab Content */}
        <div className={styles.tabContent}>
          {activeTab === 'summary' ? (
            <SessionSummaryPanel
              sessionId={session.id}
              isOwner={isOwner}
              initialAnalytics={initialAnalytics}
              initialGithubLinks={initialGithubLinks}
            />
          ) : (
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
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
