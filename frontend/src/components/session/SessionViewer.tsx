import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import type { SessionDetail, TranscriptLine } from '@/types';
import { isAssistantMessage } from '@/types';
import { fetchParsedTranscript, fetchNewTranscriptMessages } from '@/services/transcriptService';
import { tilsAPI, type TIL } from '@/services/api';
import { useVisibility } from '@/hooks/useVisibility';
import { useTranscriptFilters } from '@/hooks/useTranscriptFilters';
import { computeSessionMeta } from '@/utils/sessionMeta';
import {
  countHierarchicalCategories,
  messageMatchesFilter,
  DEFAULT_FILTER_STATE,
} from './messageCategories';
import SessionHeader from './SessionHeader';
import SessionSummaryPanel from './SessionSummaryPanel';
import ClaudeTranscriptPane from './ClaudeTranscriptPane';
import CodexTranscriptPane from './CodexTranscriptPane';
import CodexSummaryEmpty from './CodexSummaryEmpty';
import styles from './SessionViewer.module.css';

// Provider canonical names per CF-347. Legacy 'Claude Code' rows still exist
// in the database but the API normalizes them to 'claude-code' on read.
function isCodexProvider(provider: string): boolean {
  return provider === 'codex';
}

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
  /** UUID of a message to scroll to and highlight (deep-link target) */
  targetMessageUuid?: string;
  /** For Storybook: pass messages directly instead of fetching from API */
  initialMessages?: TranscriptLine[];
  /** For Storybook: pass analytics directly instead of fetching from API */
  initialAnalytics?: import('@/services/api').SessionAnalytics;
  /** For Storybook: pass GitHub links directly instead of fetching from API */
  initialGithubLinks?: import('@/services/api').GitHubLink[];
  /** For Storybook: pass raw Codex lines directly instead of fetching from API */
  initialCodexRawLines?: import('@/schemas/codexTranscript').RawCodexLine[];
}

function SessionViewer({ session, onShare, onDelete, onSessionUpdate, isOwner = true, isShared = false, activeTab: controlledTab, onTabChange, targetMessageUuid, initialMessages, initialAnalytics, initialGithubLinks, initialCodexRawLines }: SessionViewerProps) {
  // Support both controlled and uncontrolled modes
  const [uncontrolledTab, setUncontrolledTab] = useState<ViewTab>('summary');
  const activeTab = controlledTab ?? uncontrolledTab;
  const setActiveTab = onTabChange ?? setUncontrolledTab;
  const isCodex = isCodexProvider(session.provider);
  const [loading, setLoading] = useState(!initialMessages && !isCodex);
  const [error, setError] = useState<string | null>(null);
  const [messages, setMessages] = useState<TranscriptLine[]>(initialMessages ?? []);

  // Track the current line count for incremental fetching
  const lineCountRef = useRef(0);

  // Track visibility for smart polling
  const isVisible = useVisibility();

  // Cost mode toggle
  const [isCostMode, setIsCostMode] = useState(false);

  // TILs for this session (fetched once on mount).
  // Skip for Codex — TILs are message-anchored via UUIDs, which Codex
  // sessions don't have. The request would return an empty list anyway.
  const [sessionTILs, setSessionTILs] = useState<Map<string, TIL[]>>(new Map());
  useEffect(() => {
    setSessionTILs(new Map()); // Clear stale badges when navigating between sessions
    if (isCodex) return;
    tilsAPI.listForSession(session.id).then((response) => {
      const map = new Map<string, TIL[]>();
      for (const til of response.tils) {
        if (!til.message_uuid) continue;
        const existing = map.get(til.message_uuid) ?? [];
        existing.push(til);
        map.set(til.message_uuid, existing);
      }
      setSessionTILs(map);
    }).catch(() => {
      // Non-critical — TIL markers simply won't appear
    });
  }, [session.id, isCodex]);

  // Filter state - synced to URL via ?hide= param
  const {
    filterState, setFilterState,
    toggleCategory, toggleUserSubcategory, toggleAssistantSubcategory, toggleAttachmentSubcategory,
  } = useTranscriptFilters();

  // Compute hierarchical category counts
  const categoryCounts = useMemo(() => countHierarchicalCategories(messages), [messages]);

  // Filter messages based on filter state
  const filteredMessages = useMemo(() => {
    return messages.filter((message) => messageMatchesFilter(message, filterState));
  }, [messages, filterState]);

  // When a deep-link target exists but is hidden by the active filter, reset
  // filters so the target becomes visible. Runs once per target change since
  // the reset itself makes the target pass on the next check.
  useEffect(() => {
    if (!targetMessageUuid || messages.length === 0) return;

    const targetMessage = messages.find(
      (m) => 'uuid' in m && m.uuid === targetMessageUuid
    );
    if (!targetMessage) return;
    if (messageMatchesFilter(targetMessage, filterState)) return;

    setFilterState({
      ...DEFAULT_FILTER_STATE,
      system: targetMessage.type === 'system',
    }, { replace: true });
  }, [targetMessageUuid, messages, filterState, setFilterState]);

  // Get transcript file info
  const transcriptFile = useMemo(() => {
    return session.files.find((f) => f.file_type === 'transcript');
  }, [session.files]);

  const transcriptFileName = transcriptFile?.file_name;

  // Load transcript initially (skip if initialMessages provided for Storybook,
  // or if this is a Codex session — the dedicated CodexTranscriptPane handles
  // its own fetch with the codex-aware parser).
  useEffect(() => {
    if (initialMessages !== undefined) return;
    if (isCodex) return;
    loadTranscript();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session.id, initialMessages, isCodex]);

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

  // Poll for new messages when visible (skip if initialMessages provided for
  // Storybook, or for Codex — CodexTranscriptPane runs its own poll loop).
  useEffect(() => {
    if (initialMessages !== undefined || !isVisible || loading || !transcriptFileName || isCodex) {
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
  }, [initialMessages, isVisible, loading, session.id, transcriptFileName, isCodex]);

  const toggleCostMode = useCallback(() => setIsCostMode((prev) => !prev), []);

  // Track the last successfully applied suggested title to avoid duplicate updates
  const lastAppliedSuggestedTitleRef = useRef<string | null>(null);

  // Handle suggested title change from analytics
  const handleSuggestedTitleChange = useCallback((title: string) => {
    // Skip if we already applied this title
    if (title === lastAppliedSuggestedTitleRef.current) {
      return;
    }

    if (onSessionUpdate && session) {
      onSessionUpdate({
        ...session,
        suggested_session_title: title,
      });
      lastAppliedSuggestedTitleRef.current = title;
    }
  }, [session, onSessionUpdate]);

  // Compute session metadata for header
  const sessionMeta = useMemo(() => {
    // Find first assistant message to get model
    const firstAssistant = messages.find(isAssistantMessage);
    const model = firstAssistant?.message.model;

    // Compute duration and date from message timestamps (matches analytics calculation)
    const { durationMs, sessionDate } = computeSessionMeta(messages, {
      firstSeen: session.first_seen,
      lastSyncAt: session.last_sync_at,
    });

    return { model, durationMs, sessionDate };
  }, [messages, session.first_seen, session.last_sync_at]);

  // The transcript header controls (cost toggle, filter chips) only apply to
  // the Claude transcript view. Codex has no per-message filtering yet, and
  // the Summary tab doesn't show transcript chrome.
  const showTranscriptControls = activeTab === 'transcript' && !isCodex;

  return (
    <div className={styles.sessionViewer}>
      <div className={styles.mainContent}>
        <SessionHeader
          sessionId={session.id}
          title={session.custom_title ?? session.suggested_session_title ?? session.summary ?? session.first_user_message ?? undefined}
          hasCustomTitle={!!session.custom_title}
          autoTitle={session.suggested_session_title ?? session.summary ?? session.first_user_message ?? undefined}
          externalId={session.external_id}
          provider={session.provider}
          ownerEmail={session.owner_email}
          model={sessionMeta.model}
          durationMs={sessionMeta.durationMs}
          sessionDate={sessionMeta.sessionDate}
          gitInfo={session.git_info}
          onShare={onShare}
          onDelete={onDelete}
          onSessionUpdate={onSessionUpdate}
          isOwner={isOwner}
          isShared={isShared}
          sharedByEmail={session.shared_by_email}
          isCostMode={showTranscriptControls ? isCostMode : undefined}
          onToggleCostMode={showTranscriptControls ? toggleCostMode : undefined}
          categoryCounts={showTranscriptControls ? categoryCounts : undefined}
          filterState={showTranscriptControls ? filterState : undefined}
          onToggleCategory={showTranscriptControls ? toggleCategory : undefined}
          onToggleUserSubcategory={showTranscriptControls ? toggleUserSubcategory : undefined}
          onToggleAssistantSubcategory={showTranscriptControls ? toggleAssistantSubcategory : undefined}
          onToggleAttachmentSubcategory={showTranscriptControls ? toggleAttachmentSubcategory : undefined}
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
            isCodex ? (
              <CodexSummaryEmpty />
            ) : (
              <SessionSummaryPanel
                sessionId={session.id}
                isOwner={isOwner}
                initialAnalytics={initialAnalytics}
                initialGithubLinks={initialGithubLinks}
                onSuggestedTitleChange={handleSuggestedTitleChange}
              />
            )
          ) : (
            <div className={styles.timelineContainer}>
              {isCodex ? (
                <CodexTranscriptPane
                  sessionId={session.id}
                  transcriptFileName={transcriptFileName}
                  initialRawLines={initialCodexRawLines}
                />
              ) : (
                <ClaudeTranscriptPane
                  loading={loading}
                  error={error}
                  filteredMessages={filteredMessages}
                  allMessages={messages}
                  sessionId={session.id}
                  targetMessageUuid={targetMessageUuid}
                  isCostMode={isCostMode}
                  tilsByMessageUuid={sessionTILs}
                />
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default SessionViewer;
